# Groom: Designing before building

Some work you capture is fully thought through; most of it is a rough idea you jotted down to
get it out of your head. This page is about the step in between capturing and building: turning a
half-formed stub into something an autonomous run can pick up and implement without guessing.
By the end you can groom a stub interactively, let docket groom a batch of stubs on its own under
an adversarial check, and have a high-tier consultant write the final design document.

A **change** (one unit of planned work, roughly one pull request, tracked as one markdown file)
that was captured as a rough stub lands in the **needs-grooming** state (a proposed change
with neither a spec nor a trivial mark; it needs a design conversation first). Grooming is what
moves it out of that state and into **build-ready** (a proposed change that has a spec or is
marked trivial and whose dependencies are all `done`). The output of grooming is a **spec** (the
design document a change links to, written before building) — or, for genuinely mechanical work,
a `trivial` mark that skips the spec.

## Grooming a stub with a conversation

The interactive way to groom is the groom-next skill — a **skill** being a named, reusable
instruction set an agent loads for one job. It selects the next `needs-grooming`
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
groomer cannot settle confidently is handed back to your interactive queue rather than forced
through.
Such a stub shows as **auto-groom blocked — needs you** on the board. Once you have supplied the
missing context you can groom it yourself, or re-enable it so the autonomous groomer picks it up
again — the interactive groom skill offers both.

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
has settled. The consultant is the `docket-brainstorm-consultant` agent, and it fires once, at
the end: it either hands back an authored spec or returns critique concerns that send you back
into the conversation.

You opt in per run, by asking: when you run the capture or groom skill, say something like "have
a consultant write the spec." The request applies to that one run only; there is no setting that
turns it on for every run.

If the consultant's separate worker cannot be launched on this machine (its agent not installed
by `docket install`, or any other per-machine gap), the consultant brainstorm **degrades to running the
whole flow inline at the session model, with a prominent warning** — no worse than never having
opted in.

**Pinning the whole conversation, not just authorship.** The consultant pins *authorship* only;
the dialogue and option generation still run at whatever model your session is on. To pin the
*entire* design conversation to a stronger (or cheaper) model, no new machinery is needed:
capture the idea as a stub in whichever session it strikes you (skip straight past the design
step — the stub lands at needs-grooming), then run the groom skill from a session set to the
model you want. That session does the full design conversation at its own model, and can still
opt into consultant authorship on top.

Once a change is designed and build-ready, implement-next can take it from here:
[Building without supervision](./building-without-supervision.md).
