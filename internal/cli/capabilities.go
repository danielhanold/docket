package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/buildinfo"
)

// newCapabilitiesCommand builds `docket capabilities`: the one repository-,
// config-, asset-, and network-independent bootstrap. Its RunE touches no
// filesystem, no config loader, no git, no network — it walks the already-
// assembled tree it is itself registered in and hands the projection to the
// presenter. A walker validation failure is an internal error: the tree this
// binary shipped is inconsistent, and the answer is fail-closed, not partial.
func newCapabilitiesCommand(info buildinfo.Info, setResult func(app.OperationResult)) *cobra.Command {
	return &cobra.Command{
		Use:         "capabilities",
		Short:       "Emit this binary's complete executable command catalog (read-only, repository-independent)",
		Args:        cobra.NoArgs,
		Annotations: capability("capabilities", EffectRead),
		RunE: func(c *cobra.Command, _ []string) error {
			entries, err := collectCapabilities(c.Root())
			if err != nil {
				return fmt.Errorf("capability catalog construction failed: %w", err)
			}
			setResult(app.Capabilities(info, toAppCommands(entries), globalInvocation(c.Root())))
			return nil
		},
	}
}

// toAppCommands converts the cli-side CapabilityEntry projection into the
// app-side carrier type; the two share JSON tags so the conversion is a pure
// field copy across the package boundary app cannot cross the other way.
func toAppCommands(entries []CapabilityEntry) []app.CapabilityCommand {
	out := make([]app.CapabilityCommand, len(entries))
	for i, e := range entries {
		out[i] = app.CapabilityCommand{
			ID:        e.ID,
			Argv:      e.Argv,
			Signature: e.Signature,
			Effects:   e.Effects,
		}
	}
	return out
}

// globalInvocation projects the root's visible persistent flags into the
// document-level global block (today exactly --json), so the invocation-wide
// flags are stated once rather than restated on every command entry.
func globalInvocation(root *cobra.Command) app.GlobalInvocation {
	var flags []app.GlobalFlag
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		flags = append(flags, app.GlobalFlag{
			Name:    f.Name,
			Type:    f.Value.Type(),
			Default: f.DefValue,
			Usage:   f.Usage,
		})
	})
	return app.GlobalInvocation{Flags: flags}
}
