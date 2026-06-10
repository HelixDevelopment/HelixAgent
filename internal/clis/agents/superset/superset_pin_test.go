package superset

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 PIN GUARD — superset honest-error (§11.4.115 / §11.4.135)
//
// HISTORY: Superset.dashboard returned a fabricated URL for a dashboard never
// created, and Superset.chart returned {"data":"Chart data"} — BLUFF-001 /
// CONST-035 false-successes (Apache Superset is a BI web app whose dashboards/
// charts come from authenticated REST calls against a running instance, not from
// this integration).
//
// FIX: both return honest errors instead of fabricating output. status is honest
// config readback and remains.
// ---------------------------------------------------------------------------

func TestD17_Superset_DashboardAndChartAreHonestErrors(t *testing.T) {
	s := New()
	ctx := context.Background()

	if res, err := s.dashboard(ctx, map[string]interface{}{"name": "sales"}); err == nil {
		t.Fatalf("D-17 BLUFF: dashboard returned success %v — must return an honest error, never a fabricated URL (BLUFF-001).", res)
	} else if res != nil {
		t.Fatalf("D-17 BLUFF: dashboard returned a result payload %v — must be nil.", res)
	}

	if res, err := s.chart(ctx, map[string]interface{}{"type": "bar"}); err == nil {
		t.Fatalf("D-17 BLUFF: chart returned success %v — must return an honest error, never fabricated chart data (BLUFF-001).", res)
	} else if res != nil {
		t.Fatalf("D-17 BLUFF: chart returned a result payload %v — must be nil.", res)
	}
}
