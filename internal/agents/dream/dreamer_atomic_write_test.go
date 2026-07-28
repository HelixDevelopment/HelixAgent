package dream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// This file closes a §11.4.135 regression-guard gap around atomicWriteFile
// (dreamer.go ~934).
//
// atomicWriteFile is the write-side half of the C1/M4 fix: it writes to a
// temporary sibling file in the SAME directory and then os.Rename()s it into
// place, so a concurrent READER — loadMemories() / orientationPhase(), which
// memoryMu never protected, and which may live in a DIFFERENT Dreamer instance
// entirely — can never observe a torn/truncated file.
//
// Before this file, a grep of the entire dream test corpus for
// "atomicWriteFile", "loadMemories" or "orientationPhase" returned ZERO hits.
// The existing concurrency tests assert only memoryWritersPeak, which counts
// goroutines inside the WRITER critical section and is completely unaffected by
// write atomicity — reverting atomicWriteFile to a plain os.WriteFile failed
// zero tests. The atomicity property was therefore an unverified claim
// (§11.4.125(6) / §1.1).
//
// The two tests below pin the two properties that actually matter:
//
//	A. a concurrent reader never observes a partial/torn/empty file;
//	B. the in-flight temp file is never ingested by the ".json"-suffix filters
//	   in loadMemories()/orientationPhase() — i.e. an uncommitted write can
//	   never be loaded as if it were a real, committed memory.

// tornWriteReadersCount is the number of concurrent readers hammering the file
// under test. Several readers materially widen the window in which a
// non-atomic writer's truncate-then-write gap can be observed.
const tornWriteReadersCount = 4

// tornWritePayloadSize is deliberately large: os.WriteFile opens with O_TRUNC
// and then writes, so the bigger the payload the wider the window during which
// the file on disk is shorter than either the old or the new content.
const tornWritePayloadSize = 1 << 20 // 1 MiB

// tornWriteIterations is the number of write cycles performed.
const tornWriteIterations = 150

// TestAtomicWriteFile_ReaderNeverObservesTornFile is the §11.4.115-style
// regression guard for atomicWriteFile's core promise.
//
// It runs concurrent readers against a file that a writer is rewriting in a
// tight loop, alternating between two DISTINCT, equally-sized payloads. Because
// os.Rename is atomic on the same filesystem, every read must observe EXACTLY
// one of the two complete payloads — never a short read, never an empty file,
// never a mixture.
//
// PAIRED §1.1 MUTATION: replace the atomicWriteFile call below (or
// atomicWriteFile's body) with a direct os.WriteFile. os.WriteFile truncates
// the file in place before writing, so readers observe zero-length and
// partial-length content and this test FAILs.
func TestAtomicWriteFile_ReaderNeverObservesTornFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")

	payloadA := bytes.Repeat([]byte("A"), tornWritePayloadSize)
	payloadB := bytes.Repeat([]byte("B"), tornWritePayloadSize)

	// Seed the file so readers starting before the first write still find a
	// complete, valid payload rather than a legitimately-absent file.
	require.NoError(t, os.WriteFile(path, payloadA, 0644))

	var (
		mu         sync.Mutex
		tornSample string
		tornCount  int
		readCount  int
	)

	recordTorn := func(sample string) {
		mu.Lock()
		defer mu.Unlock()
		tornCount++
		if tornSample == "" {
			tornSample = sample
		}
	}

	done := make(chan struct{})
	var readers sync.WaitGroup

	for i := 0; i < tornWriteReadersCount; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			local := 0
			for {
				select {
				case <-done:
					mu.Lock()
					readCount += local
					mu.Unlock()
					return
				default:
				}

				data, err := os.ReadFile(path)
				local++
				if err != nil {
					// os.Rename never unlinks the destination from a reader's
					// point of view, so the path must always resolve. A
					// truncate-then-write writer, by contrast, can be caught
					// mid-flight.
					recordTorn(fmt.Sprintf("read error: %v", err))
					continue
				}
				if bytes.Equal(data, payloadA) || bytes.Equal(data, payloadB) {
					continue
				}
				recordTorn(fmt.Sprintf(
					"observed a file that is neither complete payload: len=%d "+
						"(expected %d), first 16 bytes=%q",
					len(data), tornWritePayloadSize, data[:min(16, len(data))]))
			}
		}()
	}

	for i := 0; i < tornWriteIterations; i++ {
		payload := payloadA
		if i%2 == 1 {
			payload = payloadB
		}
		require.NoError(t, atomicWriteFile(path, payload, 0644))
	}

	close(done)
	readers.Wait()

	// Non-vacuity: the readers must actually have read something. Without this
	// a reader goroutine that crashed or never scheduled would let the
	// zero-torn assertion below pass trivially (§11.4/§11.4.1 —
	// absence-of-error is not evidence).
	require.Greater(t, readCount, 0,
		"vacuous test: the concurrent readers performed no reads at all, so "+
			"observing zero torn reads proves nothing about write atomicity")

	require.Equal(t, 0, tornCount,
		"atomicity violated: %d of %d concurrent reads observed a file that was "+
			"neither the complete old payload nor the complete new one. First "+
			"such observation: %s. atomicWriteFile must write to a temp sibling "+
			"and os.Rename it into place; a truncate-then-write (os.WriteFile) "+
			"exposes readers such as loadMemories()/orientationPhase() to a "+
			"torn file.", tornCount, readCount, tornSample)
}

// TestAtomicWriteFile_TempFileIsNotIngestedByMemoryJSONFilter pins the second
// half of the atomicity contract: the temp file atomicWriteFile creates while a
// write is in flight must never be picked up by the ".json"-suffix filters in
// loadMemories() (dreamer.go ~795) and orientationPhase() (dreamer.go ~574).
//
// If the temp file matched that filter, a reader could load an UNCOMMITTED
// write as though it were a real, committed memory — the exact class of
// phantom-data bug the rename-into-place design exists to prevent.
//
// The temp name is not hardcoded here: it is OBSERVED from a real in-flight
// atomicWriteFile call, so this test tracks whatever naming pattern the
// production code actually uses.
//
// PAIRED §1.1 MUTATION: change atomicWriteFile's os.CreateTemp pattern so the
// temp name ends in ".json" (e.g. ".tmp-"+filepath.Base(path)+"-*.json"). The
// observed name then matches the ingestion filter, the phantom entry is loaded,
// and this test FAILs on both assertions.
func TestAtomicWriteFile_TempFileIsNotIngestedByMemoryJSONFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")

	payload := bytes.Repeat([]byte("A"), tornWritePayloadSize)

	// Observe the real temp files created by real atomicWriteFile calls.
	var (
		mu       sync.Mutex
		observed = map[string]struct{}{}
	)
	done := make(chan struct{})
	var watcher sync.WaitGroup
	watcher.Add(1)
	go func() {
		defer watcher.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() || e.Name() == filepath.Base(path) {
					continue
				}
				mu.Lock()
				observed[e.Name()] = struct{}{}
				mu.Unlock()
			}
		}
	}()

	for i := 0; i < tornWriteIterations; i++ {
		require.NoError(t, atomicWriteFile(path, payload, 0644))
	}
	close(done)
	watcher.Wait()

	mu.Lock()
	names := make([]string, 0, len(observed))
	for n := range observed {
		names = append(names, n)
	}
	mu.Unlock()

	// Non-vacuity: we must actually have caught at least one in-flight temp
	// file, otherwise the suffix assertion below is testing an empty set.
	require.NotEmpty(t, names,
		"vacuous test: no in-flight temp file was ever observed in %s across %d "+
			"atomicWriteFile calls, so the ingestion-filter assertion would be "+
			"testing an empty set", dir, tornWriteIterations)

	for _, name := range names {
		require.False(t, strings.HasSuffix(name, ".json"),
			"atomicWriteFile's in-flight temp file %q ends in \".json\", so it "+
				"matches the ingestion filter used by loadMemories() and "+
				"orientationPhase() — an uncommitted write can be loaded as a "+
				"real memory", name)
	}

	// Now prove the consequence end-to-end through the REAL reader: plant a
	// leftover temp file (as a crashed mid-write would leave behind) carrying
	// perfectly valid JSON for a memory that was never committed, alongside one
	// genuinely committed memory, and assert loadMemories() ingests only the
	// committed one.
	loadDir := t.TempDir()

	committed := MemoryEntry{
		ID:        "committed-memory",
		Category:  "fact",
		Title:     "committed",
		Content:   "this memory was committed via a completed rename",
		CreatedAt: time.Now(),
	}
	committedBytes, err := json.Marshal(committed)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(loadDir, "committed-memory.json"), committedBytes, 0644))

	phantom := MemoryEntry{
		ID:        "phantom-from-temp",
		Category:  "fact",
		Title:     "phantom",
		Content:   "this memory only ever existed inside an uncommitted temp file",
		CreatedAt: time.Now(),
	}
	phantomBytes, err := json.Marshal(phantom)
	require.NoError(t, err)
	// Reuse a REAL observed temp name so this fixture tracks production's
	// actual naming pattern rather than a hardcoded copy of it.
	require.NoError(t, os.WriteFile(filepath.Join(loadDir, names[0]), phantomBytes, 0644))

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	d := NewDreamer(DreamerConfig{MemoryDir: loadDir}, logger)

	d.loadMemories()

	loaded := d.memories.Snapshot()
	_, phantomLoaded := loaded["phantom-from-temp"]
	require.False(t, phantomLoaded,
		"loadMemories() ingested %q from the leftover temp file %q — an "+
			"uncommitted write was loaded as a real memory", phantom.ID, names[0])

	_, committedLoaded := loaded["committed-memory"]
	require.True(t, committedLoaded,
		"loadMemories() failed to ingest the genuinely committed memory, so the "+
			"phantom-absence assertion above is vacuous (the loader read nothing)")

	require.Len(t, loaded, 1,
		"loadMemories() ingested %d memories from %s; exactly one (the committed "+
			"one) is expected", len(loaded), loadDir)
}
