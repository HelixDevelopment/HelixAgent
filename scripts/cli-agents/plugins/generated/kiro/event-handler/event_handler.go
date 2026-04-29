// Event Handler Plugin
// Manages event subscriptions and handling for HelixAgent events

package main

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "strings"
    "sync"
    "time"
)

var PluginName = "event-handler"
var PluginVersion = "1.0.0"

// Event represents a HelixAgent event
type Event struct {
    Type      string                 `json:"type"`
    Timestamp time.Time              `json:"timestamp"`
    Data      map[string]interface{} `json:"data"`
}

// EventHandler manages event subscriptions
type EventHandler struct {
    helixURL   string
    subscribed []string
    handlers   map[string][]func(Event)
    ctx        context.Context
    cancel     context.CancelFunc
    mu         sync.RWMutex
}

// NewEventHandler creates a new event handler
func NewEventHandler(helixURL string, subscriptions []string) *EventHandler {
    ctx, cancel := context.WithCancel(context.Background())
    return &EventHandler{
        helixURL:   helixURL,
        subscribed: subscriptions,
        handlers:   make(map[string][]func(Event)),
        ctx:        ctx,
        cancel:     cancel,
    }
}

// Subscribe registers a handler for an event type
func (h *EventHandler) Subscribe(eventType string, handler func(Event)) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.handlers[eventType] = append(h.handlers[eventType], handler)
}

// Start begins listening for events via SSE
func (h *EventHandler) Start() error {
    go h.listen()
    return nil
}

// Stop stops the event listener
func (h *EventHandler) Stop() {
    h.cancel()
}

func (h *EventHandler) listen() {
    url := h.helixURL + "/v1/events/stream?subscribe=" + strings.Join(h.subscribed, ",")

    for {
        select {
        case <-h.ctx.Done():
            return
        default:
            h.connectSSE(url)
            time.Sleep(5 * time.Second) // Reconnect delay
        }
    }
}

func (h *EventHandler) connectSSE(url string) {
    req, err := http.NewRequestWithContext(h.ctx, "GET", url, nil)
    if err != nil {
        return
    }
    req.Header.Set("Accept", "text/event-stream")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return
    }
    defer resp.Body.Close()

    scanner := bufio.NewScanner(resp.Body)
    var eventData strings.Builder

    for scanner.Scan() {
        line := scanner.Text()

        if strings.HasPrefix(line, "data:") {
            eventData.WriteString(strings.TrimPrefix(line, "data:"))
        } else if line == "" && eventData.Len() > 0 {
            var event Event
            if err := json.Unmarshal([]byte(eventData.String()), &event); err == nil {
                h.dispatch(event)
            }
            eventData.Reset()
        }
    }
}

func (h *EventHandler) dispatch(event Event) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    // Exact match handlers
    if handlers, ok := h.handlers[event.Type]; ok {
        for _, handler := range handlers {
            go handler(event)
        }
    }

    // Wildcard handlers
    for pattern, handlers := range h.handlers {
        if strings.HasSuffix(pattern, "*") {
            prefix := strings.TrimSuffix(pattern, "*")
            if strings.HasPrefix(event.Type, prefix) {
                for _, handler := range handlers {
                    go handler(event)
                }
            }
        }
    }
}

func Init() error {
    fmt.Println("[event-handler] Plugin initialized")
    return nil
}

func Shutdown() error {
    fmt.Println("[event-handler] Plugin shutdown")
    return nil
}

func main() {
    if len(os.Args) > 1 && os.Args[1] == "--version" {
        fmt.Printf("%s v%s\n", PluginName, PluginVersion)
        return
    }
    fmt.Println("Event Handler Plugin")
}
