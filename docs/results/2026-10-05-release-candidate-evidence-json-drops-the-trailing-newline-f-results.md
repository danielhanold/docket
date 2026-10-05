<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0524 — Release-candidate evidence.json drops the trailing newline from its checksums copy](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0524-release-candidate-evidence-json-drops-the-trailing-newline-f.md)**
<!-- docket:backlink:end -->
# Release-candidate evidence.json keeps checksums.txt byte-exact — results

**Human action:** None required. The next release's Phase 2 byte-equality check is the first real-world run of the fixed step; if it passes there, nothing more is needed.

## Outcome

The "Assemble the evidence JSON" step in `.github/workflows/release-candidate.yml` now reads `candidate-head/checksums.txt` with `jq --rawfile` instead of a shell command substitution. The `checksums_txt` field in `evidence.json` is now byte-equal to the bundle's `checksums.txt`, trailing newline included, so the release protocol's Phase 2 byte-equality check no longer stops on it. The jq program body and every other evidence field are unchanged.

## Verification performed

- One-off harness (not committed): it extracted the step's real `run:` body from the workflow file, ran it on a sample `checksums.txt` with stub env and four tuple verdicts, then ran `jq -j .checksums_txt evidence.json | cmp - checksums.txt`.
  - Before the edit: step exit 0, `cmp: EOF on stdin`, cmp exit 1 (trailing newline lost).
  - After the edit: step exit 0, cmp exit 0. The other fields (`source_commit`, `head_version`, `base_version`, `smoke_result`, `live_host_acceptance`) were unchanged, and `tuples` had 4 entries.
  - A sample with no trailing newline: cmp exit 0, so nothing is added.
- An independent coordinator check with jq 1.8.2 reproduced the same result: the old `--arg "$(cat …)"` form fails `cmp`, and the `--rawfile` form passes. The ubuntu-24.04 runner ships jq 1.7; `--rawfile` exists since 1.6.
- The full suite ran through the build gate.
- Review: the lean-tier whole-branch review returned no findings.
