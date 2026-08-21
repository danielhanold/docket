#!/bin/sh
# internal/release/downloader/install.sh — POSIX release downloader for the docket binary
# (change 0317). Rendered into every candidate bundle by internal/release/render.go, which stamps
# the DOCKET_DEFAULT_VERSION placeholder below with the bundle version.
#
# CONTRACT (Tasks 5-10 and scripts/release-smoke.sh depend on every spelling here):
#   - Runtime deps are exactly: /bin/sh, curl, tar, and one of sha256sum or OpenSSL. It invokes no
#     other interpreter and no second hashing tool: the one hashing seam is sha_file(), and it
#     NEVER executes downloaded shell. The set of forbidden interpreters/helpers is enforced by the
#     spelling ban in tests/test_release_downloader.sh and by the PATH-sandbox tests (Tasks 7-8).
#   - It verifies the archive checksum BEFORE extraction, and the extracted member BEFORE it is
#     ever moved into the bin dir or run. Unverified bytes are never executed or installed.
#   - It keeps its own ownership record and REFUSES to replace a binary it does not own. There is
#     no --force path.
#   - Exit codes: 0 success; 2 usage error (bad flag / non-absolute bin dir / bad version); 1
#     every other failure. Every failure prints a one-line actionable diagnostic to stderr.
set -u

# --- Stamped default version --------------------------------------------------------------------
# render.go replaces the placeholder on the right with the bundle version. An unrendered copy run
# without --version fails version validation below (exit 2), which is the intended behavior.
DOCKET_DEFAULT_VERSION="@DOCKET_DEFAULT_VERSION@"

# --- Diagnostics --------------------------------------------------------------------------------
die() { printf '%s\n' "install.sh: $1" >&2; exit "${2:-1}"; }

usage() {
	printf '%s\n' \
"Usage: install.sh [--version <vX.Y.Z>] [--bin-dir <absolute dir>] [--harness <name>]..." \
"" \
"Downloads the docket release binary, verifies its SHA-256 against the release" \
"checksums.txt BEFORE extraction, installs it, and records ownership so a later run" \
"will only ever replace a binary it installed." \
"" \
"  --version <v>     release version to install (default: the stamped bundle version)" \
"  --bin-dir <dir>   absolute install directory (default: \${XDG_BIN_HOME:-\$HOME/.local/bin})" \
"  --harness <name>  claude|codex|cursor|opencode; repeatable; forwarded verbatim to" \
"                    'docket install --harness <name>'" >&2
}

# --- Version validation: the Task 1 safe grammar, in POSIX case (not ERE) -----------------------
#   ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$
valid_num() {
	# 0, or a non-zero-leading run of digits.
	case $1 in
		'') return 1 ;;
		0) return 0 ;;
		0*) return 1 ;;
		*[!0-9]*) return 1 ;;
		*) return 0 ;;
	esac
}

valid_version() {
	_v=$1
	case $_v in
		v?*) ;;
		*) return 1 ;;
	esac
	_rest=${_v#v}
	case $_rest in
		*-*)
			_core=${_rest%%-*}
			_pre=${_rest#*-}
			;;
		*)
			_core=$_rest
			_pre=''
			;;
	esac
	# core must be exactly MAJOR.MINOR.PATCH (exactly two dots).
	case $_core in
		*.*.*.*) return 1 ;;
		*.*.*) ;;
		*) return 1 ;;
	esac
	_major=${_core%%.*}
	_mp=${_core#*.}
	_minor=${_mp%%.*}
	_patch=${_mp#*.}
	valid_num "$_major" || return 1
	valid_num "$_minor" || return 1
	valid_num "$_patch" || return 1
	# prerelease, when a hyphen was present: leading [0-9A-Za-z], then [0-9A-Za-z.-]*.
	case $_rest in
		*-*)
			[ -n "$_pre" ] || return 1
			_first=${_pre%"${_pre#?}"}
			case $_first in
				[0-9A-Za-z]) ;;
				*) return 1 ;;
			esac
			case $_pre in
				*[!0-9A-Za-z.-]*) return 1 ;;
			esac
			;;
	esac
	return 0
}

# --- SHA-256 seam: the ONLY hashing path. Prints the lowercase-hex digest of "$1" -------------
# sha_provider is set by the prereq probe below before this is ever called.
sha_file() {
	case $sha_provider in
		sha256sum) sha256sum "$1" | { read -r _h _rest; printf '%s\n' "$_h"; } ;;
		openssl) openssl dgst -sha256 -r "$1" | { read -r _h _rest; printf '%s\n' "$_h"; } ;;
	esac
}

# --- Ownership record reader: prints the value of key "$1" from record file "$2" -----------------
record_field() {
	while IFS= read -r _line; do
		case $_line in
			"$1="*) printf '%s\n' "${_line#*=}"; return 0 ;;
		esac
	done < "$2"
	return 1
}

# --- Flag parsing (usage errors exit 2, before any probe) ---------------------------------------
version=''
bin_dir=''
harness_args=''
while [ $# -gt 0 ]; do
	case $1 in
		--version)
			[ $# -ge 2 ] || { usage; die "--version requires a value" 2; }
			version=$2; shift 2 ;;
		--bin-dir)
			[ $# -ge 2 ] || { usage; die "--bin-dir requires a value" 2; }
			bin_dir=$2; shift 2 ;;
		--harness)
			[ $# -ge 2 ] || { usage; die "--harness requires a value" 2; }
			case $2 in
				claude|codex|cursor|opencode) ;;
				*) usage; die "unknown harness: $2 (expected claude|codex|cursor|opencode)" 2 ;;
			esac
			# Word-split intentionally at forward time: values are validated to the fixed set
			# above (alnum only), so splitting an unquoted accumulator is safe.
			harness_args="$harness_args --harness $2"
			shift 2 ;;
		--help|-h)
			usage; exit 2 ;;
		*)
			usage; die "unknown flag: $1" 2 ;;
	esac
done

[ -n "$version" ] || version=$DOCKET_DEFAULT_VERSION
[ -n "$bin_dir" ] || bin_dir=${XDG_BIN_HOME:-$HOME/.local/bin}

valid_version "$version" || { usage; die "invalid version: $version" 2; }
case $bin_dir in
	/*) ;;
	*) usage; die "--bin-dir must be an absolute path: $bin_dir" 2 ;;
esac

# --- Platform map: fail BEFORE any network request for anything unsupported ----------------------
uname_s=$(uname -s) || die "cannot determine OS via uname -s"
case $uname_s in
	Darwin) goos=darwin ;;
	Linux) goos=linux ;;
	*) die "unsupported operating system: $uname_s (need Darwin or Linux)" ;;
esac
uname_m=$(uname -m) || die "cannot determine architecture via uname -m"
case $uname_m in
	x86_64|amd64) goarch=amd64 ;;
	arm64|aarch64) goarch=arm64 ;;
	*) die "unsupported architecture: $uname_m (need x86_64/amd64 or arm64/aarch64)" ;;
esac

# --- Prereq probe: before any destination change ------------------------------------------------
command -v curl >/dev/null 2>&1 || die "curl is required but was not found on PATH"
command -v tar >/dev/null 2>&1 || die "tar is required but was not found on PATH"
if command -v sha256sum >/dev/null 2>&1; then
	sha_provider=sha256sum
elif command -v openssl >/dev/null 2>&1; then
	sha_provider=openssl
else
	die "a SHA-256 tool is required but neither sha256sum nor openssl was found on PATH"
fi

# --- Scratch dir. trap cleans the dir on exit and leaves stderr diagnostics intact --------------
work=$(mktemp -d "${TMPDIR:-/tmp}/docket-release.XXXXXX") || die "cannot create scratch directory"
trap 'rm -rf "$work"' EXIT

# --- Download base. DOCKET_RELEASE_BASE_URL is the ONE documented test/mirror seam --------------
# It lets the suite point real curl at a file:// release dir so no network is touched. Nothing
# else in this script reaches the network.
base_url="${DOCKET_RELEASE_BASE_URL:-https://github.com/danielhanold/docket/releases/download}/$version"
archive="docket_${version}_${goos}_${goarch}.tar.gz"

curl -fsSL "$base_url/checksums.txt" -o "$work/checksums.txt" \
	|| die "failed to download checksums.txt from $base_url"
curl -fsSL "$base_url/$archive" -o "$work/$archive" \
	|| die "failed to download $archive from $base_url"

# --- Integrity: exactly one valid manifest line, verify hash BEFORE extraction ------------------
# Escape the dots in the archive name so the fixed filename cannot over-match in the regex.
archive_re=$(printf '%s' "$archive" | sed 's/\./\\./g')
manifest_count=$(grep -Ec "^[0-9a-f]{64}  ${archive_re}\$" "$work/checksums.txt") || manifest_count=0
[ "$manifest_count" -eq 1 ] \
	|| die "checksums.txt does not contain exactly one entry for $archive (found $manifest_count)"
manifest_line=$(grep -E "^[0-9a-f]{64}  ${archive_re}\$" "$work/checksums.txt") \
	|| die "cannot read the checksum entry for $archive"
expected_sha=${manifest_line%% *}

actual_sha=$(sha_file "$work/$archive")
[ -n "$actual_sha" ] && [ "$actual_sha" = "$expected_sha" ] \
	|| die "checksum verification failed for $archive (refusing to extract or install)"

# Listing must be exactly the single line "docket": anything else (a second member, a directory, a
# nested path, a traversal name) is refused before extraction touches the filesystem.
listing=$(tar -tzf "$work/$archive") || die "cannot list the contents of $archive"
[ "$listing" = "docket" ] \
	|| die "$archive must contain exactly one member named docket (refusing extraction)"

tar -xzf "$work/$archive" -C "$work" || die "failed to extract $archive"
{ [ -f "$work/docket" ] && [ ! -L "$work/docket" ]; } \
	|| die "extracted docket is not a regular file (refusing to install)"
chmod 755 "$work/docket" || die "cannot set mode 755 on the extracted binary"
bin_sha=$(sha_file "$work/docket")
[ -n "$bin_sha" ] || die "cannot hash the extracted binary"

# --- Ownership decision: before touching dest, decide whether replacement is allowed ------------
record="${XDG_STATE_HOME:-$HOME/.local/state}/docket/release-binary.record"
dest="$bin_dir/docket"

allow=no
if [ ! -e "$dest" ]; then
	# Fresh install: nothing at the destination.
	allow=fresh
else
	dest_sha=$(sha_file "$dest")
	rec_path=''
	rec_sha=''
	if [ -f "$record" ]; then
		rec_path=$(record_field path "$record") || rec_path=''
		rec_sha=$(record_field sha256 "$record") || rec_sha=''
	fi
	if [ -n "$rec_path" ] && [ "$rec_path" = "$dest" ] && [ -n "$rec_sha" ] \
		&& [ -n "$dest_sha" ] && [ "$dest_sha" = "$rec_sha" ]; then
		# Owned: the record names this dest and its recorded hash matches the bytes there.
		allow=owned
	elif [ -n "$dest_sha" ] && [ "$dest_sha" = "$bin_sha" ]; then
		# Interrupted completion: the bytes at dest are already the verified requested binary.
		allow=converge
	else
		allow=no
	fi
fi

[ "$allow" != no ] || die "refusing to replace $dest: it is not a docket-owned release binary \
(no record, drifted bytes, foreign binary, or a record naming a different path). Existing bytes \
preserved; there is no --force."

# --- Install sequence (spec order, verbatim) ----------------------------------------------------
# (1) Stage BESIDE dest so the final rename is a single-filesystem atomic operation.
mkdir -p "$bin_dir" || die "cannot create bin dir $bin_dir"
stage=$(mktemp "$bin_dir/.docket-stage.XXXXXX") || die "cannot create staging file in $bin_dir"
# From here the stage file must be cleaned on any exit until it is renamed into place.
trap 'rm -rf "$work"; rm -f "$stage"' EXIT
cp "$work/docket" "$stage" || die "cannot stage the binary into $bin_dir"
chmod 755 "$stage" || die "cannot set mode 755 on the staged binary"

# (2) Run the staged binary's own installer with one --harness pair per selection. On failure the
# prior binary at dest is untouched; the stage is removed by the trap.
# shellcheck disable=SC2086  # harness_args holds validated, space-separated tokens by design.
"$stage" install $harness_args || die "docket install failed; the existing binary was left untouched"

# (3) Only after the asset transaction succeeds, move the staged binary into place.
mv -f "$stage" "$dest" || die "cannot move the staged binary into $dest"

# (4) Publish the ownership record atomically: write beside it, then mv -f into place.
record_dir=$(dirname "$record")
mkdir -p "$record_dir" || die "cannot create state dir $record_dir"
record_tmp="$record.tmp.$$"
{
	printf 'path=%s\n' "$dest"
	printf 'version=%s\n' "$version"
	printf 'sha256=%s\n' "$bin_sha"
} > "$record_tmp" || { rm -f "$record_tmp"; die "cannot write the ownership record"; }
mv -f "$record_tmp" "$record" || { rm -f "$record_tmp"; die "cannot publish the ownership record"; }

# (5) Read-only verification. exec replaces this shell, so clean scratch first and drop the trap
# (traps do not fire across exec). The check's exit status becomes this script's.
rm -rf "$work"
trap - EXIT
exec "$dest" install check
