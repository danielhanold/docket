package harness

// RecursionGuard is the self-recursion guard injected into every generated
// wrapper by every renderer. It prohibits exactly one edge — docket-X
// dispatching another docket-X for the assignment it already holds — and
// explicitly preserves required dispatches to different agents. It relies on
// the generated literal name, never on a name: field, a skill preload, or
// inference from surrounding prose.
func RecursionGuard(name string) string {
	return "You are already running as `" + name + "`. Carry out this wrapper's assigned charter " +
		"directly. Do not dispatch another `" + name + "` merely to perform the current " +
		"assignment. Dispatches to different agents explicitly required by the active charter " +
		"remain required."
}
