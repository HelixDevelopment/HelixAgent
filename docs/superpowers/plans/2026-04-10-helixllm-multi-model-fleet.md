# HelixLLM Multi-Model Fleet Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Qwen3-Coder-30B-A3B with a lightweight model fleet (Qwen2.5-Coder 1.5B + 3B) using llama.cpp native router mode, with intelligent task-complexity-based routing.

**Architecture:** HelixLLM's Brain layer gains a complexity analyzer that scores incoming requests and selects the optimal model tier (fast/balanced/powerful). llama.cpp runs in native router mode serving multiple models from a single process. A hardware profiler auto-configures GPU layers, context size, and batch size at boot. Models are auto-downloaded from HuggingFace on first run.

**Tech Stack:** Go 1.26.1, llama.cpp (CUDA router mode), Gin, nvidia-smi, HuggingFace API

**Spec:** `docs/superpowers/specs/2026-04-10-helixllm-multi-model-fleet-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `HelixLLM/internal/shared/hardware/profiler.go` | GPU/CPU/RAM detection, preset profile selection |
| `HelixLLM/internal/shared/hardware/profiler_test.go` | Unit tests for hardware profiler |
| `HelixLLM/internal/brain/models/catalog.go` | Model definitions, default catalog, VRAM requirements |
| `HelixLLM/internal/brain/models/catalog_test.go` | Unit tests for model catalog |
| `HelixLLM/internal/brain/models/registry.go` | Runtime model status tracking |
| `HelixLLM/internal/brain/models/registry_test.go` | Unit tests for model registry |
| `HelixLLM/internal/brain/models/preset.go` | llama.cpp INI preset generator |
| `HelixLLM/internal/brain/models/preset_test.go` | Unit tests for preset generator |
| `HelixLLM/internal/brain/complexity.go` | Task complexity analyzer with heuristic scoring |
| `HelixLLM/internal/brain/complexity_test.go` | Unit tests for complexity analyzer |
| `HelixLLM/internal/brain/downloader.go` | HuggingFace GGUF model downloader |
| `HelixLLM/internal/brain/downloader_test.go` | Unit tests for downloader |
| `HelixLLM/internal/brain/server.go` | llama-server process lifecycle management |
| `HelixLLM/internal/brain/server_test.go` | Unit tests for server manager |
| `HelixLLM/internal/knowledge/llama_embedder.go` | Local embedding via llama-server nomic model |
| `HelixLLM/internal/knowledge/llama_embedder_test.go` | Unit tests for llama embedder |
| `HelixLLM/container/Containerfile.llamacpp-router` | CUDA-enabled multi-model container |

### Modified Files

| File | Changes |
|------|---------|
| `HelixLLM/internal/shared/config/config.go` | Add model fleet env vars to LLMConfig |
| `HelixLLM/internal/brain/brain.go` | Wire complexity analyzer into Complete/CompleteStream |
| `HelixLLM/internal/brain/llamacpp.go` | Dynamic model list from registry, pass model field |
| `HelixLLM/internal/brain/router.go` | Add complexity-based routing before provider dispatch |
| `HelixLLM/internal/gateway/router.go` | Add `/v1/hardware` and model management endpoints |
| `HelixLLM/internal/knowledge/embedding_providers.go` | Add "llama" provider to factory |
| `HelixLLM/cmd/helixllm/main.go` | Wire new components into boot sequence |

---

## Phase 1: Foundation

### Task 1: Hardware Profiler

**Files:**
- Create: `HelixLLM/internal/shared/hardware/profiler.go`
- Create: `HelixLLM/internal/shared/hardware/profiler_test.go`

- [ ] **Step 1: Write failing tests for GPU detection**

```go
// HelixLLM/internal/shared/hardware/profiler_test.go
package hardware

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNvidiaSMI_ValidOutput(t *testing.T) {
	output := "6144 MiB, 5800 MiB, NVIDIA GeForce RTX 3060, 8.6"
	gpu, err := parseNvidiaSMI(output)
	require.NoError(t, err)
	assert.True(t, gpu.Available)
	assert.Equal(t, int64(6144*1024*1024), gpu.VRAMTotal)
	assert.Equal(t, int64(5800*1024*1024), gpu.VRAMFree)
	assert.Equal(t, "NVIDIA GeForce RTX 3060", gpu.Name)
	assert.Equal(t, "8.6", gpu.ComputeCap)
}

func TestParseNvidiaSMI_Empty(t *testing.T) {
	gpu, err := parseNvidiaSMI("")
	require.NoError(t, err)
	assert.False(t, gpu.Available)
}

func TestParseProcCPUInfo_ValidOutput(t *testing.T) {
	output := `processor	: 0
model name	: AMD Ryzen 9 5950X 16-Core Processor
cpu cores	: 16
siblings	: 32
flags		: fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx fxsr sse sse2 ht syscall nx mmxext fxsr_opt pdpe1gb rdtscp lm constant_tsc rep_good nopl nonstop_tsc cpuid extd_apicid aperfmperf rapl pni pclmulqdq monitor ssse3 fma cx16 sse4_1 sse4_2 x2apic movbe popcnt aes xsave avx f16c rdrand lahf_lm cmp_legacy svm extapic cr8_legacy abm sse4a misalignsse 3dnowprefetch osvw ibs skinit wdt tce topoext perfctr_core perfctr_nb bpext perfctr_llc mwaitx cpb cat_l3 cdp_l3 hw_pstate ssbd mba ibrs ibpb stibp vmmcall fsgsbase bmi1 avx2 smep bmi2 erms invpcid cqm rdt_a rdseed adx smap clflushopt clwb sha_ni xsaveopt xsavec xgetbv1 xsaves cqm_llc cqm_occup_llc cqm_mbm_total cqm_mbm_local clzero irperf xsaveerptr rdpru wbnoinvd arat npt lbrv svm_lock nrip_save tsc_scale vmcb_clean flushbyasid decodeassists pausefilter pfthreshold avic v_vmsave_vmload vgif v_spec_ctrl umip pku ospke vaes vpclmulqdq rdpid overflow_recov succor smca fsrm
`
	cpu, err := parseProcCPUInfo(output)
	require.NoError(t, err)
	assert.Equal(t, "AMD Ryzen 9 5950X 16-Core Processor", cpu.Model)
	assert.Equal(t, 16, cpu.Cores)
	assert.Equal(t, 32, cpu.Threads)
	assert.True(t, cpu.AVX2)
	assert.False(t, cpu.AVX512)
}

func TestSelectPresetProfile(t *testing.T) {
	tests := []struct {
		name     string
		vramMB   int64
		expected string
	}{
		{"no gpu", 0, "cpu_only"},
		{"4gb", 4096, "consumer_6gb"},
		{"6gb", 6144, "consumer_6gb"},
		{"8gb", 8192, "consumer_8gb"},
		{"12gb", 12288, "high_end"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := selectPresetProfile(tt.vramMB * 1024 * 1024)
			assert.Equal(t, tt.expected, profile)
		})
	}
}

func TestInferenceThreads(t *testing.T) {
	assert.Equal(t, 14, inferenceThreads(16))
	assert.Equal(t, 2, inferenceThreads(2))
	assert.Equal(t, 2, inferenceThreads(1))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd HelixLLM && go test -v -run "TestParseNvidiaSMI|TestParseProcCPU|TestSelectPreset|TestInference" ./internal/shared/hardware/`
Expected: FAIL — package or functions not defined

- [ ] **Step 3: Implement hardware profiler**

```go
// HelixLLM/internal/shared/hardware/profiler.go
package hardware

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type GPUProfile struct {
	Available  bool   `json:"available"`
	Name       string `json:"name"`
	VRAMTotal  int64  `json:"vram_total"`
	VRAMFree   int64  `json:"vram_free"`
	ComputeCap string `json:"compute_cap"`
}

type CPUProfile struct {
	Model   string `json:"model"`
	Cores   int    `json:"cores"`
	Threads int    `json:"threads"`
	AVX2    bool   `json:"avx2"`
	AVX512  bool   `json:"avx512"`
}

type RAMProfile struct {
	Total     int64 `json:"total"`
	Available int64 `json:"available"`
}

type HardwareProfile struct {
	GPU           GPUProfile `json:"gpu"`
	CPU           CPUProfile `json:"cpu"`
	RAM           RAMProfile `json:"ram"`
	PresetProfile string     `json:"preset_profile"`
}

func Detect() (*HardwareProfile, error) {
	gpu := detectGPU()
	cpu := detectCPU()
	ram := detectRAM()
	preset := selectPresetProfile(gpu.VRAMTotal)
	return &HardwareProfile{
		GPU:           gpu,
		CPU:           cpu,
		RAM:           ram,
		PresetProfile: preset,
	}, nil
}

func detectGPU() GPUProfile {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=memory.total,memory.free,name,compute_cap",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return GPUProfile{Available: false}
	}
	gpu, err := parseNvidiaSMI(strings.TrimSpace(string(out)))
	if err != nil {
		return GPUProfile{Available: false}
	}
	return gpu
}

func parseNvidiaSMI(output string) (GPUProfile, error) {
	if output == "" {
		return GPUProfile{Available: false}, nil
	}
	parts := strings.Split(output, ", ")
	if len(parts) < 4 {
		return GPUProfile{Available: false}, fmt.Errorf("unexpected nvidia-smi output: %s", output)
	}
	totalMB, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return GPUProfile{}, fmt.Errorf("parse total VRAM: %w", err)
	}
	freeMB, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return GPUProfile{}, fmt.Errorf("parse free VRAM: %w", err)
	}
	return GPUProfile{
		Available:  true,
		VRAMTotal:  totalMB * 1024 * 1024,
		VRAMFree:   freeMB * 1024 * 1024,
		Name:       strings.TrimSpace(parts[2]),
		ComputeCap: strings.TrimSpace(parts[3]),
	}, nil
}

func detectCPU() CPUProfile {
	out, err := exec.Command("cat", "/proc/cpuinfo").Output()
	if err != nil {
		return CPUProfile{Cores: runtime.NumCPU(), Threads: runtime.NumCPU()}
	}
	cpu, err := parseProcCPUInfo(string(out))
	if err != nil {
		return CPUProfile{Cores: runtime.NumCPU(), Threads: runtime.NumCPU()}
	}
	return cpu
}

func parseProcCPUInfo(output string) (CPUProfile, error) {
	var cpu CPUProfile
	cpu.Threads = runtime.NumCPU()
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "model name":
			if cpu.Model == "" {
				cpu.Model = val
			}
		case "cpu cores":
			cores, err := strconv.Atoi(val)
			if err == nil && cores > cpu.Cores {
				cpu.Cores = cores
			}
		case "siblings":
			threads, err := strconv.Atoi(val)
			if err == nil && threads > cpu.Threads {
				cpu.Threads = threads
			}
		case "flags":
			cpu.AVX2 = strings.Contains(val, "avx2")
			cpu.AVX512 = strings.Contains(val, "avx512")
		}
	}
	if cpu.Cores == 0 {
		cpu.Cores = runtime.NumCPU()
	}
	return cpu, nil
}

func detectRAM() RAMProfile {
	out, err := exec.Command("cat", "/proc/meminfo").Output()
	if err != nil {
		return RAMProfile{}
	}
	var ram RAMProfile
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.TrimSuffix(val, " kB")
		kb, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "MemTotal":
			ram.Total = kb * 1024
		case "MemAvailable":
			ram.Available = kb * 1024
		}
	}
	return ram
}

func selectPresetProfile(vramBytes int64) string {
	vramMB := vramBytes / (1024 * 1024)
	switch {
	case vramMB >= 8192:
		return "high_end"
	case vramMB >= 6144:
		return "consumer_8gb"
	case vramMB >= 4096:
		return "consumer_6gb"
	default:
		return "cpu_only"
	}
}

func inferenceThreads(cores int) int {
	t := cores - 2
	if t < 2 {
		return 2
	}
	return t
}

func (p *HardwareProfile) InferenceThreads() int {
	return inferenceThreads(p.CPU.Cores)
}

func (p *HardwareProfile) BatchThreads() int {
	return p.CPU.Cores
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd HelixLLM && go test -v -run "TestParseNvidiaSMI|TestParseProcCPU|TestSelectPreset|TestInference" ./internal/shared/hardware/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/shared/hardware/profiler.go internal/shared/hardware/profiler_test.go
git commit -m "feat(hardware): add GPU/CPU/RAM hardware profiler with preset selection"
```

---

### Task 2: Model Catalog

**Files:**
- Create: `HelixLLM/internal/brain/models/catalog.go`
- Create: `HelixLLM/internal/brain/models/catalog_test.go`

- [ ] **Step 1: Write failing tests**

```go
// HelixLLM/internal/brain/models/catalog_test.go
package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultCatalog_HasFourModels(t *testing.T) {
	c := DefaultCatalog()
	assert.Len(t, c.Models, 4)
}

func TestDefaultCatalog_HasFastTier(t *testing.T) {
	c := DefaultCatalog()
	fast := c.ByTier(TierFast)
	require.Len(t, fast, 1)
	assert.Equal(t, "qwen2.5-coder-1.5b-instruct-q4_k_m", fast[0].ID)
	assert.True(t, fast[0].SupportsTools)
	assert.True(t, fast[0].RequiresJinja)
	assert.Equal(t, "Apache-2.0", fast[0].License)
}

func TestDefaultCatalog_HasBalancedTier(t *testing.T) {
	c := DefaultCatalog()
	balanced := c.ByTier(TierBalanced)
	require.Len(t, balanced, 1)
	assert.Equal(t, "qwen2.5-coder-3b-instruct-q4_k_m", balanced[0].ID)
}

func TestDefaultCatalog_HasEmbeddingModel(t *testing.T) {
	c := DefaultCatalog()
	embed := c.EmbeddingModels()
	require.Len(t, embed, 1)
	assert.Equal(t, "nomic-embed-text-v1.5-q4_k_m", embed[0].ID)
	assert.Equal(t, 768, embed[0].EmbeddingDims)
	assert.True(t, embed[0].IsEmbedding)
}

func TestCatalog_FilterByVRAM(t *testing.T) {
	c := DefaultCatalog()
	// 3GB VRAM should include 1.5B (1GB) + 3B (2GB) + embed (90MB) but not 8B (5GB)
	filtered := c.FilterByVRAM(3 * 1024 * 1024 * 1024)
	ids := make([]string, len(filtered))
	for i, m := range filtered {
		ids[i] = m.ID
	}
	assert.Contains(t, ids, "qwen2.5-coder-1.5b-instruct-q4_k_m")
	assert.Contains(t, ids, "qwen2.5-coder-3b-instruct-q4_k_m")
	assert.Contains(t, ids, "nomic-embed-text-v1.5-q4_k_m")
	assert.NotContains(t, ids, "functionary-small-v3.2-q4_k_m")
}

func TestCatalog_GetByID(t *testing.T) {
	c := DefaultCatalog()
	m, ok := c.Get("qwen2.5-coder-1.5b-instruct-q4_k_m")
	assert.True(t, ok)
	assert.Equal(t, int64(1_500_000_000), m.Parameters)

	_, ok = c.Get("nonexistent")
	assert.False(t, ok)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd HelixLLM && go test -v -run "TestDefaultCatalog|TestCatalog" ./internal/brain/models/`
Expected: FAIL

- [ ] **Step 3: Implement model catalog**

```go
// HelixLLM/internal/brain/models/catalog.go
package models

type ModelTier string

const (
	TierFast     ModelTier = "fast"
	TierBalanced ModelTier = "balanced"
	TierPowerful ModelTier = "powerful"
)

type ModelDefinition struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	HuggingFaceRepo string   `json:"huggingface_repo"`
	Filename        string    `json:"filename"`
	Parameters      int64     `json:"parameters"`
	Quantization    string    `json:"quantization"`
	VRAMRequired    int64     `json:"vram_required"`
	ContextLength   int       `json:"context_length"`
	Tier            ModelTier `json:"tier"`
	BFCLScore       float64   `json:"bfcl_score"`
	TPSEstimate     [2]int    `json:"tps_estimate"`
	License         string    `json:"license"`
	SupportsTools   bool      `json:"supports_tools"`
	RequiresJinja   bool      `json:"requires_jinja"`
	Architecture    string    `json:"architecture"`
	IsEmbedding     bool      `json:"is_embedding"`
	EmbeddingDims   int       `json:"embedding_dims"`
}

type Catalog struct {
	Models []ModelDefinition `json:"models"`
}

func DefaultCatalog() *Catalog {
	return &Catalog{
		Models: []ModelDefinition{
			{
				ID:              "qwen2.5-coder-1.5b-instruct-q4_k_m",
				Name:            "Qwen2.5 Coder 1.5B Instruct",
				HuggingFaceRepo: "Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF",
				Filename:        "qwen2.5-coder-1.5b-instruct-q4_k_m.gguf",
				Parameters:      1_500_000_000,
				Quantization:    "Q4_K_M",
				VRAMRequired:    1 * 1024 * 1024 * 1024, // 1GB
				ContextLength:   32768,
				Tier:            TierFast,
				BFCLScore:       52.0,
				TPSEstimate:     [2]int{180, 250},
				License:         "Apache-2.0",
				SupportsTools:   true,
				RequiresJinja:   true,
				Architecture:    "qwen2",
			},
			{
				ID:              "qwen2.5-coder-3b-instruct-q4_k_m",
				Name:            "Qwen2.5 Coder 3B Instruct",
				HuggingFaceRepo: "Qwen/Qwen2.5-Coder-3B-Instruct-GGUF",
				Filename:        "qwen2.5-coder-3b-instruct-q4_k_m.gguf",
				Parameters:      3_000_000_000,
				Quantization:    "Q4_K_M",
				VRAMRequired:    2 * 1024 * 1024 * 1024, // 2GB
				ContextLength:   32768,
				Tier:            TierBalanced,
				BFCLScore:       57.0,
				TPSEstimate:     [2]int{120, 160},
				License:         "Apache-2.0",
				SupportsTools:   true,
				RequiresJinja:   true,
				Architecture:    "qwen2",
			},
			{
				ID:              "functionary-small-v3.2-q4_k_m",
				Name:            "Functionary Small v3.2",
				HuggingFaceRepo: "meetkai/functionary-small-v3.2-GGUF",
				Filename:        "functionary-small-v3.2.Q4_0.gguf",
				Parameters:      8_000_000_000,
				Quantization:    "Q4_0",
				VRAMRequired:    5 * 1024 * 1024 * 1024, // 5GB
				ContextLength:   131072,
				Tier:            TierPowerful,
				BFCLScore:       68.4,
				TPSEstimate:     [2]int{45, 65},
				License:         "MIT",
				SupportsTools:   true,
				RequiresJinja:   false,
				Architecture:    "llama",
			},
			{
				ID:              "nomic-embed-text-v1.5-q4_k_m",
				Name:            "Nomic Embed Text v1.5",
				HuggingFaceRepo: "nomic-ai/nomic-embed-text-v1.5-GGUF",
				Filename:        "nomic-embed-text-v1.5.Q4_K_M.gguf",
				Parameters:      137_000_000,
				Quantization:    "Q4_K_M",
				VRAMRequired:    90 * 1024 * 1024, // 90MB
				ContextLength:   2048,
				License:         "Apache-2.0",
				IsEmbedding:     true,
				EmbeddingDims:   768,
				Architecture:    "nomic-bert",
			},
		},
	}
}

func (c *Catalog) ByTier(tier ModelTier) []ModelDefinition {
	var result []ModelDefinition
	for _, m := range c.Models {
		if m.Tier == tier && !m.IsEmbedding {
			result = append(result, m)
		}
	}
	return result
}

func (c *Catalog) EmbeddingModels() []ModelDefinition {
	var result []ModelDefinition
	for _, m := range c.Models {
		if m.IsEmbedding {
			result = append(result, m)
		}
	}
	return result
}

func (c *Catalog) ChatModels() []ModelDefinition {
	var result []ModelDefinition
	for _, m := range c.Models {
		if !m.IsEmbedding {
			result = append(result, m)
		}
	}
	return result
}

func (c *Catalog) FilterByVRAM(maxVRAM int64) []ModelDefinition {
	var result []ModelDefinition
	for _, m := range c.Models {
		if m.VRAMRequired <= maxVRAM {
			result = append(result, m)
		}
	}
	return result
}

func (c *Catalog) Get(id string) (ModelDefinition, bool) {
	for _, m := range c.Models {
		if m.ID == id {
			return m, true
		}
	}
	return ModelDefinition{}, false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd HelixLLM && go test -v -run "TestDefaultCatalog|TestCatalog" ./internal/brain/models/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/brain/models/catalog.go internal/brain/models/catalog_test.go
git commit -m "feat(models): add model catalog with default lightweight fleet"
```

---

### Task 3: Model Registry

**Files:**
- Create: `HelixLLM/internal/brain/models/registry.go`
- Create: `HelixLLM/internal/brain/models/registry_test.go`

- [ ] **Step 1: Write failing tests**

```go
// HelixLLM/internal/brain/models/registry_test.go
package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_AddAndGet(t *testing.T) {
	r := NewRegistry()
	def := DefaultCatalog().Models[0] // 1.5B
	r.Add(RuntimeModel{
		Definition: def,
		Status:     StatusLoaded,
		FilePath:   "/models/" + def.Filename,
		Downloaded: true,
	})
	rm, ok := r.Get(def.ID)
	require.True(t, ok)
	assert.Equal(t, StatusLoaded, rm.Status)
}

func TestRegistry_BestAvailable_FastTier(t *testing.T) {
	r := NewRegistry()
	cat := DefaultCatalog()
	for _, m := range cat.Models {
		r.Add(RuntimeModel{Definition: m, Status: StatusLoaded, Downloaded: true})
	}
	best, ok := r.BestAvailable(TierFast)
	require.True(t, ok)
	assert.Equal(t, "qwen2.5-coder-1.5b-instruct-q4_k_m", best.Definition.ID)
}

func TestRegistry_BestAvailable_FallbackToLower(t *testing.T) {
	r := NewRegistry()
	cat := DefaultCatalog()
	// Only add 1.5B (fast tier)
	r.Add(RuntimeModel{Definition: cat.Models[0], Status: StatusLoaded, Downloaded: true})
	// Request powerful — should fall back to fast
	best, ok := r.BestAvailable(TierPowerful)
	require.True(t, ok)
	assert.Equal(t, TierFast, best.Definition.Tier)
}

func TestRegistry_BestAvailable_NoneLoaded(t *testing.T) {
	r := NewRegistry()
	_, ok := r.BestAvailable(TierFast)
	assert.False(t, ok)
}

func TestRegistry_UpdateStatus(t *testing.T) {
	r := NewRegistry()
	def := DefaultCatalog().Models[0]
	r.Add(RuntimeModel{Definition: def, Status: StatusUnloaded, Downloaded: true})
	r.UpdateStatus(def.ID, StatusLoaded)
	rm, _ := r.Get(def.ID)
	assert.Equal(t, StatusLoaded, rm.Status)
}

func TestRegistry_LoadedModels(t *testing.T) {
	r := NewRegistry()
	cat := DefaultCatalog()
	r.Add(RuntimeModel{Definition: cat.Models[0], Status: StatusLoaded, Downloaded: true})
	r.Add(RuntimeModel{Definition: cat.Models[1], Status: StatusUnloaded, Downloaded: true})
	loaded := r.LoadedModels()
	assert.Len(t, loaded, 1)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd HelixLLM && go test -v -run "TestRegistry" ./internal/brain/models/`
Expected: FAIL

- [ ] **Step 3: Implement model registry**

```go
// HelixLLM/internal/brain/models/registry.go
package models

import (
	"sync"
	"time"
)

type ModelStatus string

const (
	StatusUnloaded ModelStatus = "unloaded"
	StatusLoading  ModelStatus = "loading"
	StatusLoaded   ModelStatus = "loaded"
	StatusError    ModelStatus = "error"
)

type RuntimeModel struct {
	Definition ModelDefinition `json:"definition"`
	Status     ModelStatus     `json:"status"`
	LoadedAt   time.Time       `json:"loaded_at,omitempty"`
	LastUsed   time.Time       `json:"last_used,omitempty"`
	FilePath   string          `json:"file_path"`
	Downloaded bool            `json:"downloaded"`
}

type Registry struct {
	mu     sync.RWMutex
	models map[string]*RuntimeModel
}

func NewRegistry() *Registry {
	return &Registry{
		models: make(map[string]*RuntimeModel),
	}
}

func (r *Registry) Add(rm RuntimeModel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[rm.Definition.ID] = &rm
}

func (r *Registry) Get(id string) (RuntimeModel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rm, ok := r.models[id]
	if !ok {
		return RuntimeModel{}, false
	}
	return *rm, true
}

func (r *Registry) UpdateStatus(id string, status ModelStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rm, ok := r.models[id]; ok {
		rm.Status = status
		if status == StatusLoaded {
			rm.LoadedAt = time.Now()
		}
	}
}

func (r *Registry) MarkUsed(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rm, ok := r.models[id]; ok {
		rm.LastUsed = time.Now()
	}
}

func (r *Registry) BestAvailable(tier ModelTier) (RuntimeModel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try exact tier first
	for _, rm := range r.models {
		if rm.Definition.Tier == tier && rm.Status == StatusLoaded && !rm.Definition.IsEmbedding {
			return *rm, true
		}
	}

	// Fallback chain: powerful → balanced → fast
	fallbackOrder := []ModelTier{TierBalanced, TierFast}
	if tier == TierBalanced {
		fallbackOrder = []ModelTier{TierFast}
	}
	for _, fb := range fallbackOrder {
		for _, rm := range r.models {
			if rm.Definition.Tier == fb && rm.Status == StatusLoaded && !rm.Definition.IsEmbedding {
				return *rm, true
			}
		}
	}
	return RuntimeModel{}, false
}

func (r *Registry) LoadedModels() []RuntimeModel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []RuntimeModel
	for _, rm := range r.models {
		if rm.Status == StatusLoaded {
			result = append(result, *rm)
		}
	}
	return result
}

func (r *Registry) All() []RuntimeModel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []RuntimeModel
	for _, rm := range r.models {
		result = append(result, *rm)
	}
	return result
}

func (r *Registry) ModelNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for _, rm := range r.models {
		if rm.Status == StatusLoaded && !rm.Definition.IsEmbedding {
			names = append(names, rm.Definition.ID)
		}
	}
	return names
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd HelixLLM && go test -v -run "TestRegistry" ./internal/brain/models/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/brain/models/registry.go internal/brain/models/registry_test.go
git commit -m "feat(models): add runtime model registry with tier fallback"
```

---

### Task 4: Complexity Analyzer

**Files:**
- Create: `HelixLLM/internal/brain/complexity.go`
- Create: `HelixLLM/internal/brain/complexity_test.go`

- [ ] **Step 1: Write failing tests**

```go
// HelixLLM/internal/brain/complexity_test.go
package brain

import (
	"strings"
	"testing"

	"github.com/HelixDevelopment/HelixLLM/internal/brain/models"
	"github.com/HelixDevelopment/HelixLLM/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestComplexityAnalyzer_SimpleToolCall(t *testing.T) {
	a := NewComplexityAnalyzer()
	req := &types.InternalChatRequest{
		Messages: []types.InternalMessage{
			{Role: types.RoleUser, Content: "read file main.go"},
		},
		Tools: []types.InternalTool{{Type: "function"}},
	}
	result := a.Analyze(req)
	assert.Equal(t, ComplexitySimple, result.Level)
	assert.Equal(t, models.TierFast, result.TargetTier)
}

func TestComplexityAnalyzer_ComplexMultiTool(t *testing.T) {
	a := NewComplexityAnalyzer()
	req := &types.InternalChatRequest{
		Messages: []types.InternalMessage{
			{Role: types.RoleUser, Content: "analyze and refactor the entire authentication system, compare with the previous implementation"},
		},
		Tools: make([]types.InternalTool, 5),
	}
	result := a.Analyze(req)
	assert.Equal(t, ComplexityComplex, result.Level)
	assert.Equal(t, models.TierPowerful, result.TargetTier)
	assert.True(t, result.Score >= 6)
}

func TestComplexityAnalyzer_ModerateRequest(t *testing.T) {
	a := NewComplexityAnalyzer()
	req := &types.InternalChatRequest{
		Messages: []types.InternalMessage{
			{Role: types.RoleUser, Content: "explain how " + strings.Repeat("x", 600) + " works"},
		},
		Tools: make([]types.InternalTool, 2),
	}
	result := a.Analyze(req)
	assert.Equal(t, ComplexityModerate, result.Level)
	assert.Equal(t, models.TierBalanced, result.TargetTier)
}

func TestComplexityAnalyzer_ExplicitModelOverride(t *testing.T) {
	a := NewComplexityAnalyzer()
	req := &types.InternalChatRequest{
		Model: "qwen2.5-coder-3b-instruct-q4_k_m",
		Messages: []types.InternalMessage{
			{Role: types.RoleUser, Content: "hello"},
		},
	}
	result := a.Analyze(req)
	assert.True(t, result.ModelOverride)
}

func TestComplexityAnalyzer_EmptyRequest(t *testing.T) {
	a := NewComplexityAnalyzer()
	req := &types.InternalChatRequest{}
	result := a.Analyze(req)
	assert.Equal(t, ComplexitySimple, result.Level)
	assert.Equal(t, models.TierFast, result.TargetTier)
}

func TestComplexityAnalyzer_LongConversation(t *testing.T) {
	a := NewComplexityAnalyzer()
	msgs := make([]types.InternalMessage, 8)
	for i := range msgs {
		msgs[i] = types.InternalMessage{Role: types.RoleUser, Content: "turn"}
	}
	req := &types.InternalChatRequest{Messages: msgs}
	result := a.Analyze(req)
	assert.True(t, result.Score >= 1) // conversation length adds score
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd HelixLLM && go test -v -run "TestComplexityAnalyzer" ./internal/brain/`
Expected: FAIL

- [ ] **Step 3: Implement complexity analyzer**

```go
// HelixLLM/internal/brain/complexity.go
package brain

import (
	"strings"

	"github.com/HelixDevelopment/HelixLLM/internal/brain/models"
	"github.com/HelixDevelopment/HelixLLM/pkg/types"
)

type ComplexityLevel string

const (
	ComplexitySimple   ComplexityLevel = "simple"
	ComplexityModerate ComplexityLevel = "moderate"
	ComplexityComplex  ComplexityLevel = "complex"
)

type ComplexityResult struct {
	Level         ComplexityLevel `json:"level"`
	Score         int             `json:"score"`
	Reasons       []string        `json:"reasons"`
	TargetTier    models.ModelTier `json:"target_tier"`
	ModelOverride bool            `json:"model_override"`
}

type ComplexityAnalyzer struct {
	complexKeywords []string
}

func NewComplexityAnalyzer() *ComplexityAnalyzer {
	return &ComplexityAnalyzer{
		complexKeywords: []string{
			"analyze", "compare", "explain", "refactor",
			"architect", "investigate", "comprehensive", "thorough",
		},
	}
}

func (a *ComplexityAnalyzer) Analyze(req *types.InternalChatRequest) ComplexityResult {
	if req.Model != "" {
		return ComplexityResult{
			Level:         ComplexitySimple,
			TargetTier:    models.TierFast,
			ModelOverride: true,
		}
	}

	score := 0
	var reasons []string

	// Tool count
	toolCount := len(req.Tools)
	if toolCount > 3 {
		score += 2
		reasons = append(reasons, "many tools (>3)")
	} else if toolCount > 0 {
		score += 1
		reasons = append(reasons, "has tools")
	}

	// Message content length (last user message)
	lastContent := lastUserContent(req.Messages)
	contentLen := len(lastContent)
	if contentLen > 2000 {
		score += 2
		reasons = append(reasons, "very long message (>2000 chars)")
	} else if contentLen > 500 {
		score += 1
		reasons = append(reasons, "long message (>500 chars)")
	}

	// Complexity keywords (max +3)
	keywordHits := 0
	lower := strings.ToLower(lastContent)
	for _, kw := range a.complexKeywords {
		if strings.Contains(lower, kw) {
			keywordHits++
			if keywordHits <= 3 {
				score++
				reasons = append(reasons, "keyword: "+kw)
			}
		}
	}

	// Conversation length
	if len(req.Messages) > 5 {
		score++
		reasons = append(reasons, "long conversation (>5 turns)")
	}

	// System prompt length
	for _, m := range req.Messages {
		if m.Role == types.RoleSystem && len(m.Content) > 1000 {
			score++
			reasons = append(reasons, "long system prompt (>1000 chars)")
			break
		}
	}

	level, tier := classifyScore(score)
	return ComplexityResult{
		Level:      level,
		Score:      score,
		Reasons:    reasons,
		TargetTier: tier,
	}
}

func classifyScore(score int) (ComplexityLevel, models.ModelTier) {
	switch {
	case score >= 6:
		return ComplexityComplex, models.TierPowerful
	case score >= 3:
		return ComplexityModerate, models.TierBalanced
	default:
		return ComplexitySimple, models.TierFast
	}
}

func lastUserContent(msgs []types.InternalMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == types.RoleUser {
			return msgs[i].Content
		}
	}
	return ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd HelixLLM && go test -v -run "TestComplexityAnalyzer" ./internal/brain/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/brain/complexity.go internal/brain/complexity_test.go
git commit -m "feat(brain): add task complexity analyzer with heuristic scoring"
```

---

## Phase 2: Model Management

### Task 5: Preset Generator

**Files:**
- Create: `HelixLLM/internal/brain/models/preset.go`
- Create: `HelixLLM/internal/brain/models/preset_test.go`

- [ ] **Step 1: Write failing tests**

```go
// HelixLLM/internal/brain/models/preset_test.go
package models

import (
	"strings"
	"testing"

	"github.com/HelixDevelopment/HelixLLM/internal/shared/hardware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePresets_Consumer6GB(t *testing.T) {
	profile := &hardware.HardwareProfile{
		GPU:           hardware.GPUProfile{Available: true, VRAMTotal: 6 * 1024 * 1024 * 1024},
		CPU:           hardware.CPUProfile{Cores: 16},
		PresetProfile: "consumer_6gb",
	}
	cat := DefaultCatalog()
	filtered := cat.FilterByVRAM(6 * 1024 * 1024 * 1024)
	ini, err := GeneratePresets(filtered, profile)
	require.NoError(t, err)
	assert.Contains(t, ini, "[global]")
	assert.Contains(t, ini, "flash-attn = on")
	assert.Contains(t, ini, "n-threads = 14")
	assert.Contains(t, ini, "[qwen2.5-coder-1.5b-instruct-q4_k_m]")
	assert.Contains(t, ini, "chat-template = jinja")
	assert.Contains(t, ini, "n-gpu-layers = -1")
	assert.Contains(t, ini, "[nomic-embed-text-v1.5-q4_k_m]")
	assert.Contains(t, ini, "embedding = on")
}

func TestGeneratePresets_CPUOnly(t *testing.T) {
	profile := &hardware.HardwareProfile{
		GPU:           hardware.GPUProfile{Available: false},
		CPU:           hardware.CPUProfile{Cores: 8},
		PresetProfile: "cpu_only",
	}
	models := []ModelDefinition{DefaultCatalog().Models[0]} // just 1.5B
	ini, err := GeneratePresets(models, profile)
	require.NoError(t, err)
	assert.Contains(t, ini, "n-gpu-layers = 0")
	assert.Contains(t, ini, "ctx-size = 2048")
	assert.Contains(t, ini, "n-threads = 6")
}

func TestGeneratePresets_EmbeddingModel(t *testing.T) {
	profile := &hardware.HardwareProfile{
		GPU:           hardware.GPUProfile{Available: true, VRAMTotal: 8 * 1024 * 1024 * 1024},
		CPU:           hardware.CPUProfile{Cores: 16},
		PresetProfile: "consumer_8gb",
	}
	cat := DefaultCatalog()
	ini, err := GeneratePresets(cat.Models, profile)
	require.NoError(t, err)
	// Embedding model should have embedding = on and no chat-template
	embedSection := extractSection(ini, "nomic-embed-text-v1.5-q4_k_m")
	assert.Contains(t, embedSection, "embedding = on")
	assert.NotContains(t, embedSection, "chat-template")
}

func extractSection(ini, section string) string {
	start := strings.Index(ini, "["+section+"]")
	if start < 0 {
		return ""
	}
	end := strings.Index(ini[start+1:], "\n[")
	if end < 0 {
		return ini[start:]
	}
	return ini[start : start+1+end]
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd HelixLLM && go test -v -run "TestGeneratePresets" ./internal/brain/models/`
Expected: FAIL

- [ ] **Step 3: Implement preset generator**

```go
// HelixLLM/internal/brain/models/preset.go
package models

import (
	"fmt"
	"strings"

	"github.com/HelixDevelopment/HelixLLM/internal/shared/hardware"
)

type presetProfile struct {
	GPULayers   int
	ContextSize int
	BatchSize   int
}

var presetProfiles = map[string]presetProfile{
	"cpu_only":     {GPULayers: 0, ContextSize: 2048, BatchSize: 256},
	"consumer_6gb": {GPULayers: -1, ContextSize: 4096, BatchSize: 512},
	"consumer_8gb": {GPULayers: -1, ContextSize: 8192, BatchSize: 1024},
	"high_end":     {GPULayers: -1, ContextSize: 16384, BatchSize: 1024},
}

func GeneratePresets(defs []ModelDefinition, profile *hardware.HardwareProfile) (string, error) {
	pp, ok := presetProfiles[profile.PresetProfile]
	if !ok {
		pp = presetProfiles["cpu_only"]
	}

	var b strings.Builder

	// Global section
	b.WriteString("[global]\n")
	if profile.GPU.Available {
		b.WriteString("flash-attn = on\n")
	}
	b.WriteString(fmt.Sprintf("n-threads = %d\n", profile.InferenceThreads()))
	b.WriteString(fmt.Sprintf("n-threads-batch = %d\n", profile.BatchThreads()))
	b.WriteString("\n")

	// Per-model sections
	for _, def := range defs {
		b.WriteString(fmt.Sprintf("[%s]\n", def.ID))
		b.WriteString(fmt.Sprintf("model = /models/%s\n", def.Filename))
		b.WriteString(fmt.Sprintf("n-gpu-layers = %d\n", pp.GPULayers))

		if def.IsEmbedding {
			b.WriteString("embedding = on\n")
		} else {
			b.WriteString(fmt.Sprintf("ctx-size = %d\n", pp.ContextSize))
			b.WriteString(fmt.Sprintf("n-batch = %d\n", pp.BatchSize))
			if def.RequiresJinja {
				b.WriteString("chat-template = jinja\n")
			}
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd HelixLLM && go test -v -run "TestGeneratePresets" ./internal/brain/models/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/brain/models/preset.go internal/brain/models/preset_test.go
git commit -m "feat(models): add llama.cpp INI preset generator"
```

---

### Task 6: Model Downloader

**Files:**
- Create: `HelixLLM/internal/brain/downloader.go`
- Create: `HelixLLM/internal/brain/downloader_test.go`

- [ ] **Step 1: Write failing tests**

```go
// HelixLLM/internal/brain/downloader_test.go
package brain

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloader_Download_Success(t *testing.T) {
	content := []byte("fake model data for testing")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "27")
		w.Write(content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := NewDownloader(dir)
	d.baseURL = srv.URL // override for testing

	err := d.Download(context.Background(), DownloadRequest{
		URL:      srv.URL + "/model.gguf",
		Filename: "test-model.gguf",
	})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "test-model.gguf"))
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestDownloader_EnsureModels_SkipExisting(t *testing.T) {
	dir := t.TempDir()
	// Create a fake existing model file
	err := os.WriteFile(filepath.Join(dir, "existing.gguf"), []byte("model"), 0644)
	require.NoError(t, err)

	d := NewDownloader(dir)
	exists := d.ModelExists("existing.gguf")
	assert.True(t, exists)

	missing := d.ModelExists("nonexistent.gguf")
	assert.False(t, missing)
}

func TestManifest_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	m := &ModelManifest{
		Models: map[string]ManifestEntry{
			"test": {ModelID: "test", Filename: "test.gguf", SizeBytes: 100, Verified: true},
		},
	}
	err := m.Save(filepath.Join(dir, "manifest.json"))
	require.NoError(t, err)

	loaded, err := LoadManifest(filepath.Join(dir, "manifest.json"))
	require.NoError(t, err)
	assert.Equal(t, "test", loaded.Models["test"].ModelID)
	assert.True(t, loaded.Models["test"].Verified)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd HelixLLM && go test -v -run "TestDownloader|TestManifest" ./internal/brain/`
Expected: FAIL

- [ ] **Step 3: Implement downloader**

```go
// HelixLLM/internal/brain/downloader.go
package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
)

type DownloadRequest struct {
	URL      string
	Filename string
	SHA256   string
}

type ModelManifest struct {
	Models    map[string]ManifestEntry `json:"models"`
	UpdatedAt time.Time               `json:"updated_at"`
}

type ManifestEntry struct {
	ModelID      string    `json:"model_id"`
	Filename     string    `json:"filename"`
	SHA256       string    `json:"sha256,omitempty"`
	SizeBytes    int64     `json:"size_bytes"`
	DownloadedAt time.Time `json:"downloaded_at"`
	Verified     bool      `json:"verified"`
}

type Downloader struct {
	modelsDir string
	baseURL   string
	client    *http.Client
}

func NewDownloader(modelsDir string) *Downloader {
	return &Downloader{
		modelsDir: modelsDir,
		baseURL:   "https://huggingface.co",
		client:    &http.Client{Timeout: 30 * time.Minute},
	}
}

func (d *Downloader) ModelExists(filename string) bool {
	path := filepath.Join(d.modelsDir, filename)
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

func (d *Downloader) Download(ctx context.Context, req DownloadRequest) error {
	if err := os.MkdirAll(d.modelsDir, 0755); err != nil {
		return fmt.Errorf("create models dir: %w", err)
	}

	destPath := filepath.Join(d.modelsDir, req.Filename)
	tmpPath := destPath + ".tmp"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", req.URL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if token := os.Getenv("HF_TOKEN"); token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	written, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write model: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	log.WithFields(log.Fields{
		"file": req.Filename,
		"size": written,
	}).Info("Model downloaded")

	return nil
}

func (d *Downloader) HuggingFaceURL(repo, filename string) string {
	return fmt.Sprintf("%s/%s/resolve/main/%s", d.baseURL, repo, filename)
}

func (m *ModelManifest) Save(path string) error {
	m.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadManifest(path string) (*ModelManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ModelManifest{Models: make(map[string]ManifestEntry)}, nil
		}
		return nil, err
	}
	var m ModelManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Models == nil {
		m.Models = make(map[string]ManifestEntry)
	}
	return &m, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd HelixLLM && go test -v -run "TestDownloader|TestManifest" ./internal/brain/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/brain/downloader.go internal/brain/downloader_test.go
git commit -m "feat(brain): add HuggingFace GGUF model downloader with manifest"
```

---

### Task 7: llama-server Manager

**Files:**
- Create: `HelixLLM/internal/brain/server.go`
- Create: `HelixLLM/internal/brain/server_test.go`

- [ ] **Step 1: Write failing tests**

```go
// HelixLLM/internal/brain/server_test.go
package brain

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLlamaServer_HealthCheck_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}
	}))
	defer srv.Close()

	ls := &LlamaServer{baseURL: srv.URL, client: http.DefaultClient}
	assert.True(t, ls.HealthCheck())
}

func TestLlamaServer_HealthCheck_Unhealthy(t *testing.T) {
	ls := &LlamaServer{baseURL: "http://127.0.0.1:1", client: http.DefaultClient}
	assert.False(t, ls.HealthCheck())
}

func TestLlamaServerConfig_BuildArgs(t *testing.T) {
	cfg := LlamaServerConfig{
		Port:        8080,
		ModelsDir:   "/models",
		PresetsPath: "/config/presets.ini",
		MaxModels:   3,
		Threads:     14,
		ThreadsBatch: 16,
	}
	args := cfg.BuildArgs()
	assert.Contains(t, args, "--host")
	assert.Contains(t, args, "0.0.0.0")
	assert.Contains(t, args, "--port")
	assert.Contains(t, args, "8080")
	assert.Contains(t, args, "--models-dir")
	assert.Contains(t, args, "/models")
	assert.Contains(t, args, "--models-preset")
	assert.Contains(t, args, "/config/presets.ini")
	assert.Contains(t, args, "--models-max")
	assert.Contains(t, args, "3")
	assert.Contains(t, args, "--models-autoload")
	assert.Contains(t, args, "--metrics")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd HelixLLM && go test -v -run "TestLlamaServer" ./internal/brain/`
Expected: FAIL

- [ ] **Step 3: Implement llama-server manager**

```go
// HelixLLM/internal/brain/server.go
package brain

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

type LlamaServerConfig struct {
	BinaryPath   string
	Port         int
	ModelsDir    string
	PresetsPath  string
	MaxModels    int
	Threads      int
	ThreadsBatch int
}

func (c *LlamaServerConfig) BuildArgs() []string {
	args := []string{
		"--host", "0.0.0.0",
		"--port", strconv.Itoa(c.Port),
		"--models-dir", c.ModelsDir,
		"--models-preset", c.PresetsPath,
		"--models-max", strconv.Itoa(c.MaxModels),
		"--models-autoload",
		"--threads", strconv.Itoa(c.Threads),
		"--threads-batch", strconv.Itoa(c.ThreadsBatch),
		"--metrics",
	}
	return args
}

type LlamaServer struct {
	cfg    LlamaServerConfig
	cmd    *exec.Cmd
	baseURL string
	client *http.Client
	mu     sync.Mutex
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewLlamaServer(cfg LlamaServerConfig) *LlamaServer {
	return &LlamaServer{
		cfg:     cfg,
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", cfg.Port),
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (s *LlamaServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, s.cancel = context.WithCancel(ctx)
	binary := s.cfg.BinaryPath
	if binary == "" {
		binary = "llama-server"
	}

	args := s.cfg.BuildArgs()
	s.cmd = exec.CommandContext(ctx, binary, args...)

	log.WithField("args", args).Info("Starting llama-server")

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("start llama-server: %w", err)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.cmd.Wait(); err != nil {
			log.WithError(err).Warn("llama-server exited")
		}
	}()

	return nil
}

func (s *LlamaServer) WaitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("llama-server not ready after %v", timeout)
		case <-ticker.C:
			if s.HealthCheck() {
				log.Info("llama-server is ready")
				return nil
			}
		}
	}
}

func (s *LlamaServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Signal(syscall.SIGTERM)
	}
	s.wg.Wait()
	log.Info("llama-server stopped")
	return nil
}

func (s *LlamaServer) HealthCheck() bool {
	resp, err := s.client.Get(s.baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *LlamaServer) BaseURL() string {
	return s.baseURL
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd HelixLLM && go test -v -run "TestLlamaServer" ./internal/brain/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/brain/server.go internal/brain/server_test.go
git commit -m "feat(brain): add llama-server process lifecycle manager"
```

---

## Phase 3: Infrastructure

### Task 8: Local Embedding via llama-server

**Files:**
- Create: `HelixLLM/internal/knowledge/llama_embedder.go`
- Create: `HelixLLM/internal/knowledge/llama_embedder_test.go`
- Modify: `HelixLLM/internal/knowledge/embedding_providers.go`

- [ ] **Step 1: Write failing tests**

```go
// HelixLLM/internal/knowledge/llama_embedder_test.go
package knowledge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HelixDevelopment/HelixLLM/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLlamaEmbedder_Embed_Success(t *testing.T) {
	embedding := make([]float64, 768)
	for i := range embedding {
		embedding[i] = float64(i) * 0.001
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := api.EmbeddingResponse{
			Object: "list",
			Data:   []api.EmbeddingData{{Object: "embedding", Embedding: embedding, Index: 0}},
			Model:  "nomic-embed-text-v1.5-q4_k_m",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e := NewLlamaEmbedder(srv.URL, "nomic-embed-text-v1.5-q4_k_m", 768)
	result, err := e.Embed("test text")
	require.NoError(t, err)
	assert.Len(t, result, 768)
	assert.InDelta(t, 0.001, result[1], 0.0001)
}

func TestLlamaEmbedder_Dimension(t *testing.T) {
	e := NewLlamaEmbedder("http://unused", "model", 768)
	assert.Equal(t, 768, e.Dimension())
}

func TestLlamaEmbedder_EmbedBatch(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		embedding := make([]float64, 768)
		resp := api.EmbeddingResponse{
			Object: "list",
			Data:   []api.EmbeddingData{{Object: "embedding", Embedding: embedding, Index: 0}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e := NewLlamaEmbedder(srv.URL, "model", 768)
	results, err := e.EmbedBatch([]string{"text1", "text2", "text3"})
	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, 3, callCount)
}

func TestLlamaEmbedder_ServerDown(t *testing.T) {
	e := NewLlamaEmbedder("http://127.0.0.1:1", "model", 768)
	_, err := e.Embed("test")
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd HelixLLM && go test -v -run "TestLlamaEmbedder" ./internal/knowledge/`
Expected: FAIL

- [ ] **Step 3: Implement llama embedder**

```go
// HelixLLM/internal/knowledge/llama_embedder.go
package knowledge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/HelixDevelopment/HelixLLM/pkg/api"
)

type LlamaEmbedder struct {
	baseURL   string
	model     string
	dimension int
	client    *http.Client
}

func NewLlamaEmbedder(baseURL, model string, dimension int) *LlamaEmbedder {
	return &LlamaEmbedder{
		baseURL:   baseURL,
		model:     model,
		dimension: dimension,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *LlamaEmbedder) Embed(text string) ([]float64, error) {
	req := api.EmbeddingRequest{
		Model: e.model,
		Input: text,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := e.client.Post(e.baseURL+"/v1/embeddings", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding failed: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var embResp api.EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return embResp.Data[0].Embedding, nil
}

func (e *LlamaEmbedder) EmbedBatch(texts []string) ([][]float64, error) {
	results := make([][]float64, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		results[i] = emb
	}
	return results, nil
}

func (e *LlamaEmbedder) Dimension() int {
	return e.dimension
}
```

- [ ] **Step 4: Add "llama" to the embedder factory**

In `HelixLLM/internal/knowledge/embedding_providers.go`, add to the `NewEmbedder` function's switch statement:

```go
case "llama":
    return NewLlamaEmbedder(apiKey, model, dimension), nil
```

Where `apiKey` is repurposed as the llama-server base URL when provider is "llama".

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd HelixLLM && go test -v -run "TestLlamaEmbedder" ./internal/knowledge/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
cd HelixLLM && git add internal/knowledge/llama_embedder.go internal/knowledge/llama_embedder_test.go internal/knowledge/embedding_providers.go
git commit -m "feat(knowledge): add local embedding via llama-server nomic model"
```

---

### Task 9: CUDA Router Containerfile

**Files:**
- Create: `HelixLLM/container/Containerfile.llamacpp-router`

- [ ] **Step 1: Create CUDA-enabled multi-model Containerfile**

```dockerfile
# HelixLLM/container/Containerfile.llamacpp-router
# Multi-stage build: CUDA-enabled llama-server with router mode support
# Replaces CPU-only Containerfile.llamacpp for multi-model fleet

# Stage 1: Build llama.cpp with CUDA
FROM nvidia/cuda:12.6.3-devel-ubuntu24.04 AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake git build-essential pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build
RUN git clone --depth 1 https://github.com/ggml-org/llama.cpp.git

WORKDIR /build/llama.cpp
RUN cmake -B build \
    -DGGML_CUDA=ON \
    -DGGML_CUDA_FA_ALL_QUANTS=ON \
    -DGGML_NATIVE=OFF \
    -DGGML_RPC=ON \
    -DBUILD_SHARED_LIBS=OFF \
    -DCMAKE_BUILD_TYPE=Release \
    && cmake --build build --config Release -j$(nproc) --target llama-server llama-cli rpc-server

# Stage 2: Runtime
FROM nvidia/cuda:12.6.3-runtime-ubuntu24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/llama.cpp/build/bin/llama-server /usr/local/bin/
COPY --from=builder /build/llama.cpp/build/bin/llama-cli /usr/local/bin/
COPY --from=builder /build/llama.cpp/build/bin/rpc-server /usr/local/bin/

RUN mkdir -p /models /config

EXPOSE 8080 50052

HEALTHCHECK --interval=30s --timeout=10s --start-period=120s --retries=3 \
    CMD curl -sf http://localhost:8080/health || exit 1

ENV HELIX_MODELS_MAX=3
ENV HELIX_THREADS=14
ENV HELIX_THREADS_BATCH=16

ENTRYPOINT ["llama-server"]
CMD ["--host", "0.0.0.0", "--port", "8080", \
     "--models-dir", "/models", \
     "--models-max", "3", \
     "--models-autoload", \
     "--threads", "14", \
     "--threads-batch", "16", \
     "--metrics"]
```

- [ ] **Step 2: Commit**

```bash
cd HelixLLM && git add container/Containerfile.llamacpp-router
git commit -m "feat(container): add CUDA-enabled llama-server with router mode"
```

---

## Phase 4: Wiring

### Task 10: Config Updates

**Files:**
- Modify: `HelixLLM/internal/shared/config/config.go`

- [ ] **Step 1: Add model fleet config fields to LLMConfig**

Add these fields to the existing `LLMConfig` struct in `HelixLLM/internal/shared/config/config.go`:

```go
// Add to LLMConfig struct:
ModelsDir         string `env:"HELIX_MODELS_DIR" default:"/models"`
ModelsAutoDownload bool   `env:"HELIX_MODELS_AUTO_DOWNLOAD" default:"true"`
ModelsMax         int    `env:"HELIX_MODELS_MAX" default:"3"`
ComplexityEnabled bool   `env:"HELIX_COMPLEXITY_ENABLED" default:"true"`
ComplexityDefault string `env:"HELIX_COMPLEXITY_DEFAULT_TIER" default:"fast"`
LlamaServerPort   int    `env:"HELIX_LLAMA_SERVER_PORT" default:"8080"`
LlamaServerEmbed  bool   `env:"HELIX_LLAMA_SERVER_EMBEDDED" default:"true"`
```

- [ ] **Step 2: Commit**

```bash
cd HelixLLM && git add internal/shared/config/config.go
git commit -m "feat(config): add model fleet configuration env vars"
```

---

### Task 11: Brain Router Integration

**Files:**
- Modify: `HelixLLM/internal/brain/brain.go`
- Modify: `HelixLLM/internal/brain/llamacpp.go`
- Modify: `HelixLLM/internal/brain/router.go`

- [ ] **Step 1: Update Brain struct to include complexity analyzer and model registry**

Add to `Brain` struct in `brain.go`:

```go
import "github.com/HelixDevelopment/HelixLLM/internal/brain/models"

// Add fields to Brain struct:
complexity *ComplexityAnalyzer
registry   *models.Registry
```

Add to `Config` struct:

```go
ComplexityEnabled bool
Registry          *models.Registry
```

In `New()`, initialize:

```go
b.complexity = NewComplexityAnalyzer()
b.registry = cfg.Registry
```

- [ ] **Step 2: Inject complexity analysis into Complete and CompleteStream**

Before calling `b.router.Route(req)` in both `Complete` and `CompleteStream`, add:

```go
if b.complexity != nil && b.registry != nil && req.Model == "" {
    result := b.complexity.Analyze(req)
    if !result.ModelOverride {
        if best, ok := b.registry.BestAvailable(result.TargetTier); ok {
            req.Model = best.Definition.ID
            b.registry.MarkUsed(best.Definition.ID)
        }
    }
}
```

- [ ] **Step 3: Update LlamaCppProvider to use dynamic model list from registry**

In `llamacpp.go`, the `Models()` method currently returns a static list. If a registry is available, it should return `registry.ModelNames()` instead. Add a registry field:

```go
type LlamaCppProvider struct {
    baseURL  string
    models   []string
    client   *http.Client
    registry *models.Registry
}

func (p *LlamaCppProvider) Models() []string {
    if p.registry != nil {
        return p.registry.ModelNames()
    }
    return p.models
}
```

- [ ] **Step 4: Run existing brain tests to confirm no regressions**

Run: `cd HelixLLM && go test -v ./internal/brain/...`
Expected: PASS (all existing + new tests)

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/brain/brain.go internal/brain/llamacpp.go internal/brain/router.go
git commit -m "feat(brain): wire complexity analyzer and model registry into routing"
```

---

### Task 12: Gateway Endpoints

**Files:**
- Modify: `HelixLLM/internal/gateway/router.go`

- [ ] **Step 1: Add hardware and model management endpoints**

Add to the route registration in `gateway/router.go`:

```go
// Add these routes in RegisterRoutes:
v1.GET("/hardware", HandleHardware(opts.HardwareProfile))
v1.POST("/models/:id/download", HandleModelDownload(opts.Downloader, opts.Catalog))
```

Add handler functions:

```go
func HandleHardware(profile *hardware.HardwareProfile) gin.HandlerFunc {
    return func(c *gin.Context) {
        if profile == nil {
            c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hardware profiler not initialized"})
            return
        }
        c.JSON(http.StatusOK, profile)
    }
}
```

- [ ] **Step 2: Update RouterOptions to accept new dependencies**

```go
// Add to RouterOptions struct:
HardwareProfile *hardware.HardwareProfile
Downloader      *brain.Downloader
Catalog         *models.Catalog
```

- [ ] **Step 3: Commit**

```bash
cd HelixLLM && git add internal/gateway/router.go
git commit -m "feat(gateway): add /v1/hardware and model management endpoints"
```

---

### Task 13: Boot Sequence in main.go

**Files:**
- Modify: `HelixLLM/cmd/helixllm/main.go`

- [ ] **Step 1: Wire all new components into the boot sequence**

Add the following initialization block after config loading and before Brain creation in `main.go`:

```go
import (
    "github.com/HelixDevelopment/HelixLLM/internal/shared/hardware"
    "github.com/HelixDevelopment/HelixLLM/internal/brain/models"
)

// After cfg loaded, before Brain creation:

// 1. Hardware detection
hwProfile, err := hardware.Detect()
if err != nil {
    log.WithError(err).Warn("Hardware detection failed, using defaults")
    hwProfile = &hardware.HardwareProfile{
        CPU:           hardware.CPUProfile{Cores: runtime.NumCPU()},
        PresetProfile: "cpu_only",
    }
}
log.WithFields(log.Fields{
    "gpu":     hwProfile.GPU.Available,
    "gpu_name": hwProfile.GPU.Name,
    "vram_mb": hwProfile.GPU.VRAMTotal / (1024 * 1024),
    "preset":  hwProfile.PresetProfile,
}).Info("Hardware detected")

// 2. Model catalog
catalog := models.DefaultCatalog()
available := catalog.FilterByVRAM(hwProfile.GPU.VRAMFree)

// 3. Model registry
registry := models.NewRegistry()
for _, def := range available {
    rm := models.RuntimeModel{Definition: def, Status: models.StatusUnloaded}
    rm.FilePath = filepath.Join(cfg.LLM.ModelsDir, def.Filename)
    rm.Downloaded = downloader.ModelExists(def.Filename)
    registry.Add(rm)
}

// 4. Download missing models
downloader := NewDownloader(cfg.LLM.ModelsDir)
if cfg.LLM.ModelsAutoDownload {
    for _, def := range available {
        if !downloader.ModelExists(def.Filename) {
            url := downloader.HuggingFaceURL(def.HuggingFaceRepo, def.Filename)
            log.WithField("model", def.ID).Info("Downloading model")
            if err := downloader.Download(ctx, DownloadRequest{URL: url, Filename: def.Filename}); err != nil {
                log.WithError(err).WithField("model", def.ID).Error("Failed to download model")
                continue
            }
            registry.UpdateStatus(def.ID, models.StatusUnloaded)
        }
    }
}

// 5. Generate presets and start llama-server (embedded mode)
if cfg.LLM.LlamaServerEmbed {
    var downloadedModels []models.ModelDefinition
    for _, def := range available {
        if downloader.ModelExists(def.Filename) {
            downloadedModels = append(downloadedModels, def)
        }
    }
    presetsINI, _ := models.GeneratePresets(downloadedModels, hwProfile)
    presetsPath := filepath.Join(os.TempDir(), "helixllm-presets.ini")
    os.WriteFile(presetsPath, []byte(presetsINI), 0644)

    llamaSrv := NewLlamaServer(LlamaServerConfig{
        Port:         cfg.LLM.LlamaServerPort,
        ModelsDir:    cfg.LLM.ModelsDir,
        PresetsPath:  presetsPath,
        MaxModels:    cfg.LLM.ModelsMax,
        Threads:      hwProfile.InferenceThreads(),
        ThreadsBatch: hwProfile.BatchThreads(),
    })
    if err := llamaSrv.Start(ctx); err != nil {
        log.WithError(err).Fatal("Failed to start llama-server")
    }
    defer llamaSrv.Stop()

    if err := llamaSrv.WaitReady(ctx, 120*time.Second); err != nil {
        log.WithError(err).Fatal("llama-server not ready")
    }

    // Update registry status
    for _, def := range downloadedModels {
        registry.UpdateStatus(def.ID, models.StatusLoaded)
    }

    // Override brain config to point to embedded llama-server
    cfg.LLM.LocalRPCHost = "127.0.0.1"
    cfg.LLM.LocalRPCPort = cfg.LLM.LlamaServerPort
}

// 6. Create Brain with registry
brainSvc := brain.New(brain.Config{
    LlamaCppURL:       fmt.Sprintf("http://%s:%d", cfg.LLM.LocalRPCHost, cfg.LLM.LocalRPCPort),
    LlamaCppModels:    registry.ModelNames(),
    OpenAIKey:         cfg.LLM.OpenAIKey,
    OpenAIBaseURL:     cfg.LLM.OpenAIBaseURL,
    AnthropicKey:      cfg.LLM.AnthropicKey,
    DefaultProvider:   cfg.LLM.DefaultProvider,
    ComplexityEnabled: cfg.LLM.ComplexityEnabled,
    Registry:          registry,
})

// 7. Use local embedder if llama-server available
if cfg.Knowledge.EmbeddingProvider == "local" || cfg.Knowledge.EmbeddingProvider == "llama" {
    cfg.Knowledge.EmbeddingProvider = "llama"
    // apiKey field carries the base URL for llama provider
    embedder, err = knowledge.NewEmbedder("llama",
        fmt.Sprintf("http://127.0.0.1:%d", cfg.LLM.LlamaServerPort),
        "nomic-embed-text-v1.5-q4_k_m", 768)
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd HelixLLM && go build ./cmd/helixllm/`
Expected: Compiles successfully

- [ ] **Step 3: Commit**

```bash
cd HelixLLM && git add cmd/helixllm/main.go
git commit -m "feat: wire multi-model fleet into boot sequence"
```

---

## Phase 5: Validation

### Task 14: Integration Test

**Files:**
- Create: `HelixLLM/tests/integration/multi_model_routing_test.go`

- [ ] **Step 1: Write integration test**

```go
// HelixLLM/tests/integration/multi_model_routing_test.go
package integration

import (
	"testing"

	"github.com/HelixDevelopment/HelixLLM/internal/brain"
	"github.com/HelixDevelopment/HelixLLM/internal/brain/models"
	"github.com/HelixDevelopment/HelixLLM/internal/shared/hardware"
	"github.com/HelixDevelopment/HelixLLM/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComplexity_To_Registry_Routing(t *testing.T) {
	// Setup registry with mock loaded models
	reg := models.NewRegistry()
	cat := models.DefaultCatalog()
	for _, m := range cat.Models[:2] { // 1.5B and 3B
		reg.Add(models.RuntimeModel{
			Definition: m,
			Status:     models.StatusLoaded,
			Downloaded: true,
		})
	}

	// Test simple request routes to fast model
	analyzer := brain.NewComplexityAnalyzer()
	simpleReq := &types.InternalChatRequest{
		Messages: []types.InternalMessage{
			{Role: types.RoleUser, Content: "list files"},
		},
	}
	result := analyzer.Analyze(simpleReq)
	best, ok := reg.BestAvailable(result.TargetTier)
	require.True(t, ok)
	assert.Equal(t, "qwen2.5-coder-1.5b-instruct-q4_k_m", best.Definition.ID)

	// Test complex request routes to balanced model
	complexReq := &types.InternalChatRequest{
		Messages: []types.InternalMessage{
			{Role: types.RoleUser, Content: "analyze and refactor the authentication module, compare implementations"},
		},
		Tools: make([]types.InternalTool, 4),
	}
	result2 := analyzer.Analyze(complexReq)
	best2, ok := reg.BestAvailable(result2.TargetTier)
	require.True(t, ok)
	// Should fall back to balanced since powerful isn't loaded
	assert.Equal(t, "qwen2.5-coder-3b-instruct-q4_k_m", best2.Definition.ID)
}

func TestPresetGeneration_MatchesProfile(t *testing.T) {
	profile := &hardware.HardwareProfile{
		GPU:           hardware.GPUProfile{Available: true, VRAMTotal: 6 * 1024 * 1024 * 1024},
		CPU:           hardware.CPUProfile{Cores: 16},
		PresetProfile: "consumer_6gb",
	}
	cat := models.DefaultCatalog()
	filtered := cat.FilterByVRAM(6 * 1024 * 1024 * 1024)
	ini, err := models.GeneratePresets(filtered, profile)
	require.NoError(t, err)
	assert.Contains(t, ini, "n-gpu-layers = -1")
	assert.Contains(t, ini, "ctx-size = 4096")
	assert.Contains(t, ini, "flash-attn = on")
}
```

- [ ] **Step 2: Run integration test**

Run: `cd HelixLLM && go test -v ./tests/integration/ -run "TestComplexity_To_Registry|TestPresetGeneration"`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
cd HelixLLM && git add tests/integration/multi_model_routing_test.go
git commit -m "test: add multi-model routing integration tests"
```

---

### Task 15: Challenge Script

**Files:**
- Create: `HelixLLM/challenges/scripts/multi_model_fleet_challenge.sh`

- [ ] **Step 1: Create challenge script**

```bash
#!/usr/bin/env bash
# HelixLLM Multi-Model Fleet Challenge
# Validates the lightweight model fleet implementation
set -euo pipefail

PASS=0
FAIL=0
TOTAL=0

pass() { ((PASS++)); ((TOTAL++)); echo "  PASS: $1"; }
fail() { ((FAIL++)); ((TOTAL++)); echo "  FAIL: $1"; }
check() { if eval "$2" >/dev/null 2>&1; then pass "$1"; else fail "$1"; fi; }

echo "=== HelixLLM Multi-Model Fleet Challenge ==="
echo ""

# 1. Hardware profiler exists and compiles
echo "--- Hardware Profiler ---"
check "profiler.go exists" "[ -f internal/shared/hardware/profiler.go ]"
check "profiler_test.go exists" "[ -f internal/shared/hardware/profiler_test.go ]"
check "profiler tests pass" "go test -short ./internal/shared/hardware/ -count=1"

# 2. Model catalog
echo "--- Model Catalog ---"
check "catalog.go exists" "[ -f internal/brain/models/catalog.go ]"
check "catalog has 4 default models" "go test -run TestDefaultCatalog_HasFourModels ./internal/brain/models/ -count=1"
check "catalog has fast tier" "go test -run TestDefaultCatalog_HasFastTier ./internal/brain/models/ -count=1"
check "catalog filters by VRAM" "go test -run TestCatalog_FilterByVRAM ./internal/brain/models/ -count=1"

# 3. Model registry
echo "--- Model Registry ---"
check "registry.go exists" "[ -f internal/brain/models/registry.go ]"
check "registry fallback works" "go test -run TestRegistry_BestAvailable_FallbackToLower ./internal/brain/models/ -count=1"
check "registry tracks status" "go test -run TestRegistry_UpdateStatus ./internal/brain/models/ -count=1"

# 4. Complexity analyzer
echo "--- Complexity Analyzer ---"
check "complexity.go exists" "[ -f internal/brain/complexity.go ]"
check "simple request routes to fast" "go test -run TestComplexityAnalyzer_SimpleToolCall ./internal/brain/ -count=1"
check "complex request routes to powerful" "go test -run TestComplexityAnalyzer_ComplexMultiTool ./internal/brain/ -count=1"

# 5. Preset generator
echo "--- Preset Generator ---"
check "preset.go exists" "[ -f internal/brain/models/preset.go ]"
check "generates valid INI" "go test -run TestGeneratePresets_Consumer6GB ./internal/brain/models/ -count=1"

# 6. Model downloader
echo "--- Model Downloader ---"
check "downloader.go exists" "[ -f internal/brain/downloader.go ]"
check "download and manifest work" "go test -run 'TestDownloader|TestManifest' ./internal/brain/ -count=1"

# 7. llama-server manager
echo "--- Server Manager ---"
check "server.go exists" "[ -f internal/brain/server.go ]"
check "health check works" "go test -run TestLlamaServer_HealthCheck ./internal/brain/ -count=1"
check "builds correct args" "go test -run TestLlamaServerConfig_BuildArgs ./internal/brain/ -count=1"

# 8. Local embedder
echo "--- Local Embedder ---"
check "llama_embedder.go exists" "[ -f internal/knowledge/llama_embedder.go ]"
check "embedding works" "go test -run TestLlamaEmbedder_Embed_Success ./internal/knowledge/ -count=1"

# 9. CUDA Containerfile
echo "--- Container ---"
check "router Containerfile exists" "[ -f container/Containerfile.llamacpp-router ]"
check "Containerfile has CUDA" "grep -q 'GGML_CUDA=ON' container/Containerfile.llamacpp-router"
check "Containerfile has router mode" "grep -q 'models-dir' container/Containerfile.llamacpp-router"

# 10. Config
echo "--- Configuration ---"
check "HELIX_MODELS_DIR in config" "grep -q 'HELIX_MODELS_DIR' internal/shared/config/config.go"
check "HELIX_COMPLEXITY_ENABLED in config" "grep -q 'HELIX_COMPLEXITY_ENABLED' internal/shared/config/config.go"

# 11. Integration
echo "--- Integration ---"
check "project compiles" "go build ./cmd/helixllm/"
check "all unit tests pass" "go test -short ./internal/... -count=1"

# 12. Full test suite
echo "--- Full Test Suite ---"
check "integration tests pass" "go test ./tests/integration/ -run 'TestComplexity_To_Registry|TestPresetGeneration' -count=1"

echo ""
echo "=== Results: $PASS/$TOTAL passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ] && echo "ALL CHECKS PASSED" || echo "SOME CHECKS FAILED"
exit "$FAIL"
```

- [ ] **Step 2: Make executable and commit**

```bash
cd HelixLLM && chmod +x challenges/scripts/multi_model_fleet_challenge.sh
git add challenges/scripts/multi_model_fleet_challenge.sh
git commit -m "test: add multi-model fleet challenge script (30 tests)"
```

---

### Task 16: Documentation Update

**Files:**
- Modify: `HelixLLM/CLAUDE.md`

- [ ] **Step 1: Update CLAUDE.md model section**

Replace references to Qwen3-Coder-30B-A3B as the primary model with the multi-model fleet documentation. Add a section describing:

- Default model fleet (Qwen2.5-Coder 1.5B, 3B, nomic-embed)
- llama.cpp router mode architecture
- Task complexity routing (simple→1.5B, moderate→3B, complex→8B)
- Hardware auto-profiling and preset profiles
- New env vars (`HELIX_MODELS_DIR`, `HELIX_MODELS_AUTO_DOWNLOAD`, `HELIX_COMPLEXITY_ENABLED`, etc.)
- Model download on first boot
- CUDA container build (`Containerfile.llamacpp-router`)

- [ ] **Step 2: Commit**

```bash
cd HelixLLM && git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for multi-model fleet architecture"
```

---

## Summary

| Phase | Tasks | Files Created | Files Modified |
|-------|-------|---------------|----------------|
| 1. Foundation | 1-4 | 8 | 0 |
| 2. Model Management | 5-7 | 6 | 0 |
| 3. Infrastructure | 8-9 | 3 | 1 |
| 4. Wiring | 10-13 | 0 | 5 |
| 5. Validation | 14-16 | 2 | 1 |
| **Total** | **16 tasks** | **19 files** | **7 files** |

**Dependency order:** Tasks 1-4 are independent (parallelizable). Tasks 5-7 depend on 1-2. Task 8 is independent. Task 9 is independent. Tasks 10-13 depend on all prior tasks. Tasks 14-16 depend on everything.
