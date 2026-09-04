# The daily loop

A day of docket work is a handful of steps you run by name in an agent session opened in your
repo. This page is the map; each step links to the page that covers it in full. The shape never
changes: **you** capture and merge; the loop grooms, builds, and closes out.

A **change** here is one unit of planned work, roughly one pull request, tracked as one markdown
file — the thing each step below acts on.

## The cycle

1. **Capture — write the idea down.** Describe an idea and let docket turn it into a tracked
   change committed to the backlog. The backlog is durable, so you can capture now and build
   later; the idea does not evaporate with the session.
   → [Capturing work that outlives the session](./capturing-work.md)

2. **Groom (when needed) — design the rough ones.** If you captured a rough stub rather than a
   finished design, grooming turns it into something ready to build — interactively with you, or
   hands-free for the stubs that qualify. Skip this for changes you already designed at capture
   time. → [Designing before building](./designing-before-building.md)

3. **Build — drain the backlog unattended.** The autonomous workhorse claims the next ready
   change, checks it against current reality, plans it, builds it with tests, opens a pull
   request, and stops. It never merges. Run it as many times as you have ready work; independent
   changes drain back-to-back with no input from you.
   → [Building without supervision](./building-without-supervision.md)

4. **Review and merge — your one required checkpoint.** Read the pull request the build opened
   and merge it yourself, or approve it and let close-out merge it for you. This is where you stay
   in control; everything around it is automatable, this is not.
   → [Landing changes safely](./landing-changes.md)

5. **Close out — archive the merged work.** After the merge, the change is moved to `done`, its
   branch and worktree are cleaned up, and the board is refreshed. Do it deliberately right after
   merging, or let the periodic sweep catch it.
   → [Landing changes safely](./landing-changes.md)

Between rounds, keep the backlog honest — refresh the board and read the health checks that flag
stale claims, broken links, and stalled dependencies.
→ [Keeping the backlog honest](./keeping-the-backlog-honest.md)

In one line: **you create and merge; docket grooms, implements, and closes out.**
