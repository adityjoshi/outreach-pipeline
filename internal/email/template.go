package email

import (
	"fmt"
	"strings"

	"github.com/adi/outreach-pipeline/internal/models"
)

// Compose builds personalized outreach copy for a contact.
func Compose(contact models.Contact) (subject, text, html string) {
	first := strings.TrimSpace(contact.FirstName)
	if first == "" {
		first = "there"
	}

	company := strings.TrimSpace(contact.CompanyName)
	if company == "" {
		company = contact.CompanyDomain
	}

	title := strings.TrimSpace(contact.JobTitle)
	if title == "" {
		title = "leader"
	}

	subject = fmt.Sprintf("Quick idea for %s", company)

	text = fmt.Sprintf(`Hi %s,

I came across your work as %s at %s and thought there might be a fit worth a short conversation.

We help similar companies automate outbound workflows end-to-end — from sourcing lookalike accounts to sending personalized outreach without manual handoffs.

If you're open to it, I'd love to share a 10-minute walkthrough tailored to %s.

Best,
Outreach Team
`, first, title, company, company)

	html = fmt.Sprintf(`<p>Hi %s,</p>
<p>I came across your work as <strong>%s</strong> at <strong>%s</strong> and thought there might be a fit worth a short conversation.</p>
<p>We help similar companies automate outbound workflows end-to-end — from sourcing lookalike accounts to sending personalized outreach without manual handoffs.</p>
<p>If you're open to it, I'd love to share a 10-minute walkthrough tailored to %s.</p>
<p>Best,<br>Outreach Team</p>
`, first, title, company, company)

	return subject, text, html
}
