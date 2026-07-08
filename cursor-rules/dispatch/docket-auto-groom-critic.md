## docket-auto-groom-critic — dispatch only

This agent wraps no skill and is normally dispatched **by** docket-auto-groom as its adversarial gate,
not invoked directly. If you are asked to adversarially review an auto-groom draft spec or trivial
verdict, dispatch it rather than reviewing inline.

Dispatch prompt must include the draft spec / trivial verdict under review and the dispatching skill's
verdict protocol.

Do NOT let it improve the draft — it only attacks and returns exactly one verdict.

    Task(subagent_type: "docket-auto-groom-critic", run_in_background: false,
         prompt: "Adversarially review this draft spec and return one verdict per the protocol.")
