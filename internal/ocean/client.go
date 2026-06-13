package ocean

import (
	"context"
	"fmt"
	"time"

	"github.com/adi/outreach-pipeline/internal/config"
	"github.com/adi/outreach-pipeline/internal/httpclient"
	"github.com/adi/outreach-pipeline/internal/models"
	"github.com/adi/outreach-pipeline/internal/util"
)

const baseURL = "https://api.ocean.io/v3/search/companies"

// Client finds lookalike companies via Ocean.io.
type Client struct {
	http   *httpclient.Client
	token  string
	pageSz int
}

// New creates an Ocean.io client.
func New(cfg config.Config) *Client {
	return &Client{
		http: httpclient.New(
			time.Duration(cfg.HTTPTimeoutSeconds)*time.Second,
			cfg.MaxRetries,
			cfg.RetryBackoffMS,
		),
		token:  cfg.OceanAPIToken,
		pageSz: cfg.OceanPageSize,
	}
}

type searchRequest struct {
	Size             int              `json:"size"`
	SearchAfter      string           `json:"searchAfter,omitempty"`
	CompaniesFilters companiesFilters `json:"companiesFilters"`
}

type companiesFilters struct {
	LookalikeDomains    []string `json:"lookalikeDomains"`
	ExcludeDomains      []string `json:"excludeDomains,omitempty"`
	CompanyMatchingMode string   `json:"companyMatchingMode,omitempty"`
}

type searchResponse struct {
	Companies   []companyResult `json:"companies"`
	SearchAfter string          `json:"searchAfter"`
}

type companyResult struct {
	Company companyData `json:"company"`
}

type companyData struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
}

// FindLookalikes expands a seed domain into similar company domains.
func (c *Client) FindLookalikes(ctx context.Context, seed string, max int) ([]models.Company, error) {
	seed = util.NormalizeDomain(seed)
	if seed == "" {
		return nil, fmt.Errorf("empty seed domain")
	}

	seen := map[string]struct{}{seed: {}}
	var out []models.Company
	searchAfter := ""

	for len(out) < max {
		pageSize := c.pageSz
		if remaining := max - len(out); remaining < pageSize {
			pageSize = remaining
		}

		req := searchRequest{
			Size: pageSize,
			CompaniesFilters: companiesFilters{
				LookalikeDomains:    []string{seed},
				ExcludeDomains:      []string{seed},
				CompanyMatchingMode: "precise",
			},
		}
		if searchAfter != "" {
			req.SearchAfter = searchAfter
		}

		var resp searchResponse
		err := c.http.DoJSON(ctx, "POST", baseURL, map[string]string{
			"X-Api-Token": c.token,
		}, req, &resp)
		if err != nil {
			return out, fmt.Errorf("ocean search: %w", err)
		}

		if len(resp.Companies) == 0 {
			break
		}

		for _, item := range resp.Companies {
			domain := util.NormalizeDomain(item.Company.Domain)
			if domain == "" {
				continue
			}
			if _, ok := seen[domain]; ok {
				continue
			}
			seen[domain] = struct{}{}
			out = append(out, models.Company{
				Domain: domain,
				Name:   item.Company.Name,
			})
			if len(out) >= max {
				break
			}
		}

		if resp.SearchAfter == "" {
			break
		}
		searchAfter = resp.SearchAfter
	}

	util.Logf("Ocean.io found %d lookalike companies for %s", len(out), seed)
	return out, nil
}
