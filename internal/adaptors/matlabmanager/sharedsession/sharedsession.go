// Copyright 2026 The MathWorks, Inc.

// Package sharedsession manages the beacon file the server writes after
// attaching to a MATLAB session that was shared via shareMATLABSession. The
// shared MATLAB session polls this file periodically to detect that an agent
// is connected and to update its desktop title and console output accordingly.
package sharedsession

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/matlab/matlab-mcp-server/internal/adaptors/matlabmanager/matlabsessionclient/embeddedconnector"
	"github.com/matlab/matlab-mcp-server/internal/entities"
)

const defaultHeartbeatInterval = 5 * time.Second

type beaconJSON struct {
	SessionID   string `json:"sessionId"`
	ClientName  string `json:"clientName"`
	ConnectedAt string `json:"connectedAt"`
	HeartbeatAt string `json:"heartbeatAt"`
	// HeartbeatAtUnix is the heartbeat time as Unix seconds with sub-second
	// precision, so the shared MATLAB session can detect a stale beacon
	// (e.g. after the server crashed) without parsing timestamps.
	HeartbeatAtUnix float64 `json:"heartbeatAtUnix"`
}

// Beacon is the agent-connected beacon file for one attached shared
// session. The shared MATLAB session polls the file and only considers the
// agent connected while the heartbeat stays fresh, so a crashed server
// eventually restores the MATLAB desktop title.
type Beacon struct {
	connectionDetails embeddedconnector.ConnectionDetails
	clientName        string

	heartbeatInterval time.Duration

	lock        sync.Mutex
	connectedAt time.Time

	stop chan struct{}
	done chan struct{}
}

func New(connectionDetails embeddedconnector.ConnectionDetails, clientName string) *Beacon {
	return &Beacon{
		connectionDetails: connectionDetails,
		clientName:        clientName,
		heartbeatInterval: defaultHeartbeatInterval,
		stop:              make(chan struct{}),
		done:              make(chan struct{}),
	}
}

// Start writes the beacon and keeps its heartbeat fresh until Stop is
// called. It is a no-op when the connection details were not discovered
// from a shared session file (e.g. sessions launched by the server).
func (b *Beacon) Start(logger entities.Logger) error {
	if b.connectionDetails.AgentConnectedPath == "" {
		close(b.done)
		return nil
	}

	b.lock.Lock()
	b.connectedAt = time.Now().UTC()
	b.lock.Unlock()

	if err := b.write(); err != nil {
		close(b.done)
		return err
	}

	go b.loop(logger)
	return nil
}

// Stop halts the heartbeat and removes the beacon file.
func (b *Beacon) Stop(ctx context.Context) error {
	select {
	case <-b.stop:
	default:
		close(b.stop)
	}

	select {
	case <-b.done:
	case <-ctx.Done():
	}

	if b.connectionDetails.AgentConnectedPath == "" {
		return nil
	}

	if err := os.Remove(b.connectionDetails.AgentConnectedPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (b *Beacon) loop(logger entities.Logger) {
	defer close(b.done)

	ticker := time.NewTicker(b.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stop:
			return
		case <-ticker.C:
			if err := b.write(); err != nil {
				logger.WithError(err).Debug("failed to refresh agent-connected beacon heartbeat")
			}
		}
	}
}

func (b *Beacon) write() error {
	b.lock.Lock()
	defer b.lock.Unlock()

	now := time.Now().UTC()
	data, err := json.Marshal(beaconJSON{
		SessionID:       b.connectionDetails.SessionID,
		ClientName:      b.clientName,
		ConnectedAt:     b.connectedAt.Format(time.RFC3339),
		HeartbeatAt:     now.Format(time.RFC3339),
		HeartbeatAtUnix: float64(now.UnixNano()) / 1e9,
	})
	if err != nil {
		return err
	}

	// Write via a temporary file and rename so readers (the shared MATLAB
	// session) never observe a partially written beacon.
	dir := filepath.Dir(b.connectionDetails.AgentConnectedPath)
	tmp, err := os.CreateTemp(dir, ".agentConnected-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		closeAndRemoveTemp(tmp, tmpName)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		closeAndRemoveTemp(tmp, tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	if err := os.Rename(tmpName, b.connectionDetails.AgentConnectedPath); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	return nil
}

func closeAndRemoveTemp(tmp *os.File, tmpName string) {
	_ = tmp.Close()
	_ = os.Remove(tmpName)
}
