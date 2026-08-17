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
//	emit-exit <out> <err> <n>
//	                    write out to stdout, err to stderr, exit n
//	sleep               block forever (killed by the test or by stop)
//	ignore-term <path>  ignore SIGTERM, write "ready" to path, block
//	read-stdin          exit 0 iff stdin is at EOF immediately, else 3
//	env-check           exit 0 iff both private supervisor env vars are unset
//	                    and the inherited lock fd 3 is closed (CLOEXEC held)
//	launch <root>       run svc.Launch(sleep) against <root>, print the run dir,
//	                    then exit — the separate launcher process for the gate
//	                    survival proof
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
	case "emit-exit":
		fmt.Fprint(os.Stdout, args[1])
		fmt.Fprint(os.Stderr, args[2])
		n, _ := strconv.Atoi(args[3])
		return n
	case "sleep":
		// Block effectively forever with DEFAULT signal disposition: the child
		// must die BY a group-directed SIGTERM (kind=signal, signal=15), the way
		// a real supervised command does — never catch it and exit cleanly, which
		// would fabricate an exit terminal record. A bare select{} would trip
		// Go's all-goroutines-asleep deadlock detector and exit within
		// milliseconds; a parked timer is not a deadlock (the runtime sees a
		// pending timer), so time.Sleep keeps the runtime alive without
		// registering any signal handler. Teardown (SIGKILL) and stop (SIGTERM)
		// both reach it through its group.
		time.Sleep(time.Hour)
		return 0
	case "ignore-term":
		signal.Ignore(syscall.SIGTERM)
		if err := os.WriteFile(args[1], []byte("ready"), 0o600); err != nil {
			return 91
		}
		// Block with TERM ignored so a group-directed SIGTERM cannot end this
		// child — only the unblockable SIGKILL of stop's escalation can. A bare
		// select{} would trip Go's all-goroutines-asleep deadlock detector and
		// exit within milliseconds (fabricating an exit=2 terminal record before
		// escalation ever runs); a parked timer is not a deadlock, so time.Sleep
		// keeps the runtime alive without registering any TERM handler, exactly
		// as the "sleep" mode does.
		time.Sleep(time.Hour)
		return 0
	case "read-stdin":
		buf := make([]byte, 1)
		if n, _ := os.Stdin.Read(buf); n != 0 {
			return 3
		}
		return 0
	case "env-check":
		if os.Getenv("DOCKET_GATE_SUPERVISOR_RUN_DIR") != "" ||
			os.Getenv("DOCKET_GATE_SUPERVISOR_ARGV") != "" {
			return 4
		}
		if _, err := syscall.Getpgid(0); err != nil {
			return 93 // sanity: a live process always has a group
		}
		if _, serr := os.NewFile(3, "probe").Stat(); serr == nil {
			return 5 // the inherited lock fd leaked past CLOEXEC into the child
		}
		return 0
	case "launch":
		exe, err := os.Executable()
		if err != nil {
			return 94
		}
		svc, err := NewService(exe)
		if err != nil {
			return 95
		}
		out, err := svc.Launch(LaunchRequest{
			Root: args[1],
			Cwd:  os.TempDir(),
			Argv: []string{exe, "gate-test-helper", "sleep"},
		})
		if err != nil {
			return 96
		}
		fmt.Fprintln(os.Stdout, out.RunDir)
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
