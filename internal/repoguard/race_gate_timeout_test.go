package repoguard

// Change 0465: tests/test_go_race.sh passes an explicit -timeout backstop (below Go's
// 10m per-package default) and turns an overrun into a named, readable NOT OK line
// instead of a bare goroutine-dump panic. These are behavioral tests over a COPY of
// the real wrapper (read at test time, so nothing frozen can drift) with a fake `go`
// on PATH that logs its argv. Same pattern and helpers as gofmt_toolchain_test.go
// (writeToolScript, readLog). The asserts pin the mechanism (the argv go test
// actually received) and also prove the gate is not weakened (-race, -count=1, ./...).

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

const fakeRaceGoScript = `printf 'argv:[%s]\n' "$*" >>"$GO_FAKE_LOG"
if [ "$1" = test ] && [ -n "${FAKE_GO_TIMEOUT:-}" ]; then
  printf 'panic: test timed out after 4m0s\n\ngoroutine 1 [running]:\nFAIL\tfixture/slow\t240.012s\nFAIL\n'
  exit 1
fi
exit 0
`

type raceGateFixture struct {
	root, wrapper, goLog string
	env                  []string
}

func newRaceGateFixture(t *testing.T) *raceGateFixture {
	t.Helper()
	repoRoot, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	realWrapper, err := os.ReadFile(filepath.Join(repoRoot, "tests", "test_go_race.sh"))
	if err != nil {
		t.Fatal(err)
	}
	base := testsupport.TempDir(t)
	f := &raceGateFixture{root: filepath.Join(base, "fixture"), goLog: filepath.Join(base, "go.log")}
	f.wrapper = filepath.Join(f.root, "tests", "test_go_race.sh")
	if err := os.MkdirAll(filepath.Dir(f.wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.wrapper, realWrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.wrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(base, "fakebin")
	writeToolScript(t, filepath.Join(fakeBin, "go"), fakeRaceGoScript)
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch key {
		case "PATH", "GOMODCACHE", "GOCACHE", "GOFLAGS", "GOMAXPROCS",
			"DOCKET_GO_TEST_CONCURRENCY", "GO_FAKE_LOG", "FAKE_GO_TIMEOUT":
			continue
		}
		f.env = append(f.env, kv)
	}
	// GOMODCACHE/GOCACHE pre-set so the wrapper's cache block never calls git.
	f.env = append(f.env,
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOMODCACHE="+filepath.Join(base, "gomodcache"),
		"GOCACHE="+filepath.Join(base, "gocache"),
		"GO_FAKE_LOG="+f.goLog,
	)
	return f
}

func (f *raceGateFixture) run(t *testing.T) (string, int) {
	t.Helper()
	cmd := exec.Command("bash", f.wrapper)
	cmd.Dir = f.root
	cmd.Env = f.env
	b, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return string(b), 0
	case errors.As(err, &ee):
		return string(b), ee.ExitCode()
	default:
		t.Fatalf("running the wrapper copy: %v\n%s", err, b)
		return "", -1
	}
}

// raceTestArgv returns the argv of the fake `go test` invocation.
func raceTestArgv(t *testing.T, log string) []string {
	t.Helper()
	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, "argv:[test ") {
			return strings.Fields(strings.TrimSuffix(strings.TrimPrefix(line, "argv:["), "]"))
		}
	}
	t.Fatalf("the fake go never received a `go test` invocation; log:\n%s", log)
	return nil
}

func TestRaceGatePassesTimeoutBackstopBelowGoDefault(t *testing.T) {
	f := newRaceGateFixture(t)
	out, code := f.run(t)
	if code != 0 {
		t.Fatalf("a green fake run must exit 0, got %d:\n%s", code, out)
	}
	argv := raceTestArgv(t, readLog(t, f.goLog))
	var timeout time.Duration
	found := false
	for i, a := range argv {
		val := ""
		switch {
		case a == "-timeout" && i+1 < len(argv):
			val = argv[i+1]
		case strings.HasPrefix(a, "-timeout="):
			val = strings.TrimPrefix(a, "-timeout=")
		default:
			continue
		}
		d, err := time.ParseDuration(val)
		if err != nil {
			t.Fatalf("-timeout value %q does not parse as a duration: %v", val, err)
		}
		timeout, found = d, true
	}
	if !found {
		t.Fatalf("go test -race must carry an explicit -timeout backstop; argv %q", argv)
	}
	if timeout <= 0 || timeout >= 10*time.Minute {
		t.Fatalf("the backstop %s must be positive and below Go's 10m default", timeout)
	}
	for _, want := range []string{"-race", "-count=1", "./..."} {
		if !slices.Contains(argv, want) {
			t.Fatalf("the race gate must not be weakened: argv %q lacks %q", argv, want)
		}
	}
}

var raceBackstopMarker = regexp.MustCompile(`(?m)^NOT OK - no package ran past the \S+ -timeout backstop$`)

func TestRaceGateNamesTimeoutBackstopOnOverrun(t *testing.T) {
	f := newRaceGateFixture(t)
	f.env = append(f.env, "FAKE_GO_TIMEOUT=1")
	out, code := f.run(t)
	if code == 0 {
		t.Fatalf("an overrun must fail the gate:\n%s", out)
	}
	if !raceBackstopMarker.MatchString(out) {
		t.Fatalf("an overrun must print the named backstop marker, got:\n%s", out)
	}
	for _, want := range []string{"FAIL\tfixture/slow", "PARTITION AND LANE"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the overrun diagnostic must contain %q, got:\n%s", want, out)
		}
	}
}
