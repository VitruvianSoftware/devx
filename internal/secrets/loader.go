package secrets

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/joho/godotenv"
)

// Secrets holds the user-specific values required to provision the environment.
type Secrets struct {
	CFTunnelToken string
	DevHostname   string
}

// NonInteractive bypasses the interactive terminal prompts and fails directly if required secrets are missing
var NonInteractive bool

// Load reads secrets from the given .env file. If any required value is
// missing, it falls back to an interactive huh form. Writes back to the
// .env file after a successful form submission.
func Load(envFile string) (*Secrets, error) {
	// Best-effort load — ignore file-not-found, error on parse failures
	if err := godotenv.Load(envFile); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("parsing %s: %w", envFile, err)
	}

	s := &Secrets{
		CFTunnelToken: os.Getenv("CF_TUNNEL_TOKEN"),
		DevHostname:   os.Getenv("DEV_HOSTNAME"),
	}

	if s.CFTunnelToken == "" || s.DevHostname == "" {
		if NonInteractive {
			return nil, fmt.Errorf("required secrets missing (CF_TUNNEL_TOKEN or DEV_HOSTNAME) but --non-interactive is set")
		}
		if err := s.promptAndSave(envFile); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// Save writes secrets to the given .env file.
func (s *Secrets) Save(envFile string) error {
	content := fmt.Sprintf("CF_TUNNEL_TOKEN=%s\nDEV_HOSTNAME=%s\n",
		s.CFTunnelToken, s.DevHostname)
	return os.WriteFile(envFile, []byte(content), 0600)
}

func (s *Secrets) promptAndSave(envFile string) error {
	fmt.Println()
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("devx — First Time Setup").
				Description("Enter your credentials to provision the environment.\nThese will be saved to "+envFile+"."),

			huh.NewInput().
				Title("Cloudflare Tunnel Token").
				Description("Retrieve with: cloudflared tunnel token <tunnel-name>").
				EchoMode(huh.EchoModePassword).
				Placeholder("eyJh...").
				Value(&s.CFTunnelToken),

			huh.NewInput().
				Title("Dev Machine Hostname").
				Description("Name for your VM (e.g. james-dev-machine)").
				Placeholder(defaultHostname()).
				Value(&s.DevHostname),
		),
	).WithTheme(huh.ThemeCatppuccin())

	if err := form.Run(); err != nil {
		return fmt.Errorf("secret prompt: %w", err)
	}

	if s.CFTunnelToken == "" || s.DevHostname == "" {
		return fmt.Errorf("both Cloudflare Tunnel Token and Dev Hostname are required")
	}

	return s.Save(envFile)
}

func defaultHostname() string {
	if u := os.Getenv("USER"); u != "" {
		return u + "-dev-machine"
	}
	return "my-dev-machine"
}

// Rotate runs the interactive form pre-filled with existing values and
// saves the result. Use for credential rotation.
func Rotate(envFile string) error {
	s := &Secrets{
		CFTunnelToken: os.Getenv("CF_TUNNEL_TOKEN"),
		DevHostname:   os.Getenv("DEV_HOSTNAME"),
	}
	_ = godotenv.Load(envFile)
	s.CFTunnelToken = os.Getenv("CF_TUNNEL_TOKEN")
	s.DevHostname = os.Getenv("DEV_HOSTNAME")

	// Mask the token for display but keep the underlying value
	displayToken := maskToken(s.CFTunnelToken)
	_ = displayToken

	return s.promptAndSave(envFile)
}

func maskToken(t string) string {
	if len(t) <= 8 {
		return strings.Repeat("●", len(t))
	}
	return t[:4] + strings.Repeat("●", 12) + t[len(t)-4:]
}
