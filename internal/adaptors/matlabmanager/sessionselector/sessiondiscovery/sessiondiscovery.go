// Copyright 2026 The MathWorks, Inc.

package sessiondiscovery

import (
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/matlab/matlab-mcp-server/internal/adaptors/matlabmanager/matlabsessionclient/embeddedconnector"
	"github.com/matlab/matlab-mcp-server/internal/entities"
)

const (
	sessionDetailsFileName = "sessionDetails.json"
	// sessionDetailsFilePattern matches the canonical sessionDetails.json
	// file written by older toolbox versions as well as the per-session
	// sessionDetails_<pid>.json files written when multiple MATLAB sessions
	// share themselves concurrently.
	sessionDetailsFilePattern = "sessionDetails*.json"
	sessionDetailsFilePrefix  = "sessionDetails"
	// agentConnectedFilePrefix is prepended to the remainder of the session
	// details file name (after the "sessionDetails" prefix) to derive the
	// beacon file path the server writes once it has attached to a shared
	// session. The shared MATLAB session polls this file to detect that an
	// agent is connected.
	agentConnectedFilePrefix = "agentConnected"
	agentConnectedFileName   = "agentConnected.json"
)

var ErrInvalidSessionDetails = errors.New("invalid session details")

type AppDataDirGetter interface {
	AppDataDir() (string, error)
}

type OSLayer interface {
	ReadFile(filePath string) ([]byte, error)
	Glob(pattern string) ([]string, error)
	ModTime(filePath string) (time.Time, error)
}

type sessionDetailsJSON struct {
	Port        json.Number `json:"port"`
	Certificate string      `json:"certificate"`
	APIKey      string      `json:"apiKey"`
	PID         json.Number `json:"pid"`
	BasePath    string      `json:"basePath"`
	SessionID   string      `json:"sessionId"`
}

type SessionDiscoverer struct {
	appDataDirGetter AppDataDirGetter
	osLayer          OSLayer
}

func New(appDataDirGetter AppDataDirGetter, osLayer OSLayer) *SessionDiscoverer {
	return &SessionDiscoverer{
		appDataDirGetter: appDataDirGetter,
		osLayer:          osLayer,
	}
}

func (d *SessionDiscoverer) FromSessionDetails(logger entities.Logger, sessionDetails []byte) (embeddedconnector.ConnectionDetails, error) {
	connectionDetails, _, err := d.connectionDetailsFromJSON(logger, sessionDetails)
	if err != nil {
		return embeddedconnector.ConnectionDetails{}, err
	}
	return connectionDetails, nil
}

func (d *SessionDiscoverer) connectionDetailsFromJSON(logger entities.Logger, sessionDetails []byte) (embeddedconnector.ConnectionDetails, sessionDetailsJSON, error) {
	var zeroValue embeddedconnector.ConnectionDetails
	var zeroDetails sessionDetailsJSON

	var details sessionDetailsJSON
	if err := json.Unmarshal(sessionDetails, &details); err != nil {
		return zeroValue, zeroDetails, err
	}

	port := details.Port.String()
	portAsInt, err := strconv.Atoi(port)
	if err != nil {
		logger.Debug("Failed to parse port as int")
		return zeroValue, zeroDetails, ErrInvalidSessionDetails
	}

	if portAsInt < 1 || portAsInt > 65535 {
		logger.Debug("Port is out of range")
		return zeroValue, zeroDetails, ErrInvalidSessionDetails
	}

	if details.APIKey == "" {
		logger.Debug("Invalid empty API Key")
		return zeroValue, zeroDetails, ErrInvalidSessionDetails
	}

	certificatePEM, err := d.osLayer.ReadFile(details.Certificate)
	if err != nil {
		return zeroValue, zeroDetails, err
	}

	if len(certificatePEM) == 0 {
		logger.Debug("Invalid empty certificate PEM")
		return zeroValue, zeroDetails, ErrInvalidSessionDetails
	}

	parsed, err := url.Parse(details.BasePath)
	if err != nil {
		logger.Debug("Failed to parse base path")
		return zeroValue, zeroDetails, ErrInvalidSessionDetails
	}
	if parsed.Scheme != "" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		logger.Debug("Base path must be a path only")
		return zeroValue, zeroDetails, ErrInvalidSessionDetails
	}
	if hasRelativeSegment(parsed.Path) {
		logger.Debug("Base path must not contain relative segments")
		return zeroValue, zeroDetails, ErrInvalidSessionDetails
	}

	return embeddedconnector.ConnectionDetails{
		Host:           "localhost",
		Port:           port,
		APIKey:         details.APIKey,
		CertificatePEM: certificatePEM,
		BasePath:       details.BasePath,
	}, details, nil
}

func hasRelativeSegment(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return true
		}
	}
	return false
}

func (d *SessionDiscoverer) DiscoverSessions(logger entities.Logger) []embeddedconnector.ConnectionDetails {
	appDataDir, err := d.appDataDirGetter.AppDataDir()
	if err != nil {
		logger.WithError(err).Debug("Failed to determine app data directory for session discovery")
		return nil
	}

	// Hardcoding v1 for now, if we end up having multiple version, we'll need version based handlers
	v1Dir := filepath.Join(appDataDir, "v1")
	sessionFilePaths, err := d.osLayer.Glob(filepath.Join(v1Dir, sessionDetailsFilePattern))
	if err != nil {
		logger.WithError(err).Debug("Failed to list shared MATLAB session files")
		return nil
	}
	if len(sessionFilePaths) == 0 {
		logger.Debug("No shared MATLAB session files found")
		return nil
	}
	sort.Strings(sessionFilePaths)

	var candidates []candidate
	seenByKey := map[string]int{}

	for _, sessionFilePath := range sessionFilePaths {
		data, err := d.osLayer.ReadFile(sessionFilePath)
		if err != nil {
			logger.WithError(err).With("session-details-path", sessionFilePath).Debug("Failed to read shared MATLAB session file")
			continue
		}

		connectionDetails, rawDetails, err := d.connectionDetailsFromJSON(logger, data)
		if err != nil {
			logger.WithError(err).With("session-details-path", sessionFilePath).Debug("Failed to get connection details from session details")
			continue
		}

		connectionDetails.SessionID = rawDetails.SessionID
		connectionDetails.SessionDetailsPath = sessionFilePath
		connectionDetails.AgentConnectedPath = agentConnectedFilePath(sessionFilePath)

		pid, pidErr := rawDetails.PID.Int64()
		if pidErr != nil {
			pid = math.MaxInt64
		}

		modTime, modTimeErr := d.osLayer.ModTime(sessionFilePath)
		if modTimeErr != nil {
			logger.WithError(modTimeErr).With("session-details-path", sessionFilePath).Debug("Failed to read modification time of shared MATLAB session file")
			modTime = time.Time{}
		}

		deduplicationKey := connectionDetails.Port + "|" + connectionDetails.APIKey
		if index, duplicate := seenByKey[deduplicationKey]; duplicate {
			// The same session may be described by both the canonical
			// sessionDetails.json (no session ID) and the per-process file.
			// Prefer the entry that carries a session ID.
			if candidates[index].details.SessionID == "" && rawDetails.SessionID != "" {
				candidates[index] = candidate{details: connectionDetails, pid: pid, modTime: modTime}
			} else {
				logger.With("session-details-path", sessionFilePath).Debug("Skipping duplicate shared MATLAB session file")
				continue
			}
		} else {
			seenByKey[deduplicationKey] = len(candidates)
			candidates = append(candidates, candidate{details: connectionDetails, pid: pid, modTime: modTime})
		}
	}

	// Most recently shared session first; ties broken by lowest process ID
	// and then file path for deterministic ordering.
	sort.SliceStable(candidates, func(i, j int) bool {
		if !candidates[i].modTime.Equal(candidates[j].modTime) {
			return candidates[i].modTime.After(candidates[j].modTime)
		}
		if candidates[i].pid != candidates[j].pid {
			return candidates[i].pid < candidates[j].pid
		}
		return candidates[i].details.SessionDetailsPath < candidates[j].details.SessionDetailsPath
	})

	discovered := make([]embeddedconnector.ConnectionDetails, 0, len(candidates))
	for _, candidate := range candidates {
		discovered = append(discovered, candidate.details)
	}

	return discovered
}

type candidate struct {
	details embeddedconnector.ConnectionDetails
	pid     int64
	modTime time.Time
}

func agentConnectedFilePath(sessionDetailsPath string) string {
	dir, base := filepath.Split(sessionDetailsPath)
	if base == sessionDetailsFileName {
		return filepath.Join(dir, agentConnectedFileName)
	}
	return filepath.Join(dir, agentConnectedFilePrefix+strings.TrimPrefix(base, sessionDetailsFilePrefix))
}
