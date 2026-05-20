// SPDX-License-Identifier: Apache-2.0
// Regression tests for adaptedProvider.CompleteStream goroutine-leak safety.
// See BUGFIXES.md Issue #14 for the original leak.
package debate_integration

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	digitalvasicmodels "digital.vasic.llmprovider/pkg/models"
)

// leakInnerProvider is a minimal llm.LLMProvider whose CompleteStream returns
// a channel the test controls so the test can decide whether to close it.
// If closeChan stays false and the caller never reads, the forwarder
// goroutine inside adaptedProvider will leak UNLESS it honours ctx.Done().
type leakInnerProvider struct {
	// innerCh is the stream the forwarder ranges over.
	innerCh chan *models.LLMResponse
}

func newLeakInnerProvider() *leakInnerProvider {
	return &leakInnerProvider{innerCh: make(chan *models.LLMResponse)}
}

func (p *leakInnerProvider) Complete(_ context.Context, _ *models.LLMRequest) (*models.LLMResponse, error) {
	return &models.LLMResponse{}, nil
}

func (p *leakInnerProvider) CompleteStream(_ context.Context, _ *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	return p.innerCh, nil
}

func (p *leakInnerProvider) HealthCheck() error { return nil }

func (p *leakInnerProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{}
}

func (p *leakInnerProvider) ValidateConfig(_ map[string]interface{}) (bool, []string) {
	return true, nil
}

// enforce interface
var _ llm.LLMProvider = (*leakInnerProvider)(nil)

// TestAdaptedProvider_CompleteStream_ExitsOnContextCancel proves the
// forwarder goroutine exits when the caller cancels the context, even if
// the inner channel never closes.
func TestAdaptedProvider_CompleteStream_ExitsOnContextCancel(t *testing.T) {
	inner := newLeakInnerProvider()
	adapter := NewAdaptedProvider(inner)

	ctx, cancel := context.WithCancel(context.Background())
	externalCh, err := adapter.CompleteStream(ctx, &digitalvasicmodels.LLMRequest{})
	require.NoError(t, err)

	startGoroutines := runtime.NumGoroutine()

	// Cancel context — the forwarder must exit and close externalCh.
	cancel()

	select {
	case _, ok := <-externalCh:
		// Either closed channel (ok=false) or one last value; both acceptable.
		_ = ok
	case <-time.After(2 * time.Second):
		t.Fatalf("externalCh did not close within 2s after ctx cancel — forwarder goroutine leaked")
	}

	// Give Go's runtime a tick to reap the goroutine.
	time.Sleep(50 * time.Millisecond)
	runtime.Gosched()

	endGoroutines := runtime.NumGoroutine()
	// Allow a small tolerance — testing runtime itself spawns goroutines,
	// but we must not have grown by more than a handful.
	assert.LessOrEqual(t, endGoroutines-startGoroutines, 2,
		"goroutine count grew by %d after cancel; forwarder may be leaking", endGoroutines-startGoroutines)

	// Don't close innerCh — proves the forwarder exited on ctx, not EOF.
}

// TestAdaptedProvider_CompleteStream_ExitsOnInnerClose proves the forwarder
// goroutine still exits via the legacy path — inner channel closure.
func TestAdaptedProvider_CompleteStream_ExitsOnInnerClose(t *testing.T) {
	inner := newLeakInnerProvider()
	adapter := NewAdaptedProvider(inner)

	ctx := context.Background()
	externalCh, err := adapter.CompleteStream(ctx, &digitalvasicmodels.LLMRequest{})
	require.NoError(t, err)

	// Push one frame, then close inner — forwarder must forward then exit.
	go func() {
		inner.innerCh <- &models.LLMResponse{Content: "hello"}
		close(inner.innerCh)
	}()

	received := 0
	closedOK := false
	for i := 0; i < 10; i++ {
		select {
		case resp, ok := <-externalCh:
			if !ok {
				closedOK = true
			} else if resp != nil {
				received++
			}
		case <-time.After(2 * time.Second):
			t.Fatal("externalCh stalled after inner close")
		}
		if closedOK {
			break
		}
	}

	assert.Equal(t, 1, received, "should have forwarded the single inner frame")
	assert.True(t, closedOK, "externalCh must close after inner closes")
}

// TestAdaptedProvider_CompleteStream_DoesNotBlockOnUnreadExternal proves
// that when the caller stops reading externalCh and then cancels ctx, the
// forwarder exits without blocking forever on the send side. This is the
// second leak path (send-side) that the context-guarded select fixes.
func TestAdaptedProvider_CompleteStream_DoesNotBlockOnUnreadExternal(t *testing.T) {
	inner := newLeakInnerProvider()
	adapter := NewAdaptedProvider(inner)

	ctx, cancel := context.WithCancel(context.Background())
	externalCh, err := adapter.CompleteStream(ctx, &digitalvasicmodels.LLMRequest{})
	require.NoError(t, err)

	// Fill the buffered externalCh (buf=1) by sending one through inner
	// without the test reading externalCh. The next forwarded frame will
	// pin the forwarder on the send until we cancel.
	go func() {
		inner.innerCh <- &models.LLMResponse{Content: "1"}
		inner.innerCh <- &models.LLMResponse{Content: "2"} // forwarder will block sending this
	}()

	time.Sleep(100 * time.Millisecond) // let the forwarder get into its stuck send
	cancel()

	select {
	case <-externalCh:
		// any signal is acceptable; what matters is closure
		select {
		case _, ok := <-externalCh:
			assert.False(t, ok, "externalCh should be closed")
		case <-time.After(2 * time.Second):
			t.Fatal("externalCh did not reach closed state after cancel")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("forwarder did not unblock the send on ctx.Done")
	}
}
