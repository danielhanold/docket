package codexentry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// recvStderrGrace is the longest Recv waits for the app-server process to exit
// after a read or decode error, so the server's stderr can finish copying.
var recvStderrGrace = 2 * time.Second

// StartAppServer launches Codex's native app-server transport directly. The
// argv is closed and contains neither a shell nor `codex exec`.
func StartAppServer(ctx context.Context, cwd string) (Transport, error) {
	return startAppServerCommand(exec.CommandContext(ctx, "codex", "app-server", "--stdio"), cwd)
}

// startAppServerCommand wires and starts an app-server command: stdin and
// stdout carry the JSON-RPC stream, and stderr is kept (bounded) so a read
// error can carry the server's last words.
func startAppServerCommand(cmd *exec.Cmd, cwd string) (Transport, error) {
	cmd.Dir = cwd
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &boundedBuffer{limit: 16 * 1024}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &processTransport{cmd: cmd, stdin: stdin, decoder: json.NewDecoder(bufio.NewReader(stdout)), stderr: stderr}, nil
}

type processTransport struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	decoder *json.Decoder
	stderr  *boundedBuffer

	// The process is waited exactly once, by the goroutine wait starts. waitErr
	// is written before waitDone closes, so it is read only after a receive
	// from waitDone.
	waitOnce sync.Once
	waitDone chan struct{}
	waitErr  error

	closeOnce sync.Once
}

func (p *processTransport) Send(v any) error {
	return json.NewEncoder(p.stdin).Encode(v)
}

// wait starts cmd.Wait on first use and returns a channel that closes once it
// has returned. cmd.Wait also finishes copying the process's stderr into
// p.stderr, so stderr is complete once the channel closes.
func (p *processTransport) wait() <-chan struct{} {
	p.waitOnce.Do(func() {
		p.waitDone = make(chan struct{})
		go func() {
			p.waitErr = p.cmd.Wait()
			close(p.waitDone)
		}()
	})
	return p.waitDone
}

// Recv decodes the next frame. On a read or decode error it waits up to
// recvStderrGrace for the process to exit, so a crashing server's stderr has
// finished copying, and adds that stderr (and the exit status, when the process
// has exited) to the error. A server that closed stdout but keeps running only
// delays the error by the grace period.
func (p *processTransport) Recv() (json.RawMessage, error) {
	var raw json.RawMessage
	if err := p.decoder.Decode(&raw); err != nil {
		exited := false
		var exitErr error
		select {
		case <-p.wait():
			exited, exitErr = true, p.waitErr
		case <-time.After(recvStderrGrace):
		}
		detail := p.stderr.String()
		switch {
		case exited && exitErr != nil && detail != "":
			return nil, fmt.Errorf("%w (app-server exited: %v; stderr: %s)", err, exitErr, detail)
		case exited && exitErr != nil:
			return nil, fmt.Errorf("%w (app-server exited: %v)", err, exitErr)
		case detail != "":
			return nil, fmt.Errorf("%w (app-server stderr: %s)", err, detail)
		}
		return nil, err
	}
	return raw, nil
}

// Close closes stdin, kills the process if it is still running, and returns
// the process's wait result. It is safe to call more than once and after a Recv
// whose bounded wait timed out: every call returns the same value, and the
// process is waited only once.
func (p *processTransport) Close() error {
	p.closeOnce.Do(func() {
		_ = p.stdin.Close()
		select {
		case <-p.wait():
		default:
			if p.cmd.Process != nil {
				_ = p.cmd.Process.Kill()
			}
		}
	})
	<-p.wait()
	return p.waitErr
}

type boundedBuffer struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	written := len(p)
	space := b.limit - len(b.buf)
	if space > 0 {
		if len(p) > space {
			p = p[:space]
		}
		b.buf = append(b.buf, p...)
	}
	return written, nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}
