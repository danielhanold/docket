package codex

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/install"
)

func TestNativeFixturePlanUsesProductionNativeDefinitions(t *testing.T) {
	targets := planFixture(t)
	agents := 0
	for _, target := range targets {
		if target.Kind != install.KindFile {
			continue
		}
		agents++
		body := string(target.Content)
		if !strings.Contains(body, "top-level registered named-agent dispatch") || strings.Contains(body, "foreground catalog-resolved `agent.enter`") {
			t.Fatalf("candidate definition %s does not use production native routing", target.Path)
		}
	}
	if agents != 17 {
		t.Fatalf("candidate definition count=%d want 17", agents)
	}
}
