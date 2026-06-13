package models

// Company is a target account discovered from lookalike search.
type Company struct {
	Domain string
	Name   string
}

// Contact is a decision-maker with enrichment fields filled in per stage.
type Contact struct {
	PersonID     string
	FirstName    string
	LastName     string
	FullName     string
	JobTitle     string
	LinkedInURL  string
	CompanyDomain string
	CompanyName  string
	Email        string
	EmailStatus  string
}

// OutreachResult captures the outcome of a single email send attempt.
type OutreachResult struct {
	Contact Contact
	Sent    bool
	Message string
}
