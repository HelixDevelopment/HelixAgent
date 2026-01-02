package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// SSEWriter writes Server-Sent Events.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates a new SSE writer.
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	return &SSEWriter{w: w, flusher: flusher}, nil
}

// WriteEvent writes an SSE event with optional event type and ID.
func (s *SSEWriter) WriteEvent(event, data string, id string) error {
	if id != "" {
		if _, err := fmt.Fprintf(s.w, "id: %s\n", id); err != nil {
			return err
		}
	}
	if event != "" {
		if _, err := fmt.Fprintf(s.w, "event: %s\n", event); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", data); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// WriteData writes data without event type.
func (s *SSEWriter) WriteData(data string) error {
	return s.WriteEvent("", data, "")
}

// WriteJSON writes JSON data.
func (s *SSEWriter) WriteJSON(data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.WriteData(string(bytes))
}

// WriteProgress writes a progress event.
func (s *SSEWriter) WriteProgress(progress *StreamProgress) error {
	bytes, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	return s.WriteEvent("progress", string(bytes), "")
}

// WriteDone writes the done event.
func (s *SSEWriter) WriteDone() error {
	return s.WriteData("[DONE]")
}

// WriteError writes an error event.
func (s *SSEWriter) WriteError(err error) error {
	return s.WriteEvent("error", err.Error(), "")
}

// StreamToSSE streams a string channel to SSE.
func StreamToSSE(ctx context.Context, w http.ResponseWriter, stream <-chan string) error {
	sse, err := NewSSEWriter(w)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-stream:
			if !ok {
				return sse.WriteDone()
			}
			if err := sse.WriteData(chunk); err != nil {
				return err
			}
		}
	}
}

// StreamChunksToSSE streams StreamChunk channel to SSE.
func StreamChunksToSSE(ctx context.Context, w http.ResponseWriter, stream <-chan *StreamChunk) error {
	sse, err := NewSSEWriter(w)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-stream:
			if !ok {
				return sse.WriteDone()
			}

			if chunk.Error != nil {
				if err := sse.WriteError(chunk.Error); err != nil {
					return err
				}
				return chunk.Error
			}

			if err := sse.WriteData(chunk.Content); err != nil {
				return err
			}

			if chunk.Done {
				return sse.WriteDone()
			}
		}
	}
}

// StreamWithProgressToSSE streams content with progress updates.
func StreamWithProgressToSSE(ctx context.Context, w http.ResponseWriter, stream <-chan string, progressInterval int) error {
	sse, err := NewSSEWriter(w)
	if err != nil {
		return err
	}

	tracker := NewProgressTracker(0)
	tracker.Start()

	counter := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-stream:
			if !ok {
				// Final progress
				if err := sse.WriteProgress(tracker.GetProgress()); err != nil {
					return err
				}
				return sse.WriteDone()
			}

			// Write content
			if err := sse.WriteData(chunk); err != nil {
				return err
			}

			// Update progress
			tracker.Update(chunk)
			counter++

			// Emit progress at interval
			if progressInterval > 0 && counter%progressInterval == 0 {
				if err := sse.WriteProgress(tracker.GetProgress()); err != nil {
					return err
				}
			}
		}
	}
}

// SSEEvent represents an SSE event.
type SSEEvent struct {
	ID    string `json:"id,omitempty"`
	Event string `json:"event,omitempty"`
	Data  string `json:"data"`
}

// FormatSSEEvent formats an SSE event as a string.
func FormatSSEEvent(event *SSEEvent) string {
	var result string
	if event.ID != "" {
		result += fmt.Sprintf("id: %s\n", event.ID)
	}
	if event.Event != "" {
		result += fmt.Sprintf("event: %s\n", event.Event)
	}
	result += fmt.Sprintf("data: %s\n\n", event.Data)
	return result
}
