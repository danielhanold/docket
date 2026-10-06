package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/layout"
)

// OperationInstructions is the protocol operation id of `docket instructions`.
const OperationInstructions = "instructions"

// The sections `docket instructions --section` selects from a private
// repository's instructions file: the whole file, the managed dispatch block
// with its marker lines, or everything outside that block (the promoted
// lessons).
const (
	InstructionsSectionAll      = ""
	InstructionsSectionDispatch = "dispatch"
	InstructionsSectionLessons  = "lessons"
)

// instructionsBlockName is the managed block the dispatch section selects —
// the block reposeed.PlanPrivate plans into the private file.
const instructionsBlockName = "dispatch"

// cursorStdinLimit bounds the Cursor hook payload read from stdin.
const cursorStdinLimit = 1 << 20

// ReadPrivateInstructions reads a private repository's instructions file
// (<git-common-dir>/dckt/AGENTS.md) for any directory inside any of its
// worktrees. It walks up from dir to the first `.git` entry and resolves the
// common dir from the filesystem alone — no git process — so a session-start
// hook pays almost nothing.
//
// Outside git, or in a shared repository: (nil, false, nil). Private without
// the file: (nil, true, nil). Any other probe or read error is returned, never
// reported as "nothing".
func ReadPrivateInstructions(dir string) (content []byte, private bool, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, false, err
	}
	root, found, err := findDotGit(abs)
	if err != nil || !found {
		return nil, false, err
	}
	common, ok, err := layout.CommonDirOf(root)
	if err != nil || !ok {
		return nil, false, err
	}
	mode, err := layout.Detect(common)
	if err != nil {
		return nil, false, err
	}
	if mode != layout.Private {
		return nil, false, nil
	}
	content, err = os.ReadFile(layout.PrivateInstructionsPath(common))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, true, nil
	}
	if err != nil {
		return nil, true, err
	}
	return content, true, nil
}

// findDotGit returns the nearest directory at or above abs that holds a `.git`
// entry. Not-exist moves one level up (stopping at the filesystem root); any
// other probe error is returned.
func findDotGit(abs string) (string, bool, error) {
	for d := abs; ; {
		_, err := os.Lstat(filepath.Join(d, ".git"))
		if err == nil {
			return d, true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", false, fmt.Errorf("instructions: probing %s: %w", filepath.Join(d, ".git"), err)
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false, nil
		}
		d = parent
	}
}

// SelectInstructionsSection selects one section of an instructions file. All:
// the content verbatim. Dispatch: the `dispatch` managed block with its marker
// lines. Lessons: everything outside that block. A whitespace-only dispatch or
// lessons result is nil. Malformed markers and an unknown section are errors.
func SelectInstructionsSection(content []byte, section string) ([]byte, error) {
	switch section {
	case InstructionsSectionAll:
		return content, nil
	case InstructionsSectionDispatch, InstructionsSectionLessons:
	default:
		return nil, fmt.Errorf("instructions: unknown section %q (want %q or %q)",
			section, InstructionsSectionDispatch, InstructionsSectionLessons)
	}
	doc, err := document.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("instructions: %w", err)
	}
	src := doc.Source()
	var out []byte
	block, ok := doc.Block(instructionsBlockName)
	switch {
	case section == InstructionsSectionDispatch && ok:
		out = src[block.Start.Start:block.End.End]
	case section == InstructionsSectionDispatch:
		out = nil
	case ok:
		out = append(append([]byte(nil), src[:block.Start.Start]...), src[block.End.End:]...)
	default:
		out = src
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}
	return out, nil
}

// claudeSessionStart is Claude Code's SessionStart hook output document.
type claudeSessionStart struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

// cursorSessionStart is Cursor's sessionStart hook output document.
type cursorSessionStart struct {
	AdditionalContext string `json:"additional_context"`
}

// ClaudeSessionStartOutput wraps content as one Claude Code SessionStart hook
// output line:
// {"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":…}}.
// Blank content gives nil.
func ClaudeSessionStartOutput(content []byte) []byte {
	if len(bytes.TrimSpace(content)) == 0 {
		return nil
	}
	var doc claudeSessionStart
	doc.HookSpecificOutput.HookEventName = "SessionStart"
	doc.HookSpecificOutput.AdditionalContext = string(content)
	return encodeHookLine(doc)
}

// CursorSessionStartOutput wraps content as one Cursor sessionStart hook output
// line: {"additional_context":…}. Blank content gives nil.
func CursorSessionStartOutput(content []byte) []byte {
	if len(bytes.TrimSpace(content)) == 0 {
		return nil
	}
	return encodeHookLine(cursorSessionStart{AdditionalContext: string(content)})
}

// encodeHookLine renders v as one JSON line with HTML escaping off, so the
// managed block's `<!--` markers reach the harness unchanged. The encoder
// terminates the line with "\n".
func encodeHookLine(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil
	}
	return buf.Bytes()
}

// CursorProjectDir locates the repository a Cursor sessionStart hook runs for:
// CURSOR_PROJECT_DIR when non-empty; else the first absolute workspace_roots
// entry of the stdin JSON (a `file://` prefix stripped, at most 1 MiB read);
// else "". A nil stdin is skipped. The working directory is never consulted.
func CursorProjectDir(getenv func(string) string, stdin io.Reader) string {
	if dir := getenv("CURSOR_PROJECT_DIR"); dir != "" {
		return dir
	}
	if stdin == nil {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(stdin, cursorStdinLimit))
	if err != nil {
		return ""
	}
	var payload struct {
		WorkspaceRoots []string `json:"workspace_roots"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	for _, root := range payload.WorkspaceRoots {
		root = strings.TrimPrefix(root, "file://")
		if filepath.IsAbs(root) {
			return root
		}
	}
	return ""
}

// Stable machine reasons for the instructions operation's failure results.
// Message is explanatory prose and must not be parsed.
const (
	ReasonInstructionsReadFailed     = "instructions-read-failed"
	ReasonInstructionsMarkersInvalid = "instructions-markers-invalid"
	ReasonInstructionsUnknownSection = "instructions-unknown-section"
)

// InstructionsResult is the --json form of `docket instructions`.
type InstructionsResult struct {
	Envelope
	Private bool   `json:"private"`
	Section string `json:"section,omitempty"`
	Content string `json:"content"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// HumanText is the selected content itself.
func (r InstructionsResult) HumanText() string { return r.Content }

// Instructions is the --json form: applied with the selected content, or a
// failure result carrying a stable reason and a message — invalid-input for an
// unknown section, external-failed when the file cannot be read, invalid-state
// when its markers are malformed. It is a read: Failure, which diagnoses a
// failed transaction, is never set.
func Instructions(dir, section string) InstructionsResult {
	fail := func(result Result, private bool, reason string, err error) InstructionsResult {
		return InstructionsResult{
			Envelope: NewEnvelope(OperationInstructions, result),
			Private:  private,
			Section:  section,
			Reason:   reason,
			Message:  err.Error(),
		}
	}
	switch section {
	case InstructionsSectionAll, InstructionsSectionDispatch, InstructionsSectionLessons:
	default:
		_, err := SelectInstructionsSection(nil, section)
		return fail(ResultInvalidInput, false, ReasonInstructionsUnknownSection, err)
	}
	content, private, err := ReadPrivateInstructions(dir)
	if err != nil {
		return fail(ResultExternalFailed, private, ReasonInstructionsReadFailed, err)
	}
	content, err = SelectInstructionsSection(content, section)
	if err != nil {
		return fail(ResultInvalidState, private, ReasonInstructionsMarkersInvalid, err)
	}
	return InstructionsResult{
		Envelope: NewEnvelope(OperationInstructions, ResultApplied),
		Private:  private,
		Section:  section,
		Content:  string(content),
	}
}
