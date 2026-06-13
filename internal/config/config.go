package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime settings loaded from environment variables and flags.
type Config struct {
	OceanAPIToken string

	ProspeoAPIKey string

	EazyreachAPIKey      string
	EazyreachBaseURL     string
	EazyreachEnrichPath  string
	EazyreachAuthHeader  string
	EazyreachAuthPrefix  string
	EmailProvider        string // auto, prospeo, or eazyreach

	BrevoAPIKey    string
	BrevoSenderEmail string
	BrevoSenderName  string

	MaxCompanies        int
	MaxContactsPerCo    int
	OceanPageSize       int
	ProspeoPageSize     int
	HTTPTimeoutSeconds  int
	MaxRetries          int
	RetryBackoffMS      int

	DryRun bool
	AutoYes bool
}

// Load reads configuration from environment variables.
func Load() Config {
	return Config{
		OceanAPIToken: os.Getenv("OCEAN_API_TOKEN"),

		ProspeoAPIKey: os.Getenv("PROSPEO_API_KEY"),

		EazyreachAPIKey:     envOr("EAZYREACH_API_KEY", ""),
		EazyreachBaseURL:    envOr("EAZYREACH_BASE_URL", "https://api.eazyreach.app"),
		EazyreachEnrichPath: envOr("EAZYREACH_ENRICH_PATH", "/api/v1/enrich/linkedin"),
		EazyreachAuthHeader: envOr("EAZYREACH_AUTH_HEADER", "Authorization"),
		EazyreachAuthPrefix: envOr("EAZYREACH_AUTH_PREFIX", "Bearer"),
		EmailProvider:       envOr("EMAIL_PROVIDER", "auto"),

		BrevoAPIKey:      os.Getenv("BREVO_API_KEY"),
		BrevoSenderEmail: os.Getenv("BREVO_SENDER_EMAIL"),
		BrevoSenderName:  envOr("BREVO_SENDER_NAME", "Outreach"),

		MaxCompanies:       envInt("MAX_COMPANIES", 25),
		MaxContactsPerCo:   envInt("MAX_CONTACTS_PER_COMPANY", 2),
		OceanPageSize:      envInt("OCEAN_PAGE_SIZE", 50),
		ProspeoPageSize:    25,
		HTTPTimeoutSeconds: envInt("HTTP_TIMEOUT_SECONDS", 60),
		MaxRetries:         envInt("MAX_RETRIES", 3),
		RetryBackoffMS:     envInt("RETRY_BACKOFF_MS", 500),
	}
}

// Validate checks required credentials for a live run.
func (c Config) Validate(dryRun bool) error {
	var missing []string
	if c.OceanAPIToken == "" {
		missing = append(missing, "OCEAN_API_TOKEN")
	}
	if c.ProspeoAPIKey == "" {
		missing = append(missing, "PROSPEO_API_KEY")
	}
	if c.EmailProvider == "eazyreach" && c.EazyreachAPIKey == "" {
		missing = append(missing, "EAZYREACH_API_KEY")
	}
	if !dryRun {
		if c.BrevoAPIKey == "" {
			missing = append(missing, "BREVO_API_KEY")
		}
		if c.BrevoSenderEmail == "" {
			missing = append(missing, "BREVO_SENDER_EMAIL")
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
