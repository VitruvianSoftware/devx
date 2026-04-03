package devxerr

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
)

// RecoverGcloudAuth intercepts a podman/docker stdout/stderr byte array,
// checks if the google cloud docker credential helper caused the failure,
// and if so, interactively prompts the user to re-authenticate and auto-runs
// `gcloud auth login`.
//
// Returns true if the user successfully re-authenticated, indicating the
// caller should retry the command. Returns false otherwise.
func RecoverGcloudAuth(output string) bool {
	if !strings.Contains(output, "gcloud.auth.docker-helper") && !strings.Contains(output, "There was a problem refreshing your current auth") {
		return false
	}

	var confirm bool
	fmt.Println("\n❌ Container execution blocked by an expired Google Cloud token.")
	fmt.Println("   Your local `gcloud` credential helper intercepting the pull has expired.")

	err := huh.NewConfirm().
		Title("Run 'gcloud auth login' now to refresh your token and retry?").
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()

	if err != nil || !confirm {
		return false
	}

	fmt.Println("⏳ Launching browser for Google Cloud authentication...")

	cmd := exec.Command("gcloud", "auth", "login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Authentication failed: %v\n", err)
		return false
	}

	fmt.Println("✅ Authentication successful. Retrying container execution...")
	return true
}
