package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// newSchemaCommand builds `docket schema`: a repository-, config-, git-, and
// network-independent read of the request/result payload schemas and allowed
// values, derived entirely from the live Go types. Like the capabilities
// bootstrap it mirrors, its RunE touches no filesystem, config loader, git, or
// network — it projects the allowed effect values and hands it to app.Schema,
// which reflects the operation-schema registry.
func newSchemaCommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "schema [<id>]",
		Short:       "Emit request/result payload schemas and allowed values; one operation by positional <id> or --operation (read-only, repository-independent)",
		Args:        cobra.MaximumNArgs(1),
		Annotations: capability("schema", EffectRead),
		RunE: func(c *cobra.Command, args []string) error {
			op, _ := c.Flags().GetString("operation")
			// The operation id may be given positionally or via --operation
			// (change 0360). Both at once must agree: a disagreement refuses as
			// invalid arguments rather than silently picking one.
			if len(args) == 1 {
				if op != "" && op != args[0] {
					return fmt.Errorf("schema: positional id %q and --operation %q disagree", args[0], op)
				}
				op = args[0]
			}
			effects := sortedEffectStrings()
			if op == "" {
				doc, err := app.Schema(effects)
				if err != nil {
					return fmt.Errorf("schema construction failed: %w", err)
				}
				setResult(doc)
				return nil
			}
			doc, ok, err := app.SchemaFor(op, effects)
			if err != nil {
				return fmt.Errorf("schema construction failed: %w", err)
			}
			if !ok {
				setResult(app.SchemaUnknownOperation(op))
				return nil
			}
			setResult(doc)
			return nil
		},
	}
	cmd.Flags().String("operation", "", "emit only this operation `id` (e.g. change.create); same as the positional <id>")
	return cmd
}

// sortedEffectStrings projects the allowed effect values (allEffects) into
// the sorted string slice the schema surface publishes as its `effects`
// vocabulary. It is derived from allEffects, never a second hand-maintained
// list, so a new effect flows through automatically.
func sortedEffectStrings() []string {
	out := make([]string, 0, len(allEffects))
	for e := range allEffects {
		out = append(out, string(e))
	}
	sort.Strings(out)
	return out
}
