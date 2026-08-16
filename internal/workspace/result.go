// Package workspace owns docket's local persistent feature-workspace state: the
// manifest under <common-dir>/docket/workspaces/ and the checkout at
// <primary>/.worktrees/<slug>. It composes typed values from internal/domain
// (effective-base/change-id/slug semantics) and drives Git through
// internal/gitcli; it imports no other internal package.
package workspace

import (
	"errors"
	"fmt"
)

// PrepareDisposition is the closed set of outcomes for a prepare operation.
type PrepareDisposition string

const (
	PrepareCreated   PrepareDisposition = "created"
	PrepareExisting  PrepareDisposition = "existing"
	PrepareResumed   PrepareDisposition = "resumed"
	PrepareContended PrepareDisposition = "contended"
	PrepareBlocked   PrepareDisposition = "blocked"
	PrepareFailed    PrepareDisposition = "failed"
)

// CleanupDisposition is the closed set of outcomes for a cleanup operation.
type CleanupDisposition string

const (
	CleanupCleaned      CleanupDisposition = "cleaned"
	CleanupAlreadyClean CleanupDisposition = "already-clean"
	CleanupBlocked      CleanupDisposition = "blocked"
	CleanupFailed       CleanupDisposition = "failed"
)

// PublishDisposition is the closed set of outcomes for a publish-head operation.
type PublishDisposition string

const (
	PublishPublished        PublishDisposition = "published"
	PublishAlreadyPublished PublishDisposition = "already-published"
	PublishContended        PublishDisposition = "contended"
	PublishUnknown          PublishDisposition = "unknown"
	PublishFailed           PublishDisposition = "failed"
)

// Stage names the phase a Failure arose in. Its values are drawn from the
// closed set: "validate", "lock", "fetch", "inventory", "allocate",
// "worktree", "verify", "manifest", "probe", "push", "remove".
type Stage string

// Kind is the stable, redaction-safe category of a Failure.
type Kind string

const (
	KindInvalidInput  Kind = "invalid-input"
	KindInvalidState  Kind = "invalid-state"
	KindExternal      Kind = "external"
	KindCancelled     Kind = "cancelled"
	KindTimedOut      Kind = "timed-out"
	KindInvalidOutput Kind = "invalid-output"
)

// Failure is the workspace package's typed error. Detail carries bounded,
// redacted prose only — never tokens, env values, PR body bytes, credentialed
// URLs, or unbounded stderr.
type Failure struct {
	Op     string // "prepare" | "inspect" | "cleanup" | "publish-head"
	Stage  Stage
	Kind   Kind
	Detail string // bounded, redacted
	Err    error  // wrapped cause, may be nil
}

// Error renders the operation, stage, kind, and bounded detail.
func (f *Failure) Error() string {
	if f.Detail == "" {
		return fmt.Sprintf("workspace %s/%s: %s", f.Op, f.Stage, f.Kind)
	}
	return fmt.Sprintf("workspace %s/%s: %s: %s", f.Op, f.Stage, f.Kind, f.Detail)
}

// Unwrap exposes the wrapped cause for errors.Is/As.
func (f *Failure) Unwrap() error { return f.Err }

// AsFailure is an errors.As convenience recovering a *Failure from an error.
func AsFailure(err error) (*Failure, bool) {
	var f *Failure
	if errors.As(err, &f) {
		return f, true
	}
	return nil, false
}
