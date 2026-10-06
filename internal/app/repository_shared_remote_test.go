package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposetup"
)

// fakeRefLister returns refs (or err) and records every call's patterns.
type fakeRefLister struct {
	refs  map[gitcli.RefName]gitcli.ObjectID
	err   error
	calls [][]string
}

func (f *fakeRefLister) ListRemoteRefs(_ context.Context, _ gitcli.Repository, remote gitcli.RemoteName, patterns []string) (map[gitcli.RefName]gitcli.ObjectID, error) {
	if remote != originRemote {
		return nil, errors.New("unexpected remote " + string(remote))
	}
	f.calls = append(f.calls, append([]string(nil), patterns...))
	return f.refs, f.err
}

func TestSharedRemoteMetadataFindings(t *testing.T) {
	ctx := context.Background()
	const oid = gitcli.ObjectID("0123456789012345678901234567890123456789")

	t.Run("shared-never-lists", func(t *testing.T) {
		g := &fakeRefLister{refs: map[gitcli.RefName]gitcli.ObjectID{"refs/heads/docket": oid}}
		lay := layout.Layout{Mode: layout.Shared}
		if got := sharedRemoteMetadataFindings(ctx, g, lay, gitcli.Repository{}); got != nil {
			t.Errorf("shared findings = %+v, want nil", got)
		}
		if got := sharedRemoteMetadataStatusFindings(ctx, g, lay, gitcli.Repository{}); got != nil {
			t.Errorf("shared status findings = %+v, want nil", got)
		}
		if len(g.calls) != 0 {
			t.Errorf("a shared repository listed origin %d times, want never", len(g.calls))
		}
	})

	t.Run("private-reports-docket-named-refs-sorted", func(t *testing.T) {
		g := &fakeRefLister{refs: map[gitcli.RefName]gitcli.ObjectID{
			"refs/heads/dckt":             oid,
			"refs/heads/docketeer":        oid,
			"refs/docket/finalize/7/orig": oid,
			"refs/heads/docket":           oid,
		}}
		lay := layout.Layout{Mode: layout.Private}
		got := sharedRemoteMetadataFindings(ctx, g, lay, gitcli.Repository{})
		wantRefs := []string{"refs/docket/finalize/7/orig", "refs/heads/dckt", "refs/heads/docket"}
		var refs []string
		for _, f := range got {
			refs = append(refs, f.Ref)
			if f.Code != FindingMetadataOnSharedRemote || f.Severity != reposetup.SeverityWarning || f.Repairable != nil {
				t.Errorf("finding %+v, want a %s warning with no Repairable", f, FindingMetadataOnSharedRemote)
			}
			if !strings.Contains(f.Message, "origin holds "+f.Ref+", a docket-named ref") ||
				!strings.Contains(f.Remedy, "git push origin --delete "+f.Ref) {
				t.Errorf("finding %+v does not name its ref in the message and remedy", f)
			}
		}
		if !reflect.DeepEqual(refs, wantRefs) {
			t.Errorf("refs = %v, want %v", refs, wantRefs)
		}
		if len(g.calls) != 1 || !reflect.DeepEqual(g.calls[0], []string{"refs/heads/docket", "refs/heads/dckt", "refs/docket/*"}) {
			t.Errorf("lister calls = %v, want one call with the three docket-named patterns", g.calls)
		}

		st := sharedRemoteMetadataStatusFindings(ctx, g, lay, gitcli.Repository{})
		if len(st) != len(got) {
			t.Fatalf("status findings = %+v, want %d", st, len(got))
		}
		for i, f := range st {
			if f.Code != got[i].Code || f.Severity != "warning" || f.Path != got[i].Ref ||
				f.Message != got[i].Message || f.Remedy != got[i].Remedy {
				t.Errorf("status finding %+v does not mirror %+v", f, got[i])
			}
		}
	})

	t.Run("private-list-error-is-unverified", func(t *testing.T) {
		g := &fakeRefLister{err: errors.New("origin unreachable")}
		lay := layout.Layout{Mode: layout.Private}
		got := sharedRemoteMetadataFindings(ctx, g, lay, gitcli.Repository{})
		if len(got) != 1 {
			t.Fatalf("findings = %+v, want one unverified warning", got)
		}
		f := got[0]
		if f.Code != FindingMetadataOnSharedRemoteUnverified || f.Severity != reposetup.SeverityWarning ||
			!strings.Contains(f.Message, "not proven absent") || !strings.Contains(f.Message, "origin unreachable") ||
			!strings.Contains(f.Remedy, "docket repository check") {
			t.Errorf("unverified finding = %+v", f)
		}
		st := sharedRemoteMetadataStatusFindings(ctx, g, lay, gitcli.Repository{})
		if len(st) != 1 || st[0].Code != FindingMetadataOnSharedRemoteUnverified {
			t.Errorf("unverified status findings = %+v", st)
		}
	})
}
