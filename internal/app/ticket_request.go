package app

import (
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
)

// ticketRequestFindings checks a request's ticket link: the three fields come
// together or not at all, and a complete set must pass domain.ValidateTicket.
// A field counts as given when it is non-empty. No field given yields nil; one
// or two given yields a single partial-ticket finding naming the missing keys;
// all three given yields one finding per failing field, in
// domain.TicketFields order.
func ticketRequestFindings(provider, id, url string) []StatusFinding {
	values := map[string]string{
		domain.TicketFieldProvider: provider,
		domain.TicketFieldID:       id,
		domain.TicketFieldURL:      url,
	}
	var missing []string
	for _, key := range domain.TicketFields {
		if values[key] == "" {
			missing = append(missing, key)
		}
	}
	switch len(missing) {
	case len(domain.TicketFields):
		return nil
	case 0:
	default:
		return []StatusFinding{{
			Code:     string(FCPartialTicket),
			Severity: string(domain.SeverityError),
			Message: fmt.Sprintf("a ticket link needs all of %s together; missing %s",
				strings.Join(domain.TicketFields, ", "), strings.Join(missing, ", ")),
		}}
	}

	codes := map[string]FindingCode{
		domain.TicketFieldProvider: FCInvalidTicketProvider,
		domain.TicketFieldID:       FCInvalidTicketID,
		domain.TicketFieldURL:      FCInvalidTicketURL,
	}
	var findings []StatusFinding
	for _, p := range domain.ValidateTicket(domain.Ticket{Provider: provider, ID: id, URL: url}) {
		findings = append(findings, StatusFinding{
			Code:     string(codes[p.Field]),
			Severity: string(domain.SeverityError),
			Message:  fmt.Sprintf("%s must be %s", p.Field, p.Rule),
		})
	}
	return findings
}

// ticketRequestComplete reports whether all three ticket fields are given.
func ticketRequestComplete(provider, id, url string) bool {
	return provider != "" && id != "" && url != ""
}
