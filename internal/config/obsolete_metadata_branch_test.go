package config

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestMetadataBranchIsObsoleteTombstone pins the change 0363 contract:
// metadata_branch decodes in every layer, is diagnosed as obsolete with a
// layer-aware remedy, and never resolves to an effective value or to a
// supported/deferred capability. It mirrors the runtime.bash tombstone.
func TestMetadataBranchIsObsoleteTombstone(t *testing.T) {
	// obsoleteFor resolves one layer's metadata_branch declaration and returns
	// the single obsolete-setting diagnostic it must produce. Resolution never
	// errors on it: an obsolete setting is a warning, not an invalid config, and
	// it is excluded from resolution rather than fenced.
	obsoleteFor := func(t *testing.T, src Source) Diagnostic {
		t.Helper()
		res := mustResolve(t, []Source{src}, mainCtx)
		if _, ok := res.declared["metadata_branch"]; ok {
			t.Fatalf("metadata_branch resolved to a honored declaration; it must be excluded in every layer")
		}
		obs := diagsWithCode(res, CodeObsoleteSetting)
		if len(obs) != 1 || obs[0].Path != "metadata_branch" {
			t.Fatalf("want exactly one obsolete-setting diagnostic on metadata_branch, got %v", diagSummary(res))
		}
		if obs[0].Severity != SeverityWarning || obs[0].Classification != Obsolete {
			t.Errorf("obsolete diagnostic = %s/%s, want warning/obsolete", obs[0].Severity, obs[0].Classification)
		}
		if fenced := diagsWithCode(res, CodeFencedIgnored); len(fenced) != 0 {
			t.Errorf("an obsolete setting is not a fenced setting; got %v", diagSummary(res))
		}
		return obs[0]
	}

	// (a) repository layer: the committed occurrence is change 0352's migration
	// input, so its remedy directs the operator at the migration entry point
	// rather than a hand edit (learning printed-remedy-state-validity).
	t.Run("repository layer points at migration", func(t *testing.T) {
		d := obsoleteFor(t, srcR("metadata_branch: main\n"))
		if d.Provenance == nil || d.Provenance.Layer != LayerRepository {
			t.Fatalf("provenance = %+v, want the repository layer", d.Provenance)
		}
		if !strings.Contains(d.Remedy, "docket repository check") {
			t.Errorf("repository-layer remedy = %q, want it to direct at `docket repository check`", d.Remedy)
		}
	})

	// (b) machine-local layer: migration claims no authority over a machine file,
	// so the remedy tells the operator to remove the key from that named file.
	t.Run("machine-local layer directs a hand removal", func(t *testing.T) {
		d := obsoleteFor(t, srcL("metadata_branch: docket\n"))
		if d.Provenance == nil || d.Provenance.Layer != LayerRepositoryLocal {
			t.Fatalf("provenance = %+v, want the repository-local layer", d.Provenance)
		}
		if !strings.Contains(d.Remedy, "remove metadata_branch") || !strings.Contains(d.Remedy, ".docket.local.yml") {
			t.Errorf("machine-local remedy = %q, want a removal naming the declaring file", d.Remedy)
		}
	})

	// (c) global layer: same removal shape, attributed to the global file.
	t.Run("global layer directs a hand removal", func(t *testing.T) {
		src := srcG("metadata_branch: docket\n")
		d := obsoleteFor(t, src)
		if d.Provenance == nil || d.Provenance.Layer != LayerGlobal {
			t.Fatalf("provenance = %+v, want the global layer", d.Provenance)
		}
		if !strings.Contains(d.Remedy, "remove metadata_branch") || !strings.Contains(d.Remedy, src.Name) {
			t.Errorf("global remedy = %q, want a removal naming the declaring file %q", d.Remedy, src.Name)
		}
	})

	// (d) `metadata_branch: main` selects nothing: resolution succeeds and the
	// marshaled Effective carries no metadata-branch value anywhere.
	t.Run("selects nothing and never resolves", func(t *testing.T) {
		snap, diags, err := Resolve([]Source{srcR("metadata_branch: main\nintegration_branch: develop\n")}, mainCtx)
		if err != nil {
			t.Fatalf("Resolve errored on an obsolete setting: %v (%s)", err, formatDiags(diags))
		}
		b, err := json.Marshal(snap.Effective)
		if err != nil {
			t.Fatalf("marshal Effective: %v", err)
		}
		if bytes.Contains(b, []byte(`"metadata_branch"`)) {
			t.Fatalf("effective snapshot still carries a metadata_branch value: %s", b)
		}
		// Non-vacuity companion through the same marshal: the sibling identity
		// setting is still present, so the absence above is a removed key rather
		// than a broken marshal.
		if !bytes.Contains(b, []byte(`"integration_branch"`)) {
			t.Fatalf("non-vacuity companion missing — Effective marshal shape changed: %s", b)
		}
	})

	// (e) the key never appears in Capabilities as a supported or deferred row —
	// only, at most, as the inactive, non-blocking obsolete entry.
	t.Run("never a supported or deferred capability", func(t *testing.T) {
		snap := mustSnapshot(t, srcR("metadata_branch: main\n"))
		for _, c := range snap.Capabilities {
			if c.Path != "metadata_branch" {
				continue
			}
			if c.Classification != Obsolete {
				t.Errorf("metadata_branch capability classified %q, want obsolete only", c.Classification)
			}
			if c.Active || c.MutationBlock {
				t.Errorf("metadata_branch capability = %+v, want inactive and non-blocking", c)
			}
		}
	})
}

// TestEffectiveHasNoMetadataBranchField pins the field's removal from Effective
// by JSON absence, with a non-vacuity companion through the same marshal
// (learning assert-detects-removal-not-replacement).
func TestEffectiveHasNoMetadataBranchField(t *testing.T) {
	b, _ := json.Marshal(Effective{})
	if bytes.Contains(b, []byte(`"metadata_branch"`)) {
		t.Fatalf("Effective still serializes metadata_branch: %s", b)
	}
	if !bytes.Contains(b, []byte(`"integration_branch"`)) {
		t.Fatalf("non-vacuity companion missing — marshal shape changed: %s", b)
	}
}
