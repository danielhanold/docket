package release

import (
	_ "embed"
	"fmt"
	"strings"
)

// downloaderSource is the POSIX downloader shipped verbatim in every bundle,
// embedded from internal/release/downloader/install.sh at build time. The embed
// directive requires that file to exist in the source tree (Task 4 authored it).
//
//go:embed downloader/install.sh
var downloaderSource string

// downloaderPlaceholder is the exact token the downloader carries on its
// DOCKET_DEFAULT_VERSION="@DOCKET_DEFAULT_VERSION@" line. RenderDownloader
// stamps it with the bundle's default version.
const downloaderPlaceholder = `@DOCKET_DEFAULT_VERSION@`

// RenderDownloader returns the embedded downloader with the placeholder line
// DOCKET_DEFAULT_VERSION="@DOCKET_DEFAULT_VERSION@" stamped to version. Exactly
// one placeholder must exist; zero or two is an error.
func RenderDownloader(version string) (string, error) {
	return renderDownloaderFrom(downloaderSource, version)
}

// renderDownloaderFrom is the unexported seam RenderDownloader wraps: it stamps
// an arbitrary source so a test can feed a doctored source (zero or two
// placeholders) without touching the embedded file. It replaces the single
// placeholder occurrence with version and errors unless exactly one exists — a
// missing placeholder means the render is a silent no-op that ships an
// unstamped downloader, and a duplicated placeholder means an ambiguous stamp.
func renderDownloaderFrom(src, version string) (string, error) {
	n := strings.Count(src, downloaderPlaceholder)
	if n != 1 {
		return "", fmt.Errorf("downloader source carries %d %q placeholders, want exactly 1", n, downloaderPlaceholder)
	}
	return strings.Replace(src, downloaderPlaceholder, version, 1), nil
}
