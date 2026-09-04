<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0401 — Add a source-available license: PolyForm Noncommercial plus an individual commercial exemption](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0401-add-a-source-available-license-polyform-noncommercial-plus-a.md)**
<!-- docket:backlink:end -->
# Source-available license (PolyForm Noncommercial + individual exemption) — results
Change: #401 · Branch: docs/add-a-source-available-license-polyform-noncommercial-plus-a · PR: <pending> · Plan: docs/superpowers/plans/2026-09-03-add-a-source-available-license-polyform-noncommercial-plus-a.md · ADRs: none

## Verify (human)

- [ ] **Legal review of the additional-permissions text.** The spec author is not a lawyer and `LICENSE-ADDITIONAL-PERMISSIONS.md` is custom, non-standard wording. Have the individual-commercial-exemption and retroactive-scope clauses reviewed before relying on them against a paying organization. Out of scope for this change; not a blocker to merge.
- [ ] **Confirm the copyright holder / contact line.** `LICENSE` carries `Required Notice: Copyright Daniel Hanold (https://github.com/danielhanold/docket)` and clause 3 of the additional permissions lists `danny@danielhanold.com`. Confirm both are the intended holder and contact.

## Findings

- **PolyForm body is verbatim, not paraphrased.** The `LICENSE` body was fetched byte-for-byte from the official `polyformproject/polyform-licenses` tag `1.0.0` (all 15 sections in canonical order; 4563-byte body; `diff` against the fetched file empty after assembly). The guard (`internal/repoguard/license_test.go`) asserts the identifier, the Required Notice, the pointer line, and the three exemption-clause headings by fixed-string containment — it does **not** guard byte-for-byte verbatim-ness of the whole PolyForm body, which remains a one-time manual authoring guarantee.
- **Review finding (minor) was fixed then reverted by the suite gate.** The whole-branch review raised one minor finding: `TestLicenseFiles` guards marker/heading presence, not the substance (a paraphrase keeping the markers would stay green). A fix that added distinctive-clause-phrase pins and a license-body length floor was committed (`d0247615`) but its edit was not gofmt-clean, so the full suite went red on `test_go_toolchain`. Per the bounded fix-loop's revert-and-record policy the fix was reverted (`47eb314f`) and the suite re-run green. The finding stands, recorded here and in the PR body's disposition table — the branch ships with the original marker/heading guard, which is green and was reviewed.

## Follow-ups

- **(Optional) Strengthen the license guard, gofmt-clean.** The reverted improvement — pin one distinctive sentence from each of the three exemption clauses and add a length-floor assertion on the `LICENSE` body — remains a reasonable low-risk enhancement to `internal/repoguard/license_test.go`. Capture deliberately with `docket change create` if wanted; it is not required for this change.
- **Plan deviation (no defect).** The spec sketched the guard as grep asserts in a standalone shell file, but the suite's category vocabulary is fail-closed to `go|posix-install|posix-downloader` (`internal/suiterunner/discover.go`), so a grep-only shell suite is inadmissible. The guard was implemented as Go fixed-string containment tests (`internal/repoguard/license_test.go`) driven by a `# docket-suite: go` wrapper `tests/test_license_files.sh` that pins both `--- PASS:` lines. Every spec assertion is still asserted.
- **Plan note.** The spec asked for a matching entry in the README's table-of-contents list; the README has no TOC list, so that sentence has no referent — nothing was added.
