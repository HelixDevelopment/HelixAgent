package lovable

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 STUB-BLUFF PIN GUARD — RED-on-broken-artifact + GREEN regression guard
// (§11.4.115 polarity switch / §11.4.135 standing guard)
//
// HISTORY: Lovable.createApp/edit/deploy/addFeature/connectDatabase/exportCode
// USED to FABRICATE a successful hosted build pipeline WITHOUT any real call to
// lovable.dev: createApp invented a project URL ("https://lovable.dev/p/<name>")
// + a file list + "created"; deploy invented "https://<name>.lovable.app" +
// "deployed"; addFeature invented a component list; etc. Stub bluffs per
// BLUFF-001 / CONST-035.
//
// FIX (D-17): Lovable is a HOSTED web-only builder with no headless CLI; these
// commands now return an HONEST error (ErrHostedOnly). list_projects keeps
// serving the REAL local project registry.
// ---------------------------------------------------------------------------

func TestD17_Lovable_NoFabricatedBuildPipeline(t *testing.T) {
	l := New()
	ctx := context.Background()

	cases := []struct {
		name   string
		invoke func() (interface{}, error)
	}{
		{"createApp", func() (interface{}, error) {
			return l.createApp(ctx, map[string]interface{}{"name": "MyApp"})
		}},
		{"edit", func() (interface{}, error) {
			return l.edit(ctx, map[string]interface{}{"project_id": "p", "prompt": "x"})
		}},
		{"deploy", func() (interface{}, error) {
			return l.deploy(ctx, map[string]interface{}{"project_id": "p"})
		}},
		{"addFeature", func() (interface{}, error) {
			return l.addFeature(ctx, map[string]interface{}{"project_id": "p", "feature": "auth"})
		}},
		{"connectDatabase", func() (interface{}, error) {
			return l.connectDatabase(ctx, map[string]interface{}{"project_id": "p"})
		}},
		{"exportCode", func() (interface{}, error) {
			return l.exportCode(ctx, map[string]interface{}{"project_id": "p"})
		}},
	}
	for _, c := range cases {
		res, err := c.invoke()
		if err == nil {
			t.Fatalf("D17 REGRESSION: Lovable.%s returned success %v with no real hosted call — must return an honest error (BLUFF-001 reintroduced?).", c.name, res)
		}
		if !errors.Is(err, ErrHostedOnly) {
			t.Fatalf("D17: Lovable.%s error should wrap ErrHostedOnly, got: %v", c.name, err)
		}
	}
}

// TestD17_Lovable_ListProjectsIsRealLocalState proves list_projects still serves
// the REAL local registry (honest local state, not fabricated).
func TestD17_Lovable_ListProjectsIsRealLocalState(t *testing.T) {
	l := New()
	res, err := l.listProjects(context.Background())
	if err != nil {
		t.Fatalf("list_projects returned error: %v", err)
	}
	m, _ := res.(map[string]interface{})
	// A fresh integration has an empty, real registry — count must be exactly 0,
	// reflecting real local state (no fabricated projects).
	if cnt, _ := m["count"].(int); cnt != 0 {
		t.Fatalf("D17: fresh Lovable should report 0 real local projects, got %d", cnt)
	}
}

// TestD17_Lovable_CreateAppIsStubBluff — §11.4.115 RED-on-broken-artifact, RED_MODE=1.
func TestD17_Lovable_CreateAppIsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"The standing GREEN guard is TestD17_Lovable_NoFabricatedBuildPipeline.")
	}
	l := New()
	res, err := l.createApp(context.Background(), map[string]interface{}{"name": "MyApp"})
	if err != nil {
		return
	}
	m, _ := res.(map[string]interface{})
	if p, ok := m["project"].(Project); ok && strings.Contains(p.URL, "lovable.dev/p/") {
		t.Fatalf("D17 BLUFF PINNED: Lovable.createApp fabricated a project URL %q without any real hosted call (BLUFF-001).", p.URL)
	}
}
