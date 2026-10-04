# Guide

How do I do the thing? Each page below takes one goal end to end, and its title names the docket
component it is about. Start with the five steps for the shape of the work, then read the page
for whatever step you are on. Installing docket and configuring it is its own section:
[Install and configure](../install/README.md).

- [The five steps](five-steps.md) — the handful of steps you run by name when you work with
  docket, and which page covers each one in full.
- [Change: Capturing work that outlives the session](capturing-work.md) — turn an idea into a
  tracked unit of work that survives the session it occurred to you in, so you (or an autonomous
  build run) can pick it up weeks later without re-explaining it.
- [Groom: Designing before building](designing-before-building.md) — take a half-formed stub through
  the step between capturing and building, until an autonomous run can implement it without
  guessing.
- [Build: Building without supervision](building-without-supervision.md) — hand a designed piece of
  work to implement-next and get back an open pull request, and learn what it checks, how hard
  it works on each part, and where it stops and waits for you.
- [Suite gate: Proving the build](proving-the-build.md) — how a finished branch earns the right to be
  reviewed and merged: the test run that certifies it and the durable record that run leaves behind.
- [Review: Reviewing before the human does](reviewing-before-the-human.md) — what happens to a
  finished branch between its last build commit and the pull request you read, and who touches it on
  the way.
- [Finalize: Landing changes safely](landing-changes.md) — how an approved change gets from an open
  pull request into your mainline and out of your backlog, hands-off across a whole set of changes.
- [Status: Keeping the backlog honest](keeping-the-backlog-honest.md) — tell whether your backlog
  still reflects reality, and fix it when it does not: the read-only status report, the maintenance
  sweep, reclaiming expired claims, and recovering a halted run.
- [ADRs and learnings: Remembering why](remembering-why.md) — where docket keeps the decisions it
  made and the lessons it learned, how you record a finding, and when a lesson belongs in your
  always-in-context instructions instead.
- [Metadata branch: Where the metadata lives](where-the-metadata-lives.md) — where docket keeps its
  planning records and why they sit apart from your code, and the commands that set a repository
  up, check it, and repair it.
