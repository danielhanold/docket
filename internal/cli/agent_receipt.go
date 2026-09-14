package cli

import (
	"github.com/danielhanold/docket/internal/app"
	"github.com/spf13/cobra"
)

func newAgentCheckReceiptCommand(setResult func(app.OperationResult)) *cobra.Command {
	var req app.CheckReceiptRequest
	c := &cobra.Command{Use: "check-receipt", Short: "Validate a captured native gate response", Args: cobra.NoArgs, Annotations: capability(app.OperationAgentCheckReceipt, EffectRead), RunE: func(*cobra.Command, []string) error { setResult(app.CheckAgentReceipt(req)); return nil }}
	c.Flags().StringVar(&req.Operation, "operation", "", "captured gate operation")
	c.Flags().StringVar(&req.Assignment, "assignment", "", "absolute assignment file")
	c.Flags().StringVar(&req.SHA256, "sha256", "", "assignment sha256")
	c.Flags().StringVar(&req.Stdout, "stdout", "", "file containing original stdout")
	c.Flags().StringVar(&req.Stderr, "stderr", "", "file containing original stderr")
	c.Flags().IntVar(&req.ExitCode, "exit-code", 0, "original process exit code")
	for _, f := range []string{"operation", "assignment", "sha256", "stdout"} {
		_ = c.MarkFlagRequired(f)
	}
	return c
}
