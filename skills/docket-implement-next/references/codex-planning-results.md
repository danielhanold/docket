# Codex planning and results

Resolve the complete declared resource graph before dispatch. Obtain assignment and payload document formats from the candidate's `schema --operation agent.check-inputs`; preserve every declared resource locator and digest. Planner selector values are resource logical IDs, never skill names or paths. Use this concrete mapping when those resources are present:

- `plan_skill: "plan-skill"` selects the `resources[]` entry whose `logical_id` is `plan-skill`;
- `build_skill: "build-skill"` selects the `resources[]` entry whose `logical_id` is `build-skill`;
- `results_template: "results-template"` selects the packaged results template entry whose `logical_id` is `results-template`.

For every selected root, follow local Markdown links recursively and declare every recursively linked local file as a pinned resource. The `resource_dependencies` object is a complete adjacency map: every resource, including leaf resources, appears as a resource_dependencies key, and each value names that resource's direct local Markdown dependencies by logical ID. Do not dispatch a path-only, selector-only, or top-level-only graph.

The plan writer reads the supplied plan skill and nested references, writes only its assigned artifact, runs the official backlink operation, commits only that path with one `Docket-Plan-Path` trailer, and returns `PLAN_PATH=<repo-relative-path>`. The controller verifies the commit and attaches it using fresh metadata state.

Use the exact packaged `skills/docket-implement-next/results-template.md`. Results record source validation and pending native work truthfully. Commit, publish, attach, and then run the independent final-head checkpoint; a template receipt or implementation-head pass cannot replace that checkpoint.
