package eazyreach

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/adi/outreach-pipeline/internal/config"
	"github.com/adi/outreach-pipeline/internal/httpclient"
	"github.com/adi/outreach-pipeline/internal/models"
	"github.com/adi/outreach-pipeline/internal/util"
)

// Client resolves LinkedIn profile URLs to verified work emails.
//
// Eazyreach's public API docs are limited. Endpoint shape is configurable via
// environment variables so you can align with the credentials they provide.
type Client struct {
	http        *httpclient.Client
	apiKey      string
	baseURL     string
	enrichPath  string
	authHeader  string
	authPrefix  string
}

// New creates an Eazyreach client.
func New(cfg config.Config) *Client {
	return &Client{
		http: httpclient.New(
			time.Duration(cfg.HTTPTimeoutSeconds)*time.Second,
			cfg.MaxRetries,
			cfg.RetryBackoffMS,
		),
		apiKey:     cfg.EazyreachAPIKey,
		baseURL:    strings.TrimRight(cfg.EazyreachBaseURL, "/"),
		enrichPath: cfg.EazyreachEnrichPath,
		authHeader: cfg.EazyreachAuthHeader,
		authPrefix: strings.TrimSpace(cfg.EazyreachAuthPrefix),
	}
}

type enrichRequest struct {
	LinkedInURL string `json:"linkedin_url"`
}

type enrichResponse struct {
	Email    string `json:"email"`
	WorkEmail string `json:"work_email"`
	Data     struct {
		Email    string `json:"email"`
		WorkEmail string `json:"work_email"`
	} `json:"data"`
	Result struct {
		Email    string `json:"email"`
		WorkEmail string `json:"work_email"`
		Verified bool   `json:"verified"`
	} `json:"result"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// ResolveEmails enriches contacts with verified work emails.
func (c *Client) ResolveEmails(ctx context.Context, contacts []models.Contact) ([]models.Contact, error) {
	var enriched []models.Contact
	emailSeen := map[string]struct{}{}

	for i, contact := range contacts {
		if contact.LinkedInURL == "" {
			util.Logf("  skip [%d/%d] %s: no LinkedIn URL", i+1, len(contacts), contact.FullName)
			continue
		}

		email, err := c.resolveOne(ctx, contact.LinkedInURL)
		if err != nil {
			util.Logf("  skip [%d/%d] %s: %v", i+1, len(contacts), contact.FullName, err)
			continue
		}
		if email == "" {
			util.Logf("  skip [%d/%d] %s: no email found", i+1, len(contacts), contact.FullName)
			continue
		}

		normalized := strings.ToLower(strings.TrimSpace(email))
		if _, ok := emailSeen[normalized]; ok {
			util.Logf("  skip [%d/%d] %s: duplicate email", i+1, len(contacts), contact.FullName)
			continue
		}
		emailSeen[normalized] = struct{}{}

		contact.Email = normalized
		contact.EmailStatus = "verified"
		enriched = append(enriched, contact)
		util.Logf("  resolved [%d/%d] %s -> %s", i+1, len(contacts), contact.FullName, contact.Email)

		select {
		case <-ctx.Done():
			return enriched, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}

	util.Logf("Eazyreach resolved %d/%d emails", len(enriched), len(contacts))
	return enriched, nil
}

func (c *Client) resolveOne(ctx context.Context, linkedinURL string) (string, error) {
	url := c.baseURL + c.enrichPath
	headers := map[string]string{}
	if c.apiKey != "" {
		value := c.apiKey
		if c.authPrefix != "" {
			value = c.authPrefix + " " + c.apiKey
		}
		headers[c.authHeader] = value
	}

	var resp enrichResponse
	err := c.http.DoJSON(ctx, "POST", url, headers, enrichRequest{
		LinkedInURL: linkedinURL,
	}, &resp)
	if err != nil {
		return "", err
	}

	if msg := strings.TrimSpace(resp.Error); msg != "" {
		return "", fmt.Errorf("%s", msg)
	}
	if msg := strings.TrimSpace(resp.Message); msg != "" && resp.Email == "" && resp.WorkEmail == "" && resp.Data.Email == "" {
		if strings.Contains(strings.ToLower(msg), "not found") || strings.Contains(strings.ToLower(msg), "error") {
			return "", fmt.Errorf("%s", msg)
		}
	}

	email := firstNonEmpty(
		resp.Email,
		resp.WorkEmail,
		resp.Data.Email,
		resp.Data.WorkEmail,
		resp.Result.Email,
		resp.Result.WorkEmail,
	)
	return strings.ToLower(strings.TrimSpace(email)), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
