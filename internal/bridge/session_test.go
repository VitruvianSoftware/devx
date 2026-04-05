package bridge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionSaveLoadClear(t *testing.T) {
	// Use a temp dir to avoid touching the real ~/.devx
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { os.Setenv("HOME", origHome) }()

	// Ensure .devx dir exists
	_ = os.MkdirAll(filepath.Join(tmpDir, ".devx"), 0o755)

	session := &Session{
		Kubeconfig: "/tmp/kubeconfig",
		Context:    "staging",
		StartedAt:  time.Now(),
		Entries: []SessionEntry{
			{
				Service:    "payments-api",
				Namespace:  "default",
				RemotePort: 8080,
				LocalPort:  9501,
				State:      "healthy",
				StartedAt:  time.Now(),
			},
			{
				Service:    "redis",
				Namespace:  "cache",
				RemotePort: 6379,
				LocalPort:  9502,
				State:      "healthy",
				StartedAt:  time.Now(),
			},
		},
	}

	// Save
	if err := SaveSession(session); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	// Load
	loaded, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadSession returned nil")
	}
	if loaded.Context != "staging" {
		t.Errorf("Context = %q, want %q", loaded.Context, "staging")
	}
	if len(loaded.Entries) != 2 {
		t.Errorf("len(Entries) = %d, want 2", len(loaded.Entries))
	}
	if loaded.Entries[0].Service != "payments-api" {
		t.Errorf("Entries[0].Service = %q, want %q", loaded.Entries[0].Service, "payments-api")
	}

	// IsActive
	if !IsActive() {
		t.Error("IsActive() = false after saving session")
	}

	// Clear
	if err := ClearSession(); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}

	// Verify cleared
	loaded2, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession after clear: %v", err)
	}
	if loaded2 != nil {
		t.Error("LoadSession should return nil after ClearSession")
	}
	if IsActive() {
		t.Error("IsActive() = true after ClearSession")
	}
}

func TestSessionLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	loaded, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession on missing file: %v", err)
	}
	if loaded != nil {
		t.Error("LoadSession should return nil for non-existent file")
	}
}

func TestEnvPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"payments-api", "BRIDGE_PAYMENTS_API"},
		{"redis", "BRIDGE_REDIS"},
		{"user.service", "BRIDGE_USER_SERVICE"},
		{"my-fancy-svc", "BRIDGE_MY_FANCY_SVC"},
	}

	for _, tt := range tests {
		got := envPrefix(tt.input)
		if got != tt.want {
			t.Errorf("envPrefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".devx"), 0o755)

	entries := []SessionEntry{
		{
			Service:    "payments-api",
			Namespace:  "default",
			RemotePort: 8080,
			LocalPort:  9501,
		},
		{
			Service:    "redis",
			Namespace:  "cache",
			RemotePort: 6379,
			LocalPort:  9502,
		},
	}

	if err := GenerateEnvFile(entries); err != nil {
		t.Fatalf("GenerateEnvFile: %v", err)
	}

	// Read the file and verify
	vars, err := LoadEnvVars()
	if err != nil {
		t.Fatalf("LoadEnvVars: %v", err)
	}

	expected := map[string]string{
		"BRIDGE_PAYMENTS_API_URL":  "http://127.0.0.1:9501",
		"BRIDGE_PAYMENTS_API_HOST": "127.0.0.1",
		"BRIDGE_PAYMENTS_API_PORT": "9501",
		"BRIDGE_REDIS_URL":         "http://127.0.0.1:9502",
		"BRIDGE_REDIS_HOST":        "127.0.0.1",
		"BRIDGE_REDIS_PORT":        "9502",
	}

	for k, want := range expected {
		got, ok := vars[k]
		if !ok {
			t.Errorf("missing env var %s", k)
			continue
		}
		if got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestLoadEnvVarsNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	vars, err := LoadEnvVars()
	if err != nil {
		t.Fatalf("LoadEnvVars on missing file: %v", err)
	}
	if vars != nil {
		t.Errorf("LoadEnvVars should return nil for non-existent file, got %v", vars)
	}
}
