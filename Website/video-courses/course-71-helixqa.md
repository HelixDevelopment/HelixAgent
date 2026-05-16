# Video Course 71: HelixQA Orchestration Framework

## Course Overview

**Duration:** 3 hours
**Level:** Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 06 (Testing Strategies), Course 70 (DocProcessor)

Master the HelixQA orchestration framework (`digital.vasic.helixqa`). Learn to configure autonomous QA sessions, write YAML test banks, detect crashes and ANRs in real time, collect evidence pipelines, generate issue tickets, and manage multi-platform session lifecycles across Android, Web, and Desktop targets.

---

## Learning Objectives

By the end of this course, you will be able to:

1. Configure and launch an autonomous QA session with the SessionCoordinator
2. Write YAML test banks with platform filtering and priority levels
3. Detect crashes and ANRs using the detector package across all platforms
4. Build evidence collection pipelines with screenshots, logs, and video
5. Generate structured issue tickets for AI fix pipelines
6. Manage session lifecycle: phases, coverage targets, and timeout handling

---

## Module 1: QA Orchestrator Setup (30 min)

### Video 1.1: HelixQA Architecture (15 min)

**Topics:**
- HelixQA composes: detector + validator + reporter + testbank + evidence + ticket
- The `SessionCoordinator` manages full lifecycle of autonomous QA sessions
- Dependencies: `digital.vasic.challenges`, `digital.vasic.containers`, DocProcessor, LLMOrchestrator, VisionEngine
- Configuration: platforms, speed mode, device, output directory, report format

**Key Types:**
```go
type SessionCoordinator struct {
    config       *SessionConfig
    orchestrator agent.AgentPool
    visionEngine analyzer.Analyzer
    featureMap   *feature.FeatureMap
    workers      map[string]*PlatformWorker
    phaseManager *PhaseManager
    session      *session.SessionRecorder
    coverage     coverage.CoverageTracker
}
```

### Video 1.2: SessionConfig and Defaults (15 min)

**Topics:**
- `SessionID`, `OutputDir`, `Platforms`, `Timeout`, `CoverageTarget`
- Curiosity-driven exploration: `CuriosityEnabled` and `CuriosityTimeout`
- Default: 2-hour timeout, 90% coverage target, all platforms, curiosity on
- Speed modes: slow (debugging), normal (default), fast (CI pipelines)
- Connecting the AgentPool and VisionEngine as injected dependencies

**Default Config:**
```go
func DefaultSessionConfig() *SessionConfig {
    return &SessionConfig{
        Platforms:        []string{"android", "desktop", "web"},
        Timeout:          2 * time.Hour,
        CoverageTarget:   0.90,
        CuriosityEnabled: true,
        CuriosityTimeout: 30 * time.Minute,
    }
}
```

---

## Module 2: Test Bank YAML Format (30 min)

### Video 2.1: Test Bank Structure (15 min)

**Topics:**
- YAML-based test bank files with platform and priority filtering
- Test case fields: name, description, steps, expected, platform, priority
- The `testbank` package: loading, filtering by platform, sorting by priority
- Reusing `digital.vasic.challenges` bank loading infrastructure

**Example Test Bank:**
```yaml
test_cases:
  - name: login_valid_credentials
    description: Verify login with valid email and password
    platform: all
    priority: critical
    steps:
      - action: tap_element
        target: email_field
        value: "user@example.com"
      - action: tap_element
        target: password_field
        value: "secure123"
      - action: tap_element
        target: login_button
    expected: "Dashboard screen is displayed"
```

### Video 2.2: Filtering and Prioritization (15 min)

**Topics:**
- Platform filtering: `android`, `web`, `desktop`, `all`
- Priority levels: critical, high, medium, low
- Running critical tests first for fast feedback
- Combining test banks from multiple YAML files

---

## Module 3: Crash and ANR Detection (30 min)

### Video 3.1: Detector Architecture (15 min)

**Topics:**
- The `detector` package: real-time crash and ANR detection across platforms
- `CommandRunner` interface: abstraction for command execution (testable with mocks)
- Android: ADB logcat monitoring for crash traces and ANR dialogs
- Web: browser console error monitoring via Playwright/CDP
- Desktop: JVM exception and process exit monitoring

### Video 3.2: Configuring Detection (15 min)

**Topics:**
- Step-by-step validation: `ValidateSteps` mode checks after each action
- Crash signature parsing: extracting stack traces and error messages
- ANR detection: timeout thresholds and unresponsive process signals
- Integration with the validator for immediate evidence capture on detection

**Detector Usage:**
```go
detector := detector.NewCrashDetector(
    detector.WithCommandRunner(runner),
    detector.WithPlatform(config.PlatformAndroid),
    detector.WithDevice("emulator-5554"),
    detector.WithPackage("dev.helix.agent"),
)
issues, err := detector.Check(ctx)
```

---

## Module 4: Evidence Collection Pipeline (25 min)

### Video 4.1: Evidence Types and Collection (10 min)

**Topics:**
- Evidence artifacts: screenshots, video recordings, device logs, stack traces
- The `evidence` package: centralized collection and storage
- Timestamped evidence linked to specific test steps
- Output directory structure: `qa-results/sessions/<session-id>/`

### Video 4.2: Building the Pipeline (15 min)

**Topics:**
- Automatic screenshot capture on test step failure
- Video recording start/stop around test execution
- Log collection from ADB, browser console, and JVM stderr
- Compressing and archiving evidence for CI artifact storage
- Linking evidence to the CoverageTracker from DocProcessor

---

## Module 5: Ticket Generation (20 min)

### Video 5.1: Structured Issue Tickets (10 min)

**Topics:**
- The `ticket` package generates Markdown tickets for detected issues
- Ticket fields: title, description, reproduction steps, evidence links, severity
- Designed for AI fix pipelines: structured format for LLM consumption
- Automatic severity classification based on crash type and frequency

### Video 5.2: Ticket Templates and Integration (10 min)

**Topics:**
- Customizing ticket templates for different issue trackers
- Linking tickets to test bank entries and feature IDs
- Batch ticket generation after a full QA session
- Routing tickets to appropriate teams based on platform and category

**Generated Ticket Example:**
```markdown
## Bug: Login Crash on Invalid Input
**Severity:** Critical | **Platform:** Android | **Session:** helix-1711234567

### Reproduction Steps
1. Open the application
2. Enter invalid email format in login field
3. Tap Login button
4. Application crashes with NullPointerException

### Evidence
- Screenshot: evidence/crash-001.png
- Logcat: evidence/logcat-001.txt
- Stack trace: evidence/stacktrace-001.txt
```

---

## Module 6: Session Management (25 min)

### Video 6.1: Session Lifecycle and Phases (15 min)

**Topics:**
- `PhaseManager` controls session progression: setup, execute, curiosity, report
- `SessionRecorder` logs events, timings, and coverage milestones
- Coverage-driven termination: session ends when target is reached
- Timeout handling: graceful stop with evidence collection on timeout

### Video 6.2: Hands-On Lab (10 min)

**Objective:** Run a complete autonomous QA session against a sample application.

**Steps:**
1. Write a YAML test bank with 5 test cases across 2 platforms
2. Configure a `SessionConfig` with 80% coverage target
3. Launch the session coordinator with mock agent pool and vision engine
4. Monitor phase transitions and coverage progress
5. Inspect the generated QA report and issue tickets
6. Verify evidence artifacts in the output directory

---

## Assessment

### Quiz (10 questions)

1. What are the core dependencies of HelixQA?
2. How does SessionCoordinator manage multi-platform workers?
3. What YAML fields define a test case in a test bank?
4. How does the crash detector differ across Android, Web, and Desktop?
5. What is the `CommandRunner` interface and why is it important for testing?
6. What evidence types are collected by the evidence pipeline?
7. How are issue tickets structured for AI fix pipelines?
8. What triggers the curiosity-driven exploration phase?
9. When does a session terminate before its timeout?
10. How does HelixQA integrate with DocProcessor's CoverageTracker?

### Practical Assessment

Build a complete HelixQA session:
1. Write a YAML test bank with 10 test cases (3 critical, 4 high, 3 medium)
2. Configure a SessionConfig targeting Android and Web with 85% coverage
3. Implement a mock CommandRunner that simulates 2 crashes
4. Run the session and verify crash detection triggers evidence collection
5. Inspect the generated tickets and QA report

Deliverables:
1. YAML test bank file
2. SessionConfig and launch code
3. Generated QA report and tickets
4. Evidence directory with captured artifacts

---

## Module 7: Autonomous Pipeline Deep Dive (30 min)

### Video 7.1: The 5-Phase Pipeline (15 min)

**Topics:**
- End-to-end walk-through of the autonomous pipeline phases: Learn → Plan → Execute → Curiosity → Analyze
- **Learn phase**: DocProcessor builds the feature map, LLMsVerifier selects agents, LLMOrchestrator spawns the pool
- **Plan phase**: SessionCoordinator maps features to UI screens via VisionEngine and schedules platform workers
- **Execute phase**: workers run test cases in parallel, crash detectors active continuously, evidence captured per step
- **Curiosity phase**: NavigationGraph BFS guides agents to unvisited screens; edge cases exercised
- **Analyze phase**: coverage aggregation, report generation, ticket filing, evidence archival
- Phase budget configuration: `HELIXQA_PHASES`, `HELIXQA_CURIOSITY_TIMEOUT`, `HELIXQA_COVERAGE_TARGET`

**Phase Configuration:**
```bash
# Run all 5 phases (default)
HELIXQA_PHASES=learn,plan,execute,curiosity,analyze

# Skip curiosity for fast CI runs
HELIXQA_PHASES=learn,plan,execute,analyze

# Tune curiosity budget and coverage gate
HELIXQA_CURIOSITY_TIMEOUT=30m
HELIXQA_COVERAGE_TARGET=0.90
```

### Video 7.2: PhaseManager Internals (15 min)

**Topics:**
- `PhaseManager` struct: phase registry, transition guards, coverage-driven termination
- How phase transitions are gated: coverage check before advancing to curiosity, timeout check before analyze
- `PhaseManager.Run(ctx)` event loop: dispatch → execute → evaluate → advance
- Handling phase failures: retry policy, partial results, graceful degradation
- Observability: phase start/end events emitted to EventBus, Prometheus phase-duration histograms

**Key Types:**
```go
type PhaseManager struct {
    phases      []Phase
    current     int
    coverage    coverage.CoverageTracker
    coordinator *SessionCoordinator
    eventBus    eventbus.Bus
}

type Phase interface {
    Name() string
    Run(ctx context.Context, state *SessionState) error
    ShouldSkip(state *SessionState) bool
}
```

---

## Module 8: Multi-Device Testing (25 min)

### Video 8.1: AndroidDevices Configuration (15 min)

**Topics:**
- Configuring multiple Android devices: emulators, physical devices, and remote ADB-over-TCP
- `AndroidDevices` field in `SessionConfig`: list of ADB serial addresses
- How `SessionCoordinator` spawns one `PlatformWorker` per device
- Isolated evidence directories per device: `qa-results/sessions/<id>/<device>/`
- Result merging: per-device breakdowns folded into a single aggregate report
- Parallel execution: all devices run the same test bank concurrently

**Multi-Device Config:**
```bash
# Via environment
HELIXQA_ANDROID_DEVICES=emulator-5554,emulator-5556,192.168.1.100:5555
HELIXQA_ANDROID_PACKAGE=com.example.app
HELIXQA_PARALLEL_WORKERS=3

# Via CLI
helixqa run --banks tests/banks/ --platform android \
  --devices emulator-5554,emulator-5556,192.168.1.100:5555 \
  --package com.example.app \
  --parallel
```

**SessionConfig struct with devices:**
```go
type SessionConfig struct {
    // ... existing fields ...
    AndroidDevices []string          // ADB serial list for parallel testing
    ParallelWorkers int              // Max concurrent platform workers
}
```

### Video 8.2: Worker Isolation and Result Merging (10 min)

**Topics:**
- `PlatformWorker` lifecycle: init → run bank → collect evidence → report → shutdown
- ADB connection management per worker: device claim, lock, release
- Evidence isolation: no cross-device file collisions via device-namespaced directories
- Result merging strategy: union of findings, intersection of coverage, per-device pass/fail tables
- Handling device disconnection mid-session: graceful degradation, partial results preserved

---

## Module 9: QA REST API Integration (20 min)

### Video 9.1: API Overview and Authentication (10 min)

**Topics:**
- The `/v1/qa/*` endpoint family: sessions, findings, knowledge discovery
- Authentication: Bearer token using `HELIXAGENT_API_KEY`
- Starting a session via `POST /v1/qa/sessions` vs using the CLI
- Polling session status: the `status` and `current_phase` fields
- Stopping a session early: `DELETE /v1/qa/sessions/{id}` preserves evidence

**Starting a Session via API:**
```bash
curl -s -X POST "http://localhost:7061/v1/qa/sessions" \
  -H "Authorization: Bearer $HELIX_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "project": "/path/to/MyApp",
    "platforms": ["android", "web"],
    "coverage_target": 0.85,
    "timeout": "2h",
    "report_formats": ["markdown", "json"]
  }'
```

### Video 9.2: Findings and Knowledge Discovery (10 min)

**Topics:**
- Listing findings with filters: `?severity=critical&platform=android&status=open`
- Updating finding status: `PATCH /v1/qa/sessions/{id}/findings/{finding_id}`
- Bulk status updates: `PATCH /v1/qa/sessions/{id}/findings` with `ids` array
- Triggering project knowledge discovery: `POST /v1/qa/knowledge/discover`
- Polling discovery job status before starting a session
- Full automated workflow: discover → start → poll → fetch findings → acknowledge

**Complete CI Workflow Snippet:**
```bash
# 1. Refresh feature map
curl -sf -X POST "$HELIX_URL/v1/qa/knowledge/discover" \
  -H "Authorization: Bearer $HELIX_KEY" \
  -d '{"project": "/path/to/MyApp", "force_refresh": true}'

# 2. Start session, capture session_id
SESSION_ID=$(curl -sf -X POST "$HELIX_URL/v1/qa/sessions" \
  -H "Authorization: Bearer $HELIX_KEY" \
  -d '{"project":"/path/to/MyApp","platforms":["android","web"]}' \
  | jq -r '.session_id')

# 3. Poll until done
while [[ "$(curl -sf "$HELIX_URL/v1/qa/sessions/$SESSION_ID" \
  -H "Authorization: Bearer $HELIX_KEY" | jq -r '.status')" == "running" ]]; do
  sleep 30
done

# 4. Fetch critical findings
curl -sf "$HELIX_URL/v1/qa/sessions/$SESSION_ID/findings?severity=critical" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .
```

---

## Resources

- [HelixQA CLAUDE.md](../../helix_qa/CLAUDE.md)
- [SessionCoordinator Source](../../helix_qa/pkg/autonomous/coordinator.go)
- [Config Package Source](../../helix_qa/pkg/config/config.go)
- [Detector Package](../../helix_qa/pkg/detector/)
- [Test Bank Package](../../helix_qa/pkg/testbank/)
- [User Manual 39: HelixQA Guide](../user-manuals/39-helixqa-guide.md)
- [User Manual 44: QA REST API Guide](../user-manuals/44-qa-api-guide.md)
- [Course 70: DocProcessor Deep Dive](course-70-docprocessor.md)
