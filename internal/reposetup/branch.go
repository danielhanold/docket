package reposetup

// MetadataBranchName is the ONE spelling of the fixed metadata branch (change
// 0363). Go v1 supports a single repository topology: planning metadata lives
// on this orphan branch while code lands on the independently resolved
// integration_branch. The constant is owned beside the topology classifier —
// the authority on what the branch means — and is the only spelling production
// code may use; no configuration selects another value.
const MetadataBranchName = "docket"
