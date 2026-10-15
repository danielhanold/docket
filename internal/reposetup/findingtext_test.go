package reposetup

import (
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/document"
)

const (
	locatorTail  = ", and run `docket repository repair` so the board and link blocks catch up."
	locatorInstr = "remove `172` from `depends_on`"
	locatorRel   = "docs/changes/active/0263-x.md"
)

// TestRecordLocatorHandEditThenRepairPerLayout pins that the file leads the
// remedy, so the location never attaches to the instruction's last example or
// alternative ("such as `feature` or `fix` in `<file>`").
func TestRecordLocatorHandEditThenRepairPerLayout(t *testing.T) {
	const p = "/home/u/.local/share/dckt/o/r/checkouts/r-1"
	cases := []struct {
		name string
		loc  RecordLocator
		want string
	}{
		{"shared", RecordLocator{Checkout: ".docket"},
			"In `.docket/" + locatorRel + "`, remove `172` from `depends_on`; then commit and push it from `.docket`" + locatorTail},
		{"private path", RecordLocator{Checkout: p},
			"In `" + p + "/" + locatorRel + "`, remove `172` from `depends_on`; then commit and push it from `" + p + "`" + locatorTail},
		{"private words", RecordLocator{Label: "your private metadata checkout"},
			"In `" + locatorRel + "` in your private metadata checkout, remove `172` from `depends_on`; then commit and push it from your private metadata checkout" + locatorTail},
		{"zero", RecordLocator{},
			"In `" + locatorRel + "` on the metadata branch, remove `172` from `depends_on`; then commit and push it" + locatorTail},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.loc.HandEditThenRepair(locatorInstr, locatorRel); got != c.want {
				t.Errorf("HandEditThenRepair:\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

// TestRecordLocatorEmptyRelNamesNoFile pins that an edit spanning several
// records names no file and leaves no dangling "it" in the commit clause.
func TestRecordLocatorEmptyRelNamesNoFile(t *testing.T) {
	got := RecordLocator{Checkout: ".docket"}.HandEditThenRepair(locatorInstr, "")
	want := "Remove `172` from `depends_on`; then commit and push the edited records from `.docket`" + locatorTail
	if got != want {
		t.Errorf("HandEditThenRepair(empty rel):\n got %q\nwant %q", got, want)
	}
	got = RecordLocator{Checkout: ".docket"}.RepairOrHandEdit("remove it", "delete the `claimed_at:` line", "")
	if strings.Contains(got, "push it") || !strings.Contains(got, "commit and push the edited records") {
		t.Errorf("RepairOrHandEdit(empty rel) = %q", got)
	}
}

func TestRecordLocatorRepairOrHandEdit(t *testing.T) {
	got := RecordLocator{Checkout: ".docket"}.RepairOrHandEdit("remove it", "delete the `claimed_at:` line", locatorRel)
	want := "Run `docket repository repair` to remove it (it shows the edit first). Or edit it by hand: in `.docket/" + locatorRel + "`, delete the `claimed_at:` line; then commit and push it from `.docket`."
	if got != want {
		t.Errorf("RepairOrHandEdit:\n got %q\nwant %q", got, want)
	}
}

// TestRecordLocatorSetupBlocked pins the unhealthy-setup wording: a set
// Checkout is ignored (it may not exist yet) and every repair waits on the
// setup finding, because repair refuses until the setup is fixed.
func TestRecordLocatorSetupBlocked(t *testing.T) {
	loc := RecordLocator{Checkout: ".docket", Label: "your private metadata checkout", SetupBlocked: true}
	cases := []struct{ name, got, want string }{
		{"hand edit", loc.HandEditThenRepair(locatorInstr, locatorRel),
			"In `" + locatorRel + "` on the metadata branch, remove `172` from `depends_on`; then commit and push it. Once the setup finding above is resolved, run `docket repository repair` so the board and link blocks catch up."},
		{"hand edit, several records", loc.HandEditThenRepair(locatorInstr, ""),
			"Remove `172` from `depends_on`; then commit and push the edited records. Once the setup finding above is resolved, run `docket repository repair` so the board and link blocks catch up."},
		{"repair or hand edit", loc.RepairOrHandEdit("remove it", "delete the `claimed_at:` line", locatorRel),
			"Edit it by hand: in `" + locatorRel + "` on the metadata branch, delete the `claimed_at:` line; then commit and push it. Or, once the setup finding above is resolved, run `docket repository repair` to remove it (it shows the edit first)."},
		{"run repair", loc.RunRepair(" to regenerate it"),
			"Once the setup finding above is resolved, run `docket repository repair` to regenerate it."},
		{"run repair, healthy", RecordLocator{Checkout: ".docket"}.RunRepair(""),
			"Run `docket repository repair`."},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, c.got, c.want)
		}
	}
}

// TestPlainDocumentError pins one parenthetical at most and no repeated name:
// a message that already quotes the name drops it, and otherwise the name
// leads the clause.
func TestPlainDocumentError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"name not in message", &document.Error{Kind: document.KindMarkerImbalance, Name: "artifacts", Line: 12, Msg: "start marker has no matching end"},
			"for `artifacts` on line 12, start marker has no matching end"},
		{"name already quoted", &document.Error{Kind: document.KindMarkerImbalance, Name: "backlink", Line: 3, Msg: "start marker for 'backlink' has no matching end marker"},
			"start marker for 'backlink' has no matching end marker on line 3"},
		{"duplicate field", &document.Error{Kind: document.KindDuplicateField, Name: "id", Line: 3, Msg: "field is declared more than once (first declared on line 2)"},
			"for `id` on line 3, field is declared more than once (first declared on line 2)"},
		{"no line", &document.Error{Kind: document.KindInvalidValue, Name: "id", Msg: "bad key"},
			"for `id`, bad key"},
		{"no name", &document.Error{Kind: document.KindInvalidYAML, Line: 4, Msg: "did not find expected key"},
			"did not find expected key on line 4"},
		{"plain error", errors.New("boom"), "boom"},
	}
	for _, c := range cases {
		if got := PlainDocumentError(c.err); got != c.want {
			t.Errorf("%s: PlainDocumentError = %q, want %q", c.name, got, c.want)
		}
	}
}
