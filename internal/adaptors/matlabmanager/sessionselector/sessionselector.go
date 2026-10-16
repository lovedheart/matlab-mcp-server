// Copyright 2026 The MathWorks, Inc.

package sessionselector

import (
	"errors"

	"github.com/matlab/matlab-mcp-server/internal/adaptors/application/config"
	"github.com/matlab/matlab-mcp-server/internal/adaptors/matlabmanager/matlabsessionclient/embeddedconnector"
	"github.com/matlab/matlab-mcp-server/internal/entities"
	"github.com/matlab/matlab-mcp-server/internal/messages"
)

var ErrNoMATLABSessionDiscovered = errors.New("no MATLAB session discovered")

type ConfigFactory interface {
	Config() (config.Config, messages.Error)
}

type SessionDiscoverer interface {
	FromSessionDetails(logger entities.Logger, sessionDetails []byte) (embeddedconnector.ConnectionDetails, error)
	DiscoverSessions(logger entities.Logger) []embeddedconnector.ConnectionDetails
}

type SessionSelector struct {
	configFactory     ConfigFactory
	sessionDiscoverer SessionDiscoverer
}

func New(configFactory ConfigFactory, sessionDiscoverer SessionDiscoverer) *SessionSelector {
	return &SessionSelector{
		configFactory:     configFactory,
		sessionDiscoverer: sessionDiscoverer,
	}
}

func (s *SessionSelector) SelectSessionsToAttachTo(logger entities.Logger) ([]embeddedconnector.ConnectionDetails, error) {
	config, err := s.configFactory.Config()
	if err != nil {
		return nil, err
	}

	sessionDetails := config.MATLABSessionConnectionDetails()

	if sessionDetails != "" {
		logger.Debug("Attaching to specified existing session")

		connectionDetails, err := s.sessionDiscoverer.FromSessionDetails(logger, []byte(sessionDetails))
		if err != nil {
			return nil, err
		}

		return []embeddedconnector.ConnectionDetails{connectionDetails}, nil
	}

	logger.Debug("Discovering existing MATLAB sessions to attach to")

	discoveredSessions := s.sessionDiscoverer.DiscoverSessions(logger)
	if len(discoveredSessions) == 0 {
		return nil, ErrNoMATLABSessionDiscovered
	}

	if len(discoveredSessions) > 1 {
		logger.With("session-count", len(discoveredSessions)).
			Debug("Multiple shared MATLAB sessions discovered, attaching to the first one")
	}

	return discoveredSessions, nil
}
