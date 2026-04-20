package gemini

import (
	"sync"
	"testing"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/stretchr/testify/assert"
)

// TestGeminiACP_ResponseMap_ConcurrentAccess verifies the safe.Store
// contract that now backs GeminiACPProvider.responses: the reader
// goroutine's dispatch (Get) interleaves safely with the sender's
// register/unregister pair (Put → Delete) across many goroutines.
// Replaces the legacy mutex+map simulation this test used pre-CONST-029.
func TestGeminiACP_ResponseMap_ConcurrentAccess(t *testing.T) {
	responses := safe.NewStore[int64, chan *geminiACPResponse]()

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			ch := make(chan *geminiACPResponse, 1)
			responses.Put(id, ch)
			responses.Delete(id)
		}(int64(i))
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			_, _ = responses.Get(id)
		}(int64(i))
	}

	wg.Wait()
	assert.Equal(t, 0, responses.Len())
}
