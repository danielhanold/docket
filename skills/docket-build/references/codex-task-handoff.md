# Codex task handoff

Keep the static assignment separate from live gate authority. Prepare a scope, capture the first JSON response and exit code exactly once, then write the final private worker payload with the actual child capability, scope identity, optional unchanged context and epoch, and applicable predecessor receipt. Validate the payload and its digest immediately before native dispatch. Never include the parent capability or outer gate key.

Preserve stdout, stderr, and exit status even when the command returns nonzero. `gate.drive.start` reads identity from the nested `drive` object; `gate.drive.prepare-scope` uses its top-level scope fields. WAITING may omit a run root. Claim a pending handoff; use takeover only after a returned child did not hand off. A continuation keeps task identity and accounted edits, and final acknowledgement consumes the current terminal result.
