package codexentry

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestHelperAppServerProcess is the fake app-server the process-transport tests
// start. It does nothing unless DOCKET_CODEXENTRY_TEST_SERVER names a mode.
func TestHelperAppServerProcess(t *testing.T) {
	switch os.Getenv("DOCKET_CODEXENTRY_TEST_SERVER") {
	case "":
		return
	case "stdout-closes-first":
		// Stdout reaches EOF before the server writes its error, so a client that
		// reads stderr the moment stdout ends sees nothing.
		_ = os.Stdout.Close()
		time.Sleep(100 * time.Millisecond)
		fmt.Fprint(os.Stderr, "app-server boom: config unreadable\n")
		os.Exit(7)
	case "linger":
		// Stdout ends but the process keeps running: a bounded wait must not hang.
		_ = os.Stdout.Close()
		fmt.Fprint(os.Stderr, "still running\n")
		time.Sleep(30 * time.Second)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown helper mode %q\n", os.Getenv("DOCKET_CODEXENTRY_TEST_SERVER"))
		os.Exit(2)
	}
}

func startHelperAppServer(t *testing.T, mode string) Transport {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperAppServerProcess$")
	cmd.Env = append(os.Environ(), "DOCKET_CODEXENTRY_TEST_SERVER="+mode)
	tr, err := startAppServerCommand(cmd, testsupport.TempDir(t))
	if err != nil {
		t.Fatalf("starting helper app-server (%s): %v", mode, err)
	}
	return tr
}

func TestRecvReportsStderrOfServerThatClosedStdoutFirst(t *testing.T) {
	tr := startHelperAppServer(t, "stdout-closes-first")
	_, err := tr.Recv()
	if err == nil {
		t.Fatal("Recv returned no error from a server that closed stdout")
	}
	for _, want := range []string{"app-server boom: config unreadable", "exit status 7"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Recv error %q does not contain %q", err, want)
		}
	}
	_ = tr.Close()
}

func TestRecvAndCloseDoNotHangOnLiveServer(t *testing.T) {
	old := recvStderrGrace
	recvStderrGrace = 200 * time.Millisecond
	t.Cleanup(func() { recvStderrGrace = old })

	tr := startHelperAppServer(t, "linger")

	recvErr := make(chan error, 1)
	go func() {
		_, err := tr.Recv()
		recvErr <- err
	}()
	select {
	case err := <-recvErr:
		if err == nil {
			t.Fatal("Recv returned no error from a server that closed stdout")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Recv hung on a server that closed stdout but is still running")
	}

	closeErr := make(chan error, 1)
	go func() { closeErr <- tr.Close() }()
	var first error
	select {
	case first = <-closeErr:
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung after a timed-out Recv wait")
	}

	go func() { closeErr <- tr.Close() }()
	select {
	case second := <-closeErr:
		if second != first {
			t.Fatalf("second Close returned %v, want the first Close's %v", second, first)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second Close hung")
	}
}
