package release

import (
	"strings"
	"testing"
)

// The literal placeholder token the downloader carries and RenderDownloader
// stamps. Duplicated here as an independent oracle so a test never derives its
// expectation from the code under test.
const testPlaceholder = `@DOCKET_DEFAULT_VERSION@`

func TestRenderDownloaderStampsEmbeddedSource(t *testing.T) {
	out, err := RenderDownloader("v1.2.3")
	if err != nil {
		t.Fatalf("RenderDownloader: %v", err)
	}
	if !strings.Contains(out, `DOCKET_DEFAULT_VERSION="v1.2.3"`) {
		t.Fatalf("stamped output missing DOCKET_DEFAULT_VERSION=\"v1.2.3\"; got:\n%s", out)
	}
	if strings.Contains(out, testPlaceholder) {
		t.Fatalf("stamped output still carries the raw placeholder %q", testPlaceholder)
	}
	// The embedded source must be a POSIX downloader, not an empty string: a
	// missing embed would render a trivially "stamped" empty file.
	if !strings.HasPrefix(out, "#!/bin/sh") {
		t.Fatalf("rendered downloader does not begin with the /bin/sh shebang; got:\n%.40s", out)
	}
}

func TestRenderDownloaderFromExactlyOnePlaceholder(t *testing.T) {
	src := `#!/bin/sh
DOCKET_DEFAULT_VERSION="@DOCKET_DEFAULT_VERSION@"
echo done
`
	out, err := renderDownloaderFrom(src, "v9.9.9")
	if err != nil {
		t.Fatalf("renderDownloaderFrom: %v", err)
	}
	if !strings.Contains(out, `DOCKET_DEFAULT_VERSION="v9.9.9"`) {
		t.Fatalf("single-placeholder source not stamped; got:\n%s", out)
	}
	if strings.Contains(out, testPlaceholder) {
		t.Fatalf("single-placeholder source still carries the raw placeholder")
	}
}

func TestRenderDownloaderFromZeroPlaceholders(t *testing.T) {
	src := "#!/bin/sh\nDOCKET_DEFAULT_VERSION=\"v0.0.0\"\n"
	if _, err := renderDownloaderFrom(src, "v1.2.3"); err == nil {
		t.Fatal("renderDownloaderFrom must error when the source carries zero placeholders")
	}
}

func TestRenderDownloaderFromTwoPlaceholders(t *testing.T) {
	src := "#!/bin/sh\n" +
		`A="@DOCKET_DEFAULT_VERSION@"` + "\n" +
		`B="@DOCKET_DEFAULT_VERSION@"` + "\n"
	if _, err := renderDownloaderFrom(src, "v1.2.3"); err == nil {
		t.Fatal("renderDownloaderFrom must error when the source carries two placeholders")
	}
}
