<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0540 — A finished run left active in run.json blocks set-visibility and cannot be cancelled](../changes/active/0540-a-finished-run-left-active-in-run-json-blocks-set-visibility.md)**
<!-- docket:backlink:end -->

# A finished run left active in run.json blocks set-visibility and cannot be cancelled — Results

**Human action:** Assessment pending; the review fix pass is under way.

## Outcome

Build complete: every keyed `run-done` verdict now retires its run so `run.json` ends `completed`, and `set-visibility`'s live-run list names a remedy that works. ADR-0148 records the decision.

## Verification performed

- Full suite green at 1d270d39b (build gate).
- Whole-branch review (deep tier) returned 4 findings (2 important, 2 minor): resume dead-end after a `run-unclaimed` run is completed; the `docket run verdict <key>` remedy could end a still-live dispatch; the live-scan doc comment overstates the verdict's effect; the ADR was missing (since recorded as ADR-0148). Fixes in progress.
