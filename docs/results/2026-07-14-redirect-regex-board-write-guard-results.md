# Harden the BOARD.md write guard — results

Change: #70 · Branch: feat/redirect-regex-board-write-guard · PR: (see change file) · Plan: `docs/superpowers/plans/2026-07-13-redirect-regex-board-write-guard-plan.md` · ADRs: 31

Test-only. Nothing under `scripts/` is modified by this branch (asserted, and re-verified after the rebase onto `origin/main`).

## Verify (human)

- [ ] Read ADR-0031. It reverses a premise of this change's own spec — the `REDIRECT_RE` scan over `scripts/docket-status.sh` is **kept**, not retired. If you disagree with keeping two guards, that is the decision to push back on, and the `COMPLEMENTARITY` block in `tests/test_render_board.sh` is what enforces it.
- [ ] Sanity-check the signal-to-noise call: `tests/test_render_board.sh` grew ~1000 lines, roughly 60% comment. The battery's per-row comments are load-bearing (each names a shape that really wrote the board); the preamble was cut once already. If you want it leaner, say so — the cut is mechanical.

## Findings

**Six review rounds, six real holes.** Every one was found by *executing* a fixture against a stub renderer and checking whether bytes reached a file — never by reading the guard. Each shipped GREEN under the guard as it stood at that moment:

1. `&>`, `&>>`, `>&file` — the tokenizer split on the `&` of `&>`, and the fd-dup eraser (`[0-9-]*`, which matches zero characters) deleted `>&"$f"`, a real file write.
2. **A comment ending in a backslash laundered a live redirect.** The spec's own pipeline order (join continuations, *then* strip comments) is exploitable: bash comments do **not** continue across a trailing `\`, so the next line executes — but joining first folded it into the comment, which the comment filter then deleted. Shipped order is strip-then-join.
3. A redirect past a pipe (`| cat > "$d/BOARD.md"`) — the token ended at the `|`. A pipeline *is* the renderer's stdout, so `|` must not terminate the token.
4. **The subsumption premise was false** (see Deviations).
5. Capture-then-write (`out=$(render-board.sh …); printf '%s' "$out" > "$f"`) — found by injecting into the *real* `scripts/docket-status.sh`, which already captures the renderer into `out` and already holds the board path in a variable. Closed with a one-hop taint stage.
6. `${out:-}` — **the guarded file's own house idiom** (14 occurrences in `docket-status.sh`). The taint stage enumerated *spellings* (`$out`, `${out}`) instead of describing *shape*, so the guard was green on the single most likely real regression. Now keyed on shape.

The through-line, and the reason ADR-0031 exists: **a guard written as a list of spellings is always one spelling short.** Four rounds of fixture-driven hardening all shipped green; round five found a live hole in minutes by injecting the regression into the real script. Live-tree probing belongs in the loop for any guard, not just fixtures.

**Disclosed residual (green, and really writes):** `| tee f`; `exec 3>f` + `>&3`; an `eval`-conjured redirect operator; a tainted value passed to a function or copied to a second variable; `mapfile`/`read < <(…)` captures; a metacharacter inside a quoted argument. These are named in the test file's `KNOWN, ACCEPTED GAPS` rather than chased — chasing them is the very anti-pattern above.

## Deviations from the spec

1. **`REDIRECT_RE`'s scan over `scripts/docket-status.sh` is KEPT, not retired.** The spec said the new write sentinel subsumed it. Mutation testing disproved that: the sentinel is token-scoped and is structurally blind to a write crossing a statement boundary that carries the bytes in no variable — `{ render-board.sh …; } > f`, and a wrapper function — both of which `REDIRECT_RE`'s whole-file flattened scan catches. The two guards are complementary, not nested; neither may be deleted. Locked by the `COMPLEMENTARITY` block, which asserts in both directions using the *real* regex and the *real* function. Recorded as ADR-0031.
2. **Pipeline order inverted** (strip comments → join continuations, not the spec's reverse). See finding 2 — the spec's order laundered a live write.
3. **Call sites derived with `find`, not a flat `scripts/*.sh` glob** — the glob misses `scripts/lib/*.sh`. The sweep now also covers the four root-level scripts, so the comment's scope claim matches the code (ledger #64: derive the list, never hand-shape it).
4. **fd dups are erased, but a stderr-to-file redirect (`2>/dev/null`) still reddens the guard** — deliberately conservative, and disclosed. The correct way to route this renderer's stderr is the fd dup already in use (`2>&2`).

## Follow-ups

- **The deferred filesystem-effect test — its trigger has fired.** The spec deferred it with an explicit condition: *"it earns its cost when a write path exists that a source scan cannot reach; today there is not."* Six rounds have now established, empirically, that such paths exist and that source-syntax scanning cannot close them (the residual list above). An effect test — run the orchestrator against a fixture and assert `BOARD.md`'s bytes — is syntax-independent and is the only real answer to that class. It remains path-dependent (it misses a rogue write on a branch the fixture never takes), which is why it complements the scans rather than replacing them. Worth a new change.
- Signal-to-noise: the preamble in `tests/test_render_board.sh` still carries a stage-by-stage rationale. Every paragraph documents a bug already paid for, so it was kept — but a future distill could compress it once the guard stops changing.
