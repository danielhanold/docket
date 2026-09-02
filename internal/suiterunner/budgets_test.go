package suiterunner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func TestLoadBudgetsParsesOracleFormat(t *testing.T) {
	dir := testsupport.TempDir(t)
	path := filepath.Join(dir, "runtime-budgets.tsv")
	// A comment line, a valid row, a malformed-seconds row, an unknown-mode row,
	// and a final row WITHOUT a trailing newline (must still parse).
	content := "# a comment\n" +
		"\n" +
		"tests/test_a.sh\t20\tserial\n" +
		"tests/test_b.sh\tabc\tparallel\n" +
		"tests/test_c.sh\t30\tsideways\n" +
		"tests/test_d.sh\t45\tserial"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write budgets: %v", err)
	}

	budgets, err := LoadBudgets(path)
	if err != nil {
		t.Fatalf("LoadBudgets: %v", err)
	}

	cases := map[string]budgetRow{
		"test_a.sh": {Ceiling: 20, Mode: ModeSerial},
		"test_b.sh": {Ceiling: DefaultCeiling, Mode: ModeParallel}, // malformed seconds -> default
		"test_c.sh": {Ceiling: 30, Mode: ModeParallel},             // unknown mode -> parallel
		"test_d.sh": {Ceiling: 45, Mode: ModeSerial},               // no trailing newline
	}
	if len(budgets) != len(cases) {
		t.Fatalf("LoadBudgets returned %d rows, want %d: %+v", len(budgets), len(cases), budgets)
	}
	for base, want := range cases {
		got, ok := budgets[base]
		if !ok {
			t.Fatalf("LoadBudgets missing key %q; got %+v", base, budgets)
		}
		if got != want {
			t.Fatalf("LoadBudgets[%q] = %+v, want %+v", base, got, want)
		}
	}
}

func TestLoadBudgetsMissingFileIsEmpty(t *testing.T) {
	budgets, err := LoadBudgets(filepath.Join(testsupport.TempDir(t), "nope.tsv"))
	if err != nil {
		t.Fatalf("LoadBudgets on a missing file returned error %v; want empty map, nil error", err)
	}
	if len(budgets) != 0 {
		t.Fatalf("LoadBudgets on a missing file returned %+v; want empty map", budgets)
	}
}

func TestThresholds(t *testing.T) {
	screenCases := []struct {
		secs, ceil int
		want       bool
	}{
		{150, 60, false}, // 150*2 == 60*5, not strictly greater
		{151, 60, true},
	}
	for _, c := range screenCases {
		if got := ScreenOver(c.secs, c.ceil); got != c.want {
			t.Fatalf("ScreenOver(%d,%d) = %v, want %v", c.secs, c.ceil, got, c.want)
		}
	}

	soloCases := []struct {
		secs, ceil int
		want       bool
	}{
		{90, 60, false}, // 90*2 == 60*3
		{91, 60, true},
	}
	for _, c := range soloCases {
		if got := SoloOver(c.secs, c.ceil); got != c.want {
			t.Fatalf("SoloOver(%d,%d) = %v, want %v", c.secs, c.ceil, got, c.want)
		}
	}

	thresholdCases := []struct {
		ceil int
		want string
	}{
		{60, "90"},
		{15, "22.5"},
		{10, "15"},
	}
	for _, c := range thresholdCases {
		if got := SoloThreshold(c.ceil); got != c.want {
			t.Fatalf("SoloThreshold(%d) = %q, want %q", c.ceil, got, c.want)
		}
	}
}
