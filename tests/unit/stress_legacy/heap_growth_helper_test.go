// Previously tests/stress/... — demoted to unit package under CONST-030.
package stress_legacy_test

// heapGrowthMB computes the heap growth, in MB, between a baseline and a
// later runtime.MemStats.HeapInuse reading. Shared by
// TestStress_LargePayload_Handling (provider_stress_test.go) and
// TestStress_WebSocket_RapidConnectDisconnect (websocket_stress_test.go).
//
// runtime.MemStats.HeapInuse is a uint64. The naive formula
// `float64(after-before) / 1024 / 1024` performs the subtraction in
// UNSIGNED (uint64) arithmetic BEFORE ever converting to float64: when the
// heap SHRINKS (after < before -- the normal, desirable outcome once
// runtime.GC() reclaims freed allocations) the subtraction wraps around to
// a value near 2^64, silently producing an absurd multi-terabyte "growth"
// instead of a small negative delta. See
// TestHeapGrowthMB_NoUint64Underflow (heap_growth_underflow_red_test.go)
// for the §11.4.115 RED->GREEN regression guard and forensic detail
// (measured in the wild as "heap growth: 17592186044415.94 MB" ==
// 2^44 MB == (2^64 - 63488) bytes).
//
// Converting each operand to float64 BEFORE subtracting avoids the wrap
// entirely -- heap sizes here are on the order of megabytes, far below
// float64's 53-bit exact-integer mantissa, so the subtraction is exact and
// correctly signed regardless of which operand is larger.
func heapGrowthMB(before, after uint64) float64 {
	return (float64(after) - float64(before)) / 1024 / 1024
}
