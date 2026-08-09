package config

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// TestContainerMCPPortAgreesWithURL is the RED-baseline / standing guard for
// the port-table migration drift in generator_container.go (§11.4.115).
//
// THE DEFECT (measured on the pre-fix tree, 2026-08-09)
//
// MCPContainerPorts was migrated to the 82xx allocation, and getMCPURL reads
// that table — but every ContainerMCPServerConfig literal kept a hand-written
// 9xxx `Port:` value from the pre-migration scheme. The result was a config
// that contradicted itself on all 71 containerized entries, e.g.
//
//	fetch:  URL=http://localhost:8200/sse   Port=9101
//	github: URL=http://localhost:8220/sse   Port=9401
//
// A consumer that dials cfg.URL reaches 8200; a consumer that composes an
// address from cfg.Port reaches 9101, where nothing is listening.
//
// POLARITY (§11.4.115) — one source, two roles:
//
//	RED_MODE=1 — assert the drift IS present. Captured on the pre-fix tree,
//	    where it PASSed by reproducing all 71 mismatches; that PASS is the
//	    proof this test can actually see the defect rather than agreeing with
//	    whatever the code happens to do. Re-runnable against any pre-fix
//	    checkout. On the fixed tree it correctly FAILs ("cannot see the
//	    defect"), which is why it is not the committed default.
//	RED_MODE=0 (default, post-fix) — the standing regression guard: every
//	    entry's Port and URL agree AND both equal the MCPContainerPorts
//	    allocation. This is what `go test ./...` runs.
func TestContainerMCPPortAgreesWithURL(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	gen := NewContainerMCPConfigGenerator("http://localhost:8080")
	mcps := gen.GenerateContainerMCPs()
	if len(mcps) == 0 {
		t.Fatal("generator produced no MCP configs — nothing to assert against")
	}

	// Index the allocation table so we can check BOTH that Port and URL agree
	// with each other AND that they agree with the single source of truth.
	// Checking only "Port appears in URL" would still pass if both drifted to
	// the same wrong value.
	table := make(map[string]int, len(MCPContainerPorts))
	for _, p := range MCPContainerPorts {
		table[p.Name] = p.Port
	}

	var selfContradictory, offTable []string
	containerBacked := 0

	for name, cfg := range mcps {
		allocated, inTable := table[name]
		if !inTable {
			// e.g. "helixagent": a remote endpoint on the HelixAgent server
			// itself, not one of the containerized MCP ports.
			if cfg.Port != 0 {
				offTable = append(offTable, fmt.Sprintf(
					"%s: Port=%d but the name has no MCPContainerPorts entry", name, cfg.Port))
			}
			continue
		}
		containerBacked++

		if !strings.Contains(cfg.URL, fmt.Sprintf(":%d/", cfg.Port)) {
			selfContradictory = append(selfContradictory, fmt.Sprintf(
				"%s: Port=%d does not appear in URL=%s", name, cfg.Port, cfg.URL))
		}
		if cfg.Port != allocated {
			offTable = append(offTable, fmt.Sprintf(
				"%s: Port=%d but MCPContainerPorts allocates %d", name, cfg.Port, allocated))
		}
	}
	sort.Strings(selfContradictory)
	sort.Strings(offTable)

	if containerBacked == 0 {
		t.Fatal("no container-backed MCPs matched MCPContainerPorts — the test " +
			"is not exercising the code under test")
	}

	if redMode {
		if len(selfContradictory) == 0 {
			t.Fatalf("RED_MODE=1: expected port/URL drift to be present across the "+
				"%d container-backed entries, but every entry agrees. This test "+
				"cannot see the defect it is supposed to catch.", containerBacked)
		}
		t.Logf("RED_MODE=1: defect reproduced — %d of %d container-backed entries "+
			"contradict themselves:\n  %s",
			len(selfContradictory), containerBacked,
			strings.Join(selfContradictory, "\n  "))
		return
	}

	if len(selfContradictory) > 0 {
		t.Errorf("%d of %d entries have a Port that does not appear in their own URL:\n  %s",
			len(selfContradictory), containerBacked, strings.Join(selfContradictory, "\n  "))
	}
	if len(offTable) > 0 {
		t.Errorf("%d entries carry a Port that MCPContainerPorts does not allocate for them:\n  %s",
			len(offTable), strings.Join(offTable, "\n  "))
	}
	if !t.Failed() {
		t.Logf("all %d container-backed MCPs: Port == MCPContainerPorts allocation "+
			"and appears in URL", containerBacked)
	}
}
