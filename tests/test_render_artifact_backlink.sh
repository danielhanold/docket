#!/usr/bin/env bash
# tests/test_render_artifact_backlink.sh — golden + idempotency + insert/replace + fallback +
# frontmatter-extraction + arg/config-failure tests for scripts/render-artifact-backlink.sh
# (change 0136). Plain bash; hermetic fixtures; no network/gh.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT/scripts/render-artifact-backlink.sh"
fail=0
ok(){ printf 'ok   - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

make_config_stub(){ # $1 dir
  cat > "$1/docket-config.sh" <<'EOF'
#!/usr/bin/env bash
echo "METADATA_BRANCH=docket"
echo "CHANGES_DIR=docs/changes"
EOF
  chmod +x "$1/docket-config.sh"
}

# Render in GitHub mode with hermetic config + explicit --repo.
render(){ # $1 artifact-file  $2 change-file ; extra args follow
  local af="$1" cf="$2"; shift 2
  DOCKET_CONFIG="$tmp/docket-config.sh" GIT=git \
    bash "$SCRIPT" --artifact-file "$af" --change-file "$cf" --repo danielhanold/docket "$@"
}

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
make_config_stub "$tmp"

# A change file at an active/ canonical path. Its BODY opens a line with `title:` to lock the
# frontmatter-anchored read (an unanchored reader could stray into the body).
mkdir -p "$tmp/docs/changes/active" "$tmp/docs/changes/archive"
cf_active="$tmp/docs/changes/active/0136-artifact-backlinks.md"
cat > "$cf_active" <<'EOF'
---
id: 136
slug: artifact-backlinks
title: Artifact back-links — a link home
status: in-progress
---

## Why

title: this body line opens with the field name and must NOT be read as the title.
EOF

# ---- Case A: insert into a spec that has NO block yet (GitHub mode, active path) ----
spec="$tmp/spec.md"
printf '# Some spec heading\n\nSpec body.\n' > "$spec"
render "$spec" "$cf_active" || no "case A: render exits 0"
cat > "$tmp/golden_A.md" <<'EOF'
<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0136 — Artifact back-links — a link home](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0136-artifact-backlinks.md)**
<!-- docket:backlink:end -->

# Some spec heading

Spec body.
EOF
if diff -u "$tmp/golden_A.md" "$spec" >/dev/null; then ok "case A: block inserted at top, GitHub mode, active path"; else no "case A: block inserted at top, GitHub mode, active path"; diff -u "$tmp/golden_A.md" "$spec" || true; fi

# ---- Case B: idempotency — a second run is byte-identical ----
cp "$spec" "$tmp/spec_before.md"
render "$spec" "$cf_active" || no "case B: second render exits 0"
if diff -u "$tmp/spec_before.md" "$spec" >/dev/null; then ok "case B: second render is byte-identical (idempotent)"; else no "case B: second render is byte-identical (idempotent)"; fi

# ---- Case C: replace an existing (stale) block in place ----
plan="$tmp/plan.md"
cat > "$plan" <<'EOF'
<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0136 — STALE TITLE](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/OLD.md)**
<!-- docket:backlink:end -->

# Plan heading

Plan body.
EOF
render "$plan" "$cf_active" || no "case C: render exits 0"
cat > "$tmp/golden_C.md" <<'EOF'
<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0136 — Artifact back-links — a link home](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0136-artifact-backlinks.md)**
<!-- docket:backlink:end -->

# Plan heading

Plan body.
EOF
if diff -u "$tmp/golden_C.md" "$plan" >/dev/null; then ok "case C: stale block replaced in place"; else no "case C: stale block replaced in place"; diff -u "$tmp/golden_C.md" "$plan" || true; fi

# ---- Case D: archive/ canonical path is reflected in the URL ----
cf_arch="$tmp/docs/changes/archive/2026-07-30-0136-artifact-backlinks.md"
cp "$cf_active" "$cf_arch"
results="$tmp/results.md"
printf '# Results\n' > "$results"
render "$results" "$cf_arch" || no "case D: render exits 0"
grep -qF 'blob/docket/docs/changes/archive/2026-07-30-0136-artifact-backlinks.md' "$results" \
  && ok "case D: archive path reflected in URL" || no "case D: archive path reflected in URL"

# ---- Case E: bare-path fallback when no --repo and a non-GitHub remote ----
bare="$tmp/bare"; mkdir -p "$bare"; ( cd "$bare" && git init -q && git remote add origin https://example.com/x.git )
art_fb="$bare/spec.md"; printf '# Heading\n' > "$art_fb"
DOCKET_CONFIG="$tmp/docket-config.sh" GIT=git bash "$SCRIPT" --artifact-file "$art_fb" --change-file "$cf_active" || no "case E: render exits 0"
grep -qF '> ↩ **Change 0136 — Artifact back-links — a link home** — `docs/changes/active/0136-artifact-backlinks.md`' "$art_fb" \
  && ok "case E: bare-path fallback (no GitHub remote)" || { no "case E: bare-path fallback (no GitHub remote)"; sed -n '1,4p' "$art_fb"; }

# ---- Case F: arg validation ----
DOCKET_CONFIG="$tmp/docket-config.sh" bash "$SCRIPT" --change-file "$cf_active" >/dev/null 2>&1; [ $? -eq 2 ] && ok "missing --artifact-file exits 2" || no "missing --artifact-file exits 2"
DOCKET_CONFIG="$tmp/docket-config.sh" bash "$SCRIPT" --artifact-file "$spec" >/dev/null 2>&1; [ $? -eq 2 ] && ok "missing --change-file exits 2" || no "missing --change-file exits 2"
DOCKET_CONFIG="$tmp/docket-config.sh" bash "$SCRIPT" --artifact-file "$tmp/nope.md" --change-file "$cf_active" >/dev/null 2>&1; [ $? -eq 2 ] && ok "nonexistent --artifact-file exits 2" || no "nonexistent --artifact-file exits 2"
DOCKET_CONFIG="$tmp/docket-config.sh" bash "$SCRIPT" --artifact-file "$spec" --change-file "$cf_active" --bogus >/dev/null 2>&1; [ $? -eq 2 ] && ok "unknown flag exits 2" || no "unknown flag exits 2"

# ---- Case G: config-resolution failure exits 1 ----
printf '#!/usr/bin/env bash\nexit 3\n' > "$tmp/badconfig.sh"; chmod +x "$tmp/badconfig.sh"
art_g="$tmp/g.md"; printf '# H\n' > "$art_g"
DOCKET_CONFIG="$tmp/badconfig.sh" bash "$SCRIPT" --artifact-file "$art_g" --change-file "$cf_active" --repo danielhanold/docket >/dev/null 2>&1
[ $? -eq 1 ] && ok "config failure exits 1" || no "config failure exits 1"

exit $fail
