package prospeo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/adi/outreach-pipeline/internal/models"
	"github.com/adi/outreach-pipeline/internal/util"
)

const enrichURL = "https://api.prospeo.io/enrich-person"

type enrichRequest struct {
	OnlyVerifiedEmail bool       `json:"only_verified_email"`
	Data              enrichData `json:"data"`
}

type enrichData struct {
	PersonID    string `json:"person_id,omitempty"`
	LinkedInURL string `json:"linkedin_url,omitempty"`
}

type enrichResponse struct {
	Error     bool   `json:"error"`
	ErrorCode string `json:"error_code"`
	Person    struct {
		Email struct {
			Status  string `json:"status"`
			Revealed bool  `json:"revealed"`
			Email   string `json:"email"`
		} `json:"email"`
	} `json:"person"`
}

// EnrichEmails resolves verified work emails via Prospeo enrich-person.
func (c *Client) EnrichEmails(ctx context.Context, contacts []models.Contact) ([]models.Contact, error) {
	var enriched []models.Contact
	emailSeen := map[string]struct{}{}

	for i, contact := range contacts {
		email, status, err := c.enrichOne(ctx, contact)
		if err != nil {
			util.Logf("  skip [%d/%d] %s: %v", i+1, len(contacts), contact.FullName, err)
			continue
		}
		if email == "" {
			util.Logf("  skip [%d/%d] %s: no verified email", i+1, len(contacts), contact.FullName)
			continue
		}

		normalized := strings.ToLower(strings.TrimSpace(email))
		if _, ok := emailSeen[normalized]; ok {
			util.Logf("  skip [%d/%d] %s: duplicate email", i+1, len(contacts), contact.FullName)
			continue
		}
		emailSeen[normalized] = struct{}{}

		contact.Email = normalized
		contact.EmailStatus = status
		enriched = append(enriched, contact)
		util.Logf("  resolved [%d/%d] %s -> %s (prospeo)", i+1, len(contacts), contact.FullName, contact.Email)

		select {
		case <-ctx.Done():
			return enriched, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}

	util.Logf("Prospeo enriched %d/%d emails", len(enriched), len(contacts))
	return enriched, nil
}

func (c *Client) enrichOne(ctx context.Context, contact models.Contact) (email, status string, err error) {
	data := enrichData{}
	if contact.PersonID != "" {
		data.PersonID = contact.PersonID
	} else if contact.LinkedInURL != "" {
		data.LinkedInURL = contact.LinkedInURL
	} else {
		return "", "", fmt.Errorf("no person_id or linkedin_url")
	}

	var resp enrichResponse
	err = c.http.DoJSON(ctx, "POST", enrichURL, map[string]string{
		"X-KEY": c.apiKey,
	}, enrichRequest{
		OnlyVerifiedEmail: true,
		Data:              data,
	}, &resp)
	if err != nil {
		return "", "", err
	}
	if resp.Error {
		if resp.ErrorCode == "NO_MATCH" {
			return "", "", fmt.Errorf("no verified email found")
		}
		return "", "", fmt.Errorf("prospeo enrich: %s", resp.ErrorCode)
	}

	email = strings.TrimSpace(resp.Person.Email.Email)
	if email == "" || strings.Contains(email, "*") {
		return "", "", fmt.Errorf("email not revealed")
	}

	status = resp.Person.Email.Status
	if status == "" {
		status = "verified"
	}
	return email, status, nil
}
