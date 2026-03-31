package envvault

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/joho/godotenv"
)

// PullAll iterates through a list of secret provider URIs, fetches the secrets from each,
// and merges them into a single map. Later URIs override earlier ones.
func PullAll(uris []string) (map[string]string, error) {
	result := make(map[string]string)

	for _, uri := range uris {
		vars, err := pullOne(uri)
		if err != nil {
			return nil, fmt.Errorf("failed fetching %q: %w", uri, err)
		}
		for k, v := range vars {
			result[k] = v
		}
	}

	return result, nil
}

func pullOne(uri string) (map[string]string, error) {
	if strings.HasPrefix(uri, "1password://") || strings.HasPrefix(uri, "op://") {
		return pull1Password(uri)
	}
	if strings.HasPrefix(uri, "bitwarden://") || strings.HasPrefix(uri, "bw://") {
		return pullBitwarden(uri)
	}
	if strings.HasPrefix(uri, "gcp://") {
		return pullGCP(uri)
	}
	if strings.HasPrefix(uri, "file://") {
		path := strings.TrimPrefix(uri, "file://")
		return godotenv.Read(path)
	}
	
	// Default to trying to read it as a local dotenv file if no protocol
	if !strings.Contains(uri, "://") {
		return godotenv.Read(uri)
	}

	return nil, fmt.Errorf("unsupported vault provider scheme in URI: %s", uri)
}

func pull1Password(uri string) (map[string]string, error) {
	if _, err := exec.LookPath("op"); err != nil {
		return nil, fmt.Errorf("1Password CLI 'op' not found in PATH")
	}

	ref := strings.TrimPrefix(uri, "1password://")
	if !strings.HasPrefix(ref, "op://") {
		ref = "op://" + ref
	}

	cmd := exec.Command("op", "read", ref)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("op read failed: %w\nOutput: %s", err, string(out))
	}

	return godotenv.Unmarshal(string(out))
}

func pullBitwarden(uri string) (map[string]string, error) {
	if _, err := exec.LookPath("bw"); err != nil {
		return nil, fmt.Errorf("Bitwarden CLI 'bw' not found in PATH. Make sure you are logged in and session is unlocked.")
	}

	item := strings.TrimPrefix(uri, "bitwarden://")
	item = strings.TrimPrefix(item, "bw://")

	cmd := exec.Command("bw", "get", "notes", item)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("bw get notes failed: %w\nOutput: %s", err, string(out))
	}

	return godotenv.Unmarshal(string(out))
}

func pullGCP(uri string) (map[string]string, error) {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return nil, fmt.Errorf("gcloud CLI not found in PATH")
	}

	// URI: gcp://projects/123/secrets/my-secret/versions/latest
	secretPath := strings.TrimPrefix(uri, "gcp://")

	cmd := exec.Command("gcloud", "secrets", "versions", "access", "latest", "--secret="+secretPath, "--project="+getProjectFromPath(secretPath))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gcloud secrets access failed: %w\nOutput: %s", err, string(out))
	}

	return godotenv.Unmarshal(string(out))
}

func getProjectFromPath(secretPath string) string {
	// Simple heuristic if they put the raw name vs projects/.../secrets/...
	parts := strings.Split(secretPath, "/")
	for i, p := range parts {
		if p == "projects" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return os.Getenv("CLOUDSDK_CORE_PROJECT")
}
