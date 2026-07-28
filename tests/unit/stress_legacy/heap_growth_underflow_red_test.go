// Previously tests/stress/... — demoted to unit package under CONST-030.
package stress_legacy_test

import (
	"os"
	"testing"
)

// TestHeapGrowthMB_NoUint64Underflow is the §11.4.115 RED->GREEN
// polarity-switch regression guard for the uint64-underflow-then-float64
// oracle inversion shared by TestStress_LargePayload_Handling
// (provider_stress_test.go:373) and TestStress_WebSocket_RapidConnectDisconnect
// (websocket_stress_test.go:248).
//
// Both sites computed heap growth as:
//
//	heapGrowthMB := float64(after.HeapInuse-before.HeapInuse) / 1024 / 1024
//
// runtime.MemStats.HeapInuse is uint64. `after.HeapInuse - before.HeapInuse`
// executes in UINT64 arithmetic BEFORE the float64() conversion ever sees
// it. When the heap SHRINKS (after < before) -- the normal, desirable
// outcome once runtime.GC() reclaims freed allocations -- the subtraction
// wraps around to a value near 2^64. float64() of that wrapped value,
// divided by 1024*1024, yields a number on the order of 1.7e13 "MB".
//
// TestStress_LargePayload_Handling asserts `assert.Less(heapGrowthMB,
// 500.0)`, so a genuinely-healthy shrinking heap FAILS the test (1.7e13 is
// not < 500) -- an INVERTED oracle: the test fails precisely when memory
// management works well, and it has never been able to detect the leak it
// was written to catch. The measured failure ("heap growth: 17592186044415.94
// MB") is exactly 2^44 MB, i.e. a (2^64 - 63488)-byte value -- the heap had
// shrunk by ~62 KiB when the underflow occurred.
//
// This RED test reproduces the underflow DETERMINISTICALLY via the shared
// heapGrowthMB helper (heap_growth_helper_test.go) with a synthetic
// before/after pair -- no real GC / memory pressure needed.
//
//   - RED_MODE=1 (run against the PRE-FIX helper): asserts the computed
//     value for a shrinking heap is the absurd multi-terabyte underflow
//     artifact (> 1e12 MB), reproducing the defect.
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): asserts the
//     computed value is a small, correctly-SIGNED NEGATIVE number when the
//     heap shrinks, AND that a genuine large heap growth still correctly
//     exceeds the 500MB leak-detection budget the real stress tests assert
//     against -- proving the oracle works in BOTH directions post-fix.
func TestHeapGrowthMB_NoUint64Underflow(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	const oneMB = uint64(1024 * 1024)

	// Shrink case: heap went DOWN by 63488 bytes (~62 KiB) after GC
	// reclaimed ten 1MB payloads -- the exact scenario that tripped
	// TestStress_LargePayload_Handling in the wild.
	before := uint64(50 * 1024 * 1024) // 50MB baseline
	after := before - 63488            // heap shrank by ~62 KiB

	shrinkGrowth := heapGrowthMB(before, after)

	if redMode {
		if shrinkGrowth < 1e12 {
			t.Fatalf("RED_MODE=1: expected the pre-fix uint64-subtraction-before-float64-conversion "+
				"bug to produce an absurd underflow artifact (> 1e12 MB) for a shrinking heap "+
				"(before=%d after=%d), got %.4f MB -- the bug is already fixed, flip RED_MODE=0",
				before, after, shrinkGrowth)
		}
		t.Logf("RED_MODE=1: reproduced the uint64 underflow -- heapGrowthMB(%d, %d) = %.4f MB "+
			"(absurd multi-terabyte artifact from a shrinking heap, as measured in the wild: "+
			"reported \"heap growth: 17592186044415.94 MB\" == 2^44 MB == (2^64 - 63488) bytes)",
			before, after, shrinkGrowth)
		return
	}

	// GREEN guard: a shrinking heap MUST yield a small, correctly-signed
	// NEGATIVE number -- never the absurd underflow artifact.
	if shrinkGrowth >= 0 || shrinkGrowth < -1.0 {
		t.Fatalf("RED_MODE=0 (GREEN guard): shrinking heap (before=%d after=%d, a %d-byte decrease) "+
			"must yield a small negative MB delta; got %.6f MB",
			before, after, before-after, shrinkGrowth)
	}
	t.Logf("shrink case OK: heapGrowthMB(%d, %d) = %.6f MB (correctly negative, no underflow)",
		before, after, shrinkGrowth)

	// Growth case: heap genuinely grew by 600MB -- the oracle must still
	// correctly flag this as exceeding the 500MB budget the real tests
	// assert against, proving the fix did not neuter leak detection.
	growBefore := uint64(50 * 1024 * 1024)
	growAfter := growBefore + 600*oneMB
	growGrowth := heapGrowthMB(growBefore, growAfter)

	if growGrowth < 500.0 {
		t.Fatalf("RED_MODE=0 (GREEN guard): a genuine 600MB heap growth must exceed the 500MB "+
			"leak-detection budget; got %.4f MB -- the oracle would fail to catch a real leak",
			growGrowth)
	}
	if growGrowth > 700.0 {
		t.Fatalf("RED_MODE=0 (GREEN guard): expected ~600MB growth, got an implausible %.4f MB",
			growGrowth)
	}
	t.Logf("growth case OK: heapGrowthMB(%d, %d) = %.4f MB (correctly flagged as exceeding "+
		"the 500MB leak-detection budget)", growBefore, growAfter, growGrowth)
}
