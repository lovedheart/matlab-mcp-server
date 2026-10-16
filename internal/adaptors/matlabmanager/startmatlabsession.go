// Copyright 2025-2026 The MathWorks, Inc.

package matlabmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/matlab/matlab-mcp-server/internal/adaptors/matlabmanager/asyncrunner"
	"github.com/matlab/matlab-mcp-server/internal/adaptors/matlabmanager/matlabservices/datatypes"
	"github.com/matlab/matlab-mcp-server/internal/adaptors/matlabmanager/matlabsessionclient/embeddedconnector"
	"github.com/matlab/matlab-mcp-server/internal/adaptors/matlabmanager/matlabsessionstore"
	"github.com/matlab/matlab-mcp-server/internal/adaptors/matlabmanager/sharedsession"
	"github.com/matlab/matlab-mcp-server/internal/entities"
)

var ErrMATLABSessionNotAlive = errors.New("session is not alive")

const greetingTimeout = 3 * time.Second

const titleTimeout = 20 * time.Second

func (m *MATLABManager) StartMATLABSession(ctx context.Context, sessionLogger entities.Logger, startRequest entities.SessionDetails) (entities.SessionID, error) {
	var zeroValue entities.SessionID
	var client matlabsessionstore.MATLABSessionClientWithCleanup

	switch request := startRequest.(type) {
	case entities.LocalSessionDetails:
		localSessionLogger := sessionLogger.With("matlab-root", request.MATLABRoot)
		// For now, we return embedded connector details, to decouple the session start logic from the client creation.
		embeddedConnectorEndpoint, sessionCleanup, err := m.matlabServices.StartLocalMATLABSession(
			ctx,
			localSessionLogger,
			datatypes.LocalSessionDetails{
				MATLABRoot:             request.MATLABRoot,
				IsStartingDirectorySet: request.IsStartingDirectorySet,
				StartingDirectory:      request.StartingDirectory,
				ShowMATLABDesktop:      request.ShowMATLABDesktop,
			},
		)
		if err != nil {
			return zeroValue, err
		}
		embeddedConnectorClient, err := m.clientFactory.New(embeddedConnectorEndpoint)
		if err != nil {
			if cleanupErr := sessionCleanup(); cleanupErr != nil {
				sessionLogger.WithError(cleanupErr).Error("Failed to clean up session after client factory error")
			}
			return zeroValue, err
		}
		correlationID := retrieveCorrelationID(ctx, sessionLogger, embeddedConnectorClient)
		gateClient, _, _ := m.indicateConnection(sessionLogger, embeddedConnectorClient, request.ShowMATLABDesktop)
		client = newCleanupSessionClient(gateClient, func(ctx context.Context, sessionLogger entities.Logger) error {
			if _, err := embeddedConnectorClient.Eval(ctx, sessionLogger, entities.EvalRequest{Code: "exit()"}); err != nil {
				return err
			}
			return sessionCleanup()
		})
		sessionID := m.sessionStore.Add(client, correlationID)
		return sessionID, nil

	case entities.AttachToExistingSession:
		sessionLogger.Info("Attaching to existing session")

		candidates, err := m.sessionSelector.SelectSessionsToAttachTo(sessionLogger)
		if err != nil {
			return zeroValue, err
		}

		var connectionDetails embeddedconnector.ConnectionDetails
		var embeddedConnectorClient entities.MATLABSessionClient
		var lastClientFactoryErr error

		for _, candidate := range candidates {
			client, err := m.clientFactory.New(candidate)
			if err != nil {
				lastClientFactoryErr = err
				sessionLogger.WithError(err).Debug("Failed to create client for candidate MATLAB session")
				continue
			}

			if !client.Ping(ctx, sessionLogger).IsAlive {
				sessionLogger.Debug("Candidate MATLAB session is not alive, trying next candidate")
				continue
			}

			connectionDetails = candidate
			embeddedConnectorClient = client
			break
		}

		if embeddedConnectorClient == nil {
			if lastClientFactoryErr != nil {
				return zeroValue, lastClientFactoryErr
			}
			return zeroValue, ErrMATLABSessionNotAlive
		}

		correlationID := retrieveCorrelationID(ctx, sessionLogger, embeddedConnectorClient)

		if connectionDetails.SessionID != "" {
			// Newer toolbox versions: the shared MATLAB session polls the
			// beacon and updates its own desktop title and console output, so
			// the server does not touch the title or print the greeting here.
			beacon := sharedsession.New(connectionDetails, attachedSessionClientName(m.clientInfoProvider))
			if err := beacon.Start(sessionLogger); err != nil {
				sessionLogger.WithError(err).Warn("failed to write agent-connected beacon for shared MATLAB session")
			}
			client = newCleanupSessionClient(embeddedConnectorClient, func(ctx context.Context, sessionLogger entities.Logger) error {
				if err := beacon.Stop(ctx); err != nil {
					sessionLogger.WithError(err).Warn("failed to remove agent-connected beacon on detach")
				}
				sessionLogger.Debug("Skipping session stop for externally managed MATLAB session")
				return nil
			})
			sessionID := m.sessionStore.Add(client, correlationID)
			return sessionID, nil
		}

		const showMATLABDesktop = true
		gateClient, titleTask, originalTitle := m.indicateConnection(sessionLogger, embeddedConnectorClient, showMATLABDesktop)
		client = newCleanupSessionClient(gateClient, func(ctx context.Context, sessionLogger entities.Logger) error {
			if titleTask.Wait(ctx) && *originalTitle != "" {
				if err := m.connectionIndicator.RestoreTitle(ctx, sessionLogger, embeddedConnectorClient, *originalTitle); err != nil {
					sessionLogger.WithError(err).Warn("failed to restore MATLAB desktop title on detach")
				}
			}
			sessionLogger.Debug("Skipping session stop for externally managed MATLAB session")
			return nil
		})
		sessionID := m.sessionStore.Add(client, correlationID)
		return sessionID, nil

	default:
		return zeroValue, fmt.Errorf("unknown request type: %T", request)
	}
}

func (m *MATLABManager) indicateConnection(sessionLogger entities.Logger, client entities.MATLABSessionClient, showMATLABDesktop bool) (*greetingSessionClient, *asyncrunner.Task, *string) {
	greetingInfo := m.greetingInfo(showMATLABDesktop)

	greetingTask := asyncrunner.Run(context.Background(), sessionLogger, greetingTimeout,
		"failed to display MCP connection greeting in MATLAB command window",
		func(ctx context.Context) error {
			return m.connectionIndicator.ShowGreeting(ctx, sessionLogger, client, greetingInfo)
		})

	var originalTitle string
	titleTask := asyncrunner.Run(context.Background(), sessionLogger, titleTimeout,
		"failed to set MATLAB desktop title on connect",
		func(ctx context.Context) error {
			title, err := m.connectionIndicator.ApplyConnectedTitle(ctx, sessionLogger, client, greetingInfo)
			if err != nil {
				return err
			}
			originalTitle = title
			return nil
		})

	return newGreetingSessionClient(client, greetingTask), titleTask, &originalTitle
}

func (m *MATLABManager) greetingInfo(showMATLABDesktop bool) entities.GreetingInfo {
	return entities.GreetingInfo{
		ClientInfo:        m.clientInfoProvider.GetClientInfo(),
		ShowMATLABDesktop: showMATLABDesktop,
	}
}

// attachedSessionClientName returns the human-readable name of the connected
// MCP client, preferring the display title over the machine name.
func attachedSessionClientName(provider MCPClientInfoProvider) string {
	clientInfo := provider.GetClientInfo()
	if clientInfo.Title != "" {
		return clientInfo.Title
	}
	return clientInfo.Name
}
