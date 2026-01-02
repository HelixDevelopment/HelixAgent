package streaming

import (
	"strings"
	"sync"
	"time"
)

// StreamProgress contains progress information for streaming.
type StreamProgress struct {
	TokensGenerated    int     `json:"tokens_generated"`
	ChunksReceived     int     `json:"chunks_received"`
	CharactersReceived int     `json:"characters_received"`
	ElapsedSeconds     float64 `json:"elapsed_seconds"`
	TokensPerSecond    float64 `json:"tokens_per_second"`
	CharsPerSecond     float64 `json:"chars_per_second"`
	EstimatedRemaining float64 `json:"estimated_remaining,omitempty"`
	PercentComplete    float64 `json:"percent_complete,omitempty"`
}

// ProgressTracker tracks streaming progress.
type ProgressTracker struct {
	mu                 sync.Mutex
	estimatedTokens    int
	tokensGenerated    int
	chunksReceived     int
	charactersReceived int
	startTime          time.Time
	lastUpdate         time.Time
}

// NewProgressTracker creates a new progress tracker.
func NewProgressTracker(estimatedTokens int) *ProgressTracker {
	return &ProgressTracker{
		estimatedTokens: estimatedTokens,
	}
}

// Start begins progress tracking.
func (t *ProgressTracker) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.startTime = time.Now()
	t.lastUpdate = t.startTime
	t.tokensGenerated = 0
	t.chunksReceived = 0
	t.charactersReceived = 0
}

// Update updates progress with new content.
func (t *ProgressTracker) Update(content string) *StreamProgress {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Approximate token count by words
	tokens := len(strings.Fields(content))
	t.tokensGenerated += tokens
	t.chunksReceived++
	t.charactersReceived += len(content)
	t.lastUpdate = time.Now()

	return t.getProgressLocked()
}

// UpdateTokens updates progress with explicit token count.
func (t *ProgressTracker) UpdateTokens(tokens int) *StreamProgress {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.tokensGenerated += tokens
	t.chunksReceived++
	t.lastUpdate = time.Now()

	return t.getProgressLocked()
}

// GetProgress returns current progress.
func (t *ProgressTracker) GetProgress() *StreamProgress {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.getProgressLocked()
}

func (t *ProgressTracker) getProgressLocked() *StreamProgress {
	elapsed := time.Since(t.startTime).Seconds()

	var tps, cps float64
	if elapsed > 0 {
		tps = float64(t.tokensGenerated) / elapsed
		cps = float64(t.charactersReceived) / elapsed
	}

	var remaining, percent float64
	if t.estimatedTokens > 0 {
		if tps > 0 {
			remaining = float64(t.estimatedTokens-t.tokensGenerated) / tps
			if remaining < 0 {
				remaining = 0
			}
		}
		percent = (float64(t.tokensGenerated) / float64(t.estimatedTokens)) * 100
		if percent > 100 {
			percent = 100
		}
	}

	return &StreamProgress{
		TokensGenerated:    t.tokensGenerated,
		ChunksReceived:     t.chunksReceived,
		CharactersReceived: t.charactersReceived,
		ElapsedSeconds:     elapsed,
		TokensPerSecond:    tps,
		CharsPerSecond:     cps,
		EstimatedRemaining: remaining,
		PercentComplete:    percent,
	}
}

// Reset resets the tracker.
func (t *ProgressTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tokensGenerated = 0
	t.chunksReceived = 0
	t.charactersReceived = 0
	t.startTime = time.Time{}
	t.lastUpdate = time.Time{}
}

// SetEstimatedTokens updates the estimated total tokens.
func (t *ProgressTracker) SetEstimatedTokens(tokens int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.estimatedTokens = tokens
}

// ProgressCallback is called with progress updates.
type ProgressCallback func(*StreamProgress)

// ThrottledCallback wraps a callback to limit update frequency.
type ThrottledCallback struct {
	callback   ProgressCallback
	interval   time.Duration
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewThrottledCallback creates a callback that fires at most once per interval.
func NewThrottledCallback(callback ProgressCallback, interval time.Duration) *ThrottledCallback {
	return &ThrottledCallback{
		callback: callback,
		interval: interval,
	}
}

// Call invokes the callback if the interval has passed.
func (tc *ThrottledCallback) Call(progress *StreamProgress) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if time.Since(tc.lastUpdate) >= tc.interval {
		tc.callback(progress)
		tc.lastUpdate = time.Now()
	}
}

// ForceCall invokes the callback regardless of interval.
func (tc *ThrottledCallback) ForceCall(progress *StreamProgress) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.callback(progress)
	tc.lastUpdate = time.Now()
}
