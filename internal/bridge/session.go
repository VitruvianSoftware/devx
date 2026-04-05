package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SessionEntry represents a single active port-forward in the session file.
type SessionEntry struct {
	Service    string    `json:"service"`
	Namespace  string    `json:"namespace"`
	RemotePort int       `json:"remote_port"`
	LocalPort  int       `json:"local_port"`
	State      string    `json:"state"`
	PID        int       `json:"pid,omitempty"`
	StartedAt  time.Time `json:"started_at"`
}

// InterceptEntry represents an active traffic intercept (Idea 46.2).
type InterceptEntry struct {
	Service          string            `json:"service"`
	Namespace        string            `json:"namespace"`
	TargetPort       int               `json:"target_port"`
	LocalPort        int               `json:"local_port"`
	Mode             string            `json:"mode"`              // "steal" or "mirror"
	AgentPod         string            `json:"agent_pod"`
	SessionID        string            `json:"session_id"`
	OriginalSelector map[string]string `json:"original_selector"` // for restore on teardown
	StartedAt        time.Time         `json:"started_at"`
}

// Session represents the full bridge session state persisted to ~/.devx/bridge.json.
type Session struct {
	Kubeconfig string           `json:"kubeconfig"`
	Context    string           `json:"context"`
	Entries    []SessionEntry   `json:"entries"`
	Intercepts []InterceptEntry `json:"intercepts,omitempty"` // Idea 46.2
	StartedAt  time.Time        `json:"started_at"`
}

// SessionPath returns the path to the bridge session file (~/.devx/bridge.json).
func SessionPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".devx", "bridge.json"), nil
}

// EnvPath returns the path to the bridge env file (~/.devx/bridge.env).
func EnvPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".devx", "bridge.env"), nil
}

// SaveSession persists the session state to disk.
func SaveSession(session *Session) error {
	path, err := SessionPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling session: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// LoadSession reads the session state from disk.
// Returns nil, nil if the file does not exist.
func LoadSession() (*Session, error) {
	path, err := SessionPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &session, nil
}

// ClearSession removes the session file and env file from disk.
func ClearSession() error {
	for _, pathFn := range []func() (string, error){SessionPath, EnvPath} {
		path, err := pathFn()
		if err != nil {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", path, err)
		}
	}
	return nil
}

// IsActive checks whether a bridge session exists and has entries or intercepts.
func IsActive() bool {
	session, err := LoadSession()
	if err != nil || session == nil {
		return false
	}
	return len(session.Entries) > 0 || len(session.Intercepts) > 0
}
