package codexentry

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// recordingRegistrar records the handle it was registered with and (via onRegister)
// lets a test observe WHEN registration happened relative to the sent frames.
type recordingRegistrar struct {
	handle     string
	calls      int
	onRegister func()
	err        error
}

func (r *recordingRegistrar) RegisterParticipant(h string) error {
	r.handle = h
	r.calls++
	if r.onRegister != nil {
		r.onRegister()
	}
	return r.err
}

// recordingCanceller records every CancelRun invocation.
type recordingCanceller struct {
	mu     sync.Mutex
	calls  int
	reason string
}

func (c *recordingCanceller) CancelRun(reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.reason = reason
	return nil
}

func (c *recordingCanceller) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// TestAgentEnterRegistersLifecycleParticipant proves the entry registers its native
// task handle (the thread id) as a participant BEFORE the coordinator turn starts,
// and that a registration failure aborts the entry before any turn is sent.
func TestAgentEnterRegistersLifecycleParticipant(t *testing.T) {
	base := []string{
		`{"jsonrpc":"2.0","id":1,"result":{}}`,
		`{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"root-thread"}}}`,
		`{"jsonrpc":"2.0","id":3,"result":{"turn":{"id":"root-turn"}}}`,
		`{"method":"item/completed","params":{"threadId":"root-thread","turnId":"root-turn","item":{"type":"agentMessage","phase":"final_answer","text":"OK"}}}`,
		`{"method":"turn/completed","params":{"threadId":"root-thread","turn":{"id":"root-turn","status":"completed","items":[]}}}`,
	}

	t.Run("registers before turn/start", func(t *testing.T) {
		tr := &scriptedTransport{recv: append([]string(nil), base...)}
		reg := &recordingRegistrar{}
		sentAtRegistration := -1
		reg.onRegister = func() { sentAtRegistration = len(tr.sent) }
		c := Client{Start: func(context.Context, string) (Transport, error) { return tr, nil }, Registrar: reg}
		got, err := c.Enter(context.Background(), validRequest())
		if err != nil {
			t.Fatalf("Enter: %v", err)
		}
		if got.Output != "OK" {
			t.Fatalf("output = %q", got.Output)
		}
		if reg.handle != "root-thread" {
			t.Fatalf("registered handle = %q, want root-thread", reg.handle)
		}
		// initialize, initialized, thread/start are sent; turn/start is NOT yet.
		if sentAtRegistration != 3 {
			t.Fatalf("registration observed %d sent frames, want 3 (before turn/start)", sentAtRegistration)
		}
		if len(tr.sent) != 4 || tr.sent[3]["method"] != "turn/start" {
			t.Fatalf("turn/start not the fourth frame: %#v", tr.sent)
		}
	})

	t.Run("registration failure aborts before turn/start", func(t *testing.T) {
		tr := &scriptedTransport{recv: append([]string(nil), base...)}
		reg := &recordingRegistrar{err: io.ErrClosedPipe}
		c := Client{Start: func(context.Context, string) (Transport, error) { return tr, nil }, Registrar: reg}
		_, err := c.Enter(context.Background(), validRequest())
		if err == nil || !strings.Contains(err.Error(), "registering lifecycle participant") {
			t.Fatalf("error = %v, want registering lifecycle participant", err)
		}
		// Only initialize, initialized, thread/start were sent — never turn/start.
		if len(tr.sent) != 3 {
			t.Fatalf("sent %d frames, want 3 (turn never started): %#v", len(tr.sent), tr.sent)
		}
	})
}

// TestToolCallTimeoutWithLiveOwnerPreservesGate proves that with a Canceller wired
// but NO Stop signal delivered, an ordinary (even slow) turn completes and the run
// is never cancelled — a tool-call timeout with a live owner is not a Stop.
func TestToolCallTimeoutWithLiveOwnerPreservesGate(t *testing.T) {
	tr := &scriptedTransport{recv: []string{
		`{"jsonrpc":"2.0","id":1,"result":{}}`,
		`{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"t"}}}`,
		`{"jsonrpc":"2.0","id":3,"result":{"turn":{"id":"u"}}}`,
		`{"method":"item/completed","params":{"threadId":"t","turnId":"u","item":{"type":"agentMessage","phase":"final_answer","text":"DONE"}}}`,
		`{"method":"turn/completed","params":{"threadId":"t","turn":{"id":"u","status":"completed","items":[]}}}`,
	}}
	canceller := &recordingCanceller{}
	// A signal source that never fires: the owner is alive and quiet.
	quiet := make(chan os.Signal)
	c := Client{
		Start:        func(context.Context, string) (Transport, error) { return tr, nil },
		Canceller:    canceller,
		signalSource: func() (<-chan os.Signal, func()) { return quiet, func() {} },
	}
	got, err := c.Enter(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Enter: %v", err)
	}
	if got.Output != "DONE" {
		t.Fatalf("output = %q", got.Output)
	}
	if canceller.count() != 0 {
		t.Fatalf("CancelRun called %d times with a live, quiet owner; want 0", canceller.count())
	}
}

// gatedTransport delivers pre-frames immediately, then BLOCKS the next Recv until
// release is closed, then delivers post-frames, then EOF. It lets a test place a
// Stop signal precisely between the turn's start and its completion.
type gatedTransport struct {
	pre     []string
	post    []string
	release chan struct{}
	mu      sync.Mutex
	i       int
	closed  bool
}

func (g *gatedTransport) Send(any) error { return nil }

func (g *gatedTransport) Recv() (json.RawMessage, error) {
	g.mu.Lock()
	if g.i < len(g.pre) {
		raw := g.pre[g.i]
		g.i++
		g.mu.Unlock()
		return json.RawMessage(raw), nil
	}
	g.mu.Unlock()
	// Pre-frames drained: block until the test releases the turn's completion.
	<-g.release
	g.mu.Lock()
	defer g.mu.Unlock()
	j := g.i - len(g.pre)
	if j < len(g.post) {
		raw := g.post[j]
		g.i++
		return json.RawMessage(raw), nil
	}
	return nil, io.EOF
}

func (g *gatedTransport) Close() error { g.closed = true; return nil }

// TestSignalCancelInvokesCancellerAndKeepsObserving proves a delivered Stop signal
// invokes the run's cancellation path exactly once and then the wait KEEPS observing
// the turn until it produces terminal output — the transport is not the receipt.
func TestSignalCancelInvokesCancellerAndKeepsObserving(t *testing.T) {
	g := &gatedTransport{
		pre: []string{
			`{"jsonrpc":"2.0","id":1,"result":{}}`,
			`{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"t"}}}`,
			`{"jsonrpc":"2.0","id":3,"result":{"turn":{"id":"u"}}}`,
		},
		post: []string{
			`{"method":"item/completed","params":{"threadId":"t","turnId":"u","item":{"type":"agentMessage","phase":"final_answer","text":"FINAL"}}}`,
			`{"method":"turn/completed","params":{"threadId":"t","turn":{"id":"u","status":"completed","items":[]}}}`,
		},
		release: make(chan struct{}),
	}
	sig := make(chan os.Signal, 1)
	canceller := &recordingCanceller{}
	c := Client{
		Start:        func(context.Context, string) (Transport, error) { return g, nil },
		Canceller:    canceller,
		signalSource: func() (<-chan os.Signal, func()) { return sig, func() {} },
	}

	type outcome struct {
		out Result
		err error
	}
	res := make(chan outcome, 1)
	go func() {
		out, err := c.Enter(context.Background(), validRequest())
		res <- outcome{out, err}
	}()

	// Deliver the Stop while the turn is mid-flight (the transport is blocked before
	// the completion frames).
	deadline := time.Now().Add(2 * time.Second)
	sig <- syscall.SIGTERM
	for canceller.count() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("CancelRun was not invoked after the Stop signal")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Now let the turn produce its terminal output. Observation must continue past
	// the cancel and return the final message.
	close(g.release)
	select {
	case o := <-res:
		if o.err != nil {
			t.Fatalf("Enter after cancel: %v", o.err)
		}
		if o.out.Output != "FINAL" {
			t.Fatalf("output = %q, want FINAL (observed to completion)", o.out.Output)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Enter did not return after the turn completed")
	}
	if canceller.count() != 1 {
		t.Fatalf("CancelRun called %d times, want exactly 1", canceller.count())
	}
	if !strings.Contains(canceller.reason, "cancelling the run") {
		t.Fatalf("cancel reason = %q", canceller.reason)
	}
}
