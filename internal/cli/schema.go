package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// newSchemaCommand builds `docket schema`: a repository-, config-, git-, and
// network-independent read of the request/result payload schemas and closed
// vocabularies, derived entirely from the live Go types. Like the capabilities
// bootstrap it mirrors, its RunE touches no filesystem, config loader, git, or
// network — it projects the closed effect vocabulary and hands it to app.Schema,
// which reflects the operation-schema registry.
func newSchemaCommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "schema",
		Short:       "Emit request/result payload schemas and closed vocabularies (read-only, repository-independent)",
		Args:        cobra.NoArgs,
		Annotations: capability("schema", EffectRead),
		RunE: func(c *cobra.Command, _ []string) error {
			op, _ := c.Flags().GetString("operation")
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
	cmd.Flags().String("operation", "", "emit only this operation `id` (e.g. change.create)")
	return cmd
}

// sortedEffectStrings projects the closed effect vocabulary (allEffects) into
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
