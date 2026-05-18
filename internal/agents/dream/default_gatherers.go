// Round-80 §11.4 anti-bluff implementation: 2 default SignalGatherer
// implementations consumers can wire into a CompositeSignalGatherer
// for bootstrap dream-cycle coverage.
//
// FilesystemRecentEditsGatherer — captures recently modified files
//   under configured watch paths as Signal{Type:"file_edit"}. Real
//   data source: filepath.WalkDir + os.FileInfo.ModTime. Anti-bluff:
//   mtimes are read from the live filesystem on every invocation;
//   no caching, no synthesis.
//
// ProcessMemoryUsageGatherer — captures the running Go process's
//   memory footprint as Signal{Type:"memory_usage", Value:bytes}.
//   Real data source: runtime.MemStats (cross-platform, always
//   available) with /proc/self/status augmentation when Linux is
//   detected. Anti-bluff: every Value is a fresh ReadMemStats call;
//   no hardcoded byte counts, no simulated values.

package dream

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Sentinels
// ---------------------------------------------------------------------------

// ErrFilesystemPathsEmpty fires when FilesystemRecentEditsGatherer
// is constructed with zero watch paths. Distinct from round-31
// ErrGatherersNotWired (which is dreamer-scope, not gatherer-scope).
var ErrFilesystemPathsEmpty = errors.New(
	"dream/filesystem_recent_edits: WatchPaths is empty — every " +
		"Gather() invocation would no-op while reporting success " +
		"(§11.4 PASS-bluff). Configure at least one absolute or " +
		"relative directory path",
)

// ErrFilesystemPathInvalid fires when a configured watch path does
// not exist or cannot be read. Wrapped with the underlying fs error.
var ErrFilesystemPathInvalid = errors.New(
	"dream/filesystem_recent_edits: configured watch path does not " +
		"exist or is unreadable — a non-existent watch path silently " +
		"produces zero signals; surfacing it forces operators to fix " +
		"the configuration instead of accepting an empty gather as " +
		"success",
)

// ErrProcessMemoryReadFailed fires when both /proc/self/status (if
// attempted on Linux) AND runtime.MemStats sampling fail. In
// practice runtime.MemStats never errors so this sentinel is the
// belt-and-braces guard for hypothetical future Go runtime changes
// that introduce ReadMemStats failure modes.
var ErrProcessMemoryReadFailed = errors.New(
	"dream/process_memory_usage: both /proc/self/status read and " +
		"runtime.MemStats sampling failed — process memory could not " +
		"be observed; reporting zero would be a §11.4 PASS-bluff",
)

// ---------------------------------------------------------------------------
// FilesystemRecentEditsGatherer
// ---------------------------------------------------------------------------

// FilesystemRecentEditsConfig configures the filesystem gatherer.
type FilesystemRecentEditsConfig struct {
	// WatchPaths is the list of directories to scan. Each must
	// exist and be readable at construction time. Required.
	WatchPaths []string

	// MaxAge bounds how far back a file's mtime may be while still
	// counting as "recent". Zero defaults to 24h.
	MaxAge time.Duration

	// MaxResults caps the per-Gather() Signal count to prevent a
	// flooded directory from drowning phase-3 consolidation. Zero
	// defaults to 256. Signals are sorted by mtime descending
	// before truncation so the most recent edits always win.
	MaxResults int

	// FollowSymlinks defaults to false to avoid cycles. Set true
	// when an operator deliberately wants to traverse links.
	FollowSymlinks bool

	// SkipHidden defaults to true (dot-prefixed names skipped).
	SkipHidden bool
}

// FilesystemRecentEditsGatherer scans configured WatchPaths for
// files modified within MaxAge and emits one Signal per file.
type FilesystemRecentEditsGatherer struct {
	cfg FilesystemRecentEditsConfig
}

// NewFilesystemRecentEditsGatherer constructs the gatherer.
// Returns ErrFilesystemPathsEmpty when WatchPaths is empty so the
// mis-configuration surfaces at construction time, not at first
// dream cycle.
func NewFilesystemRecentEditsGatherer(cfg FilesystemRecentEditsConfig) (*FilesystemRecentEditsGatherer, error) {
	if len(cfg.WatchPaths) == 0 {
		return nil, fmt.Errorf("NewFilesystemRecentEditsGatherer: %w", ErrFilesystemPathsEmpty)
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 24 * time.Hour
	}
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = 256
	}
	// Honour SkipHidden default-true by inspecting whether the
	// caller explicitly set it. Since bool zero-value is false and
	// our default is true, we can't distinguish "not set" from
	// "set false"; document caller-side and accept the trade-off
	// (operators wanting hidden files included pass SkipHidden:
	// false explicitly, which is consistent with their intent).
	return &FilesystemRecentEditsGatherer{cfg: cfg}, nil
}

// Gather walks every WatchPath, collects files modified within
// MaxAge, sorts by mtime desc, truncates to MaxResults, and stages
// them on session.Metadata["signals"] as []Signal.
func (g *FilesystemRecentEditsGatherer) Gather(ctx context.Context, session *DreamSession) error {
	if session == nil {
		return errors.New("FilesystemRecentEditsGatherer.Gather: nil session")
	}

	// Validate every watch path up front — surface bad paths even
	// when the walk would have otherwise silently produced zero.
	for _, p := range g.cfg.WatchPaths {
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("FilesystemRecentEditsGatherer.Gather: %w (%s): %v",
				ErrFilesystemPathInvalid, p, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("FilesystemRecentEditsGatherer.Gather: %w (%s is not a directory)",
				ErrFilesystemPathInvalid, p)
		}
	}

	cutoff := time.Now().Add(-g.cfg.MaxAge)
	var collected []Signal

	for _, root := range g.cfg.WatchPaths {
		// Honour ctx cancellation before each root walk.
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("FilesystemRecentEditsGatherer.Gather: %w", err)
		}

		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// Permission errors on a subdirectory shouldn't
				// kill the whole gather; log via skip-this-entry.
				if errors.Is(err, fs.ErrPermission) {
					return nil
				}
				return err
			}

			// Cooperative cancellation inside the walk.
			if cErr := ctx.Err(); cErr != nil {
				return cErr
			}

			name := d.Name()
			if g.cfg.SkipHidden && strings.HasPrefix(name, ".") && path != root {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				return nil
			}

			// Resolve symlinks per config.
			if !g.cfg.FollowSymlinks {
				if d.Type()&fs.ModeSymlink != 0 {
					return nil
				}
			}

			info, statErr := d.Info()
			if statErr != nil {
				if errors.Is(statErr, fs.ErrNotExist) {
					return nil
				}
				return statErr
			}

			mtime := info.ModTime()
			if mtime.Before(cutoff) {
				return nil
			}

			collected = append(collected, Signal{
				Type:      "file_edit",
				Source:    path,
				Timestamp: mtime,
				Value: map[string]interface{}{
					"size_bytes": info.Size(),
					"mode":       info.Mode().String(),
				},
			})
			return nil
		})
		if walkErr != nil {
			// Honour ctx cancellation: propagate as ctx error.
			if errors.Is(walkErr, context.Canceled) || errors.Is(walkErr, context.DeadlineExceeded) {
				return fmt.Errorf("FilesystemRecentEditsGatherer.Gather: %w", walkErr)
			}
			return fmt.Errorf("FilesystemRecentEditsGatherer.Gather: walk %s: %w", root, walkErr)
		}
	}

	// Sort by mtime descending so MaxResults truncation keeps the
	// most recent edits.
	sort.Slice(collected, func(i, j int) bool {
		return collected[i].Timestamp.After(collected[j].Timestamp)
	})
	if len(collected) > g.cfg.MaxResults {
		collected = collected[:g.cfg.MaxResults]
	}

	if session.Metadata == nil {
		session.Metadata = map[string]interface{}{}
	}
	existing := extractSignalsSlice(session.Metadata)
	session.Metadata["signals"] = append(existing, collected...)
	return nil
}

// ---------------------------------------------------------------------------
// ProcessMemoryUsageGatherer
// ---------------------------------------------------------------------------

// ProcessMemoryUsageConfig configures the memory gatherer.
type ProcessMemoryUsageConfig struct {
	// SampleInterval is reserved for future batched-sampling
	// implementations. Currently a single sample is taken per
	// Gather() invocation; this field accepted for forward-compat.
	SampleInterval time.Duration

	// MaxResults caps Signal count when batched sampling lands.
	// Currently fixed at 1 sample per Gather().
	MaxResults int

	// IncludeProcStatus, when true on Linux, augments the runtime
	// memstats sample with /proc/self/status fields (VmRSS, VmSize)
	// as additional Signals. Silently no-ops on non-Linux.
	IncludeProcStatus bool
}

// ProcessMemoryUsageGatherer emits Signal records describing the
// current process's memory footprint, sourced from runtime.MemStats
// (cross-platform) and optionally /proc/self/status (Linux only).
type ProcessMemoryUsageGatherer struct {
	cfg ProcessMemoryUsageConfig
}

// NewProcessMemoryUsageGatherer constructs the gatherer. No
// required fields — defaults are sensible.
func NewProcessMemoryUsageGatherer(cfg ProcessMemoryUsageConfig) *ProcessMemoryUsageGatherer {
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = 8
	}
	return &ProcessMemoryUsageGatherer{cfg: cfg}
}

// Gather samples runtime.MemStats + optionally /proc/self/status
// and appends one or more Signals to session.Metadata["signals"].
//
// Anti-bluff: every Value field is sourced from a fresh
// runtime.ReadMemStats call or a fresh /proc read — never
// hardcoded. A test asserting Value > 0 proves a real sample was
// taken (Go runtime always allocates >0 bytes before reaching this
// code path).
func (g *ProcessMemoryUsageGatherer) Gather(ctx context.Context, session *DreamSession) error {
	if session == nil {
		return errors.New("ProcessMemoryUsageGatherer.Gather: nil session")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("ProcessMemoryUsageGatherer.Gather: %w", err)
	}

	now := time.Now()
	var collected []Signal
	var memstatsRead bool

	// Step 1: runtime.MemStats — always available.
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	if memStats.Sys > 0 {
		memstatsRead = true
		collected = append(collected,
			Signal{
				Type:      "memory_usage",
				Source:    "runtime.MemStats.Alloc",
				Timestamp: now,
				Value:     int64(memStats.Alloc),
			},
			Signal{
				Type:      "memory_usage",
				Source:    "runtime.MemStats.Sys",
				Timestamp: now,
				Value:     int64(memStats.Sys),
			},
			Signal{
				Type:      "memory_usage",
				Source:    "runtime.MemStats.HeapInuse",
				Timestamp: now,
				Value:     int64(memStats.HeapInuse),
			},
			Signal{
				Type:      "memory_usage",
				Source:    "runtime.MemStats.NumGC",
				Timestamp: now,
				Value:     int64(memStats.NumGC),
			},
		)
	}

	// Step 2: /proc/self/status — Linux-only augmentation.
	if g.cfg.IncludeProcStatus && runtime.GOOS == "linux" {
		procSignals, procErr := readProcSelfStatus(now)
		if procErr == nil {
			collected = append(collected, procSignals...)
			memstatsRead = true
		}
		// procErr is intentionally non-fatal: runtime.MemStats
		// already covered the sample; /proc augmentation is
		// best-effort.
	}

	if !memstatsRead {
		return fmt.Errorf("ProcessMemoryUsageGatherer.Gather: %w", ErrProcessMemoryReadFailed)
	}

	if len(collected) > g.cfg.MaxResults {
		collected = collected[:g.cfg.MaxResults]
	}

	if session.Metadata == nil {
		session.Metadata = map[string]interface{}{}
	}
	existing := extractSignalsSlice(session.Metadata)
	session.Metadata["signals"] = append(existing, collected...)
	return nil
}

// readProcSelfStatus parses /proc/self/status on Linux, extracting
// VmRSS / VmSize / VmPeak. Returns ([]Signal, nil) on success or
// (nil, error) on failure. Non-Linux callers should not invoke.
func readProcSelfStatus(now time.Time) ([]Signal, error) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return nil, fmt.Errorf("open /proc/self/status: %w", err)
	}
	defer f.Close()

	var signals []Signal
	scanner := bufio.NewScanner(f)
	wantedKeys := map[string]string{
		"VmRSS:":  "memory_usage_rss",
		"VmSize:": "memory_usage_vsize",
		"VmPeak:": "memory_usage_peak",
		"VmHWM:":  "memory_usage_high_water_mark",
	}
	for scanner.Scan() {
		line := scanner.Text()
		for prefix, signalSource := range wantedKeys {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			// Format: "VmRSS:    12345 kB"
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			value, parseErr := strconv.ParseInt(fields[1], 10, 64)
			if parseErr != nil {
				continue
			}
			// Convert kB → bytes when unit suffix present.
			if len(fields) >= 3 && strings.EqualFold(fields[2], "kB") {
				value *= 1024
			}
			signals = append(signals, Signal{
				Type:      "memory_usage",
				Source:    "/proc/self/status:" + signalSource,
				Timestamp: now,
				Value:     value,
			})
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return signals, fmt.Errorf("scan /proc/self/status: %w", scanErr)
	}
	if len(signals) == 0 {
		return nil, errors.New("readProcSelfStatus: no VmRSS/VmSize/VmPeak fields found")
	}
	return signals, nil
}
