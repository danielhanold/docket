package domain

import (
	"testing"
	"time"
)

// leaseNow is the injected clock reading every lease test evaluates against.
var leaseNow = time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)

// leaseTTL is the TTL every lease test uses unless it says otherwise.
const leaseTTL = 4

// leaseChange builds an in-progress change whose claim stamp is supplied as a
// ready-made OptionalTime, so a test can express absent, empty, malformed, and
// present stamps without going through a parser.
func leaseChange(status Status, stamp OptionalTime, branch string) Change {
	cs := ChangeSpec{
		ID:        7,
		Slug:      "lease-slug",
		Type:      "feat",
		Status:    status,
		RawStatus: string(status),
		ClaimedAt: stamp,
	}
	if branch != "" {
		cs.Branch = OptionalString{State: FieldPresent, Value: branch}
	}
	return NewChange(cs)
}

// leaseStamp renders a present claim stamp offset from leaseNow: a negative
// offset is a claim taken in the past.
func leaseStamp(offset time.Duration) OptionalTime {
	at := leaseNow.Add(offset)
	return OptionalTime{State: FieldPresent, Value: at, Raw: at.Format(time.RFC3339)}
}

func TestEvaluateLeaseStates(t *testing.T) {
	cases := []struct {
		name   string
		status Status
		stamp  OptionalTime
		want   LeaseState
	}{
		{"well under ttl", StatusInProgress, leaseStamp(-time.Hour), LeaseFresh},
		{"one second under ttl", StatusInProgress, leaseStamp(-(leaseTTL*time.Hour - time.Second)), LeaseFresh},
		{"exactly at ttl is fresh", StatusInProgress, leaseStamp(-leaseTTL * time.Hour), LeaseFresh},
		{"one second past ttl", StatusInProgress, leaseStamp(-(leaseTTL*time.Hour + time.Second)), LeaseExpired},
		{"far past ttl", StatusInProgress, leaseStamp(-100 * time.Hour), LeaseExpired},
		{"stamp in the future", StatusInProgress, leaseStamp(2 * time.Hour), LeaseFresh},
		{"absent stamp", StatusInProgress, OptionalTime{}, LeaseMissing},
		{"empty stamp", StatusInProgress, OptionalTime{State: FieldEmpty}, LeaseMissing},
		{"malformed stamp", StatusInProgress, OptionalTime{State: FieldMalformed, Raw: "yesterday"}, LeaseMalformed},
		{"proposed change", StatusProposed, leaseStamp(-100 * time.Hour), LeaseNotInProgress},
		{"blocked change", StatusBlocked, leaseStamp(-100 * time.Hour), LeaseNotInProgress},
		{"done change", StatusDone, OptionalTime{}, LeaseNotInProgress},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateLease(leaseChange(tc.status, tc.stamp, ""), leaseNow, leaseTTL)
			if got != tc.want {
				t.Fatalf("EvaluateLease = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEvaluateLeaseStatusOutranksStamp(t *testing.T) {
	// An archived record with a malformed stamp is still reported by status:
	// the stamp of a change holding no lease is not a lease diagnosis.
	got := EvaluateLease(leaseChange(StatusKilled, OptionalTime{State: FieldMalformed, Raw: "??"}, ""), leaseNow, leaseTTL)
	if got != LeaseNotInProgress {
		t.Fatalf("EvaluateLease = %q, want %q", got, LeaseNotInProgress)
	}
}
