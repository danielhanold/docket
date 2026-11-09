package app

import (
	"reflect"
	"strings"
	"testing"
)

const testTicketURL = "https://tickets.example.com/browse/CLOUD-123"

func ticketCodes(findings []StatusFinding) []string {
	var codes []string
	for _, f := range findings {
		codes = append(codes, f.Code)
	}
	return codes
}

func TestTicketRequestFindings(t *testing.T) {
	cases := []struct {
		name              string
		provider, id, url string
		want              []string
	}{
		{"none", "", "", "", nil},
		{"missing id", "jira", "", testTicketURL, []string{string(FCPartialTicket)}},
		{"missing url", "jira", "CLOUD-123", "", []string{string(FCPartialTicket)}},
		{"only url", "", "", testTicketURL, []string{string(FCPartialTicket)}},
		{"valid", "jira", "CLOUD-123", testTicketURL, nil},
		{"bad provider", "JIRA", "CLOUD-123", testTicketURL, []string{string(FCInvalidTicketProvider)}},
		{"blank id", "jira", "   ", testTicketURL, []string{string(FCInvalidTicketID)}},
		{"relative url", "jira", "CLOUD-123", "/browse/X", []string{string(FCInvalidTicketURL)}},
		{"all bad", "JIRA", " ", "x", []string{string(FCInvalidTicketProvider), string(FCInvalidTicketID), string(FCInvalidTicketURL)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ticketRequestFindings(tc.provider, tc.id, tc.url)
			if codes := ticketCodes(got); !reflect.DeepEqual(codes, tc.want) {
				t.Fatalf("codes = %v, want %v", codes, tc.want)
			}
			for _, f := range got {
				if f.Severity != "error" {
					t.Errorf("finding %q severity = %q, want error", f.Code, f.Severity)
				}
				if f.Message == "" {
					t.Errorf("finding %q has an empty message", f.Code)
				}
			}
		})
	}
}

func TestTicketRequestFindingsNameTheKeys(t *testing.T) {
	partial := ticketRequestFindings("jira", "", "")
	if len(partial) != 1 || !strings.Contains(partial[0].Message, "ticket_id") || !strings.Contains(partial[0].Message, "ticket_url") {
		t.Errorf("partial-ticket message must name the missing keys: %+v", partial)
	}
	if strings.Contains(partial[0].Message, "ticket_provider is missing") {
		t.Errorf("partial-ticket message names a given key as missing: %q", partial[0].Message)
	}
	for _, f := range ticketRequestFindings("JIRA", " ", "x") {
		var key string
		switch f.Code {
		case string(FCInvalidTicketProvider):
			key = "ticket_provider"
		case string(FCInvalidTicketID):
			key = "ticket_id"
		case string(FCInvalidTicketURL):
			key = "ticket_url"
		}
		if !strings.Contains(f.Message, key) {
			t.Errorf("finding %q message %q does not name %s", f.Code, f.Message, key)
		}
	}
}
