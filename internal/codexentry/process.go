package codexentry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// StartAppServer launches Codex's native app-server transport directly. The
// argv is closed and contains neither a shell nor `codex exec`.
func StartAppServer(ctx context.Context, cwd string) (Transport, error) {
	cmd := exec.CommandContext(ctx, "codex", "app-server", "--stdio")
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
	once    sync.Once
	err     error
}

func (p *processTransport) Send(v any) error {
	return json.NewEncoder(p.stdin).Encode(v)
}

func (p *processTransport) Recv() (json.RawMessage, error) {
	var raw json.RawMessage
	if err := p.decoder.Decode(&raw); err != nil {
		if detail := p.stderr.String(); detail != "" {
			return nil, fmt.Errorf("%w (app-server stderr: %s)", err, detail)
		}
		return nil, err
	}
	return raw, nil
}

func (p *processTransport) Close() error {
	p.once.Do(func() {
		_ = p.stdin.Close()
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		p.err = p.cmd.Wait()
	})
	return p.err
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
