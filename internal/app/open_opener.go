package app

// open_opener.go is the seam through which `docket open` hands a resolved
// target (a URL or a local file path) to the platform's opener. Production
// wiring uses NewSystemOpener with exec.LookPath and RunOpenerProcess; tests
// inject fakes so no real application is ever launched.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// openerWaitDelay bounds how long cmd.Wait keeps draining the opener's stderr
// pipe after the opener exits. xdg-open can hand that pipe to the long-lived
// application it launches; without the bound a pipe-holding grandchild keeps
// cmd.Run blocked until the user closes that application.
const openerWaitDelay = 2 * time.Second

// Opener hands one target, a URL or an absolute file path, to whatever
// application the platform associates with it.
type Opener interface {
	Open(ctx context.Context, target string) error
}

// NoOpenerError reports that the platform has no supported opener, or that its
// opener command is not on PATH. The caller can still print the target.
type NoOpenerError struct {
	GOOS string
}

// Error renders the refusal with its remedy.
func (e *NoOpenerError) Error() string {
	return "no opener available on " + e.GOOS + " — open it yourself, or use --print"
}

// openerCommand names the opener command for goos: "open" on darwin,
// "xdg-open" on linux, and "" where docket supports none.
func openerCommand(goos string) string {
	switch goos {
	case "darwin":
		return "open"
	case "linux":
		return "xdg-open"
	default:
		return ""
	}
}

// systemOpener resolves the platform opener through lookPath and launches it
// through run.
type systemOpener struct {
	goos     string
	lookPath func(string) (string, error)
	run      func(ctx context.Context, bin, target string) error
}

// NewSystemOpener returns the Opener for goos. lookPath resolves the opener
// command (exec.LookPath in production) and run launches it
// (RunOpenerProcess in production).
func NewSystemOpener(goos string, lookPath func(string) (string, error), run func(ctx context.Context, bin, target string) error) Opener {
	return systemOpener{goos: goos, lookPath: lookPath, run: run}
}

// Open launches the platform opener on target, or returns *NoOpenerError
// without launching anything when the platform has none or it is not on PATH.
func (o systemOpener) Open(ctx context.Context, target string) error {
	name := openerCommand(o.goos)
	if name == "" {
		return &NoOpenerError{GOOS: o.goos}
	}
	bin, err := o.lookPath(name)
	if err != nil {
		return &NoOpenerError{GOOS: o.goos}
	}
	return o.run(ctx, bin, target)
}

// RunOpenerProcess runs bin with target as its only argument (no shell) and
// waits for it to exit, bounding the stderr drain by openerWaitDelay.
// Stdin/stdout stay unset (the null device), so opener chatter never
// reaches docket's protocol stdout; a non-zero exit carries the opener's stderr.
func RunOpenerProcess(ctx context.Context, bin, target string) error {
	cmd := exec.CommandContext(ctx, bin, target)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.WaitDelay = openerWaitDelay
	err := cmd.Run()
	// ErrWaitDelay means the opener exited 0 and only a grandchild still held
	// stderr: the open itself succeeded.
	if errors.Is(err, exec.ErrWaitDelay) {
		return nil
	}
	if err != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return fmt.Errorf("%s: %w: %s", filepath.Base(bin), err, detail)
		}
		return fmt.Errorf("%s: %w", filepath.Base(bin), err)
	}
	return nil
}
