// SPDX-License-Identifier: Apache-2.0
package concurrency

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"digital.vasic.concurrency/pkg/safe"
)

// TestSafeIntegration_HelixAgentImports asserts that HelixAgent code
// can import and use `digital.vasic.concurrency/pkg/safe` end-to-end.
// The primitives are the CONST-029 foundation — if this breaks,
// Pattern-A migrations cannot proceed.
func TestSafeIntegration_HelixAgentImports(t *testing.T) {
	t.Parallel()

	t.Run("Store_basic_roundtrip", func(t *testing.T) {
		t.Parallel()
		s := safe.NewStore[string, int]()
		s.Put("a", 1)
		s.Put("b", 2)
		v, ok := s.Get("a")
		assert.True(t, ok)
		assert.Equal(t, 1, v)
		assert.Equal(t, 2, s.Len())
	})

	t.Run("Store_Update_atomic", func(t *testing.T) {
		t.Parallel()
		s := safe.NewStore[string, int64]()

		var wg sync.WaitGroup
		const goroutines = 8
		const iterations = 500
		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < iterations; i++ {
					s.Update("counter", func(cur int64, _ bool) (int64, bool) {
						return cur + 1, true
					})
				}
			}()
		}
		wg.Wait()

		v, _ := s.Get("counter")
		assert.Equal(t, int64(goroutines*iterations), v,
			"Update must provide atomic RMW — sum must equal N×M exactly")
	})

	t.Run("Slice_basic", func(t *testing.T) {
		t.Parallel()
		sl := safe.NewSlice[string]()
		sl.Append("a")
		sl.AppendAll("b", "c")
		assert.Equal(t, 3, sl.Len())
		assert.Equal(t, []string{"a", "b", "c"}, sl.Snapshot())
	})

	t.Run("Slice_UpdateAt_exclusive", func(t *testing.T) {
		t.Parallel()
		type entry struct {
			id      int
			claimed atomic.Bool
		}
		sl := safe.NewSlice[*entry]()
		const n = 200
		for i := 0; i < n; i++ {
			sl.Append(&entry{id: i})
		}

		var claimed atomic.Int64
		var wg sync.WaitGroup
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < n; i++ {
					_ = sl.UpdateAt(
						func(e *entry) bool { return e.id == i && !e.claimed.Load() },
						func(e *entry) *entry {
							if e.claimed.CompareAndSwap(false, true) {
								claimed.Add(1)
							}
							return e
						},
					)
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, int64(n), claimed.Load(),
			"each entry claimed exactly once under concurrent UpdateAt")
	})

	t.Run("Snapshot_iteration_is_independent", func(t *testing.T) {
		t.Parallel()
		s := safe.NewStore[int, int]()
		for i := 0; i < 50; i++ {
			s.Put(i, i*i)
		}

		snap := s.Snapshot()
		// Caller mutation of the snapshot must not leak back.
		snap[999] = -1
		assert.False(t, s.Has(999))
	})
}
