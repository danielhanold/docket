# Workflow roles: rebinding any step with the `skills:` map

docket is a thin lifecycle layer wrapped around a workflow engine, and each of its **five workflow
invocation points is a pluggable role**. The optional `skills:` map, in any config layer (see
[Repo config](config-layers.md) for the layers), rebinds a role either to a different skill (its
name is handed to the harness verbatim) or to the sentinel `auto`, which means "no skill — the
running agent (a separately launched worker with its own context, pinned to a model and effort)
does the step inline at its own model."

| Role | Default | Where it runs |
|---|---|---|
| `brainstorm` | `superpowers:brainstorming` | up-front design, before the spec |
| `plan` | `superpowers:writing-plans` | the task plan built from the spec |
| `build` | `docket-build` | executing the plan task by task |
| `review` | `docket-review` | whole-branch review before the pull request |
| `finish` | `superpowers:finishing-a-development-branch` | pushing the branch and opening the pull request |

Three roles default to the superpowers engine; docket owns `build` and `review` itself. To run the
superpowers engine everywhere, bind `build` and `review` explicitly to their superpowers
equivalents. If a bound skill cannot be invoked at runtime — superpowers is not installed, or a
custom name is misspelled — docket **degrades to that role's `auto` fallback with a prominent
warning**, so a repository without superpowers still works out of the box.

One binding carries a caveat worth knowing before you set it: **`build: auto` is dual-purpose.** It
authorizes inline building *and* the in-branch fix workers docket runs on review findings, which
execute the build role's own contract — there is deliberately no separate key for those fix
workers. The full shape of the map, the `auto` sentinel, and each role's fallback are documented
once in the `docket-convention` skill's *Skill layer* section; consult it there rather than copying
examples, and see [Config keys](../reference/config-keys.md) for where the shape is owned.
