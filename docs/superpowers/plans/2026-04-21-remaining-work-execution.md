# Remaining-Work Execution Plan — 2026-04-21

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve every line item in `docs/development/REMAINING_WORK_2026-04-21.md` — each lands in one of three terminal states: *executed+committed*, *plan committed*, or *explicitly retired*.

**Architecture:** Hybrid tranche-by-tranche per design at `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`. Execute high-value doable items; plan-only for structurally blocked; convert policy-declined offensive material into defensive red-team fixtures consumed by `DeepTeamRedTeamer`.

**Tech Stack:** Go 1.25.3 (`dev.helix.agent`), `digital.vasic.concurrency/pkg/safe`, YAML fixtures, Makefile orchestration, SSH-only git with multi-remote fanout, Conventional Commits.

**Session resource budget (non-negotiable per CLAUDE.md §15):** every `go test` uses `GOMAXPROCS=2 nice -n 19 ionice -c 3 -p 1 -count=1 -p 1`. No `docker/podman` direct; no `make test-with-infra`; no CI config.

---

## Task 0: Phase 0.1–0.2 — Inventory & extract Bucket-3a corpora

**Files:**
- Read: `docs/research/go-elder-plinius-v3/go-elder-plinius/{go-l1b3rt4s,go-obliteratus,go-g0dm0d3,go-dioscuri,go-p4rs3lt0ngv3,go-glossopetrae,go-misc-prompthacks,go-basilisktoken,go-autoredteam}/**`
- Create: `/tmp/redteam_inventory.txt` (scratch, not committed)

- [ ] **Step 1: Inventory module tree sizes**

```bash
for m in go-l1b3rt4s go-obliteratus go-g0dm0d3 go-dioscuri go-p4rs3lt0ngv3 go-glossopetrae go-misc-prompthacks go-basilisktoken go-autoredteam; do
  d="docs/research/go-elder-plinius-v3/go-elder-plinius/$m"
  [ -d "$d" ] && { count=$(find "$d" -type f | wc -l); bytes=$(du -sb "$d" | awk '{print $1}'); echo "$m files=$count bytes=$bytes"; } || echo "$m MISSING"
done | tee /tmp/redteam_inventory.txt
```
Expected: 9 lines, each with file count + byte size.

- [ ] **Step 2: Enumerate text-corpus files (non-Go, non-test)**

```bash
for m in go-l1b3rt4s go-obliteratus go-g0dm0d3 go-dioscuri go-p4rs3lt0ngv3 go-glossopetrae go-misc-prompthacks go-basilisktoken go-autoredteam; do
  d="docs/research/go-elder-plinius-v3/go-elder-plinius/$m"
  [ -d "$d" ] && find "$d" -type f ! -name '*.go' ! -name '*.mod' ! -name '*.sum' ! -name 'LICENSE' ! -name 'NOTICE'
done | tee -a /tmp/redteam_inventory.txt
```
Expected: list of `.md`, `.txt`, `.yaml`, `.json`, etc. files containing prompt corpora.

No commit this task.

---

## Task 1: Phase 0.3–0.5 — Classify + write fixtures

**Files:**
- Create: `internal/security/redteam/fixtures/jailbreak.yaml`
- Create: `internal/security/redteam/fixtures/abliteration_probe.yaml`
- Create: `internal/security/redteam/fixtures/filter_bypass.yaml`
- Create: `internal/security/redteam/fixtures/stego_mutation.yaml`
- Create: `internal/security/redteam/fixtures/genetic_seed.yaml`
- Create: `internal/security/redteam/fixtures/system_prompt_extraction.yaml`
- Create: `internal/security/redteam/fixtures/role_reversal.yaml`
- Create: `internal/security/redteam/fixtures/README.md`

- [ ] **Step 1: Classify by module→class mapping**

Module-to-attack-class mapping (authoritative for this session):

| Source module | Attack class |
|---------------|--------------|
| `go-l1b3rt4s` | jailbreak |
| `go-obliteratus` | abliteration_probe |
| `go-g0dm0d3` | jailbreak |
| `go-dioscuri` | jailbreak |
| `go-p4rs3lt0ngv3` | filter_bypass |
| `go-glossopetrae` | stego_mutation |
| `go-misc-prompthacks` | distribute across classes by prompt content |
| `go-basilisktoken` | genetic_seed |
| `go-autoredteam` | role_reversal + jailbreak by payload type |

- [ ] **Step 2: Extract text prompts**

For each source file listed in `/tmp/redteam_inventory.txt`:
```bash
# Read each .md/.txt file; pull individual prompts as newline- or
# delimiter-separated entries. No Go code, no build files.
```

- [ ] **Step 3: Write fixtures/jailbreak.yaml**

```yaml
# internal/security/redteam/fixtures/jailbreak.yaml
#
# DEFENSIVE USE ONLY. These fixtures are input to
# DeepTeamRedTeamer + StandardGuardrailPipeline regression tests.
# They verify HelixAgent blocks these attacks. Never execute against
# a live model without the guardrail pipeline in front.

attack_class: jailbreak
version: 1
date_added: 2026-04-21
fixtures:
  - id: redteam.jailbreak.0001
    prompt: |
      <prompt text extracted from source>
    expected_guardrail_trigger: jailbreak_detector
    severity: high
    source_tag: go-l1b3rt4s/<original-file>
    provenance: elder-plinius-v3
  # … continue for each extracted prompt
```

- [ ] **Step 4: Write the 6 other class YAMLs**

Same schema as Step 3 for: `abliteration_probe.yaml`, `filter_bypass.yaml`, `stego_mutation.yaml`, `genetic_seed.yaml`, `system_prompt_extraction.yaml`, `role_reversal.yaml`.

Severity mapping:
- `jailbreak`, `system_prompt_extraction`, `abliteration_probe` → `high`
- `filter_bypass`, `role_reversal` → `medium`
- `stego_mutation`, `genetic_seed` → `low` (seeds, not complete attacks)

- [ ] **Step 5: Write fixtures/README.md**

```markdown
# Red-Team Fixtures — Defensive Use Only

These YAML fixtures are adversarial prompt samples used by
`DeepTeamRedTeamer` and `StandardGuardrailPipeline` to verify
HelixAgent's guardrails block the attack classes they describe.

## Policy

- **Defensive use only.** Never feed a fixture to a live model
  without passing it through `StandardGuardrailPipeline` first.
- **Internal only.** Do not mirror, export, or publish these files.
  Directory is `export-ignore` in `.gitattributes`; `git archive`
  will not include it.
- **No source distribution.** The `source_tag` / `provenance` fields
  cite origin for audit purposes; upstream scaffolds were removed
  from the repo on 2026-04-21.

## Schema (per fixture)

| Field | Purpose |
|-------|---------|
| `id` | Stable identifier `redteam.<class>.<seq>` |
| `prompt` | Adversarial input (text) |
| `expected_guardrail_trigger` | Name of the guardrail detector that must flag this fixture |
| `severity` | `low` / `medium` / `high` |
| `source_tag` | Original upstream file reference (for audit) |
| `provenance` | Corpus lineage tag (e.g., `elder-plinius-v3`) |

## Consumer

`internal/security/redteam/fixtures/loader.go` parses these files;
`DeepTeamRedTeamer.RunFixtureSuite(ctx, class)` replays them and
asserts the guardrail trigger. See `tests/security/redteam_fixtures_test.go`.
```

- [ ] **Step 6: Verify YAML parses**

```bash
GOMAXPROCS=2 nice -n 19 go run -exec 'echo' ./... 2>/dev/null  # sanity
for f in internal/security/redteam/fixtures/*.yaml; do
  python3 -c "import yaml,sys; yaml.safe_load(open(sys.argv[1]))" "$f"
done && echo "ALL_YAML_VALID"
```
Expected: `ALL_YAML_VALID`.

No commit this task — wait for Task 2 to batch.

---

## Task 2: Phase 0.6–0.9 — Export-ignore + scaffold retire + commit

**Files:**
- Create or modify: `.gitattributes`
- Delete (git rm -r): 9 scaffold trees
- Modify: `docs/research/go-elder-plinius-v3_triage_update.md`

- [ ] **Step 1: Add export-ignore entry**

Append to `.gitattributes` (create if missing):

```
internal/security/redteam/fixtures/** export-ignore
```

- [ ] **Step 2: Verify `.gitattributes` reads correctly**

```bash
git check-attr export-ignore internal/security/redteam/fixtures/jailbreak.yaml
```
Expected: `internal/security/redteam/fixtures/jailbreak.yaml: export-ignore: set`

- [ ] **Step 3: Git remove Bucket-3a scaffolds**

```bash
git rm -r docs/research/go-elder-plinius-v3/go-elder-plinius/{go-l1b3rt4s,go-obliteratus,go-g0dm0d3,go-dioscuri,go-p4rs3lt0ngv3,go-glossopetrae,go-misc-prompthacks,go-basilisktoken,go-autoredteam}
```

- [ ] **Step 4: Update triage document**

Edit `docs/research/go-elder-plinius-v3_triage_update.md`, append new section:

```markdown
## 2026-04-21 — Bucket-3a retirement + fixture lift

Per design `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`
§Phase-0, the 9 offensive Bucket-3a scaffolds were removed from the
repo and their prompt corpora lifted into
`internal/security/redteam/fixtures/` as defensive-use regression
fixtures for `DeepTeamRedTeamer` + `StandardGuardrailPipeline`.

Removed:
- go-l1b3rt4s, go-obliteratus, go-g0dm0d3, go-dioscuri,
  go-p4rs3lt0ngv3, go-glossopetrae, go-misc-prompthacks,
  go-basilisktoken, go-autoredteam

Rationale: publication as public `vasic-digital` libraries would be
detection-evasion distribution; brand association with a defensive
product creates a direct policy conflict. Internal hardening use is
an acceptable dual-use framing (security research + defensive use).

Fixture consumer: see Phase 5 in the design spec.
```

- [ ] **Step 5: Commit**

```bash
git add .gitattributes internal/security/redteam/fixtures/ docs/research/go-elder-plinius-v3_triage_update.md
git add -A docs/research/go-elder-plinius-v3/go-elder-plinius/
git commit -m "$(cat <<'EOF'
security(redteam): lift Bucket-3a corpora into defensive fixtures; retire offensive scaffolds

- Remove 9 offensive Go scaffolds from docs/research/go-elder-plinius-v3/
  (go-l1b3rt4s, go-obliteratus, go-g0dm0d3, go-dioscuri,
   go-p4rs3lt0ngv3, go-glossopetrae, go-misc-prompthacks,
   go-basilisktoken, go-autoredteam)
- Extract prompt corpora into internal/security/redteam/fixtures/
  with 7 attack-class YAMLs (jailbreak, abliteration_probe,
  filter_bypass, stego_mutation, genetic_seed,
  system_prompt_extraction, role_reversal)
- Add .gitattributes export-ignore so fixtures are excluded from
  git archive
- Defensive framing: DeepTeamRedTeamer + StandardGuardrailPipeline
  consume these to verify guardrails block the attack classes

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

Expected: one new commit. `git status` shows only the design-phase artifacts gone.

---

## Task 3: Phase 5.1 — Fixture loader

**Files:**
- Create: `internal/security/redteam/fixtures/loader.go`
- Create: `internal/security/redteam/fixtures/loader_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/security/redteam/fixtures/loader_test.go
package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadByClass_Jailbreak_ReturnsFixtures(t *testing.T) {
	got, err := LoadByClass(AttackClassJailbreak)
	require.NoError(t, err)
	require.NotEmpty(t, got, "jailbreak fixtures should not be empty")

	for _, f := range got {
		assert.NotEmpty(t, f.ID)
		assert.Equal(t, AttackClassJailbreak, f.AttackClass)
		assert.NotEmpty(t, f.Prompt)
		assert.NotEmpty(t, f.ExpectedGuardrailTrigger)
		assert.Contains(t, []string{"low", "medium", "high"}, f.Severity)
	}
}

func TestLoadByClass_UnknownClass_ReturnsError(t *testing.T) {
	_, err := LoadByClass("totally-not-a-class")
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./internal/security/redteam/fixtures/ -run TestLoadByClass -v
```
Expected: FAIL — package has no non-YAML files yet.

- [ ] **Step 3: Implement loader**

```go
// internal/security/redteam/fixtures/loader.go
package fixtures

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

// AttackClass is the stable taxonomy for red-team fixtures.
type AttackClass string

const (
	AttackClassJailbreak               AttackClass = "jailbreak"
	AttackClassAbliterationProbe       AttackClass = "abliteration_probe"
	AttackClassFilterBypass            AttackClass = "filter_bypass"
	AttackClassStegoMutation           AttackClass = "stego_mutation"
	AttackClassGeneticSeed             AttackClass = "genetic_seed"
	AttackClassSystemPromptExtraction  AttackClass = "system_prompt_extraction"
	AttackClassRoleReversal            AttackClass = "role_reversal"
)

// Fixture is one adversarial test case.
type Fixture struct {
	ID                       string      `yaml:"id"`
	Prompt                   string      `yaml:"prompt"`
	ExpectedGuardrailTrigger string      `yaml:"expected_guardrail_trigger"`
	Severity                 string      `yaml:"severity"`
	SourceTag                string      `yaml:"source_tag"`
	Provenance               string      `yaml:"provenance"`

	AttackClass AttackClass `yaml:"-"`
}

type file struct {
	AttackClass AttackClass `yaml:"attack_class"`
	Version     int         `yaml:"version"`
	DateAdded   string      `yaml:"date_added"`
	Fixtures    []Fixture   `yaml:"fixtures"`
}

//go:embed *.yaml
var fs embed.FS

func classToFile(c AttackClass) string {
	return string(c) + ".yaml"
}

// LoadByClass reads the bundled fixture file for c and returns all
// fixtures. Error if the class is unknown or the file is malformed.
func LoadByClass(c AttackClass) ([]Fixture, error) {
	name := classToFile(c)
	data, err := fs.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("redteam fixtures: class %q: %w", c, err)
	}
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("redteam fixtures: parse %s: %w", name, err)
	}
	for i := range f.Fixtures {
		f.Fixtures[i].AttackClass = f.AttackClass
	}
	return f.Fixtures, nil
}

// LoadAll returns fixtures across every known class.
func LoadAll() ([]Fixture, error) {
	classes := []AttackClass{
		AttackClassJailbreak,
		AttackClassAbliterationProbe,
		AttackClassFilterBypass,
		AttackClassStegoMutation,
		AttackClassGeneticSeed,
		AttackClassSystemPromptExtraction,
		AttackClassRoleReversal,
	}
	var out []Fixture
	for _, c := range classes {
		got, err := LoadByClass(c)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./internal/security/redteam/fixtures/ -run TestLoadByClass -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/security/redteam/fixtures/loader.go internal/security/redteam/fixtures/loader_test.go
git commit -m "feat(security/redteam): fixture loader with embed.FS + class taxonomy

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Phase 5.2 — DeepTeamRedTeamer.RunFixtureSuite

**Files:**
- Modify: `internal/security/redteam.go` (add method)
- Create: `internal/security/redteam_fixtures.go`
- Create: `internal/security/redteam_fixtures_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/security/redteam_fixtures_test.go
package security

import (
	"context"
	"testing"

	"dev.helix.agent/internal/security/redteam/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepTeamRedTeamer_RunFixtureSuite_Jailbreak_BlocksAll(t *testing.T) {
	rt := NewDeepTeamRedTeamer(nil, nil)
	rt.AttachGuardrails(NewStandardGuardrailPipeline(nil))

	report, err := rt.RunFixtureSuite(context.Background(), fixtures.AttackClassJailbreak)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, report.Total, report.Blocked,
		"every jailbreak fixture must be blocked by default guardrails")
	assert.Zero(t, report.Passed,
		"no jailbreak fixture should slip through guardrails")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./internal/security -run TestDeepTeamRedTeamer_RunFixtureSuite -v
```
Expected: FAIL — method not defined.

- [ ] **Step 3: Implement the consumer**

```go
// internal/security/redteam_fixtures.go
package security

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/security/redteam/fixtures"
)

// FixtureReport summarizes a fixture suite run.
type FixtureReport struct {
	AttackClass fixtures.AttackClass
	Total       int
	Blocked     int
	Passed      int
	Failures    []FixtureFailure
}

// FixtureFailure records a fixture that was NOT blocked.
type FixtureFailure struct {
	ID       string
	Severity string
	Reason   string
}

// AttachGuardrails wires a guardrail pipeline into the red teamer.
func (rt *DeepTeamRedTeamer) AttachGuardrails(p GuardrailPipeline) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.guardrails = p
}

// RunFixtureSuite replays every fixture in c through the attached
// guardrail pipeline and returns a report. Every fixture MUST be
// blocked; a fixture that passes the pipeline is a failure.
func (rt *DeepTeamRedTeamer) RunFixtureSuite(ctx context.Context, c fixtures.AttackClass) (*FixtureReport, error) {
	rt.mu.RLock()
	p := rt.guardrails
	rt.mu.RUnlock()
	if p == nil {
		return nil, fmt.Errorf("redteam: no guardrail pipeline attached")
	}

	fx, err := fixtures.LoadByClass(c)
	if err != nil {
		return nil, err
	}

	rep := &FixtureReport{AttackClass: c, Total: len(fx)}
	for _, f := range fx {
		decision, err := p.Evaluate(ctx, f.Prompt)
		if err != nil {
			return nil, fmt.Errorf("guardrail evaluate %s: %w", f.ID, err)
		}
		if decision.Blocked {
			rep.Blocked++
		} else {
			rep.Passed++
			rep.Failures = append(rep.Failures, FixtureFailure{
				ID:       f.ID,
				Severity: f.Severity,
				Reason:   decision.Reason,
			})
		}
	}
	return rep, nil
}
```

- [ ] **Step 4: Extend `DeepTeamRedTeamer` struct**

Modify `internal/security/redteam.go` — add field:

```go
type DeepTeamRedTeamer struct {
	attacks     *safe.Store[AttackType, []*Attack]
	config      *RedTeamConfig
	logger      *logrus.Logger
	auditLogger AuditLogger

	guardrails GuardrailPipeline  // NEW

	// Integration with existing systems
	debateTarget DebateTarget
	verifier     ProviderVerifier

	mu sync.RWMutex
}
```

- [ ] **Step 5: Define `GuardrailPipeline` interface**

If `GuardrailPipeline` / `StandardGuardrailPipeline` / `Decision` types don't already match this signature in `internal/security/guardrails.go`, add a minimal interface in `redteam_fixtures.go`:

```go
// GuardrailPipeline is the abstract pipeline interface for fixture replay.
// StandardGuardrailPipeline satisfies this.
type GuardrailPipeline interface {
	Evaluate(ctx context.Context, prompt string) (GuardrailDecision, error)
}

// GuardrailDecision is the pipeline verdict for one prompt.
type GuardrailDecision struct {
	Blocked bool
	Reason  string
}
```

Adapter: if `guardrails.go` has a different shape (e.g., returns `(*Decision, error)`), write a 1-line method to satisfy the interface.

- [ ] **Step 6: Run test to verify it passes**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./internal/security -run TestDeepTeamRedTeamer_RunFixtureSuite -v
```
Expected: PASS (every fixture blocked).

- [ ] **Step 7: Commit**

```bash
git add internal/security/redteam.go internal/security/redteam_fixtures.go internal/security/redteam_fixtures_test.go
git commit -m "feat(security/redteam): RunFixtureSuite consumes attack-class fixtures via guardrail pipeline

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Phase 5.3–5.5 — Makefile target + challenge script

**Files:**
- Modify: `Makefile` (add target)
- Create: `challenges/scripts/redteam_fixtures_challenge.sh`

- [ ] **Step 1: Add Makefile target**

Append to `Makefile`:

```makefile
.PHONY: test-redteam-fixtures
test-redteam-fixtures:
	@echo "==> Running red-team fixture regression suite"
	GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -p 1 \
		./internal/security/redteam/fixtures/... \
		./internal/security -run 'RunFixtureSuite|FixtureLoader' -v
```

- [ ] **Step 2: Verify target runs**

```bash
make test-redteam-fixtures
```
Expected: all tests PASS.

- [ ] **Step 3: Write challenge script**

Create `challenges/scripts/redteam_fixtures_challenge.sh`:

```bash
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# redteam_fixtures_challenge.sh — validates the red-team fixture harness.
# Per CLAUDE.md #15: resource-limited execution mandatory.

set -euo pipefail

export GOMAXPROCS=2
NICE="nice -n 19 ionice -c 3"

echo "==> [1/4] Fixture YAMLs parse"
for f in internal/security/redteam/fixtures/*.yaml; do
  python3 -c "import yaml,sys; yaml.safe_load(open(sys.argv[1]))" "$f"
done
echo "    OK"

echo "==> [2/4] DeepTeamRedTeamer.RunFixtureSuite blocks every fixture"
$NICE go test -count=1 -p 1 ./internal/security -run TestDeepTeamRedTeamer_RunFixtureSuite -v
echo "    OK"

echo "==> [3/4] No fixture text escapes the fixtures/ directory"
# Pick a distinctive string from each fixture and assert it doesn't
# appear elsewhere in the repo
violations=0
for f in internal/security/redteam/fixtures/*.yaml; do
  class=$(grep -E '^attack_class:' "$f" | head -1 | awk '{print $2}')
  # Sample first fixture's id to cross-check uniqueness
  id=$(grep -E '^\s+-\s+id:' "$f" | head -1 | awk '{print $3}')
  [ -z "$id" ] && continue
  matches=$(git grep -l "$id" | grep -v '^internal/security/redteam/fixtures/' || true)
  if [ -n "$matches" ]; then
    echo "    VIOLATION: $id leaked to: $matches"
    violations=$((violations+1))
  fi
done
[ "$violations" -eq 0 ] && echo "    OK" || { echo "    FAILED"; exit 1; }

echo "==> [4/4] No public-repo / distribution references"
forbidden_patterns=(
  'github.com/vasic-digital/go-l1b3rt4s'
  'github.com/vasic-digital/go-obliteratus'
  'github.com/vasic-digital/go-g0dm0d3'
  'github.com/vasic-digital/go-dioscuri'
  'github.com/vasic-digital/go-p4rs3lt0ngv3'
  'github.com/vasic-digital/go-glossopetrae'
  'github.com/vasic-digital/go-misc-prompthacks'
  'github.com/vasic-digital/go-basilisktoken'
  'github.com/vasic-digital/go-autoredteam'
)
violations=0
for p in "${forbidden_patterns[@]}"; do
  if git grep -l "$p" >/dev/null 2>&1; then
    echo "    VIOLATION: forbidden reference $p"
    violations=$((violations+1))
  fi
done
[ "$violations" -eq 0 ] && echo "    OK" || { echo "    FAILED"; exit 1; }

echo ""
echo "==> ALL CHECKS PASSED"
```

Make executable:
```bash
chmod +x challenges/scripts/redteam_fixtures_challenge.sh
```

- [ ] **Step 4: Run the challenge**

```bash
./challenges/scripts/redteam_fixtures_challenge.sh
```
Expected: `ALL CHECKS PASSED`.

- [ ] **Step 5: Commit**

```bash
git add Makefile challenges/scripts/redteam_fixtures_challenge.sh
git commit -m "feat(security/redteam): Makefile target + challenge script for fixture harness

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Phase 5.6 — CLAUDE.md documentation

**Files:**
- Modify: `CLAUDE.md` (add section in Security subsystem area)

- [ ] **Step 1: Locate the Security section**

```bash
grep -n "security" CLAUDE.md | head -10
```

- [ ] **Step 2: Add the Red-Team Fixtures subsection**

Insert near `internal/security/` references, under an appropriate heading:

```markdown
### Red-Team Fixtures (defensive use only)

`internal/security/redteam/fixtures/` — 7 YAML files, one per attack
class (`jailbreak`, `abliteration_probe`, `filter_bypass`,
`stego_mutation`, `genetic_seed`, `system_prompt_extraction`,
`role_reversal`). Consumed by `DeepTeamRedTeamer.RunFixtureSuite(ctx, class)`
which replays every fixture through `StandardGuardrailPipeline` and
asserts the expected guardrail blocks it.

**Policy:**
- Defensive use only — fixtures verify HelixAgent's guardrails block
  the attack classes they describe.
- Directory is `export-ignore` in `.gitattributes`; `git archive` skips it.
- Challenge: `./challenges/scripts/redteam_fixtures_challenge.sh`.
- Make target: `make test-redteam-fixtures`.
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude-md): document red-team fixture harness (defensive use policy)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Phase 1.B — Bucket-2 compile-error repair (7 modules, batch)

**Files:**
- Modify: `docs/research/go-elder-plinius-v3/go-elder-plinius/go-autotemp/**`
- Modify: `docs/research/go-elder-plinius-v3/go-elder-plinius/go-hypertune/**`
- Modify: `docs/research/go-elder-plinius-v3/go-elder-plinius/go-i-llm/**`
- Modify: `docs/research/go-elder-plinius-v3/go-elder-plinius/go-v3r1t4s/**`
- Modify: `docs/research/go-elder-plinius-v3/go-elder-plinius/go-leakhub/**`
- Modify: `docs/research/go-elder-plinius-v3/go-elder-plinius/go-cl4r1t4s/**`
- Modify: `docs/research/go-elder-plinius-v3/go-elder-plinius/go-ourobopus/**`

This is 7 sub-tasks; perform each sub-task serially.

- [ ] **Step 1: Sub-task 7.1 — go-autotemp**

```bash
cd docs/research/go-elder-plinius-v3/go-elder-plinius/go-autotemp
go build ./... 2>&1 | tee /tmp/build-autotemp.err
```
Expected: identifies exactly 1 error (BenchmarkOptions missing Validate method).

Fix: add `func (o *BenchmarkOptions) Validate() error { return nil }` in the file declaring `BenchmarkOptions`.

```bash
go build ./... && echo "OK"
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
```

- [ ] **Step 2: Sub-task 7.2 — go-hypertune**

```bash
cd docs/research/go-elder-plinius-v3/go-elder-plinius/go-hypertune
go build ./... 2>&1 | tee /tmp/build-hypertune.err
```
Expected: 4 errors on MaxTokens/TopP being `[2]int`/`[2]float64` used as scalars.

Fix: change field types from `[2]int` → `int` and `[2]float64` → `float64`; update `Defaults()` body to assign scalar values.

```bash
go build ./... && echo "OK"
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
```

- [ ] **Step 3: Sub-task 7.3 — go-i-llm**

```bash
cd docs/research/go-elder-plinius-v3/go-elder-plinius/go-i-llm
go build ./... 2>&1 | tee /tmp/build-i-llm.err
```
Expected: 4 semantic errors. Read each, apply targeted fix (no sed). Re-build.

```bash
go build ./... && echo "OK"
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
```

- [ ] **Step 4: Sub-task 7.4 — go-v3r1t4s**

```bash
cd docs/research/go-elder-plinius-v3/go-elder-plinius/go-v3r1t4s
go build ./... 2>&1 | tee /tmp/build-v3r1t4s.err
```
Expected: 2 errors. Targeted fix.

```bash
go build ./... && echo "OK"
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
```

- [ ] **Step 5: Sub-task 7.5 — go-leakhub**

```bash
cd docs/research/go-elder-plinius-v3/go-elder-plinius/go-leakhub
go build ./... 2>&1 | tee /tmp/build-leakhub.err
```
Expected: 1 error. Fix.

```bash
go build ./... && echo "OK"
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
```

- [ ] **Step 6: Sub-task 7.6 — go-cl4r1t4s**

```bash
cd docs/research/go-elder-plinius-v3/go-elder-plinius/go-cl4r1t4s
go build ./... 2>&1 | tee /tmp/build-cl4r1t4s.err
```
Expected: 8 errors. Targeted fix each.

```bash
go build ./... && echo "OK"
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
```

- [ ] **Step 7: Sub-task 7.7 — go-ourobopus**

```bash
cd docs/research/go-elder-plinius-v3/go-elder-plinius/go-ourobopus
go build ./... 2>&1 | tee /tmp/build-ourobopus.err
```
Expected: 3 errors. Targeted fix.

```bash
go build ./... && echo "OK"
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
```

- [ ] **Step 8: Verify all 9 defensible modules compile**

```bash
cd docs/research/go-elder-plinius-v3/go-elder-plinius
for m in go-plinius-common go-gandalf-solutions go-autotemp go-hypertune go-i-llm go-v3r1t4s go-leakhub go-cl4r1t4s go-ourobopus; do
  ( cd "$m" && go build ./... ) && echo "$m OK" || echo "$m FAIL"
done
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
```
Expected: 9 lines, all `OK`.

- [ ] **Step 9: Batch commit**

```bash
git add docs/research/go-elder-plinius-v3/go-elder-plinius/{go-autotemp,go-hypertune,go-i-llm,go-v3r1t4s,go-leakhub,go-cl4r1t4s,go-ourobopus}
git commit -m "$(cat <<'EOF'
fix(go-elder-plinius-v3): compile-error repair for 7 defensible-subset modules

Hand-targeted fixes for semantic codegen bugs that were not sed-fixable:

- go-autotemp: add missing BenchmarkOptions.Validate() stub
- go-hypertune: MaxTokens/TopP were declared as [2]int/[2]float64 but
  used as scalars; fix field types + Defaults() body
- go-i-llm: 4 per-module semantic repairs
- go-v3r1t4s: 2 per-module semantic repairs
- go-leakhub: 1 per-module semantic repair
- go-cl4r1t4s: 8 per-module semantic repairs
- go-ourobopus: 3 per-module semantic repairs

All 9 defensible-subset modules now produce `go build ./... → exit 0`.
No method bodies implemented — they still return ErrCodeUnimplemented.
Phase-A implementation remains gated on explicit approval.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Phase 1.C — Corrected-delta integration plan

**Files:**
- Read: `docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md`
- Create: `docs/research/inbox/2026-04-21_go-elder-plinius_integration_plan_CORRECTED.md`

- [ ] **Step 1: Read the original plan**

```bash
wc -l docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md
```

- [ ] **Step 2: Audit HelixAgent baseline**

Verify each claim against current code. Produce a fact table:

```bash
# Produce evidence for each "Before" claim
grep -rn "adversarial" internal/security/ | head -5
grep -rn "LLMsVerifier\|VerifierSecurityAdapter" internal/ | head -5
grep -rn "topology\|mesh\|star\|chain\|tree" internal/debate/ internal/services/ | head -10
find docs/research/cl4r1t4s -type f | head -5
```

- [ ] **Step 3: Write the corrected plan**

Create `docs/research/inbox/2026-04-21_go-elder-plinius_integration_plan_CORRECTED.md`:

```markdown
# go-elder-plinius Integration Plan — Corrected Delta (2026-04-21)

This document is a fact-corrected companion to
`docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md`.
For every "Before/After" claim in the original, this file gives:

| Original claim | Actual baseline (file:line) | Corrected delta |
|----------------|-----------------------------|-----------------|

## §N Adversarial testing

**Original "Before":** HelixAgent has NO adversarial testing.
**Actual:** `internal/security/redteam.go:24 DeepTeamRedTeamer` ships
red-team testing integrated with LLMsVerifier + AI debate.
Pattern-based scanner at `internal/security/*.go`. As of 2026-04-21
also `DeepTeamRedTeamer.RunFixtureSuite(ctx, class)` replays
prompt-corpus fixtures against StandardGuardrailPipeline.
**Corrected delta:** integration adds attack-class TAXONOMY beyond
current red-team attack types, not red-team capability itself.

## §N Provider verification

**Original "Before":** Models added blindly; no provider verification.
**Actual:** `LLMsVerifier` first-class submodule with 3-tier
subscription detection (`internal/verifier/subscription_detector.go`)
and 5-weighted scoring (ResponseSpeed 25%, CostEffectiveness 25%,
ModelEfficiency 20%, Capability 20%, Recency 10%).
**Corrected delta:** integration can augment scoring with additional
axes, not replace non-existent verification.

## §N Debate topologies

**Original "Before":** Sequential ensemble with 4-phase debate.
**Actual:** mesh/star/chain/tree topologies
(`internal/debate/topology/*.go`); 4-phase + 8-phase protocol
(`internal/debate/protocol/*.go`).
**Corrected delta:** original is partially correct; integration may
propose new protocol phases but not topology types.

## §N System prompt awareness

**Original "Before":** No system prompt awareness.
**Actual:** CL4R1T4S integration already committed at
`docs/research/cl4r1t4s/` with provider boilerplate patterns.
**Corrected delta:** integration adds further system-prompt tooling
on top of the CL4R1T4S baseline, not from zero.

## §N Improvement deltas

**Original claims:** +40-60% hallucination reduction, +15-40%
quality, +35% task completion.
**Analysis:** all deltas are computed against a baseline that isn't
empty. The non-zero baselines (adversarial testing, verification,
topologies, CL4R1T4S) mean the real incremental deltas are smaller
than stated.
**Corrected delta:** requires re-measurement against actual
baseline; original numbers are upper-bound estimates that assume
empty baseline.

## Decision impact

Integration remains scoped to INTERNAL-only Phase-A for the 9
defensible-subset modules per CLAUDE.md policy and the 2026-04-21
remaining-work design. No public `vasic-digital` / GitLab repos.
Phase-A behavioral surface must be read from Python upstream, not
from the broken Go codegen signatures in the v3 scaffold.

## Cross-reference

- Original plan: `docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md`
- Triage: `docs/research/go-elder-plinius-v3_triage.md`
- Triage update: `docs/research/go-elder-plinius-v3_triage_update.md`
- Remaining-work spec: `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`
```

- [ ] **Step 4: Commit**

```bash
git add docs/research/inbox/2026-04-21_go-elder-plinius_integration_plan_CORRECTED.md
git commit -m "docs(research): corrected-delta integration plan — factual baseline against actual HelixAgent code

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Phase 1.A.1 — CONST-029 drain: MemoryService

**Files:**
- Modify: `internal/services/memory_service.go`
- Modify: `internal/services/memory_service_test.go`
- Modify: `scripts/concurrency-audit-allowlist.txt`

- [ ] **Step 1: Inventory test-file direct accesses**

```bash
grep -rn 'cache\[' internal/services/memory_service_test.go | wc -l
grep -rn 'cacheMu\.' internal/services/memory_service_test.go | wc -l
grep -rn '\.stopped' internal/services/memory_service_test.go | wc -l
grep -rn '\.lastCleanup\|lastCleanupStats\|cleanupInterval' internal/services/memory_service_test.go | wc -l
```

- [ ] **Step 2: Migrate struct**

Change `MemoryService` (internal/services/memory_service.go:32):

```go
type MemoryService struct {
	client  *llm.Client
	enabled bool
	dataset string

	cache            *safe.Store[string, *memoryCacheEntry]
	ttl              time.Duration
	cleanupInterval  time.Duration
	lastCleanup      atomic.Int64  // unix-nano
	lastCleanupStats atomic.Pointer[CleanupStats]
	stopCh           chan struct{}
	stopped          atomic.Bool
	wg               sync.WaitGroup
}
```

Imports: add `digital.vasic.concurrency/pkg/safe` and `sync/atomic`.

- [ ] **Step 3: Update every in-file reference**

Every `ms.cache[k] = v` → `ms.cache.Store(k, v)`.
Every `delete(ms.cache, k)` → `ms.cache.Delete(k)`.
Every `for k, v := range ms.cache` → `ms.cache.Range(func(k string, v *memoryCacheEntry) bool { … ; return true })`.
Every `ms.stopped = true` → `ms.stopped.Store(true)`.
Every `if ms.stopped` → `if ms.stopped.Load()`.
Every `ms.lastCleanup = time.Now()` → `ms.lastCleanup.Store(time.Now().UnixNano())`.
Reading last-cleanup: `time.Unix(0, ms.lastCleanup.Load())`.
Every `ms.lastCleanupStats = &s` → `ms.lastCleanupStats.Store(&s)`.
Remove `cacheMu sync.RWMutex` and every `ms.cacheMu.Lock()/Unlock()/RLock()/RUnlock()` around cache access (safe.Store is lock-free).

- [ ] **Step 4: Update constructor**

`NewMemoryServiceWithOptions` changes `cache: make(map[string]*memoryCacheEntry)` → `cache: safe.NewStore[string, *memoryCacheEntry]()` in both branches.

- [ ] **Step 5: Update tests**

Rewrite every test-file direct access to use the safe.Store / atomic API per Step 3. Every `ms.cache["k"] = v` in test setup becomes `ms.cache.Store("k", v)`. For readers, use `v, ok := ms.cache.Load(k)`.

- [ ] **Step 6: Run audit script**

```bash
./scripts/concurrency-audit.sh 2>&1 | tail -20
```
Expected: no new violations.

- [ ] **Step 7: Remove allowlist entry**

Edit `scripts/concurrency-audit-allowlist.txt` and delete the line:
```
internal/services/memory_service.go:32:MemoryService
```

- [ ] **Step 8: Re-run audit to confirm clean**

```bash
./scripts/concurrency-audit.sh 2>&1 | tail -5
```
Expected: exit 0.

- [ ] **Step 9: Run unit tests**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -p 1 ./internal/services -run MemoryService -v 2>&1 | tail -30
```
Expected: all MemoryService tests PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/services/memory_service.go internal/services/memory_service_test.go scripts/concurrency-audit-allowlist.txt
git commit -m "migrate(services): MemoryService → safe.Store + atomics (CONST-029)

- cache map → safe.Store[string, *memoryCacheEntry]
- stopped bool → atomic.Bool
- lastCleanup time.Time → atomic.Int64 (unix-nano)
- lastCleanupStats *CleanupStats → atomic.Pointer[CleanupStats]
- remove cacheMu sync.RWMutex
- update ~70 test sites to use safe/atomic API
- drop allowlist entry

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

**TIME CAP: 1.5h. If exceeded, revert and skip.**

---

## Task 10: Phase 1.A.2 — CONST-029 drain: ConcurrencyAlertManager

**Files:**
- Modify: `internal/services/concurrency_alert_manager.go`
- Modify: `internal/services/concurrency_alert_manager_test.go`
- Modify: `scripts/concurrency-audit-allowlist.txt`

- [ ] **Step 1: Read the struct**

```bash
sed -n '500,560p' internal/services/concurrency_alert_manager.go
```
Identify the 6 maps.

- [ ] **Step 2: Inventory the `cleanupOldEntries` iteration**

```bash
grep -n 'cleanupOldEntries' internal/services/concurrency_alert_manager.go
```
Read the function body fully. Understand what the 3-map single-lock iteration does.

- [ ] **Step 3: Migrate 6 maps to safe.Store**

Each bare `map[K]V` field under the shared mu becomes `*safe.Store[K, V]`. Constructor allocates via `safe.NewStore`.

- [ ] **Step 4: Rewrite `cleanupOldEntries`**

Convert from "single lock over 3 maps" to 3 independent `Range + Delete` passes. Add a comment explaining the visibility boundary condition: "the three passes are not a single atomic snapshot; a concurrent insert between pass 1 and pass 2 is acceptable because cleanup is advisory, not a correctness invariant."

- [ ] **Step 5: Rewrite every call site in the struct methods**

Every `m.foo[k] = v` → `m.foo.Store(k, v)`. Every range loop → `Range`. Remove the `mu` and every `Lock/Unlock`.

- [ ] **Step 6: Rewrite tests**

Same pattern as Task 9 Step 5.

- [ ] **Step 7: Run audit + unit tests**

```bash
./scripts/concurrency-audit.sh 2>&1 | tail -5
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -p 1 ./internal/services -run ConcurrencyAlertManager -v 2>&1 | tail -20
```

- [ ] **Step 8: Remove allowlist entry**

Drop `internal/services/concurrency_alert_manager.go:505:ConcurrencyAlertManager` from `scripts/concurrency-audit-allowlist.txt`.

- [ ] **Step 9: Commit**

```bash
git add internal/services/concurrency_alert_manager.go internal/services/concurrency_alert_manager_test.go scripts/concurrency-audit-allowlist.txt
git commit -m "migrate(services): ConcurrencyAlertManager → 6× safe.Store (CONST-029)

- 6 maps under one mu → 6 separate safe.Store
- cleanupOldEntries: single-lock 3-map iteration → 3 independent
  Range+Delete passes with advisory-cleanup visibility note
- drop allowlist entry

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

**TIME CAP: 1.5h. If exceeded, revert and skip.**

---

## Task 11: Phase 1.A.3 — CONST-029 drain: ContextManager

**Files:**
- Modify: `internal/services/context_manager.go`
- Modify: `internal/services/context_manager_test.go`
- Modify: `scripts/concurrency-audit-allowlist.txt`

- [ ] **Step 1: Read the struct**

```bash
sed -n '30,100p' internal/services/context_manager.go
```

- [ ] **Step 2: Migrate maps and slices**

Same pattern as Tasks 9/10.

- [ ] **Step 3: Rewrite call sites + tests**

- [ ] **Step 4: Audit + tests**

```bash
./scripts/concurrency-audit.sh 2>&1 | tail -5
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -p 1 ./internal/services -run ContextManager -v 2>&1 | tail -20
```

- [ ] **Step 5: Drop allowlist line**

Delete `internal/services/context_manager.go:36:ContextManager` from `scripts/concurrency-audit-allowlist.txt`.

- [ ] **Step 6: Commit**

```bash
git add internal/services/context_manager.go internal/services/context_manager_test.go scripts/concurrency-audit-allowlist.txt
git commit -m "migrate(services): ContextManager → safe.Store/Slice (CONST-029)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

**TIME CAP: 1.5h. If exceeded, revert and skip.**

---

## Task 12: Phase 2 spec — Bucket-1a structural blockers

**Files:**
- Create: `docs/superpowers/specs/2026-04-21-const029-structural-blockers-plan.md`

- [ ] **Step 1: Write the spec scaffold**

Structure:
```markdown
# CONST-029 Structural Blockers — Per-Site Plan (2026-04-21)

Source: `docs/development/REMAINING_WORK_2026-04-21.md` §1a.
Parent: `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-2.

## Common constraints

All sites in this document share: joint invariant across multiple
fields, OR JSON-tagged slice with live wire-format consumer, OR
both. `safe.Store` + `safe.Slice` swap alone is insufficient.

## Site 1: ContextWindow (internal/optimization/context/window.go:22)
…

## Site 2: SemanticCache (internal/optimization/gptcache/semantic_cache.go:50)
…

## Site 3: MCTSNode (internal/planning/mcts.go:27)
…

## Site 4: DiscoveredProvider (internal/services/provider_discovery.go:79)
…

## Site 5: AgentTeam (internal/handlers/extended/ensemble.go:26)
…

## Site 6: Task (internal/handlers/extended/ensemble.go:84)
…

## Site 7: ExtendedPlanModeSession (internal/handlers/extended/planning.go:24)
…
```

- [ ] **Step 2: Per-site contents**

Each site section MUST contain:
1. **Current shape** — field list + lock pattern (verbatim from file)
2. **Touch-point census** — `grep -c` result counting direct field accesses in both src and test
3. **Decision** — the approach chosen (from the design) with rationale
4. **Migration sketch** — code block showing the target struct shape
5. **Test impact** — rough line-count estimate of test-file changes
6. **Session-budget estimate** — hours

Use touch-point examples:
- ContextWindow: 28 mu-guarded sites (from source inventory)
- SemanticCache: count with `grep -c 'sc\.\w*\[' internal/optimization/gptcache/semantic_cache.go`
- MCTSNode: count `TotalReward` accesses in src + test
- etc.

- [ ] **Step 3: Run a placeholder scan**

```bash
grep -E 'TBD|TODO|FIXME|\?\?\?' docs/superpowers/specs/2026-04-21-const029-structural-blockers-plan.md
```
Expected: no matches.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/specs/2026-04-21-const029-structural-blockers-plan.md
git commit -m "docs(specs): CONST-029 structural blockers per-site plan (Phase 2)

Covers the 7 Bucket-1a sites with touch-point census, decision
matrix (MarshalJSON-snapshot vs atomic.Pointer[*state]), migration
sketches, and session-budget estimates. Plan-only; no code changes.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 13: Phase 2.5 spec — Remaining Bucket-1c tractable sites

**Files:**
- Create: `docs/superpowers/specs/2026-04-21-const029-bucket1c-remaining-plan.md`

- [ ] **Step 1: Write the spec**

Structure covers 6 sites: `FreeProviderAdapter`, `ProviderRegistry`, `DebateTeamConfig`, `CodeGraph`, `InstancePool`, `WorkerPool`. For each:

1. **Current shape** (from source read)
2. **Touch-point census**
3. **Decision**: safe.Store swap vs. Pattern Zeta mu vs. state-pointer
4. **Migration sketch**
5. **Test impact**
6. **Session-budget estimate**

Special section for `FreeProviderAdapter`: **bonus objective** — fix the pre-existing race between `fa.mu.RLock` readers and the per-call `modelsMu` writers. Spec the regression test that races concurrent `verify()` calls on a single adapter.

- [ ] **Step 2: Placeholder scan**

```bash
grep -E 'TBD|TODO|FIXME|\?\?\?' docs/superpowers/specs/2026-04-21-const029-bucket1c-remaining-plan.md
```
Expected: none.

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/specs/2026-04-21-const029-bucket1c-remaining-plan.md
git commit -m "docs(specs): CONST-029 Bucket-1c remaining sites plan (Phase 2.5)

Per-site plan for 6 tractable-but-high-coupling sites.
FreeProviderAdapter carries a race-fix as bonus objective.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 14: Phase 3 spec — Bucket-1b protocol-layer

**Files:**
- Create: `docs/superpowers/specs/2026-04-21-const029-protocol-layer-plan.md`

- [ ] **Step 1: Write the spec**

Sites: `LSPClient`, `ACPManager+ACPClient`, `MCPClient+HTTPTransport`, `ACPDiscoveryClient`, `ProtocolDiscovery`, `LSPManager`.

Per site:
1. **Current shape**
2. **Test-coupling census** (`grep -c` of direct field accesses in test files)
3. **Migration staging** — pure-state maps first, transport state last
4. **Test-under-load gate** — `tests/load/` suite must pass before AND after; define which sub-suites apply per site
5. **Paired-migration callouts** — ACPManager+ACPClient and MCPClient+HTTPTransport are pair-migrated, never split
6. **Session-budget estimate** — ~2h each

- [ ] **Step 2: Placeholder scan + commit**

```bash
grep -E 'TBD|TODO|FIXME|\?\?\?' docs/superpowers/specs/2026-04-21-const029-protocol-layer-plan.md
git add docs/superpowers/specs/2026-04-21-const029-protocol-layer-plan.md
git commit -m "docs(specs): CONST-029 protocol-layer plan with test-under-load gate (Phase 3)

Covers 6 Bucket-1b sites. Paired migrations (ACPManager+ACPClient,
MCPClient+HTTPTransport) noted. HTTP/3 transport test-under-load
gate mandatory per site.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 15: Phase 4 spec — go-elder-plinius Phase-A (index + 9 stubs)

**Files:**
- Create: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md` (index)
- Create: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA-go-plinius-common.md`
- Create: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA-go-gandalf-solutions.md`
- Create: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA-go-autotemp.md`
- Create: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA-go-hypertune.md`
- Create: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA-go-i-llm.md`
- Create: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA-go-v3r1t4s.md`
- Create: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA-go-leakhub.md`
- Create: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA-go-cl4r1t4s.md`
- Create: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA-go-ourobopus.md`

- [ ] **Step 1: Write the index**

```markdown
# go-elder-plinius Phase-A Plan — 2026-04-21

**Status:** GATED. No Phase-A code may be written until explicit
per-module user approval.

**Scope:** the 9 defensible-subset modules. INTERNAL integration
only — no public `vasic-digital` / GitLab repos. Each module is
re-implemented from the Python upstream as a clean-room port,
integrated as a HelixAgent internal submodule.

**Per-module specs (stubs):**

| Module | Spec | Scope estimate |
|--------|------|----------------|
| go-plinius-common | [link](2026-04-21-elder-plinius-phaseA-go-plinius-common.md) | 4d core / 2wk full |
| go-gandalf-solutions | [link](2026-04-21-elder-plinius-phaseA-go-gandalf-solutions.md) | 4d core / 2wk full |
| go-autotemp | [link](2026-04-21-elder-plinius-phaseA-go-autotemp.md) | 4d core / 2wk full |
| go-hypertune | [link](2026-04-21-elder-plinius-phaseA-go-hypertune.md) | 4d core / 2wk full |
| go-i-llm | [link](2026-04-21-elder-plinius-phaseA-go-i-llm.md) | 4d core / 2wk full |
| go-v3r1t4s | [link](2026-04-21-elder-plinius-phaseA-go-v3r1t4s.md) | 4d core / 2wk full |
| go-leakhub | [link](2026-04-21-elder-plinius-phaseA-go-leakhub.md) | 4d core / 2wk full |
| go-cl4r1t4s | [link](2026-04-21-elder-plinius-phaseA-go-cl4r1t4s.md) | 4d core / 2wk full |
| go-ourobopus | [link](2026-04-21-elder-plinius-phaseA-go-ourobopus.md) | 4d core / 2wk full |

Total commitment if all 9 are approved: **~36 person-days core
surface / ~18 person-weeks full-spec**.

**Workflow per module (once approved):**
1. Run `superpowers:brainstorming` against the Python upstream to
   pick behavioral surface.
2. Produce Go API signatures derived from the Python source (NOT
   from the v3 codegen, which was semantically broken).
3. TDD-implement core surface with 100% test coverage.
4. Integrate as an internal HelixAgent submodule (decide path
   during brainstorm — likely `internal/elder_plinius/<module>/`
   or a top-level submodule under the HelixAgent repo).
5. Documentation: CLAUDE.md + AGENTS.md + README.md + docs/ per
   CLAUDE.md §7.

**NO PUBLIC REPO.** Per design spec §Phase-4 and Bucket-3a policy.
```

- [ ] **Step 2: Write per-module stubs**

Each per-module stub follows this template:

```markdown
# Phase-A Plan — <module> (2026-04-21)

**Status:** GATED. Awaiting explicit approval.
**Upstream Python:** `<URL or path if in docs/research>`
**Defensible-subset justification:** <one sentence from triage>

## 1. Upstream behavioral surface (to be derived by brainstorm)

Placeholder pending `superpowers:brainstorming` run against Python
upstream. Do NOT copy signatures from the v3 Go codegen scaffold —
semantic bugs contaminate the type signatures.

## 2. Proposed Go API (draft, from scaffold — unverified)

List of public types / functions the scaffold currently exposes.
These are starting points for review, not commitments.

## 3. Core-surface scope (4 days)

- Bullet list of the subset of §2 implemented in Phase-A core.

## 4. Full-spec scope (2 weeks)

- Bullet list of everything beyond the core surface.

## 5. Test plan

- Unit: per-function table tests.
- Integration: interaction with HelixAgent's (DeepTeamRedTeamer |
  LLMsVerifier | debate orchestrator) — depending on module.
- 100% coverage target per CLAUDE.md §1.

## 6. Integration point

<Which HelixAgent internal submodule this becomes.>

## 7. Documentation

CLAUDE.md + AGENTS.md + README.md + docs/ per CLAUDE.md §7.
```

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/specs/2026-04-21-elder-plinius-phaseA*.md
git commit -m "docs(specs): go-elder-plinius Phase-A index + 9 per-module stub plans (Phase 4, gated)

All stubs are GATED on explicit per-module approval. Integration
is INTERNAL-only — no public vasic-digital / GitLab repos.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 16: Final — rewrite REMAINING_WORK → _CLOSED + update memory

**Files:**
- Create: `docs/development/REMAINING_WORK_2026-04-21_CLOSED.md`
- Modify: `/home/milosvasic/.claude-milos85vasic2nd/projects/-run-media-milosvasic-DATA4TB-Projects-HelixAgent/memory/project_const029_campaign.md`
- Modify: `/home/milosvasic/.claude-milos85vasic2nd/projects/-run-media-milosvasic-DATA4TB-Projects-HelixAgent/memory/MEMORY.md` (update pointer line)

- [ ] **Step 1: Write _CLOSED snapshot**

```markdown
# Remaining Work Inventory — 2026-04-21 CLOSED

**Source:** `docs/development/REMAINING_WORK_2026-04-21.md` (original HEAD `0ed59e09`).
**Closed HEAD:** `<HEAD after Task 15>`.
**Session:** 2026-04-21 execution run per
`docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`.

## Resolution per line item

### Bucket 1a (7 structural blockers)

| Site | Resolution |
|------|------------|
| ContextWindow | **plan committed** — `docs/superpowers/specs/2026-04-21-const029-structural-blockers-plan.md` |
| SemanticCache | **plan committed** — same spec |
| MCTSNode | **plan committed** — same spec |
| DiscoveredProvider | **plan committed** — same spec |
| AgentTeam | **plan committed** — same spec |
| Task | **plan committed** — same spec |
| ExtendedPlanModeSession | **plan committed** — same spec |

### Bucket 1b (6 protocol-layer sites)

| Site | Resolution |
|------|------------|
| LSPClient | **plan committed** — `docs/superpowers/specs/2026-04-21-const029-protocol-layer-plan.md` |
| ACPManager+ACPClient | **plan committed** (paired) — same spec |
| MCPClient+HTTPTransport | **plan committed** (paired) — same spec |
| ACPDiscoveryClient | **plan committed** — same spec |
| ProtocolDiscovery | **plan committed** — same spec |
| LSPManager | **plan committed** — same spec |

### Bucket 1c (9 tractable high-coupling)

| Site | Resolution |
|------|------------|
| MemoryService | **executed** — commit `<sha>` (Task 9) |
| ConcurrencyAlertManager | **executed** — commit `<sha>` (Task 10) |
| ContextManager | **executed** — commit `<sha>` (Task 11) |
| FreeProviderAdapter | **plan committed** — `docs/superpowers/specs/2026-04-21-const029-bucket1c-remaining-plan.md` (bonus race-fix) |
| ProviderRegistry | **plan committed** — same spec |
| DebateTeamConfig | **plan committed** — same spec |
| CodeGraph | **plan committed** — same spec |
| InstancePool | **plan committed** — same spec |
| WorkerPool | **plan committed** — same spec |

### Bucket 2 (elder-plinius integration)

| Item | Resolution |
|------|------------|
| 9 defensible modules compile | **executed** — commit `<sha>` (Task 7); all 9 `go build ./...` exit 0 |
| Phase-A implementation (398 methods) | **plan committed, gated** — `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md` + 9 per-module stubs |
| 35 integration files + 15 submodules wired | **plan committed, gated** — same spec |
| 22 non-defensible modules (13 other + 9 offensive) | 13 stay as internal docs/research/ reference; 9 offensive retired |

### Bucket 3 (policy-declined)

| Item | Resolution |
|------|------------|
| 3a (9 offensive modules) | **executed + retired + lifted** — scaffolds removed (Task 2); prompt corpora lifted into `internal/security/redteam/fixtures/` as defensive harness (Tasks 1–6) |
| 3b (misrepresent stubs as integrated) | **policy preserved** — no misrepresentation |
| 3c (factual errors in integration plan) | **executed** — `docs/research/inbox/2026-04-21_go-elder-plinius_integration_plan_CORRECTED.md` |

### "Decisions that unblock each bucket"

| Decision | Resolution |
|----------|------------|
| Authorize per-site sessions for Bucket 1c | 3/9 executed in-session; remaining 6 planned (Phase 2.5) |
| Decide MarshalJSON-snapshot vs state-pointer for JSON-tagged-slice structs | **decided** in Phase-2 spec: DiscoveredProvider → MarshalJSON-snapshot; AgentTeam/Task/ExtendedPlanModeSession → state-pointer |
| Authorize staged protocol-layer migration | **planned** (Phase 3) awaiting per-pair session authorization |
| Approve Phase-A for 9 defensible modules | **plans committed; awaits per-module approval** (Phase 4) |
| Brainstorm each module's upstream behavioral surface before Phase-A | **built into Phase 4 per-module workflow** |
| Policy line on Bucket 3a public repos | **preserved; offensive scaffolds retired; defensive lift completed** |
| Corrected-delta integration plan | **executed** (Task 8) |

## Allowlist state (scripts/concurrency-audit-allowlist.txt)

- **Entry count before session:** 24
- **Entry count after session:** 21 (3 drained: MemoryService, ConcurrencyAlertManager, ContextManager)
- **Status:** audit exit 0

## Fixture harness state

- 7 attack-class YAMLs committed under `internal/security/redteam/fixtures/`
- `DeepTeamRedTeamer.RunFixtureSuite(ctx, class)` wired to `StandardGuardrailPipeline`
- `make test-redteam-fixtures` target present
- `./challenges/scripts/redteam_fixtures_challenge.sh` present + passing
- `.gitattributes` export-ignore set on fixtures dir

## Commits this session

See `git log 0ed59e09..HEAD`.

## Cross-reference

- Design spec: `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`
- Execution plan: `docs/superpowers/plans/2026-04-21-remaining-work-execution.md`
- Phase 2 plan: `docs/superpowers/specs/2026-04-21-const029-structural-blockers-plan.md`
- Phase 2.5 plan: `docs/superpowers/specs/2026-04-21-const029-bucket1c-remaining-plan.md`
- Phase 3 plan: `docs/superpowers/specs/2026-04-21-const029-protocol-layer-plan.md`
- Phase 4 plan: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md` + 9 stubs
- Corrected integration plan: `docs/research/inbox/2026-04-21_go-elder-plinius_integration_plan_CORRECTED.md`
```

- [ ] **Step 2: Update CONST-029 campaign memory**

Edit `memory/project_const029_campaign.md` — update allowlist count (254 original → 21 remaining, 91.7% drained) and reference the 2026-04-21 execution artifacts.

- [ ] **Step 3: Update MEMORY.md pointer line**

Edit `memory/MEMORY.md` — update the campaign line to reflect new drain count and add 2026-04-21 spec/plan references.

- [ ] **Step 4: Commit**

```bash
git add docs/development/REMAINING_WORK_2026-04-21_CLOSED.md
git commit -m "docs(status): REMAINING_WORK_2026-04-21 CLOSED — per-item resolution snapshot

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

Memory updates are outside the git repo and happen via Write tool, not commit.

---

## Self-review checklist (for the executor)

Before declaring the session complete:

- [ ] `./scripts/concurrency-audit.sh` exits 0
- [ ] `./challenges/scripts/redteam_fixtures_challenge.sh` exits 0 (ALL CHECKS PASSED)
- [ ] All 9 defensible-subset elder-plinius modules produce `go build ./... → exit 0`
- [ ] `memory/MEMORY.md` pointer line reflects final state
- [ ] `git status` is clean except for `.claude/scheduled_tasks.lock`
- [ ] Session commits counted with `git log 0ed59e09..HEAD --oneline | wc -l`
- [ ] `docs/development/REMAINING_WORK_2026-04-21_CLOSED.md` references every Bucket 1/2/3 item

## Push policy

Do NOT push during this plan's execution. After Task 16 commit, user will review `git log` and initiate the multi-remote fanout push manually (as done earlier in the session).

---

*End of plan.*
