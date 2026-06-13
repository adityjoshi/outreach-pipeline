package pipeline

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/adi/outreach-pipeline/internal/brevo"
	"github.com/adi/outreach-pipeline/internal/config"
	"github.com/adi/outreach-pipeline/internal/enrich"
	"github.com/adi/outreach-pipeline/internal/eazyreach"
	"github.com/adi/outreach-pipeline/internal/models"
	"github.com/adi/outreach-pipeline/internal/ocean"
	"github.com/adi/outreach-pipeline/internal/prospeo"
	"github.com/adi/outreach-pipeline/internal/util"
)

// Pipeline chains all outreach stages end to end.
type Pipeline struct {
	cfg       config.Config
	ocean     *ocean.Client
	prospeo   *prospeo.Client
	eazyreach *eazyreach.Client
	enricher  *enrich.Resolver
	brevo     *brevo.Client
}

// New wires stage clients from configuration.
func New(cfg config.Config) *Pipeline {
	pr := prospeo.New(cfg)
	ez := eazyreach.New(cfg)
	return &Pipeline{
		cfg:       cfg,
		ocean:     ocean.New(cfg),
		prospeo:   pr,
		eazyreach: ez,
		enricher:  enrich.New(cfg, ez, pr),
		brevo:     brevo.New(cfg),
	}
}

// Run executes Ocean -> Prospeo -> Eazyreach -> (confirm) -> Brevo.
func (p *Pipeline) Run(ctx context.Context, seedDomain string) error {
	seedDomain = util.NormalizeDomain(seedDomain)
	if seedDomain == "" {
		return fmt.Errorf("invalid seed domain")
	}

	util.Logf("Starting pipeline for seed domain: %s", seedDomain)

	util.LogStage(1, "Ocean.io — find lookalike companies")
	companies, err := p.ocean.FindLookalikes(ctx, seedDomain, p.cfg.MaxCompanies)
	if err != nil {
		return fmt.Errorf("stage 1 failed: %w", err)
	}
	if len(companies) == 0 {
		return fmt.Errorf("stage 1 returned no companies")
	}

	domains := make([]string, len(companies))
	for i, c := range companies {
		domains[i] = c.Domain
	}

	util.LogStage(2, "Prospeo — find decision-makers")
	contacts, err := p.prospeo.FindDecisionMakers(ctx, domains, p.cfg.MaxContactsPerCo)
	if err != nil {
		return fmt.Errorf("stage 2 failed: %w", err)
	}
	if len(contacts) == 0 {
		return fmt.Errorf("stage 2 returned no contacts with LinkedIn URLs")
	}

	util.LogStage(3, "Resolve work emails")
	enriched, err := p.enricher.ResolveEmails(ctx, contacts)
	if err != nil {
		return fmt.Errorf("stage 3 failed: %w", err)
	}
	if len(enriched) == 0 {
		return fmt.Errorf("stage 3 returned no verified emails")
	}

	printSummary(seedDomain, companies, enriched)

	if p.cfg.DryRun {
		util.Logf("Dry run complete — skipping email send")
		return nil
	}

	if !p.cfg.AutoYes {
		ok, err := confirmSend(len(enriched))
		if err != nil {
			return err
		}
		if !ok {
			util.Logf("Aborted before sending emails")
			return nil
		}
	}

	util.LogStage(4, "Brevo — send outreach")
	if err := p.brevo.ValidateSender(); err != nil {
		return fmt.Errorf("brevo config: %w", err)
	}

	results, err := p.brevo.SendOutreach(ctx, enriched)
	if err != nil {
		return fmt.Errorf("stage 4 failed: %w", err)
	}

	sent := 0
	for _, r := range results {
		if r.Sent {
			sent++
		}
	}
	util.Logf("Pipeline complete: %d emails sent", sent)
	return nil
}

func printSummary(seed string, companies []models.Company, contacts []models.Contact) {
	util.Logf("")
	util.Logf("=== Outreach summary (review before send) ===")
	util.Logf("Seed domain:        %s", seed)
	util.Logf("Lookalike companies: %d", len(companies))
	util.Logf("Contacts w/ email:   %d", len(contacts))
	util.Logf("")
	util.Logf("%-24s %-28s %-22s %s", "NAME", "TITLE", "COMPANY", "EMAIL")
	util.Logf("%s", strings.Repeat("-", 100))

	limit := len(contacts)
	if limit > 20 {
		limit = 20
	}
	for i := 0; i < limit; i++ {
		c := contacts[i]
		util.LogSummary("%-24s %-28s %-22s %s", truncate(c.FullName, 24), truncate(c.JobTitle, 28), truncate(c.CompanyName, 22), c.Email)
	}
	if len(contacts) > limit {
		util.Logf("... and %d more contacts", len(contacts)-limit)
	}
	util.Logf("")
}

func confirmSend(count int) (bool, error) {
	fmt.Fprintf(os.Stderr, "Send %d personalized emails via Brevo? [y/N]: ", count)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
