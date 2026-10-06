package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// The --hook dialects `docket instructions` wraps its output for.
const (
	instructionsHookClaude = "claude"
	instructionsHookCursor = "cursor"
)

// newInstructionsCommand builds `docket instructions`: a read-only print of a
// private repository's agent instructions (.git/dckt/AGENTS.md), and nothing
// anywhere else. Its plain and hook forms print raw bytes rather than a result
// document, so they report through setRaw; only --json goes through setResult.
//
// The hook modes are what a harness runs at every session start, so they never
// fail a session: a runtime error is silence and exit 0. An invalid flag value
// is still an argument error (exit 2) in every mode.
func newInstructionsCommand(stdin io.Reader, jsonMode func() bool, setResult func(app.OperationResult), setRaw func(out []byte, errText string, code int)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instructions",
		Short: "Print this private repository's agent instructions (.git/dckt/AGENTS.md); nothing elsewhere",
		Long: "Print this private repository's agent instructions — the file .git/dckt/AGENTS.md — from\n" +
			"any of its worktrees. Outside a private repository it prints nothing and exits 0.\n\n" +
			"--section dispatch prints only the managed dispatch block; --section lessons prints\n" +
			"everything outside it. --hook claude or --hook cursor wraps the output as that\n" +
			"harness's session-start hook document; the Cursor form locates the repository from\n" +
			"CURSOR_PROJECT_DIR or the workspace_roots of the hook payload on stdin, never the\n" +
			"working directory. Hook modes never fail a session: an error prints nothing and\n" +
			"exits 0.",
		Args:        cobra.NoArgs,
		Annotations: capability("instructions", EffectRead),
		RunE: func(c *cobra.Command, _ []string) error {
			hook, _ := c.Flags().GetString("hook")
			section, _ := c.Flags().GetString("section")
			switch hook {
			case "", instructionsHookClaude, instructionsHookCursor:
			default:
				return fmt.Errorf("invalid --hook %q: want %q or %q", hook, instructionsHookClaude, instructionsHookCursor)
			}
			switch section {
			case app.InstructionsSectionAll, app.InstructionsSectionDispatch, app.InstructionsSectionLessons:
			default:
				return fmt.Errorf("invalid --section %q: want %q or %q", section, app.InstructionsSectionDispatch, app.InstructionsSectionLessons)
			}
			if hook != "" && jsonMode() {
				return errors.New("--json cannot be combined with --hook")
			}

			cwd, err := os.Getwd()
			if jsonMode() {
				if err != nil {
					return err
				}
				setResult(app.Instructions(cwd, section))
				return nil
			}

			dir := cwd
			if hook == instructionsHookCursor {
				// Cursor runs the hook from no particular directory: the
				// repository comes from the hook's own context or nowhere.
				dir, err = app.CursorProjectDir(os.Getenv, hookStdin(stdin)), nil
				if dir == "" {
					setRaw(nil, "", 0)
					return nil
				}
			}
			var content []byte
			if err == nil {
				content, _, err = app.ReadPrivateInstructions(dir)
			}
			if err == nil {
				content, err = app.SelectInstructionsSection(content, section)
			}
			if err != nil {
				if hook != "" {
					setRaw(nil, "", 0)
					return nil
				}
				setRaw(nil, "docket instructions: "+err.Error(), 1)
				return nil
			}
			switch hook {
			case instructionsHookClaude:
				content = app.ClaudeSessionStartOutput(content)
			case instructionsHookCursor:
				content = app.CursorSessionStartOutput(content)
			}
			setRaw(content, "", 0)
			return nil
		},
	}
	cmd.Flags().String("hook", "", "wrap the output as a session-start hook document for `harness`: claude or cursor")
	cmd.Flags().String("section", "", "print only one `section`: dispatch (the managed block) or lessons (everything else)")
	return cmd
}

// hookStdin is the reader the Cursor hook payload is read from: stdin, unless
// it is a terminal (a person running the command by hand), which would block.
func hookStdin(stdin io.Reader) io.Reader {
	f, ok := stdin.(*os.File)
	if !ok {
		return stdin
	}
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice != 0 {
		return nil
	}
	return stdin
}
