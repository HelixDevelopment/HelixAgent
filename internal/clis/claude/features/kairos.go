// Package features provides KAIROS (Always-On Assistant) implementation.
package features

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dev.helix.agent/internal/clis/claude/api"
)

// KAIROS implements the Always-On Assistant feature
type KAIROS struct {
	client      *api.Client
	enabled     bool
	mu          sync.RWMutex
	ticker      *time.Ticker
	stopCh      chan struct{}
	wg          sync.WaitGroup
	logDir      string
	blockBudget time.Duration // 15-second blocking budget
	briefMode   bool
}

// KAIROSTool represents an exclusive KAIROS tool
type KAIROSTool string

const (
	ToolSendUserFile     KAIROSTool = "send_user_file"
	ToolPushNotification KAIROSTool = "push_notification"
	ToolSubscribePR      KAIROSTool = "subscribe_pr"
)

// TickPrompt represents a tick event sent to KAIROS
type TickPrompt struct {
	Type      string           `json:"type"`
	Timestamp int64            `json:"timestamp"`
	Context   *ObservedContext `json:"context"`
}

// ObservedContext represents context observed by KAIROS
type ObservedContext struct {
	ActiveFiles    []string          `json:"active_files,omitempty"`
	RecentCommands []string          `json:"recent_commands,omitempty"`
	SystemState    map[string]string `json:"system_state,omitempty"`
	Notifications  []Notification    `json:"notifications,omitempty"`
}

// Notification represents a notification
type Notification struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// NewKAIROS creates a new KAIROS instance
func NewKAIROS(client *api.Client) *KAIROS {
	return &KAIROS{
		client:      client,
		enabled:     true,
		stopCh:      make(chan struct{}),
		logDir:      filepath.Join(os.TempDir(), "kairos-logs"),
		blockBudget: 15 * time.Second,
		briefMode:   true,
	}
}

// Start starts the KAIROS always-on assistant
func (k *KAIROS) Start() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.enabled {
		return nil
	}

	// Create log directory
	if err := os.MkdirAll(k.logDir, 0755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	// Start ticker for periodic ticks
	k.ticker = time.NewTicker(30 * time.Second)

	k.wg.Add(1)
	go k.run()

	log.Println("[KAIROS] Always-on assistant started")
	return nil
}

// Stop stops KAIROS
func (k *KAIROS) Stop() {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.ticker != nil {
		k.ticker.Stop()
	}

	close(k.stopCh)
	k.wg.Wait()

	log.Println("[KAIROS] Always-on assistant stopped")
}

// run is the main KAIROS loop
func (k *KAIROS) run() {
	defer k.wg.Done()

	for {
		select {
		case <-k.stopCh:
			return
		case <-k.ticker.C:
			k.tick()
		}
	}
}

// tick handles a tick event
func (k *KAIROS) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), k.blockBudget)
	defer cancel()

	// Observe current context
	observed := k.observeContext()

	// Log observation
	if err := k.logObservation(observed); err != nil {
		log.Printf("[KAIROS] Failed to log observation: %v", err)
	}

	// Decide whether to act
	shouldAct := k.decideAction(observed)

	if shouldAct {
		// Act within blocking budget
		if err := k.act(ctx, observed); err != nil {
			log.Printf("[KAIROS] Action failed: %v", err)
		}
	}
}

// observeContext observes the current context
func (k *KAIROS) observeContext() *ObservedContext {
	// In real implementation, this would observe:
	// - Currently open files
	// - Recent terminal commands
	// - System notifications
	// - Git status

	return &ObservedContext{
		ActiveFiles:    []string{},
		RecentCommands: []string{},
		SystemState:    map[string]string{},
		Notifications:  []Notification{},
	}
}

// decideAction decides whether to take action based on context
func (k *KAIROS) decideAction(ctx *ObservedContext) bool {
	// Check for actionable notifications
	for _, notif := range ctx.Notifications {
		if notif.Type == "urgent" || notif.Type == "build_failed" {
			return true
		}
	}

	// Check for long-running processes
	// In real implementation, check if processes need attention

	// Default: don't act too frequently
	return false
}

// act performs an action within the blocking budget
func (k *KAIROS) act(ctx context.Context, observed *ObservedContext) error {
	// Actions are brief by design
	start := time.Now()

	// Example action: Send notification about important event
	if len(observed.Notifications) > 0 {
		notif := observed.Notifications[0]
		return k.sendNotification(ctx, &notif)
	}

	elapsed := time.Since(start)
	if elapsed > k.blockBudget {
		log.Printf("[KAIROS] Warning: Action exceeded blocking budget (%v > %v)", elapsed, k.blockBudget)
	}

	return nil
}

// sendNotification sends a notification to the user
func (k *KAIROS) sendNotification(ctx context.Context, notif *Notification) error {
	// In real implementation, this would:
	// - Use desktop notifications
	// - Send to connected devices
	// - Update status bar

	log.Printf("[KAIROS] Notification: %s - %s", notif.Title, notif.Message)
	return nil
}

// SendUserFile sends a file to the user (KAIROS exclusive tool)
func (k *KAIROS) SendUserFile(ctx context.Context, filePath string, message string) error {
	log.Printf("[KAIROS] Sending file to user: %s", filePath)

	// In real implementation:
	// - Copy file to accessible location
	// - Send notification with link
	// - Log the action

	return k.logAction("send_user_file", map[string]string{
		"file":    filePath,
		"message": message,
	})
}

// PushNotification pushes a notification to user's device
func (k *KAIROS) PushNotification(ctx context.Context, title, message string) error {
	log.Printf("[KAIROS] Push notification: %s - %s", title, message)

	return k.logAction("push_notification", map[string]string{
		"title":   title,
		"message": message,
	})
}

// SubscribePR subscribes to pull request activity
func (k *KAIROS) SubscribePR(ctx context.Context, repo string, prNumber int) error {
	log.Printf("[KAIROS] Subscribing to PR: %s#%d", repo, prNumber)

	return k.logAction("subscribe_pr", map[string]string{
		"repo": repo,
		"pr":   fmt.Sprintf("%d", prNumber),
	})
}

// logObservation logs an observation to the daily log
func (k *KAIROS) logObservation(ctx *ObservedContext) error {
	date := time.Now().Format("2006-01-02")
	logFile := filepath.Join(k.logDir, fmt.Sprintf("kairos-%s.log", date))

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	entry := fmt.Sprintf("[%s] Observation: %+v\n", time.Now().Format(time.RFC3339), ctx)
	_, err = f.WriteString(entry)
	return err
}

// logAction logs an action to the daily log
func (k *KAIROS) logAction(action string, details map[string]string) error {
	date := time.Now().Format("2006-01-02")
	logFile := filepath.Join(k.logDir, fmt.Sprintf("kairos-%s.log", date))

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	entry := fmt.Sprintf("[%s] Action: %s, Details: %+v\n",
		time.Now().Format(time.RFC3339), action, details)
	_, err = f.WriteString(entry)
	return err
}

// GetDailyLog returns the current day's log
func (k *KAIROS) GetDailyLog() (string, error) {
	date := time.Now().Format("2006-01-02")
	logFile := filepath.Join(k.logDir, fmt.Sprintf("kairos-%s.log", date))

	data, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	return string(data), nil
}

// Enable enables KAIROS
func (k *KAIROS) Enable() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.enabled = true
}

// Disable disables KAIROS
func (k *KAIROS) Disable() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.enabled = false
}

// IsEnabled returns whether KAIROS is enabled
func (k *KAIROS) IsEnabled() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.enabled
}

// SetBriefMode sets brief mode for concise responses
func (k *KAIROS) SetBriefMode(enabled bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.briefMode = enabled
}

// IsBriefMode returns whether brief mode is enabled
func (k *KAIROS) IsBriefMode() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.briefMode
}
