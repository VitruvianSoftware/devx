package envvault

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
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
	if err := checkBitwardenUnlocked(); err != nil {
		return nil, err
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
	if err := checkBitwardenUnlocked(); err != nil {
		return err
	}

	item := strings.TrimPrefix(uri, "bitwarden://")
	item = strings.TrimPrefix(item, "bw://")

	// 1. Get current item JSON just to verify existance and get the exact ID
	getCmd := exec.Command("bw", "get", "item", item)
	itemRaw, err := getCmd.Output()
	
	var itemJSON map[string]interface{}
	isNew := false
	
	if err != nil {
		fmt.Printf("Bitwarden item %q not found. Creating it as a new Secure Note...\n", item)
		isNew = true
		
		tmplCmd := exec.Command("bw", "get", "template", "item")
		itemRaw, err = tmplCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to fetch Bitwarden item template: %w", err)
		}
	}

	// 2. Unmarshal into generic map, edit notes natively handling quotes, marshal back
	if err := json.Unmarshal(itemRaw, &itemJSON); err != nil {
		return fmt.Errorf("failed to unmarshal bw item JSON: %w (output: %q)", err, string(itemRaw))
	}

	if isNew {
		itemJSON["type"] = 2 // Secure Note
		itemJSON["name"] = item
		itemJSON["secureNote"] = map[string]interface{}{"type": 0}

	}
	itemJSON["notes"] = string(content)

	editedRaw, err := json.Marshal(itemJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal edited bw item JSON: %w", err)
	}

	// 3. Encode via bw encode
	encodeCmd := exec.Command("bw", "encode")
	encodeCmd.Stdin = bytes.NewReader(editedRaw)
	encodedRaw, err := encodeCmd.Output()
	if err != nil {
		return fmt.Errorf("bw encode failed: %w\nOutput: %s", err, string(encodedRaw))
	}
	
	if isNew {
		createCmd := exec.Command("bw", "create", "item")
		createCmd.Stdin = bytes.NewReader(encodedRaw)
		if createOut, err := createCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("bw create item failed: %w\nOutput: %s", err, string(createOut))
		}
		return nil
	}

	// 4. Push edit via bw edit item using the exact ID rather than name
	itemID := item
	if idVal, ok := itemJSON["id"].(string); ok && idVal != "" {
		itemID = idVal
	}

	editCmd := exec.Command("bw", "edit", "item", itemID)
	// bw edit expects the encoded JSON payload on stdin
	editCmd.Stdin = bytes.NewReader(encodedRaw)
	
	if editOut, err := editCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bw edit item failed: %w\nOutput: %s", err, string(editOut))
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

func checkBitwardenUnlocked() error {
	if _, err := exec.LookPath("bw"); err != nil {
		return fmt.Errorf("Bitwarden CLI 'bw' not found in PATH")
	}
	
	cmd := exec.Command("bw", "status")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("bw status failed: %w", err)
	}
	
	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return fmt.Errorf("failed to parse bw status JSON: %w (output: %s)", err, string(out))
	}
	
	if status.Status == "locked" {
		fmt.Println("🔒 Bitwarden vault is locked. Prompting for unlock...")
		
		var password string
		err := huh.NewInput().
			Title("Bitwarden Master Password").
			EchoMode(huh.EchoModePassword).
			Value(&password).
			Run()
			
		if err != nil {
			return fmt.Errorf("unlock cancelled")
		}
		
		unlockCmd := exec.Command("bw", "unlock", password, "--raw")
		sessionRaw, unlockErr := unlockCmd.Output()
		if unlockErr != nil {
			return fmt.Errorf("failed to unlock Bitwarden. Check your master password")
		}
		
		sessionKey := strings.TrimSpace(string(sessionRaw))
		if sessionKey != "" {
			if err := os.Setenv("BW_SESSION", sessionKey); err != nil {
				return fmt.Errorf("failed to inject BW_SESSION into environment: %w", err)
			}
			fmt.Println("🔓 Vault unlocked! Continuing operations...")
		} else {
			return fmt.Errorf("bw unlock returned an empty session key")
		}
	} else if status.Status == "unauthenticated" {
		fmt.Println("🚫 Bitwarden vault is unauthenticated.")
		
		var loginMethod string
		err := huh.NewSelect[string]().
			Title("How would you like to authenticate to Bitwarden?").
			Options(
				huh.NewOption("Interactive (Email, Password, 2FA)", "interactive"),
				huh.NewOption("API Key (Client ID & Secret)", "apikey"),
				huh.NewOption("SSO (Single Sign-On)", "sso"),
			).
			Value(&loginMethod).
			Run()

		if err != nil {
			return fmt.Errorf("login cancelled")
		}

		args := []string{"login"}
		if loginMethod == "apikey" {
			args = append(args, "--apikey")
		} else if loginMethod == "sso" {
			args = append(args, "--sso")
		}

		loginCmd := exec.Command("bw", args...)
		loginCmd.Stdin = os.Stdin
		loginCmd.Stdout = os.Stdout
		loginCmd.Stderr = os.Stderr
		
		if err := loginCmd.Run(); err != nil {
			return fmt.Errorf("interactive bw login failed: %w", err)
		}
		
		// Recursively self-evaluate since 'bw login' leaves the CLI in a 'locked' state in this shell context
		return checkBitwardenUnlocked()
	} else if status.Status != "unlocked" {
		return fmt.Errorf("Bitwarden vault is %s. Please resolve this state manually.", status.Status)
	}
	
	return nil
}
