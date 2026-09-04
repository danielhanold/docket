# Groom: Designing before building

Some work you capture is fully thought through; most of it is a rough idea you jotted down to
get it out of your head. This page is about the step in between capturing and building: turning a
half-formed stub into something an autonomous run can pick up and implement without guessing.
By the end you can groom a stub interactively, let docket groom a batch of stubs on its own under
an adversarial check, have a high-tier consultant write the final design document, and tune the
whole conversation to the reader you actually are.

A **change** (one unit of planned work, roughly one pull request, tracked as one markdown file)
that was captured as a rough stub lands in the **needs-brainstorm** state (a proposed change
with neither a spec nor a trivial mark; it needs a design conversation first). Grooming is what
moves it out of that state and into **build-ready** (a proposed change that has a spec or is
marked trivial and whose dependencies are all merged). The output of grooming is a **spec** (the
design document a change links to, written before building) — or, for genuinely mechanical work,
a `trivial` mark that skips the spec.

## Grooming a stub with a conversation

The interactive way to groom is the groom-next skill — a **skill** being a named, reusable
instruction set an agent loads for one job. It selects the next `needs-brainstorm`
stub and designs it *with you*, in a back-and-forth, until the design is settled and a spec is
written. Selection is automatic — it picks the next eligible stub deterministically — but the
design conversation is not: it is a real dialogue, the same way capturing a fully-designed change
is. It writes markdown only; it never touches branches or code.

When the dialogue settles, the stub comes out the other side as a build-ready change with a
linked spec, and the [board](./capturing-work.md) (the generated overview of every change and
its state, never edited by hand) shows it ready to build. From here the autonomous build can pick
it up — see
[Building without supervision](./building-without-supervision.md).

## Grooming a batch with no human

When you have a pile of rough stubs and do not want to sit through each conversation, the
auto-groom skill drains the ones you have marked auto-groomable with **no human in the room**.
Each stub is designed by a default-biased self-brainstorm, and — this is the safety rail — every
resulting design is gated by an **adversarial critic**: a separately-run reviewer whose only job
is to attack the draft. A design that survives the critic exits as a build-ready spec; one the
critic rejects (or that comes out genuinely trivial) is handled accordingly, and anything the
loop cannot settle confidently is handed back to your interactive queue rather than forced
through.

Two things are deliberately never autonomous: killing a change and deferring one. Those are
judgment calls that stay with you. Like interactive grooming, auto-groom writes markdown only —
never branches, worktrees, or code.

## Having a consultant write the spec

By default the design step (in both the capture and groom skills) runs the standard brainstorming
method: the dialogue and the resulting spec are both produced inline, at whatever model your
session is running. **The consultant brainstorm is an opt-in alternative** that keeps the design
conversation exactly where it is — with you, inline, at the session's model — but adds a pinned,
high-tier design **consultant** (an **agent** — a separately launched worker with its own
context, pinned to a model and effort) that authors, or audits, the final spec once the dialogue
has settled. The consultant fires once, at the end: it either hands back an authored spec or
returns critique concerns that send you back into the conversation.

There are two ways to opt in:

- **Per-invocation, spoken.** Just say so when you run the interactive skill — for example, ask
  it to "have a consultant write the spec." This wins for that one run, regardless of any
  configured default.
- **Durable, in config.** Bind the `brainstorm` role in your committed or global config, and
  every design step — capture and groom alike — goes through the consultant. The binding is a
  skill named in the `skills:` map:

  ```yaml
  skills:
    brainstorm: docket-brainstorm
  ```

If the consultant's separate worker cannot be launched on this machine (its agents not synced to
this machine, or any other per-machine gap), the consultant brainstorm **degrades to running the
whole flow inline at the session model, with a prominent warning** — no worse than never having
opted in.

**Pinning the whole conversation, not just authorship.** The consultant pins *authorship* only;
the dialogue and option generation still run at whatever model your session is on. To pin the
*entire* design conversation to a stronger (or cheaper) model, no new machinery is needed:
capture the idea as a stub in whichever session it strikes you (skip straight past the design
step — the stub lands at needs-brainstorm), then run the groom skill from a session set to the
model you want. That session does the full design conversation at its own model, and can still
opt into consultant authorship on top.

## Shaping the conversation to your reader (`dummy_mode`)

Docket talks to you in the vocabulary of the repository it is running in. If that repo is a pile
of bash and you read YAML but not bash, or it is Terraform and you have never seen a state file,
every design question and every run report arrives one translation short — and you end up asking
"say that more simply" over and over. `dummy_mode` moves that translation to the source: you
describe the reader once, and docket's **human-facing** prose is written for that reader from
then on. This whole guide is written under that default reader.

```yaml
dummy_mode:
  enabled: true
  persona: "Comfortable with git and YAML, cannot read bash. Explain scripts by outcome."
  surfaces: all   # or a subset — see the tokens below
```

`persona` is free text and must be **one quoted line**. A YAML block scalar (`>` or `|`) and a
`#` inside the value — quoted or not — are both hard configuration errors, because the config
reader is line-oriented and strips from the first `#`. A `#` *after* the closing quote is an
ordinary trailing comment and is fine. Leave it blank (or omit it) to get the shipped default: a
mid-level engineer who knows architecture, has working-level fluency in any one language, and is
told every project-internal term with a gloss on first use.

`enabled` is `false` by default, and the key is global-able: set it per-repo, in your global
config, or in the machine-local layer (the layer model lives in
[Repo config](../install/config-layers.md)). It is *primarily* a per-repo setting,
since the same person is an expert in one repo's domain and a novice in another's.

**Five surfaces are eligible**, in two modes:

| Token | Covers | Mode |
|---|---|---|
| `dialogue` | design and groom conversation, and any human-present prompt | **replace** |
| `reports` | the prose a skill prints back to you at the end of a run | **replace** |
| `results` | the results file a change writes on close-out | **additive** |
| `change-sections` | skill-authored sections in a change file (`## Run halted`, `## Finalize blocked`, …) | **additive** |
| `pr` | the pull-request body | **additive** |

**Replace** means the prose itself is written calibrated to your persona — there is no second,
technical copy, because the decisions and the artifacts behind it are unchanged. **Additive**
means the artifact keeps its full technical content and *gains* an authored `### In plain terms`
block alongside it, written at the same moment as its parent.

**Agent-facing artifacts are never simplified.** The plan (the task-by-task breakdown a build
follows, written on the feature branch), spec files, learnings findings (the loop's memory of
lessons from past builds, curated by a human), build evidence (the committed record of that gate
run, read by the reviewer), and script contracts keep full technical density, and an
`### In plain terms` block is never a decision input — reconcile (a check at build time that the
change is still worth doing and its assumptions still hold, before any code is written), review,
planning, and every build worker read the technical content only. Simplifying what the loop reads
would degrade the loop.

You can also ask for it **ad hoc**: say "enable dummy mode" in a session and the same rules apply
for the rest of that session, using the same configured persona, writing nothing to disk. The
reverse request turns it back off.

### A gallery of personas

Five worked examples, spanning different application types and languages. Compose your own along
the axes they use.

**1. Shell/CLI tooling repo (docket itself) — PM-technical reader**

```yaml
dummy_mode:
  enabled: true
  persona: "Comfortable with git, GitHub PRs, and YAML. Cannot read bash or awk — explain script behavior by outcome, never by code. Does not know docket's internal vocabulary (worktree, CAS push, claim lease, orphan branch) — use each term only with a one-clause gloss."
```

Effect: a design question like "should the CAS retry re-run preflight before the re-push?"
becomes "when two sessions save at the same time, one loses — should the loser automatically
re-sync and retry, or stop and ask you?"

**2. Python data-pipeline repo — analyst reader**

```yaml
dummy_mode:
  enabled: true
  persona: "Data analyst: fluent in SQL and pandas, reads simple Python. No infrastructure background — Docker, Airflow scheduling internals, and IAM are unknown terms. Frame trade-offs in terms of data freshness, correctness, and cost."
  surfaces: [dialogue, reports]
```

**3. TypeScript/React web app — designer/founder reader**

```yaml
dummy_mode:
  enabled: true
  persona: "Non-engineer founder: thinks in user flows and screens, not components or state. Knows what an API is, not what REST vs GraphQL implies. Avoid all TypeScript jargon; describe changes by what the user sees and what could break for them."
```

**4. Terraform/infrastructure repo — application-developer reader**

```yaml
dummy_mode:
  enabled: true
  persona: "Backend application developer (Go, Postgres) new to infrastructure-as-code: plan vs apply, state files, and drift are new concepts. Comfortable with networking basics. Always spell out blast radius: what a change destroys or recreates."
  surfaces: [dialogue, results, pr]
```

**5. iOS/Swift app — backend-engineer reader**

```yaml
dummy_mode:
  enabled: true
  persona: "Backend engineer fluent in Java and REST APIs, new to mobile: SwiftUI, the app lifecycle, and App Store review constraints are unfamiliar. Map iOS concepts to server-side analogies where one exists; flag where the analogy breaks."
```

Compose your own along the axes those five use: **subject-matter gaps** (knows CI/CD, new to
release engineering), **language gaps** (reads Python, not bash), **tooling/vocabulary gaps**
(docket's own terms), and **framing preferences** (what dimensions trade-offs should be expressed
in).

Once a change is designed and build-ready, the autonomous loop can take it from here:
[Building without supervision](./building-without-supervision.md).
