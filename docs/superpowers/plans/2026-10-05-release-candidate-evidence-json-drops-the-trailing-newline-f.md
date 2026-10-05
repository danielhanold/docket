<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0524 — Release-candidate evidence.json drops the trailing newline from its checksums copy](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0524-release-candidate-evidence-json-drops-the-trailing-newline-f.md)**
<!-- docket:backlink:end -->
# Release-candidate evidence.json keeps checksums.txt byte-exact Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `evidence.json`'s `checksums_txt` field byte-equal to the candidate bundle's `checksums.txt`, trailing newline included, by having jq read the file with `--rawfile` instead of receiving it through a shell command substitution.

**Architecture:** This is a two-line edit to one workflow step. In `.github/workflows/release-candidate.yml`, `summary` job, step "Assemble the evidence JSON": delete the `checksums="$(cat candidate-head/checksums.txt)"` line, and replace the jq argument `--arg checksums "$checksums"` with `--rawfile checksums candidate-head/checksums.txt`. The jq program still binds `$checksums`, so its body (`checksums_txt: $checksums`) does not change. No test exercises workflow steps today, and this change adds none. A one-off local harness proves the fix instead: it extracts the step's real `run:` body from the workflow file, runs it on a fixture, and `cmp`s the extracted field against the fixture file. It goes red before the edit and green after.

**Tech Stack:** GitHub Actions workflow YAML, bash, jq (`--rawfile` has existed since jq 1.6; the ubuntu-24.04 runner ships 1.7, and local macOS has `/usr/bin/jq`), awk, `cmp`.

**Spec:** None, because the change is `trivial: true`. The change body's `## What changes` section is the spec: `docs/changes/active/0524-release-candidate-evidence-json-drops-the-trailing-newline-f.md` on the `docket` branch (synchronized copy at `/Users/homer/dev/docket/.docket/docs/changes/active/0524-release-candidate-evidence-json-drops-the-trailing-newline-f.md`).

## Global Constraints

- Touch only `.github/workflows/release-candidate.yml`, and in it only the two lines named above. The change's "Out of scope" section rules out any other change to the release-candidate workflow or to the evidence schema: no other step, no new field, and no renamed jq variable.
- Do not edit `docs/release/v1.0.0-alpha.1/candidate/evidence.json`. It is a point-in-time release record, and its missing trailing newline is history.
- No new test, no new CI gate, and no committed proof script. The proof is a one-off run in a `mktemp -d` scratch directory (`"${TMPDIR:-/tmp}/rc0524.XXXXXX"`) outside the worktree.
- The proof transcript (red run before the edit, green runs after) goes in the task's COMPLETE report so the results file can record it. Do not author a results file in the feature tree.
- Shell discipline (AGENTS.md): capture output before you grep it under `pipefail`, and template every `mktemp`.
- The build gate runs the whole suite through `build.test_command`, not only this proof.

## Review Focus

1. **A `checksums.txt` that ends in a newline** is the real case (goreleaser writes one per line). The field must keep that final `\n`. Task 1 Steps 2 and 4 pin it with `cmp`.
2. **A `checksums.txt` with no trailing newline** must come through unchanged, with no newline added. Task 1 Step 5 pins it.
3. **The rest of the evidence object** (`source_commit`, versions, `smoke_result`, `tuples`, `live_host_acceptance`) must be unchanged by the edit. Task 1 Step 4 checks the keys and values.
4. **The step's `set -euo pipefail` and its "written regardless" guarantee** must still hold. If `candidate-head/checksums.txt` is missing, the old line already failed under `set -e`, and `--rawfile` fails the same way. That is not a regression. The harness runs the real step body end to end with exit 0 on the happy path.
5. **The harness running a stale copy of the step** would prove nothing. Step 3 confirms with `git diff` that the edit landed, and Step 4 extracts the body from the edited file and greps it for the new argument before reading the result.

---

### Task 1: Read checksums.txt with `jq --rawfile` in the evidence step

**Files:**
- Modify: `.github/workflows/release-candidate.yml` (`summary` job, step `- name: Assemble the evidence JSON`. Delete the line `checksums="$(cat candidate-head/checksums.txt)"`; in the `jq -n \` argument list, replace `--arg checksums "$checksums" \` with `--rawfile checksums candidate-head/checksums.txt \`)
- Test: none committed. Use the one-off harness below.

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: an `evidence.json` whose `checksums_txt` string equals the exact bytes of `candidate-head/checksums.txt`.

- [ ] **Step 1: Build the one-off proof harness (outside the worktree)**

Shell state does not persist between tool calls. Note the printed `$H` path, and set `WT` and `H` again at the start of every later step's command. Define the harness as a function in a scratch script. It extracts the step's real `run: |` body from a given workflow file, strips the 10-space YAML indent, runs it on a fixture with stub env, and compares the field byte for byte.

```bash
WT=/Users/homer/dev/docket/.worktrees/release-candidate-evidence-json-drops-the-trailing-newline-f
H="$(mktemp -d "${TMPDIR:-/tmp}/rc0524.XXXXXX")"
cat >"$H/prove.sh" <<'EOF'
#!/usr/bin/env bash
# usage: prove.sh <workflow.yml> <with-trailing-newline: yes|no>
set -uo pipefail
wf="$1"; nl="$2"
d="$(mktemp -d "${TMPDIR:-/tmp}/rc0524run.XXXXXX")"; cd "$d" || exit 2
awk '/- name: Assemble the evidence JSON/{f=1;next} f&&/^        run: \|/{r=1;next} r&&/^      - name:/{exit} r{sub(/^          /,"");print}' "$wf" >step.sh
mkdir -p candidate-head verdicts
if [ "$nl" = yes ]; then
  printf 'aaa  docket_x_darwin_amd64.tar.gz\nbbb  install.sh\n' >candidate-head/checksums.txt
else
  printf 'aaa  docket_x_darwin_amd64.tar.gz\nbbb  install.sh' >candidate-head/checksums.txt
fi
for t in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64; do
  printf '{"tuple":"%s","result":"success"}\n' "$t" >"verdicts/$t.json"
done
COMMIT=deadbeef HEAD_VERSION=v1 BASE_VERSION=v0 SMOKE_RESULT=success GITHUB_STEP_SUMMARY=/dev/null \
  bash step.sh >/dev/null
echo "step-exit=$?"
jq -j .checksums_txt evidence.json | cmp - candidate-head/checksums.txt
echo "cmp-exit=$?"
echo "dir=$d"
EOF
chmod +x "$H/prove.sh"
echo "$H"
```

- [ ] **Step 2: Run the harness against the unfixed workflow and watch it fail**

Run: `"$H/prove.sh" "$WT/.github/workflows/release-candidate.yml" yes`
Expected: `step-exit=0`, then `cmp: EOF on stdin` and `cmp-exit=1`. Command substitution dropped the trailing newline. Copy this output into the COMPLETE report as the RED evidence.

- [ ] **Step 3: Make the two-line edit**

In `.github/workflows/release-candidate.yml`, step "Assemble the evidence JSON", delete this line:

```bash
          checksums="$(cat candidate-head/checksums.txt)"
```

and change this jq argument:

```bash
            --arg checksums "$checksums" \
```

to:

```bash
            --rawfile checksums candidate-head/checksums.txt \
```

Leave the jq program body (`checksums_txt: $checksums,`) and every other line as they are. Then run `git -C "$WT" diff`. Expected: exactly one deleted line and one changed line, both in that step.

- [ ] **Step 4: Re-run the harness on the edited workflow and confirm it passes**

Run:

```bash
out="$("$H/prove.sh" "$WT/.github/workflows/release-candidate.yml" yes)"; echo "$out"
d="$(sed -n 's/^dir=//p' <<<"$out")"
grep -F -e '--rawfile checksums candidate-head/checksums.txt' "$d/step.sh"
grep -c -F -e 'checksums="$(cat' "$d/step.sh"
jq -S 'del(.checksums_txt, .tuples)' "$d/evidence.json"
jq '.tuples | length' "$d/evidence.json"
```

Expected: `step-exit=0` and `cmp-exit=0`. The extracted `step.sh` contains the `--rawfile` line, and the `checksums="$(cat` count is `0`. The remaining object is `base_version: "v0"`, `head_version: "v1"`, `live_host_acceptance: "outstanding — see docs/release/four-harness-acceptance.md"`, `smoke_result: "success"`, `source_commit: "deadbeef"`, and `tuples` has length `4`. Copy this output into the COMPLETE report as the GREEN evidence.

- [ ] **Step 5: Confirm a file without a trailing newline passes through unchanged**

Run: `"$H/prove.sh" "$WT/.github/workflows/release-candidate.yml" no`
Expected: `step-exit=0` and `cmp-exit=0`, so no newline was added. Add this output to the COMPLETE report.

- [ ] **Step 6: Commit**

```bash
git -C "$WT" add .github/workflows/release-candidate.yml && git -C "$WT" commit -m "fix(release): keep checksums.txt byte-exact in candidate evidence.json

Read checksums.txt with jq --rawfile instead of a shell command
substitution, which stripped the trailing newline and broke the release
protocol's byte-equality check between evidence.json and checksums.txt."
```

Then remove the scratch directories (`rm -rf "$H"` plus each `dir=` printed above).
