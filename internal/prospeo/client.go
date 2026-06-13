package prospeo

import (
	"context"
	"fmt"
	"time"

	"github.com/adi/outreach-pipeline/internal/config"
	"github.com/adi/outreach-pipeline/internal/httpclient"
	"github.com/adi/outreach-pipeline/internal/models"
	"github.com/adi/outreach-pipeline/internal/util"
)

const searchURL = "https://api.prospeo.io/search-person"

// Client finds decision-makers at target companies.
type Client struct {
	http   *httpclient.Client
	apiKey string
}

// New creates a Prospeo client.
func New(cfg config.Config) *Client {
	return &Client{
		http: httpclient.New(
			time.Duration(cfg.HTTPTimeoutSeconds)*time.Second,
			cfg.MaxRetries,
			cfg.RetryBackoffMS,
		),
		apiKey: cfg.ProspeoAPIKey,
	}
}

type searchRequest struct {
	Page    int     `json:"page"`
	Filters filters `json:"filters"`
}

type filters struct {
	Company struct {
		Websites struct {
			Include []string `json:"include"`
		} `json:"websites"`
	} `json:"company"`
	PersonSeniority struct {
		Include []string `json:"include"`
	} `json:"person_seniority"`
	MaxPersonPerCompany int `json:"max_person_per_company,omitempty"`
}

type searchResponse struct {
	Error      bool   `json:"error"`
	ErrorCode  string `json:"error_code"`
	FilterErr  string `json:"filter_error"`
	Results    []result `json:"results"`
	Pagination struct {
		CurrentPage int `json:"current_page"`
		TotalPage   int `json:"total_page"`
		TotalCount  int `json:"total_count"`
	} `json:"pagination"`
}

type result struct {
	Person  person  `json:"person"`
	Company company `json:"company"`
}

type person struct {
	PersonID        string `json:"person_id"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	FullName        string `json:"full_name"`
	LinkedInURL     string `json:"linkedin_url"`
	CurrentJobTitle string `json:"current_job_title"`
}

type company struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// FindDecisionMakers searches C-suite and VP contacts for the given domains.
func (c *Client) FindDecisionMakers(ctx context.Context, domains []string, maxPerCompany int) ([]models.Contact, error) {
	if len(domains) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(domains))
	seen := map[string]struct{}{}
	for _, d := range domains {
		d = util.NormalizeDomain(d)
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		normalized = append(normalized, d)
	}

	var contacts []models.Contact
	linkedinSeen := map[string]struct{}{}

	for start := 0; start < len(normalized); start += 500 {
		end := start + 500
		if end > len(normalized) {
			end = len(normalized)
		}
		batch := normalized[start:end]

		page := 1
		for {
			req := searchRequest{
				Page: page,
				Filters: filters{
					PersonSeniority: struct {
						Include []string `json:"include"`
					}{
						Include: []string{"C-Suite", "Vice President"},
					},
					MaxPersonPerCompany: maxPerCompany,
				},
			}
			req.Filters.Company.Websites.Include = batch

			var resp searchResponse
			err := c.http.DoJSON(ctx, "POST", searchURL, map[string]string{
				"X-KEY": c.apiKey,
			}, req, &resp)
			if err != nil {
				return contacts, fmt.Errorf("prospeo search page %d: %w", page, err)
			}
			if resp.Error {
				if resp.ErrorCode == "NO_RESULTS" {
					break
				}
				return contacts, fmt.Errorf("prospeo error %s: %s", resp.ErrorCode, resp.FilterErr)
			}

			for _, row := range resp.Results {
				contact := models.Contact{
					PersonID:      row.Person.PersonID,
					FirstName:     row.Person.FirstName,
					LastName:      row.Person.LastName,
					FullName:      row.Person.FullName,
					JobTitle:      row.Person.CurrentJobTitle,
					LinkedInURL:   row.Person.LinkedInURL,
					CompanyDomain: util.NormalizeDomain(row.Company.Domain),
					CompanyName:   row.Company.Name,
				}
				if contact.FullName == "" {
					contact.FullName = fmt.Sprintf("%s %s", contact.FirstName, contact.LastName)
				}
				if contact.LinkedInURL == "" {
					continue
				}
				key := contact.LinkedInURL
				if _, ok := linkedinSeen[key]; ok {
					continue
				}
				linkedinSeen[key] = struct{}{}
				contacts = append(contacts, contact)
			}

			if page >= resp.Pagination.TotalPage || len(resp.Results) == 0 {
				break
			}
			page++

			// Respect Prospeo search rate limits on free tiers.
			select {
			case <-ctx.Done():
				return contacts, ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}

	util.Logf("Prospeo found %d decision-makers with LinkedIn URLs", len(contacts))
	return contacts, nil
}
