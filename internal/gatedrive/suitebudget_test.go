package gatedrive

import (
	"sort"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// sampleSuiteBudgetKey builds a SuiteBudgetKey with a value in every field, so a
// persisted-record inspection sees a well-populated record.
func sampleSuiteBudgetKey() SuiteBudgetKey {
	return SuiteBudgetKey{
		RepoIdentity: "repo-x",
		ChangeID:     "0421",
		Phase:        "build",
	}
}

// TestReserveSuiteAttemptCountsAndExhausts proves a limit-4 budget grants exactly
// attempts 1,2,3,4 in order, then the fifth reservation is refused with the typed
// exhausted error and no state change: usage stays (4,4).
func TestReserveSuiteAttemptCountsAndExhausts(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	key := sampleSuiteBudgetKey()

	for want := 1; want <= 4; want++ {
		attempt, snapped, err := s.ReserveSuiteAttempt(key, 4)
		if err != nil {
			t.Fatalf("reservation %d: unexpected error: %v", want, err)
		}
		if attempt != want {
			t.Fatalf("reservation %d: attempt = %d, want %d", want, attempt, want)
		}
		if snapped != 4 {
			t.Fatalf("reservation %d: snappedLimit = %d, want 4", want, snapped)
		}
	}

	// The fifth reservation is refused with the typed exhausted error.
	attempt, _, err := s.ReserveSuiteAttempt(key, 4)
	if !isStoreKind(err, ErrSuiteBudgetExhausted) {
		t.Fatalf("fifth reservation must fail ErrSuiteBudgetExhausted, got attempt=%d err=%v", attempt, err)
	}

	// The refused call changed no state.
	used, limit, err := s.SuiteBudgetUsage(key)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage: %v", err)
	}
	if used != 4 || limit != 4 {
		t.Fatalf("after exhaustion usage = (%d,%d), want (4,4)", used, limit)
	}
}

// TestReserveSuiteAttemptSnapshotsLimitOnCreate proves the limit parameter is
// consulted only on the first reservation: a later reservation passing a larger
// limit is still capped at the snapshotted value; and a fresh key with limit 1
// grants exactly one attempt.
func TestReserveSuiteAttemptSnapshotsLimitOnCreate(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	key := sampleSuiteBudgetKey()

	if attempt, snapped, err := s.ReserveSuiteAttempt(key, 4); err != nil || attempt != 1 || snapped != 4 {
		t.Fatalf("first reserve = (%d,%d,%v), want (1,4,nil)", attempt, snapped, err)
	}
	// A later reservation passing 99 must still enforce the snapshotted 4.
	attempt, snapped, err := s.ReserveSuiteAttempt(key, 99)
	if err != nil {
		t.Fatalf("second reserve: %v", err)
	}
	if attempt != 2 || snapped != 4 {
		t.Fatalf("second reserve = (%d,%d), want (2,4) — the config parameter must not rewrite an owned budget", attempt, snapped)
	}

	// A fresh key created at limit 1 grants attempt 1 then exhausts.
	fresh := SuiteBudgetKey{RepoIdentity: "repo-y", ChangeID: "0999", Phase: "build"}
	if attempt, snapped, err := s.ReserveSuiteAttempt(fresh, 1); err != nil || attempt != 1 || snapped != 1 {
		t.Fatalf("fresh limit-1 reserve = (%d,%d,%v), want (1,1,nil)", attempt, snapped, err)
	}
	if _, _, err := s.ReserveSuiteAttempt(fresh, 1); !isStoreKind(err, ErrSuiteBudgetExhausted) {
		t.Fatalf("second fresh reserve must exhaust, got %v", err)
	}
}

// TestReserveSuiteAttemptLimitOne proves a limit-1 budget grants exactly the
// initial attempt and nothing more.
func TestReserveSuiteAttemptLimitOne(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	key := sampleSuiteBudgetKey()

	if attempt, snapped, err := s.ReserveSuiteAttempt(key, 1); err != nil || attempt != 1 || snapped != 1 {
		t.Fatalf("limit-1 first reserve = (%d,%d,%v), want (1,1,nil)", attempt, snapped, err)
	}
	if _, _, err := s.ReserveSuiteAttempt(key, 1); !isStoreKind(err, ErrSuiteBudgetExhausted) {
		t.Fatalf("limit-1 second reserve must exhaust, got %v", err)
	}
	if used, limit, _ := s.SuiteBudgetUsage(key); used != 1 || limit != 1 {
		t.Fatalf("limit-1 usage = (%d,%d), want (1,1)", used, limit)
	}
}

// TestReserveSuiteAttemptConcurrent proves N goroutines racing one key at limit 4
// grant exactly 4 distinct attempt numbers (1..4) and the rest exhaust — a lost
// launch can never overrun the configured bound. Run under -race.
func TestReserveSuiteAttemptConcurrent(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	key := sampleSuiteBudgetKey()

	const n = 16
	var wg sync.WaitGroup
	attempts := make([]int, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			a, _, err := s.ReserveSuiteAttempt(key, 4)
			attempts[idx] = a
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	var granted []int
	for i := 0; i < n; i++ {
		if errs[i] == nil {
			granted = append(granted, attempts[i])
			continue
		}
		if !isStoreKind(errs[i], ErrSuiteBudgetExhausted) {
			t.Fatalf("loser %d must fail ErrSuiteBudgetExhausted, got %v", i, errs[i])
		}
	}
	sort.Ints(granted)
	want := []int{1, 2, 3, 4}
	if len(granted) != len(want) {
		t.Fatalf("granted %v, want exactly 4 distinct attempts", granted)
	}
	for i := range want {
		if granted[i] != want[i] {
			t.Fatalf("granted attempts = %v, want %v (distinct 1..4)", granted, want)
		}
	}
	if used, limit, _ := s.SuiteBudgetUsage(key); used != 4 || limit != 4 {
		t.Fatalf("after concurrent race usage = (%d,%d), want (4,4)", used, limit)
	}
}

// TestSuiteBudgetPersistsAcrossStoreReopen proves consumed budget survives a
// fresh Store over the same repo directory — recovery/continuation preserve what
// was already spent rather than resetting it.
func TestSuiteBudgetPersistsAcrossStoreReopen(t *testing.T) {
	dir := testsupport.TempDir(t)
	key := sampleSuiteBudgetKey()

	s1 := OpenStore(dir)
	if _, _, err := s1.ReserveSuiteAttempt(key, 4); err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	if _, _, err := s1.ReserveSuiteAttempt(key, 4); err != nil {
		t.Fatalf("second reserve: %v", err)
	}

	s2 := OpenStore(dir)
	used, limit, err := s2.SuiteBudgetUsage(key)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage after reopen: %v", err)
	}
	if used != 2 || limit != 4 {
		t.Fatalf("after reopen usage = (%d,%d), want (2,4)", used, limit)
	}
	// The reopened store continues counting from the persisted state.
	if attempt, _, err := s2.ReserveSuiteAttempt(key, 4); err != nil || attempt != 3 {
		t.Fatalf("reopened reserve = (%d,%v), want attempt 3", attempt, err)
	}
}

// TestReserveSuiteAttemptInvalidLimit proves a sub-1 limit at creation fails
// closed with the typed invalid-request error and persists no record.
func TestReserveSuiteAttemptInvalidLimit(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	key := sampleSuiteBudgetKey()

	if _, _, err := s.ReserveSuiteAttempt(key, 0); !isStoreKind(err, ErrInvalidRequest) {
		t.Fatalf("limit 0 must fail ErrInvalidRequest, got %v", err)
	}
	// No record was created.
	used, limit, err := s.SuiteBudgetUsage(key)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage: %v", err)
	}
	if used != 0 || limit != 0 {
		t.Fatalf("after refused create usage = (%d,%d), want (0,0)", used, limit)
	}
}
