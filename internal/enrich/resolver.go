package enrich

import (
	"context"
	"strings"

	"github.com/adi/outreach-pipeline/internal/config"
	"github.com/adi/outreach-pipeline/internal/eazyreach"
	"github.com/adi/outreach-pipeline/internal/models"
	"github.com/adi/outreach-pipeline/internal/prospeo"
	"github.com/adi/outreach-pipeline/internal/util"
)

// Resolver resolves work emails, trying Eazyreach then falling back to Prospeo.
type Resolver struct {
	provider       string
	skipEazyreach  bool
	eazyreach      *eazyreach.Client
	prospeo        *prospeo.Client
}

// New creates an email resolver from config.
func New(cfg config.Config, ez *eazyreach.Client, pr *prospeo.Client) *Resolver {
	return &Resolver{
		provider:      strings.ToLower(cfg.EmailProvider),
		skipEazyreach: shouldSkipEazyreach(cfg),
		eazyreach:     ez,
		prospeo:       pr,
	}
}

func shouldSkipEazyreach(cfg config.Config) bool {
	key := strings.ToLower(strings.TrimSpace(cfg.EazyreachAPIKey))
	switch key {
	case "", "placeholder", "local-dev", "dummy", "test":
		return true
	default:
		return false
	}
}

// ResolveEmails enriches contacts with verified work emails.
func (r *Resolver) ResolveEmails(ctx context.Context, contacts []models.Contact) ([]models.Contact, error) {
	switch r.provider {
	case "prospeo":
		util.Logf("Using Prospeo enrich-person for email resolution")
		return r.prospeo.EnrichEmails(ctx, contacts)
	case "eazyreach":
		return r.eazyreach.ResolveEmails(ctx, contacts)
	default: // auto
		return r.resolveAuto(ctx, contacts)
	}
}

func (r *Resolver) resolveAuto(ctx context.Context, contacts []models.Contact) ([]models.Contact, error) {
	if r.skipEazyreach {
		util.Logf("Eazyreach not configured — using Prospeo enrich-person")
		return r.prospeo.EnrichEmails(ctx, contacts)
	}

	enriched, err := r.eazyreach.ResolveEmails(ctx, contacts)
	if err == nil && len(enriched) > 0 {
		return enriched, nil
	}

	if len(enriched) == 0 {
		util.Logf("Eazyreach returned no emails — falling back to Prospeo enrich-person")
	} else {
		util.Logf("Eazyreach partial (%d/%d) — enriching remainder via Prospeo", len(enriched), len(contacts))
	}

	prospeoOut, perr := r.prospeo.EnrichEmails(ctx, contacts)
	if perr != nil && len(enriched) == 0 {
		return nil, perr
	}

	return mergeEnriched(enriched, prospeoOut), nil
}

func mergeEnriched(primary, fallback []models.Contact) []models.Contact {
	seen := map[string]struct{}{}
	out := make([]models.Contact, 0, len(primary)+len(fallback))

	for _, c := range primary {
		if _, ok := seen[c.Email]; ok {
			continue
		}
		seen[c.Email] = struct{}{}
		out = append(out, c)
	}
	for _, c := range fallback {
		if _, ok := seen[c.Email]; ok {
			continue
		}
		seen[c.Email] = struct{}{}
		out = append(out, c)
	}
	return out
}
