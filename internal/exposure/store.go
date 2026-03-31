// Package exposure persists local metadata for active tunnel exposures
// that cannot be retrieved from the Cloudflare API (e.g. the target port).
package exposure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Entry records metadata about a single exposed tunnel.
type Entry struct {
	TunnelName string `json:"tunnel_name"`
	TunnelID   string `json:"tunnel_id"`
	Port       string `json:"port"`
	Domain     string `json:"domain"`
}

// stateDir returns ~/.config/devx, creating it if necessary.
func stateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "devx")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func statePath() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "exposures.json"), nil
}

// Save adds or updates an exposure entry.
func Save(entry Entry) error {
	entries, _ := LoadAll() // ignore read errors — start fresh

	// Upsert by tunnel name
	found := false
	for i, e := range entries {
		if e.TunnelName == entry.TunnelName {
			entries[i] = entry
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, entry)
	}

	return writeAll(entries)
}

// Remove deletes all stored exposure entries.
func RemoveAll() error {
	return writeAll(nil)
}

// Remove deletes a single entry by tunnel name.
func RemoveByName(tunnelName string) error {
	entries, _ := LoadAll()
	var filtered []Entry
	for _, e := range entries {
		if e.TunnelName != tunnelName {
			filtered = append(filtered, e)
		}
	}
	return writeAll(filtered)
}

// LoadAll reads all persisted exposure entries.
func LoadAll() ([]Entry, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return entries, nil
}

// LookupPort returns the port for a tunnel name, or "" if unknown.
func LookupPort(tunnelName string) string {
	entries, _ := LoadAll()
	for _, e := range entries {
		if e.TunnelName == tunnelName {
			return e.Port
		}
	}
	return ""
}

func writeAll(entries []Entry) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
