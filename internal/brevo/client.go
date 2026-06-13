package brevo

import (
	"context"
	"fmt"
	"time"

	"github.com/adi/outreach-pipeline/internal/config"
	"github.com/adi/outreach-pipeline/internal/email"
	"github.com/adi/outreach-pipeline/internal/httpclient"
	"github.com/adi/outreach-pipeline/internal/models"
	"github.com/adi/outreach-pipeline/internal/util"
)

const sendURL = "https://api.brevo.com/v3/smtp/email"

// Client sends outreach emails through Brevo.
type Client struct {
	http       *httpclient.Client
	apiKey     string
	senderName string
	senderEmail string
}

// New creates a Brevo client.
func New(cfg config.Config) *Client {
	return &Client{
		http: httpclient.New(
			time.Duration(cfg.HTTPTimeoutSeconds)*time.Second,
			cfg.MaxRetries,
			cfg.RetryBackoffMS,
		),
		apiKey:      cfg.BrevoAPIKey,
		senderName:  cfg.BrevoSenderName,
		senderEmail: cfg.BrevoSenderEmail,
	}
}

type sendRequest struct {
	Sender      recipient `json:"sender"`
	To          []recipient `json:"to"`
	Subject     string    `json:"subject"`
	HTMLContent string    `json:"htmlContent"`
	TextContent string    `json:"textContent"`
}

type recipient struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type sendResponse struct {
	MessageID string `json:"messageId"`
}

// SendOutreach sends personalized emails to all contacts.
func (c *Client) SendOutreach(ctx context.Context, contacts []models.Contact) ([]models.OutreachResult, error) {
	results := make([]models.OutreachResult, 0, len(contacts))

	for i, contact := range contacts {
		subject, textBody, htmlBody := email.Compose(contact)
		req := sendRequest{
			Sender: recipient{
				Email: c.senderEmail,
				Name:  c.senderName,
			},
			To: []recipient{{
				Email: contact.Email,
				Name:  contact.FullName,
			}},
			Subject:     subject,
			TextContent: textBody,
			HTMLContent: htmlBody,
		}

		var resp sendResponse
		err := c.http.DoJSON(ctx, "POST", sendURL, map[string]string{
			"api-key": c.apiKey,
		}, req, &resp)
		if err != nil {
			results = append(results, models.OutreachResult{
				Contact: contact,
				Sent:    false,
				Message: err.Error(),
			})
			util.Logf("  failed [%d/%d] %s: %v", i+1, len(contacts), contact.Email, err)
			continue
		}

		results = append(results, models.OutreachResult{
			Contact: contact,
			Sent:    true,
			Message: resp.MessageID,
		})
		util.Logf("  sent [%d/%d] %s (messageId=%s)", i+1, len(contacts), contact.Email, resp.MessageID)

		select {
		case <-ctx.Done():
			return results, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}

	sent := 0
	for _, r := range results {
		if r.Sent {
			sent++
		}
	}
	util.Logf("Brevo sent %d/%d emails", sent, len(contacts))
	return results, nil
}

// ValidateSender ensures sender configuration is present.
func (c *Client) ValidateSender() error {
	if c.senderEmail == "" {
		return fmt.Errorf("BREVO_SENDER_EMAIL is required")
	}
	if c.apiKey == "" {
		return fmt.Errorf("BREVO_API_KEY is required")
	}
	return nil
}
