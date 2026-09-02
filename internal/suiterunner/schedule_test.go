package suiterunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// serialTarget writes a serial-mode fixture and returns its Target.
func serialTarget(t *testing.T, dir, name, body string) Target {
	tgt := writeScript(t, dir, name, body)
	tgt.Mode = ModeSerial
	return tgt
}

// collectResults gathers onDone callbacks into a base->rc map under a mutex,
// since parallel-lane callbacks fire concurrently.
type resultSink struct {
	mu   sync.Mutex
	byRC map[string]int
}

func newResultSink() *resultSink { return &resultSink{byRC: make(map[string]int)} }

func (s *resultSink) onDone(t Target, r Result) {
	s.mu.Lock()
	s.byRC[t.Base] = r.RC
	s.mu.Unlock()
}

func TestSchedulePartitionAndOrder(t *testing.T) {
	// Parallel targets with distinct ceilings 10/60/30, two ceiling-60 ties to
	// exercise the path tiebreak, plus one serial target.
	targets := []Target{
		{Path: "z/test_c10.sh", Base: "test_c10.sh", Ceiling: 10, Mode: ModeParallel},
		{Path: "z/test_c60b.sh", Base: "test_c60b.sh", Ceiling: 60, Mode: ModeParallel},
		{Path: "z/test_c30.sh", Base: "test_c30.sh", Ceiling: 30, Mode: ModeParallel},
		{Path: "a/test_c60a.sh", Base: "test_c60a.sh", Ceiling: 60, Mode: ModeParallel},
		{Path: "z/test_s.sh", Base: "test_s.sh", Ceiling: 20, Mode: ModeSerial},
	}
	par, ser := Schedule(targets)

	// Parallel lane: ceiling-descending, ties broken by path-ascending.
	wantPar := []string{"a/test_c60a.sh", "z/test_c60b.sh", "z/test_c30.sh", "z/test_c10.sh"}
	var gotPar []string
	for _, tg := range par {
		gotPar = append(gotPar, tg.Path)
	}
	if fmt.Sprint(gotPar) != fmt.Sprint(wantPar) {
		t.Fatalf("parallel order = %v, want %v", gotPar, wantPar)
	}

	// Serial lane: discovery (input) order preserved.
	if len(ser) != 1 || ser[0].Base != "test_s.sh" {
		t.Fatalf("serial lane = %v, want exactly [test_s.sh]", ser)
	}
}

// TestSerialTargetsNeverOverlap is the concurrency-safety mutation guard (spec
// mutation "schedule an unsafe target concurrently"). Each serial-mode fixture
// grabs an exclusive mkdir lock, sleeps, then releases it. Run sequentially
// (the serial lane), the lock is always free when the next target grabs it and
// every target exits 0. Mutating the scheduler to run serial targets
// concurrently makes the second mkdir fail on the held lock -> NOT OK -> rc 1.
func TestSerialTargetsNeverOverlap(t *testing.T) {
	scripts := testsupport.TempDir(t)
	work := testsupport.TempDir(t)
	overlap := testsupport.TempDir(t)

	body := "" +
		"mkdir \"$OVERLAP_DIR/lock\" || { echo 'NOT OK - overlap'; exit 1; }\n" +
		"sleep 0.2\n" +
		"rmdir \"$OVERLAP_DIR/lock\"\n" +
		"echo 'ok - exclusive'\n"

	var targets []Target
	for _, n := range []string{"s1", "s2", "s3"} {
		targets = append(targets, serialTarget(t, scripts, n, body))
	}
	par, ser := Schedule(targets)
	if len(par) != 0 || len(ser) != 3 {
		t.Fatalf("partition = par:%d ser:%d, want par:0 ser:3", len(par), len(ser))
	}

	sink := newResultSink()
	cfg := Config{Bash: bashPath(t), Jobs: 4, Work: work, ExtraEnv: []string{"OVERLAP_DIR=" + overlap}}
	unlaunched := runLanes(context.Background(), cfg, par, ser, newProcRegistry(), sink.onDone)
	if len(unlaunched) != 0 {
		t.Fatalf("unlaunched = %v, want none", unlaunched)
	}
	for _, tg := range targets {
		if rc := sink.byRC[tg.Base]; rc != 0 {
			t.Fatalf("serial target %s rc = %d, want 0 (overlap detected)", tg.Base, rc)
		}
	}
}

// TestParallelBoundIsRespected proves at most Jobs targets run concurrently.
// Each parallel fixture creates its own slot dir, counts live slots, fails if
// the count exceeds Jobs, holds the slot briefly, then removes it. With Jobs=2
// and 6 targets the live count never exceeds 2. A mutation that ignores the
// semaphore bound launches all 6 at once -> count > 2 -> NOT OK -> rc 1.
func TestParallelBoundIsRespected(t *testing.T) {
	scripts := testsupport.TempDir(t)
	work := testsupport.TempDir(t)
	slots := testsupport.TempDir(t)

	var targets []Target
	for i := 0; i < 6; i++ {
		slot := fmt.Sprintf("slot.%d", i)
		body := "" +
			"mkdir \"$SLOT_DIR/" + slot + "\" || { echo 'NOT OK - mkdir'; exit 1; }\n" +
			"set -- \"$SLOT_DIR\"/slot.*\n" +
			"n=$#\n" +
			"if [ \"$n\" -gt \"$MAX_JOBS\" ]; then echo \"NOT OK - bound $n > $MAX_JOBS\"; rmdir \"$SLOT_DIR/" + slot + "\"; exit 1; fi\n" +
			"sleep 0.1\n" +
			"rmdir \"$SLOT_DIR/" + slot + "\"\n" +
			"echo 'ok - within bound'\n"
		targets = append(targets, writeScript(t, scripts, fmt.Sprintf("b%d", i), body))
	}
	par, ser := Schedule(targets)
	if len(par) != 6 || len(ser) != 0 {
		t.Fatalf("partition = par:%d ser:%d, want par:6 ser:0", len(par), len(ser))
	}

	sink := newResultSink()
	cfg := Config{Bash: bashPath(t), Jobs: 2, Work: work, ExtraEnv: []string{"SLOT_DIR=" + slots, "MAX_JOBS=2"}}
	unlaunched := runLanes(context.Background(), cfg, par, ser, newProcRegistry(), sink.onDone)
	if len(unlaunched) != 0 {
		t.Fatalf("unlaunched = %v, want none", unlaunched)
	}
	for _, tg := range targets {
		if rc := sink.byRC[tg.Base]; rc != 0 {
			t.Fatalf("target %s rc = %d, want 0 (bound exceeded)", tg.Base, rc)
		}
	}
}

// TestCancelStopsScheduling proves ctx-cancel stops launching and reports the
// never-launched targets. Jobs=1 serializes the parallel lane; the first
// target's onDone cancels the context, so targets 2..4 never launch and no
// durable result files exist for them.
func TestCancelStopsScheduling(t *testing.T) {
	scripts := testsupport.TempDir(t)
	work := testsupport.TempDir(t)

	var par []Target
	for _, n := range []string{"c1", "c2", "c3", "c4"} {
		par = append(par, writeScript(t, scripts, n, "echo 'ok - "+n+"'\n"))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var doneBases []string
	onDone := func(tg Target, _ Result) {
		mu.Lock()
		doneBases = append(doneBases, tg.Base)
		mu.Unlock()
		cancel() // the first completion cancels the run
	}

	cfg := Config{Bash: bashPath(t), Jobs: 1, Work: work}
	unlaunched := runLanes(ctx, cfg, par, nil, newProcRegistry(), onDone)

	if len(doneBases) != 1 || doneBases[0] != "test_c1.sh" {
		t.Fatalf("completed = %v, want exactly [test_c1.sh]", doneBases)
	}

	var gotUn []string
	for _, tg := range unlaunched {
		gotUn = append(gotUn, tg.Base)
	}
	sort.Strings(gotUn)
	wantUn := []string{"test_c2.sh", "test_c3.sh", "test_c4.sh"}
	if fmt.Sprint(gotUn) != fmt.Sprint(wantUn) {
		t.Fatalf("unlaunched = %v, want %v", gotUn, wantUn)
	}

	// No durable result files for the never-launched targets.
	statDir := filepath.Join(work, "stat")
	for _, b := range wantUn {
		p := filepath.Join(statDir, statFileName(b))
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("result file %s exists for a never-launched target", p)
		}
	}
}
