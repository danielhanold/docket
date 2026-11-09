package domain

import (
	"slices"
	"testing"
)

const fixtureTicketURL = "https://tickets.example.com/browse/CLOUD-123"

// problemFields returns the Field of each problem, in order.
func problemFields(ps []TicketProblem) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Field)
	}
	return out
}

func checkTicketField(t *testing.T, field string, valid, invalid []string, mk func(string) Ticket) {
	t.Helper()
	for _, v := range valid {
		if ps := ValidateTicket(mk(v)); ps != nil {
			t.Errorf("%s %q: got problems %+v; want none", field, v, ps)
		}
	}
	for _, v := range invalid {
		ps := ValidateTicket(mk(v))
		if got := problemFields(ps); !slices.Equal(got, []string{field}) {
			t.Errorf("%s %q: got problem fields %v; want [%s]", field, v, got, field)
		}
		for _, p := range ps {
			if p.Rule == "" {
				t.Errorf("%s %q: problem has empty Rule", field, v)
			}
		}
	}
}

func TestValidateTicketProvider(t *testing.T) {
	checkTicketField(t, TicketFieldProvider,
		[]string{"jira", "azure-devops", "x1"},
		[]string{"", "JIRA", "1jira", "ji ra", "-x"},
		func(v string) Ticket { return Ticket{Provider: v, ID: "CLOUD-123", URL: fixtureTicketURL} })
}

func TestValidateTicketID(t *testing.T) {
	checkTicketField(t, TicketFieldID,
		[]string{"CLOUD-123", " CLOUD-123 "},
		[]string{"", "   ", "A\nB", "A\rB"},
		func(v string) Ticket { return Ticket{Provider: "jira", ID: v, URL: fixtureTicketURL} })
}

func TestValidateTicketURL(t *testing.T) {
	checkTicketField(t, TicketFieldURL,
		[]string{fixtureTicketURL, "http://h/x", "HTTPS://Tickets.Example.com/x"},
		[]string{"", "javascript:alert(1)", "/browse/X", "mailto:a@b.c", "https:///x", "ftp://h/x", "tickets.example.com/x"},
		func(v string) Ticket { return Ticket{Provider: "jira", ID: "CLOUD-123", URL: v} })
}

func TestValidateTicketReportsEveryBadField(t *testing.T) {
	ps := ValidateTicket(Ticket{Provider: "JIRA", ID: "", URL: "x"})
	if got := problemFields(ps); !slices.Equal(got, TicketFields) {
		t.Fatalf("problem fields = %v; want %v", got, TicketFields)
	}
	want := []string{
		"a lowercase word: a letter, then letters, digits, or hyphens",
		"non-empty, on one line",
		"an absolute http or https web address",
	}
	for i, p := range ps {
		if p.Rule != want[i] {
			t.Errorf("problem %d Rule = %q; want %q", i, p.Rule, want[i])
		}
	}
}

func TestTicketFieldsOrder(t *testing.T) {
	want := []string{"ticket_provider", "ticket_id", "ticket_url"}
	if !slices.Equal(TicketFields, want) {
		t.Fatalf("TicketFields = %v; want %v", TicketFields, want)
	}
}

func TestChangeTicketLinkedness(t *testing.T) {
	present := func(v string) OptionalString { return OptionalString{State: FieldPresent, Value: v} }
	absent := OptionalString{}
	empty := OptionalString{State: FieldEmpty}
	malformed := OptionalString{State: FieldMalformed, Value: "{a: b}"}

	cases := []struct {
		name          string
		provider, id  OptionalString
		url           OptionalString
		wantOK        bool
		wantHasFields bool
	}{
		{"all valid", present("jira"), present("CLOUD-123"), present(fixtureTicketURL), true, true},
		{"two present", present("jira"), present("CLOUD-123"), absent, false, true},
		{"bad url", present("jira"), present("CLOUD-123"), present("javascript:x"), false, true},
		{"all absent", absent, absent, absent, false, false},
		{"all empty", empty, empty, empty, false, false},
		{"one malformed", absent, malformed, absent, false, true},
		{"present but blank value", present(""), absent, absent, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewChange(ChangeSpec{TicketProvider: tc.provider, TicketID: tc.id, TicketURL: tc.url})
			got, ok := c.Ticket()
			if ok != tc.wantOK {
				t.Fatalf("Ticket() ok = %v; want %v", ok, tc.wantOK)
			}
			if ok {
				want := Ticket{Provider: tc.provider.Value, ID: tc.id.Value, URL: tc.url.Value}
				if got != want {
					t.Errorf("Ticket() = %+v; want %+v", got, want)
				}
			} else if got != (Ticket{}) {
				t.Errorf("Ticket() = %+v on not-linked; want zero", got)
			}
			if h := c.HasTicketFields(); h != tc.wantHasFields {
				t.Errorf("HasTicketFields() = %v; want %v", h, tc.wantHasFields)
			}
		})
	}
}
