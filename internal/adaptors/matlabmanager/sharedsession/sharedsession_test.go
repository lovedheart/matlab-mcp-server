// Copyright 2026 The MathWorks, Inc.

package sharedsession

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matlab/matlab-mcp-server/internal/adaptors/matlabmanager/matlabsessionclient/embeddedconnector"
	"github.com/matlab/matlab-mcp-server/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBeacon(t *testing.T, interval time.Duration) (*Beacon, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "agentConnected_123.json")
	details := embeddedconnector.ConnectionDetails{
		SessionID:          "session-guid",
		SessionDetailsPath: filepath.Join(filepath.Dir(path), "sessionDetails_123.json"),
		AgentConnectedPath: path,
	}
	b := New(details, "Test Client")
	b.heartbeatInterval = interval
	return b, path
}

func readBeaconFile(t *testing.T, path string) beaconJSON {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test controls the path
	require.NoError(t, err)
	var parsed beaconJSON
	require.NoError(t, json.Unmarshal(data, &parsed))
	return parsed
}

func TestBeaconStartStop(t *testing.T) {
	logger := testutils.NewInspectableLogger()
	b, path := newTestBeacon(t, time.Hour)

	require.NoError(t, b.Start(logger))

	parsed := readBeaconFile(t, path)
	assert.Equal(t, "session-guid", parsed.SessionID)
	assert.Equal(t, "Test Client", parsed.ClientName)
	assert.NotEmpty(t, parsed.ConnectedAt)
	assert.NotEmpty(t, parsed.HeartbeatAt)

	require.NoError(t, b.Stop(t.Context()))
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestBeaconHeartbeatRefreshes(t *testing.T) {
	logger := testutils.NewInspectableLogger()
	b, path := newTestBeacon(t, 10*time.Millisecond)

	require.NoError(t, b.Start(logger))
	first := readBeaconFile(t, path)

	time.Sleep(100 * time.Millisecond)
	second := readBeaconFile(t, path)

	assert.Equal(t, first.ConnectedAt, second.ConnectedAt, "connectedAt must stay fixed")
	assert.Greater(t, second.HeartbeatAtUnix, first.HeartbeatAtUnix, "heartbeatAtUnix must advance")

	require.NoError(t, b.Stop(t.Context()))
}

func TestBeaconNoopWithoutSharedSession(t *testing.T) {
	logger := testutils.NewInspectableLogger()
	b := New(embeddedconnector.ConnectionDetails{}, "client")

	require.NoError(t, b.Start(logger))
	require.NoError(t, b.Stop(t.Context()))
}
