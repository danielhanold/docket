package domain

import (
	"net/url"
	"strings"
)

// Ticket is a change's link to one ticket in an outside ticket system. The
// three values are stored exactly as given and never derived from each other.
type Ticket struct {
	Provider string
	ID       string
	URL      string
}

// The three frontmatter keys that hold a change's ticket link.
const (
	TicketFieldProvider = "ticket_provider"
	TicketFieldID       = "ticket_id"
	TicketFieldURL      = "ticket_url"
)

// TicketFields lists the ticket keys in the fixed order every caller reports
// and writes them in.
var TicketFields = []string{TicketFieldProvider, TicketFieldID, TicketFieldURL}

// TicketProblem names one ticket field that fails validation and the
// reader-facing rule it must meet.
type TicketProblem struct {
	Field string
	Rule  string
}

const (
	ticketRuleProvider = "a lowercase word: a letter, then letters, digits, or hyphens"
	ticketRuleID       = "non-empty, on one line"
	ticketRuleURL      = "an absolute http or https web address"
)

// ValidateTicket checks every ticket field and returns one problem per failing
// field, in TicketFields order; nil when the ticket is valid. It is the single
// validation rule shared by the decoder, change creation, and ticket linking.
func ValidateTicket(t Ticket) []TicketProblem {
	var problems []TicketProblem
	if !ValidTypeToken(t.Provider) {
		problems = append(problems, TicketProblem{Field: TicketFieldProvider, Rule: ticketRuleProvider})
	}
	if strings.TrimSpace(t.ID) == "" || strings.ContainsAny(t.ID, "\r\n") {
		problems = append(problems, TicketProblem{Field: TicketFieldID, Rule: ticketRuleID})
	}
	if !validTicketURL(t.URL) {
		problems = append(problems, TicketProblem{Field: TicketFieldURL, Rule: ticketRuleURL})
	}
	return problems
}

// validTicketURL reports whether s parses as an absolute http or https URL
// with a host. url.Parse lowercases the scheme, so "HTTPS://h/x" is accepted.
func validTicketURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// TicketProvider returns the optional ticket provider field.
func (c Change) TicketProvider() OptionalString { return c.spec.TicketProvider }

// TicketID returns the optional ticket id field.
func (c Change) TicketID() OptionalString { return c.spec.TicketID }

// TicketURL returns the optional ticket URL field.
func (c Change) TicketURL() OptionalString { return c.spec.TicketURL }

func (c Change) ticketFieldValues() []OptionalString {
	return []OptionalString{c.spec.TicketProvider, c.spec.TicketID, c.spec.TicketURL}
}

// HasTicketFields reports whether the change carries any ticket field with
// content: a malformed one, or a present one with a non-empty value.
func (c Change) HasTicketFields() bool {
	for _, f := range c.ticketFieldValues() {
		if f.State == FieldMalformed || (f.State == FieldPresent && f.Value != "") {
			return true
		}
	}
	return false
}

// Ticket returns the change's ticket link. ok is true only when all three
// fields are present, non-empty, and pass ValidateTicket; anything else is
// "not linked" and returns the zero Ticket.
func (c Change) Ticket() (Ticket, bool) {
	for _, f := range c.ticketFieldValues() {
		if f.State != FieldPresent || f.Value == "" {
			return Ticket{}, false
		}
	}
	t := Ticket{Provider: c.spec.TicketProvider.Value, ID: c.spec.TicketID.Value, URL: c.spec.TicketURL.Value}
	if ValidateTicket(t) != nil {
		return Ticket{}, false
	}
	return t, true
}
