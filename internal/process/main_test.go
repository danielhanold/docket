package process

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// TestMain routes the two re-exec roles of the test binary:
//   - supervisor mode (env var set by Launch) -> RunSupervisorFromEnv
//   - child helper mode (argv marker) -> runTestHelper
//
// go test itself never sets either, so ordinary runs fall through to m.Run.
func TestMain(m *testing.M) {
	if SupervisorRequested() {
		os.Exit(RunSupervisorFromEnv())
	}
	if len(os.Args) > 1 && os.Args[1] == "gate-test-helper" {
		os.Exit(runTestHelper(os.Args[2:]))
	}
	os.Exit(m.Run())
}

// runTestHelper is the purpose-built child command. Modes:
//
//	exit <n>            exit with code n
//	emit <out> <err>    write out to stdout, err to stderr, exit 0
//	sleep               block forever (killed by the test or by stop)
//	ignore-term <path>  ignore SIGTERM, write "ready" to path, block
//	read-stdin          exit 0 iff stdin is at EOF immediately, else 3
func runTestHelper(args []string) int {
	if len(args) == 0 {
		return 90
	}
	switch args[0] {
	case "exit":
		n, _ := strconv.Atoi(args[1])
		return n
	case "emit":
		fmt.Fprint(os.Stdout, args[1])
		fmt.Fprint(os.Stderr, args[2])
		return 0
	case "sleep":
		select {}
	case "ignore-term":
		signal.Ignore(syscall.SIGTERM)
		if err := os.WriteFile(args[1], []byte("ready"), 0o600); err != nil {
			return 91
		}
		select {}
	case "read-stdin":
		buf := make([]byte, 1)
		if n, _ := os.Stdin.Read(buf); n != 0 {
			return 3
		}
		return 0
	}
	return 92
}

// helperArgv builds the child argv for a helper mode.
func helperArgv(t *testing.T, mode string, extra ...string) []string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{exe, "gate-test-helper", mode}, extra...)
}

// waitFor polls fn under a generous outer deadline; correctness rests on the
// state transition, never on the interval.
func waitFor(t *testing.T, what string, deadline time.Duration, fn func() bool) {
	t.Helper()
	end := time.Now().Add(deadline)
	for !fn() {
		if time.Now().After(end) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
