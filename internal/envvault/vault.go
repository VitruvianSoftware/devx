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

// PushAll reads the env configuration and pushes the provided dotEnvContent
// to all configured remote vaults. It skips local file:// references.
func PushAll(uris []string, dotEnvContent []byte) error {
	for _, uri := range uris {
		if err := pushOne(uri, dotEnvContent); err != nil {
			return fmt.Errorf("failed pushing to %q: %w", uri, err)
		}
	}
	return nil
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

func pushOne(uri string, content []byte) error {
	if strings.HasPrefix(uri, "file://") || !strings.Contains(uri, "://") {
		// Do not push to local files, push is purely for populating remote vaults
		return nil
	}

	if strings.HasPrefix(uri, "1password://") || strings.HasPrefix(uri, "op://") {
		return push1Password(uri, content)
	}
	if strings.HasPrefix(uri, "bitwarden://") || strings.HasPrefix(uri, "bw://") {
		return pushBitwarden(uri, content)
	}
	if strings.HasPrefix(uri, "gcp://") {
		return pushGCP(uri, content)
	}

	return fmt.Errorf("unsupported vault provider scheme in URI: %s", uri)
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

func push1Password(uri string, content []byte) error {
	if _, err := exec.LookPath("op"); err != nil {
		return fmt.Errorf("1Password CLI 'op' not found in PATH")
	}

	ref := strings.TrimPrefix(uri, "1password://")
	ref = strings.TrimPrefix(ref, "op://")
	
	parts := strings.Split(ref, "/")
	if len(parts) < 3 {
		return fmt.Errorf("invalid 1Password URI for pushing, expected vault/item/field")
	}
	vault, item, field := parts[0], parts[1], parts[2]

	cmd := exec.Command("op", "item", "edit", item, "--vault", vault, fmt.Sprintf("%s[text]=%s", field, string(content)))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("op item edit failed: %w\nOutput: %s", err, string(out))
	}

	return nil
}

func pushBitwarden(uri string, content []byte) error {
	if _, err := exec.LookPath("bw"); err != nil {
		return fmt.Errorf("Bitwarden CLI 'bw' not found in PATH")
	}

	item := strings.TrimPrefix(uri, "bitwarden://")
	item = strings.TrimPrefix(item, "bw://")

	// 1. Get current item JSON just to verify existance
	getCmd := exec.Command("bw", "get", "item", item)
	_, err := getCmd.Output()
	if err != nil {
		// If it doesn't exist, we'd ideally create it. For now fail.
		return fmt.Errorf("bw get item failed: %w\n(Note: Bitwarden item must already exist to push to it)", err)
	}

	// 2. We don't even need to heavily parse it, we can just replace .notes securely using jq?
	// But it's safer to avoid jq dependency. We can do string search/replace or basic JSON unmarshal.
	// Actually, doing this natively in Go is very robust
	
	// Poor man's JSON edit to preserve exact structure without full unmarshal complexity for generic types:
	// But Bitwarden requires base64 encoding (bw encode).
	// Let's use an inline python/ruby or just simple map unmarshalling.
	// We'll rely on executing a shell command that sets the notes field and pipes to bw encode -> edit.
	// "jq '.notes = $val'" is easiest, but requires jq. We'll use a small python script.
	pythonScript := fmt.Sprintf(`
import json, sys
data = json.load(sys.stdin)
data["notes"] = %q
print(json.dumps(data))
`, string(content))

	editFlow := fmt.Sprintf("bw get item %q | python3 -c '%s' | bw encode | bw edit item %q", item, pythonScript, item)
	flowCmd := exec.Command("sh", "-c", editFlow)
	if flowOut, flowErr := flowCmd.CombinedOutput(); flowErr != nil {
		return fmt.Errorf("bw edit pipeline failed: %w\nOutput: %s", flowErr, string(flowOut))
	}

	return nil
}

func pushGCP(uri string, content []byte) error {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return fmt.Errorf("gcloud CLI not found in PATH")
	}

	secretPath := strings.TrimPrefix(uri, "gcp://")

	cmd := exec.Command("gcloud", "secrets", "versions", "add", secretPath, "--data-file=-", "--project="+getProjectFromPath(secretPath))
	cmd.Stdin = strings.NewReader(string(content))
	
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gcloud secrets versions add failed: %w\nOutput: %s", err, string(out))
	}

	return nil
}
