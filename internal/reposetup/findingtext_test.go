package reposetup

import (
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/document"
)

const (
	locatorTail  = ", then run `docket repository repair` so the board and link blocks catch up."
	locatorInstr = "remove `172` from `depends_on`"
	locatorRel   = "docs/changes/active/0263-x.md"
)

func TestRecordLocatorHandEditThenRepairPerLayout(t *testing.T) {
	const p = "/home/u/.local/share/dckt/o/r/checkouts/r-1"
	cases := []struct {
		name string
		loc  RecordLocator
		want string
	}{
		{"shared", RecordLocator{Checkout: ".docket"},
			"Remove `172` from `depends_on` in `.docket/" + locatorRel + "`, commit and push it from `.docket`" + locatorTail},
		{"private path", RecordLocator{Checkout: p},
			"Remove `172` from `depends_on` in `" + p + "/" + locatorRel + "`, commit and push it from `" + p + "`" + locatorTail},
		{"private words", RecordLocator{Label: "your private metadata checkout"},
			"Remove `172` from `depends_on` in `" + locatorRel + "` in your private metadata checkout, commit and push it from your private metadata checkout" + locatorTail},
		{"zero", RecordLocator{},
			"Remove `172` from `depends_on` in `" + locatorRel + "` on the metadata branch, commit and push it" + locatorTail},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.loc.HandEditThenRepair(locatorInstr, locatorRel); got != c.want {
				t.Errorf("HandEditThenRepair:\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestRecordLocatorEmptyRelNamesNoFile(t *testing.T) {
	got := RecordLocator{Checkout: ".docket"}.HandEditThenRepair(locatorInstr, "")
	if strings.Contains(got, "``") || strings.Contains(got, " in `") {
		t.Errorf("empty rel named a file: %q", got)
	}
	if !strings.Contains(got, "commit and push it from `.docket`") {
		t.Errorf("empty rel lost the commit clause: %q", got)
	}
}

func TestRecordLocatorRepairOrHandEdit(t *testing.T) {
	got := RecordLocator{Checkout: ".docket"}.RepairOrHandEdit("remove it", "delete the `claimed_at:` line", locatorRel)
	want := "Run `docket repository repair` to remove it (it shows the edit first), or delete the `claimed_at:` line in `.docket/" + locatorRel + "` by hand and commit and push it from `.docket`."
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
			"Remove `172` from `depends_on` in `" + locatorRel + "` on the metadata branch, commit and push it. Once the setup finding above is resolved, run `docket repository repair` so the board and link blocks catch up."},
		{"repair or hand edit", loc.RepairOrHandEdit("remove it", "delete the `claimed_at:` line", locatorRel),
			"Delete the `claimed_at:` line in `" + locatorRel + "` on the metadata branch by hand and commit and push it, or, once the setup finding above is resolved, run `docket repository repair` to remove it (it shows the edit first)."},
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

func TestPlainDocumentError(t *testing.T) {
	err := &document.Error{Kind: document.KindMarkerImbalance, Name: "artifacts", Line: 12, Msg: "start marker has no matching end"}
	if got, want := PlainDocumentError(err), "start marker has no matching end (`artifacts`) on line 12"; got != want {
		t.Errorf("PlainDocumentError(document.Error) = %q, want %q", got, want)
	}
	if got := PlainDocumentError(errors.New("boom")); got != "boom" {
		t.Errorf("PlainDocumentError(plain) = %q, want %q", got, "boom")
	}
}
