package audit

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCoverageGapJSON(t *testing.T) {
	gap := CoverageGap{
		SourceFile:   "example.go",
		Package:      "example",
		HasAnyTest:   true,
		TestFiles:    []string{"test1.go", "test2.go"},
		MissingTypes: []string{"TypeA", "TypeB"},
	}

	data, _ := json.Marshal(gap)
	var gap2 CoverageGap
	_ = json.Unmarshal(data, &gap2)

	if !reflect.DeepEqual(gap, gap2) {
		t.Error("CoverageGap JSON round-trip failed")
	}
}

func TestTodoMarkerSeverity(t *testing.T) {
	cases := []struct {
		marker           string
		expectedSeverity Severity
	}{
		{marker: "TODO", expectedSeverity: SeverityIncomplete},
		{marker: "FIXME", expectedSeverity: SeverityBroken},
		{marker: "XXX", expectedSeverity: SeverityBroken},
		{marker: "HACK", expectedSeverity: SeverityOptimization},
		{marker: "NOTE", expectedSeverity: SeverityDocs},
	}

	for _, c := range cases {
		tm := TodoMarker{Marker: c.marker}
		tm.Classify()

		if tm.Severity != c.expectedSeverity {
			t.Errorf("Expected %s for %s, got %s", c.expectedSeverity, c.marker, tm.Severity)
		}
	}
}

func TestSkipClassification(t *testing.T) {
	reason := "Test skipped because it is not available in short mode"
	se := SkipEntry{Reason: reason}
	se.Classify()

	if se.Category != CategoryInfrastructure {
		t.Errorf("Expected infrastructure category for reason %q, got %s", reason, se.Category)
	}
}

func TestSkipClassificationFlakyGuard(t *testing.T) {
	reason := "Test skipped because it involves sleep"
	se := SkipEntry{Reason: reason}
	se.Classify()

	if se.Category != CategoryFlakyGuard {
		t.Errorf("Expected flaky-guard category for reason %q, got %s", reason, se.Category)
	}
}

func TestSkipClassificationUnimplemented(t *testing.T) {
	reason := "Test skipped because it is not implemented"
	se := SkipEntry{Reason: reason}
	se.Classify()

	if se.Category != CategoryUnimplemented {
		t.Errorf("Expected unimplemented category for reason %q, got %s", reason, se.Category)
	}
}
