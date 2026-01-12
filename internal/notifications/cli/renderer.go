package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"dev.helix.agent/internal/models"
)

// Renderer handles CLI output rendering
type Renderer struct {
	config    *RenderConfig
	client    CLIClient
	writer    io.Writer
	isTTY     bool
	spinnerIdx int
	mu        sync.Mutex
}

// NewRenderer creates a new CLI renderer
func NewRenderer(config *RenderConfig, client CLIClient) *Renderer {
	if config == nil {
		config = DefaultRenderConfig()
	}

	return &Renderer{
		config: config,
		client: client,
		writer: os.Stdout,
		isTTY:  isTTY(),
	}
}

// SetWriter sets the output writer
func (r *Renderer) SetWriter(w io.Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writer = w
}

// RenderProgressBar renders a progress bar
func (r *Renderer) RenderProgressBar(content *ProgressBarContent) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isTTY || r.config.ColorScheme == ColorSchemeNone {
		return r.renderPlainProgressBar(content)
	}

	return r.renderColorProgressBar(content)
}

// renderPlainProgressBar renders a plain text progress bar
func (r *Renderer) renderPlainProgressBar(content *ProgressBarContent) string {
	width := r.config.Width - 20 // Leave room for percentage and status
	if width < 10 {
		width = 10
	}

	filled := int(float64(width) * content.Progress / 100.0)
	if filled > width {
		filled = width
	}

	var bar string
	switch r.config.ProgressStyle {
	case ProgressBarStyleASCII:
		bar = fmt.Sprintf("[%s%s>%s]",
			strings.Repeat(ProgressASCIIFilled, filled),
			ProgressASCIITip,
			strings.Repeat(ProgressASCIIEmpty, width-filled-1))
	default:
		bar = fmt.Sprintf("[%s%s]",
			strings.Repeat("=", filled),
			strings.Repeat(" ", width-filled))
	}

	return fmt.Sprintf("%s %.1f%% %s", bar, content.Progress, content.Message)
}

// renderColorProgressBar renders a colored progress bar
func (r *Renderer) renderColorProgressBar(content *ProgressBarContent) string {
	width := r.config.Width - 20
	if width < 10 {
		width = 10
	}

	filled := int(float64(width) * content.Progress / 100.0)
	if filled > width {
		filled = width
	}

	var bar strings.Builder

	bar.WriteString("[")

	switch r.config.ProgressStyle {
	case ProgressBarStyleUnicode:
		bar.WriteString(ColorGreen)
		bar.WriteString(strings.Repeat(ProgressFilled, filled))
		bar.WriteString(ColorReset)
		bar.WriteString(ColorDim)
		bar.WriteString(strings.Repeat(ProgressEmpty, width-filled))
		bar.WriteString(ColorReset)

	case ProgressBarStyleBlock:
		bar.WriteString(ColorGreen)
		bar.WriteString(strings.Repeat("▓", filled))
		bar.WriteString(ColorReset)
		bar.WriteString(ColorDim)
		bar.WriteString(strings.Repeat("░", width-filled))
		bar.WriteString(ColorReset)

	case ProgressBarStyleDots:
		bar.WriteString(ColorGreen)
		bar.WriteString(strings.Repeat("●", filled))
		bar.WriteString(ColorReset)
		bar.WriteString(ColorDim)
		bar.WriteString(strings.Repeat("○", width-filled))
		bar.WriteString(ColorReset)

	default: // ASCII
		bar.WriteString(ColorGreen)
		bar.WriteString(strings.Repeat(ProgressASCIIFilled, filled))
		bar.WriteString(ColorReset)
		bar.WriteString(strings.Repeat(ProgressASCIIEmpty, width-filled))
	}

	bar.WriteString("]")

	// Add percentage
	bar.WriteString(fmt.Sprintf(" %s%.1f%%%s", ColorBold, content.Progress, ColorReset))

	// Add message if present
	if content.Message != "" {
		bar.WriteString(fmt.Sprintf(" %s%s%s", ColorDim, content.Message, ColorReset))
	}

	return bar.String()
}

// RenderResourceGauge renders resource usage gauges
func (r *Renderer) RenderResourceGauge(content *ResourceGaugeContent) string {
	var sb strings.Builder

	if !r.isTTY || r.config.ColorScheme == ColorSchemeNone {
		sb.WriteString(fmt.Sprintf("  CPU: %.1f%% | ", content.CPUPercent))
		sb.WriteString(fmt.Sprintf("Memory: %s | ", formatBytes(content.MemoryBytes)))
		sb.WriteString(fmt.Sprintf("I/O: R:%s W:%s | ", formatBytes(content.IOReadBytes), formatBytes(content.IOWriteBytes)))
		sb.WriteString(fmt.Sprintf("Net: S:%s R:%s", formatBytes(content.NetBytesSent), formatBytes(content.NetBytesRecv)))
		return sb.String()
	}

	// Colorized output
	sb.WriteString("  Resources:\n")

	// CPU gauge
	sb.WriteString(fmt.Sprintf("  %s├─%s CPU:    %s\n",
		ColorDim, ColorReset,
		r.renderMiniGauge(content.CPUPercent, 10, r.getCPUColor(content.CPUPercent))))

	// Memory gauge
	memPercent := content.MemoryPercent
	if content.MemoryMax > 0 {
		memPercent = float64(content.MemoryBytes) / float64(content.MemoryMax) * 100
	}
	memLabel := fmt.Sprintf("%s / %s", formatBytes(content.MemoryBytes), formatBytes(content.MemoryMax))
	sb.WriteString(fmt.Sprintf("  %s├─%s Memory: %s %s%s%s\n",
		ColorDim, ColorReset,
		r.renderMiniGauge(memPercent, 10, r.getMemoryColor(memPercent)),
		ColorDim, memLabel, ColorReset))

	// I/O
	sb.WriteString(fmt.Sprintf("  %s└─%s I/O:    R:%s W:%s\n",
		ColorDim, ColorReset,
		formatBytes(content.IOReadBytes),
		formatBytes(content.IOWriteBytes)))

	return sb.String()
}

// renderMiniGauge renders a small gauge bar
func (r *Renderer) renderMiniGauge(percent float64, width int, color string) string {
	filled := int(float64(width) * percent / 100.0)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	return fmt.Sprintf("[%s%s%s%s%s] %.0f%%",
		color,
		strings.Repeat(ProgressFilled, filled),
		ColorReset,
		strings.Repeat(ProgressEmpty, width-filled),
		ColorReset,
		percent)
}

// getCPUColor returns color based on CPU usage
func (r *Renderer) getCPUColor(percent float64) string {
	if percent >= 90 {
		return ColorRed
	} else if percent >= 70 {
		return ColorYellow
	}
	return ColorGreen
}

// getMemoryColor returns color based on memory usage
func (r *Renderer) getMemoryColor(percent float64) string {
	if percent >= 90 {
		return ColorRed
	} else if percent >= 75 {
		return ColorYellow
	}
	return ColorGreen
}

// RenderStatusTable renders a status table
func (r *Renderer) RenderStatusTable(content *StatusTableContent) string {
	if len(content.Tasks) == 0 {
		return "No tasks in queue."
	}

	var sb strings.Builder

	if !r.isTTY || r.config.ColorScheme == ColorSchemeNone {
		return r.renderPlainStatusTable(content)
	}

	// Header
	sb.WriteString(fmt.Sprintf("%s%s%s\n", BoxDoubleTopLeft, strings.Repeat(BoxDoubleHorizontal, r.config.Width-2), BoxDoubleTopRight))
	sb.WriteString(fmt.Sprintf("%s%s%-*s%s%s\n",
		BoxDoubleVertical, ColorBold, r.config.Width-2, "  BACKGROUND TASKS", ColorReset, BoxDoubleVertical))
	sb.WriteString(fmt.Sprintf("%s%s%s\n", BoxDoubleTeeLeft, strings.Repeat(BoxDoubleHorizontal, r.config.Width-2), BoxDoubleTeeRight))

	// Column headers
	sb.WriteString(fmt.Sprintf("%s %s%-10s %-20s %-12s %-8s %-20s%s %s\n",
		BoxDoubleVertical, ColorBold,
		"STATUS", "NAME", "TYPE", "PROGRESS", "DURATION",
		ColorReset, BoxDoubleVertical))
	sb.WriteString(fmt.Sprintf("%s%s%s\n", BoxDoubleTeeLeft, strings.Repeat(BoxHorizontal, r.config.Width-2), BoxDoubleTeeRight))

	// Rows
	for _, task := range content.Tasks {
		status := models.TaskStatus(task.Status)
		icon := GetStatusIcon(status)
		color := GetStatusColor(status)

		name := truncateString(task.Name, 20)
		taskType := truncateString(task.Type, 12)

		sb.WriteString(fmt.Sprintf("%s %s%s %-8s%s %-20s %-12s %5.1f%% %-20s %s\n",
			BoxDoubleVertical,
			color, icon, status, ColorReset,
			name, taskType,
			task.Progress,
			formatDuration(task.Duration),
			BoxDoubleVertical))
	}

	// Footer
	sb.WriteString(fmt.Sprintf("%s%s%s\n", BoxDoubleBottomLeft, strings.Repeat(BoxDoubleHorizontal, r.config.Width-2), BoxDoubleBottomRight))

	return sb.String()
}

// renderPlainStatusTable renders a plain text status table
func (r *Renderer) renderPlainStatusTable(content *StatusTableContent) string {
	var sb strings.Builder

	sb.WriteString("BACKGROUND TASKS\n")
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	sb.WriteString(fmt.Sprintf("%-10s %-20s %-12s %-8s %-20s\n",
		"STATUS", "NAME", "TYPE", "PROGRESS", "DURATION"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	for _, task := range content.Tasks {
		sb.WriteString(fmt.Sprintf("%-10s %-20s %-12s %5.1f%% %-20s\n",
			task.Status,
			truncateString(task.Name, 20),
			truncateString(task.Type, 12),
			task.Progress,
			formatDuration(task.Duration)))
	}

	sb.WriteString(strings.Repeat("-", 80) + "\n")
	return sb.String()
}

// RenderTaskPanel renders a complete task panel
func (r *Renderer) RenderTaskPanel(content *TaskPanelContent) string {
	if content.Task == nil {
		return ""
	}

	var sb strings.Builder
	task := content.Task

	if !r.isTTY || r.config.ColorScheme == ColorSchemeNone {
		return r.renderPlainTaskPanel(content)
	}

	status := task.Status
	color := GetStatusColor(status)
	icon := GetStatusIcon(status)

	// Panel header
	sb.WriteString(fmt.Sprintf("%s%s Task: %s (%s)\n",
		BoxTopLeft, BoxHorizontal, task.TaskName, task.ID[:8]))
	sb.WriteString(fmt.Sprintf("%s  Status: %s%s %s%s\n",
		BoxVertical, color, icon, status, ColorReset))

	// Progress bar
	if content.Progress != nil {
		sb.WriteString(fmt.Sprintf("%s  Progress: %s\n",
			BoxVertical, r.RenderProgressBar(content.Progress)))
	}

	// Resources
	if r.config.ShowResources && content.Resources != nil {
		sb.WriteString(fmt.Sprintf("%s\n", BoxVertical))
		sb.WriteString(r.RenderResourceGauge(content.Resources))
	}

	// Logs
	if r.config.ShowLogs && len(content.Logs) > 0 {
		sb.WriteString(fmt.Sprintf("%s\n", BoxVertical))
		sb.WriteString(fmt.Sprintf("%s  %sLatest Log:%s\n", BoxVertical, ColorDim, ColorReset))
		logCount := r.config.LogLines
		if len(content.Logs) < logCount {
			logCount = len(content.Logs)
		}
		for i := len(content.Logs) - logCount; i < len(content.Logs); i++ {
			log := content.Logs[i]
			levelColor := getLevelColor(log.Level)
			sb.WriteString(fmt.Sprintf("%s  %s [%s%s%s] %s%s\n",
				BoxVertical,
				log.Timestamp.Format("15:04:05"),
				levelColor, log.Level, ColorReset,
				log.Message, ColorReset))
		}
	}

	// ETA
	if content.ETA != nil {
		sb.WriteString(fmt.Sprintf("%s\n", BoxVertical))
		sb.WriteString(fmt.Sprintf("%s  ETA: ~%s\n", BoxVertical, formatDuration(*content.ETA)))
	}

	// Panel footer
	sb.WriteString(fmt.Sprintf("%s%s\n",
		BoxBottomLeft, strings.Repeat(BoxHorizontal, r.config.Width-2)))

	return sb.String()
}

// renderPlainTaskPanel renders a plain text task panel
func (r *Renderer) renderPlainTaskPanel(content *TaskPanelContent) string {
	var sb strings.Builder
	task := content.Task

	sb.WriteString(fmt.Sprintf("Task: %s (%s)\n", task.TaskName, task.ID[:8]))
	sb.WriteString(fmt.Sprintf("Status: %s\n", task.Status))

	if content.Progress != nil {
		sb.WriteString(fmt.Sprintf("Progress: %s\n", r.renderPlainProgressBar(content.Progress)))
	}

	if r.config.ShowResources && content.Resources != nil {
		sb.WriteString(fmt.Sprintf("CPU: %.1f%% | Memory: %s\n",
			content.Resources.CPUPercent,
			formatBytes(content.Resources.MemoryBytes)))
	}

	if content.ETA != nil {
		sb.WriteString(fmt.Sprintf("ETA: ~%s\n", formatDuration(*content.ETA)))
	}

	return sb.String()
}

// RenderDashboard renders the full dashboard
func (r *Renderer) RenderDashboard(content *DashboardContent) string {
	var sb strings.Builder

	if !r.isTTY || r.config.ColorScheme == ColorSchemeNone {
		return r.renderPlainDashboard(content)
	}

	// Title
	title := content.Title
	if title == "" {
		title = "HELIXAGENT BACKGROUND TASKS"
	}
	padding := (r.config.Width - len(title) - 4) / 2
	sb.WriteString(fmt.Sprintf("%s%s%s\n",
		BoxDoubleTopLeft, strings.Repeat(BoxDoubleHorizontal, r.config.Width-2), BoxDoubleTopRight))
	sb.WriteString(fmt.Sprintf("%s%s%s%s%s%s\n",
		BoxDoubleVertical, strings.Repeat(" ", padding), ColorBold, title, ColorReset,
		strings.Repeat(" ", r.config.Width-padding-len(title)-2)+string(BoxDoubleVertical)))
	sb.WriteString(fmt.Sprintf("%s%s%s\n",
		BoxDoubleTeeLeft, strings.Repeat(BoxDoubleHorizontal, r.config.Width-2), BoxDoubleTeeRight))

	// Worker stats
	sb.WriteString(fmt.Sprintf("%s Workers: %d active, %d idle | Tasks: %d completed, %d failed %s\n",
		BoxDoubleVertical,
		content.WorkerStats.ActiveWorkers,
		content.WorkerStats.IdleWorkers,
		content.WorkerStats.TasksCompleted,
		content.WorkerStats.TasksFailed,
		BoxDoubleVertical))

	// Queue stats
	sb.WriteString(fmt.Sprintf("%s Queue: %d pending, %d running %s\n",
		BoxDoubleVertical,
		content.QueueStats.PendingCount,
		content.QueueStats.RunningCount,
		BoxDoubleVertical))

	// Separator
	sb.WriteString(fmt.Sprintf("%s%s%s\n",
		BoxDoubleTeeLeft, strings.Repeat(BoxHorizontal, r.config.Width-2), BoxDoubleTeeRight))

	// Tasks
	for _, taskPanel := range content.Tasks {
		sb.WriteString(r.RenderTaskPanel(&taskPanel))
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString(fmt.Sprintf("%s%s%s\n",
		BoxDoubleBottomLeft, strings.Repeat(BoxDoubleHorizontal, r.config.Width-2), BoxDoubleBottomRight))
	sb.WriteString(fmt.Sprintf("  %s%s Powered by HelixAgent %s\n",
		ColorDim, "✨", ColorReset))

	return sb.String()
}

// renderPlainDashboard renders a plain text dashboard
func (r *Renderer) renderPlainDashboard(content *DashboardContent) string {
	var sb strings.Builder

	title := content.Title
	if title == "" {
		title = "HELIXAGENT BACKGROUND TASKS"
	}

	sb.WriteString(strings.Repeat("=", 80) + "\n")
	sb.WriteString(fmt.Sprintf("  %s\n", title))
	sb.WriteString(strings.Repeat("=", 80) + "\n")

	sb.WriteString(fmt.Sprintf("Workers: %d active, %d idle | Tasks: %d completed, %d failed\n",
		content.WorkerStats.ActiveWorkers,
		content.WorkerStats.IdleWorkers,
		content.WorkerStats.TasksCompleted,
		content.WorkerStats.TasksFailed))

	sb.WriteString(fmt.Sprintf("Queue: %d pending, %d running\n",
		content.QueueStats.PendingCount,
		content.QueueStats.RunningCount))

	sb.WriteString(strings.Repeat("-", 80) + "\n")

	for _, taskPanel := range content.Tasks {
		sb.WriteString(r.renderPlainTaskPanel(&taskPanel))
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetSpinnerFrame returns the current spinner frame
func (r *Renderer) GetSpinnerFrame() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	frame := SpinnerFrames[r.spinnerIdx]
	r.spinnerIdx = (r.spinnerIdx + 1) % len(SpinnerFrames)
	return frame
}

// Helper functions

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func getLevelColor(level string) string {
	switch strings.ToLower(level) {
	case "error", "fatal", "panic":
		return ColorRed
	case "warn", "warning":
		return ColorYellow
	case "info":
		return ColorCyan
	case "debug", "trace":
		return ColorDim
	default:
		return ColorWhite
	}
}

func isTTY() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
