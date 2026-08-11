# check-test-source-hygiene.sh — the pre-execution source-hygiene checker for the test suite

## Purpose

Refuses test source whose backticks the shell would **execute**. Introduced in change 0221.

A backtick in a test file runs when the **shell reads the line** — before `assert` is called,
before any helper prints anything. A verbatim-quoted guard anchor pasted into a test description
is therefore not data, it is a command, and the test then reports green on output it produced
itself. Change 0212 shipped exactly that: a multi-line double-quoted `SITES="…"` block whose
anchor text carried `git checkout .`.

The executing vector is **source evaluation**, with a second one at `eval "$2"`. It is *not* the
helper's print: parameter expansion does not re-trigger command substitution, so a backtick already
sitting in a variable's value is inert through `printf '%s' "$1"`. Normalizing the assert helpers is
drift control; this checker is the safety mechanism.

This script only ever **reads** the paths it is given. It never sources, executes, or otherwise
evaluates one — which is the whole point, since detection that requires execution is not
prevention. `tests/fixtures/hygiene/red/sentinel.sh` pins that property: it writes a marker file if
anything ever runs it, and `tests/test_assert_hygiene.sh` asserts the marker is absent after a scan
that flagged the file.

## Usage

```
check-test-source-hygiene.sh <path>...
```

| Argument | Required | Description |
|---|---|---|
| `<path>...` | yes | One or more readable regular files to scan. Each is scanned independently: quote and heredoc state never bleeds across a file boundary. |

`-h` / `--help` prints this script's leading comment block. `--` ends option parsing, so a path
beginning with `-` can be passed.

## Behavior

### Rule (b) — the quoting scan

A whole-file shell-quoting state machine (`NORMAL`, `SQ`, `DQ`, plus heredoc-body mode) runs over
every path handed in. **State is carried across lines, not reset per line.** That is load-bearing,
not an optimization: the 0212 incident lived inside a multi-line double-quoted assignment, and a
line-local scanner cannot see it — `tests/fixtures/hygiene/red/dq_sites_block.sh` exists to fail if
anyone rewrites the scanner line-locally.

Each violation prints one line to **stdout**:

```
<path>:<line>: <CLASS>: <message>
```

At most one line per `(line, class)` pair — two backticks delimiting one substitution are one
defect — emitted in line order.

| Class | What it catches | Why it executes |
|---|---|---|
| `NORMAL-BACKTICK` | Legacy `` `cmd` `` substitution in unquoted code position. | Runs at source evaluation. `$(cmd)` is the house style; the tree has no live use. |
| `DQ-BACKTICK` | **Any** backtick inside double quotes — bare **or** backslash-escaped. | Bare, it runs at source evaluation. Escaped, the escape is consumed at source evaluation, so what reaches `$2` is a *bare* backtick and it runs one evaluation later, at `eval`. One class for both spellings deliberately: the escaped form is not an accepted residual, it is the same defect displaced in time. |
| `HEREDOC-BACKTICK` | A backtick in a heredoc body whose delimiter is **unquoted**. | `<<EOF` and `<<-EOF` substitute in the body. `<<'EOF'`, `<<"EOF"` and `<<\EOF` do not, and are skipped. |
| `EVAL-BACKTICK` | A backtick in the single-quoted **condition** (argument 2) of an assert-family call that is not immediately preceded by a backslash. | Source quoting protects the *first* evaluation only; `eval "$2"` re-parses the value and runs it there. The house idiom `'grep -qF "\`span\`" "$f"'` carries the backslash, survives `eval` as a literal, and stays legal. |

**The assert-family name set is derived, never enumerated.** Any function in the scanned file whose
definition body contains `eval "$2"` contributes its own name, whatever it is spelled; `assert` is
seeded as a floor, because a file may *call* the helper without defining it. A hand-list of names
would fail on the first file whose house idiom differs.

**Command position** arms the rule: a word counts as a command word at the start of a logical line,
or after `;`, `|`, `&`, `(`, `{`, or one of `then` / `do` / `else` / `elif` / `if` / `while` /
`until` / `time` / `!`. This keeps `# assert …` prose and `printf 'assert %s'` from arming it.
Argument 1 — the description — is printed through `printf '%s'` and is inert, so it is never
flagged under the eval rule.

### Rule (a) — the definition allowlist

**Not implemented in this file yet.** It lands in the same change and adds exactly one class,
`DEFN-DRIFT`. Nothing in the interface above changes when it does.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Every path scanned clean. No output. |
| 1 | At least one violation; every one of them is on stdout. |
| 2 | Usage error: no paths given, an unrecognized flag, a path that is not a readable regular file, or the scanner itself failed on a file. Diagnostics go to stderr. |

## Invariants

- **Read-only, always.** No path handed to this script is ever sourced, executed, opened for
  writing, or evaluated. The scanner reads bytes.
- **Per-file isolation.** One `awk` invocation per file, under `LC_ALL=C`. An unterminated quote or
  heredoc in one file cannot change the verdict on the next, and `substr`/`length` agree on byte
  semantics regardless of the ambient locale or which `awk` is installed.
- **It cannot flag itself**, by construction rather than by an exception entry. The scanner writes
  no literal backtick in executable position anywhere — every one it needs is built with
  `sprintf("%c", 96)` — and its `awk` program lives in a single-quoted literal. Two rules bind an
  editor of that literal: no apostrophe may appear inside it, not even in prose (one would close
  the literal), and no literal backtick either (a backslash-escaped backtick in an `awk` string is
  an undefined escape — `gawk` warns and POSIX does not define it).
- **No allowlist, no exception entries.** Scope is by the path list the caller passes, never by an
  in-file suppression comment. Appeasing the guard with scattered exceptions is what the change
  that introduced it forbids.

## Limitations — read these before trusting a green

- **It protects suite runs only.** `scripts/run-tests.sh` calls this synchronously over its targets
  before the first job launches, so a violation aborts a suite run with **zero** test files
  executed. A file run directly — `bash tests/test_x.sh` — **bypasses the preflight entirely**.
  That is an accepted residual, taken rather than paying for a preamble in 100+ test files.
- **It reads source, not values.** A condition assembled at runtime —
  `cond="grep $pat"; assert "d" "$cond"` — is not modeled and cannot be: the bytes `eval` finally
  sees do not exist until the test runs. The same holds for a helper reached through a variable, an
  alias, or `"$@"`.
- **`$(…)` *is* modeled as a fresh quoting context**, along with parenthesized groups nested inside
  one. That is not optional polish. Without it, the everyday `var="$(awk -v q="X" …)"` shape reads
  the inner opening quote as the *closing* quote of the outer one, and the machine runs inverted
  from there to end of file — losing real violations exactly as readily as inventing false ones.
  It was measured, not theorized: `tests/test_sync_agents_runners.sh` desynced at its
  `_between="$(awk -v q="…"` line and produced 61 phantom heredoc hits over the following 300
  lines. `tests/fixtures/hygiene/green/nested_substitution.sh` pins both halves — remove either
  frame and it reddens.
  The residual runs the other way: a `)` that is not a real close-paren pops the frame early. A
  `case` pattern label inside a command substitution is the shape that would do it. The tests tree
  has none today, and one would surface as a burst of misclassified hits — loud — rather than as a
  silent miss.
- **`<<X` is read as a heredoc only in unquoted code position.** `<<<X` is a here-string and is
  never consumed as one (the suite has roughly two thousand of them). An arithmetic left-shift
  inside `$(( … ))` is not modeled; the tests tree has no such site, and one would surface as a
  phantom heredoc — a loud false positive — rather than as a silent miss.
- **Fixtures are red on purpose.** `tests/fixtures/hygiene/red/*.sh` are hazardous by construction.
  A caller that scans the whole `tests/` tree must exclude `tests/fixtures/`; the runner's own
  `tests/test_*.sh` glob already does, which is why the fixtures live outside it.

## Related

- `tests/test_assert_hygiene.sh` — the regression test, driving every fixture in both directions.
- `tests/fixtures/hygiene/red/*.sh`, `tests/fixtures/hygiene/green/*.sh` — the executable
  specification of the rules above.
- `scripts/run-tests.md` — the runner contract, including the preflight wiring and its exit code.
