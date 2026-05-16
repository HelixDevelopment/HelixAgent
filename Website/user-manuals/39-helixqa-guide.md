# User Manual 39: HelixQA Guide

## Overview

HelixQA (`digital.vasic.helixqa`) is a QA orchestration framework for cross-platform testing with real-time crash detection, step validation, evidence collection, and automated ticket generation. It supports Android, Web, and Desktop platforms.

## Prerequisites

- Go 1.24+
- Sibling module directories: `../Challenges`, `../Containers`
- For Android testing: ADB installed, emulator or device connected
- For Web testing: Playwright or a Chromium-based browser
- For Desktop testing: X11 display available (Linux) or native window manager
- (Optional) LLM API keys for autonomous QA sessions

## Step 1: Install HelixQA

Build from source:

```bash
cd HelixQA
make build
# Binary output: bin/helixqa
```

Verify:

```bash
./bin/helixqa version
```

## Step 2: Create Test Banks in YAML

Test banks define the cases HelixQA will execute. Create a file `tests/banks/core.yaml`:

```yaml
version: "1.0"
name: "Core Feature Tests"
test_cases:
  - id: TC-001
    name: "Create new document"
    category: functional
    priority: critical
    platforms: [android, web, desktop]
    steps:
      - name: "Open app"
        action: "Launch application"
        expected: "Main editor screen visible"
      - name: "Create document"
        action: "Tap/click new document button"
        expected: "Empty document created with cursor active"
    tags: [core, smoke]
    documentation_refs:
      - type: user_guide
        section: "3.1"
        path: "docs/USER_MANUAL.md"
```

Fields: `id` (unique), `priority` (critical/high/medium/low), `platforms` (android/web/desktop), `steps` with action/expected pairs, `tags` for filtering.

## Step 3: Configure Platform Detectors

HelixQA ships 3 crash/ANR detectors:

| Detector | Platform | Mechanism |
|----------|----------|-----------|
| `android` | Android | ADB-based: `pidof`, `logcat`, `screencap` |
| `web` | Web | Browser process monitoring via `pgrep` |
| `desktop` | Desktop | JVM/process monitoring via `pgrep`, `kill` |

Configure via `.env`:

```bash
# Android
HELIXQA_ANDROID_DEVICE=emulator-5554
HELIXQA_ANDROID_PACKAGE=com.example.app

# Web
HELIXQA_WEB_URL=http://localhost:4200
HELIXQA_WEB_BROWSER=chromium

# Desktop
HELIXQA_DESKTOP_PROCESS=myapp
HELIXQA_DESKTOP_DISPLAY=:0
```

## Step 4: Run the QA Pipeline

Run tests against a specific platform:

```bash
helixqa run --banks tests/banks/ --platform android \
  --device emulator-5554 --package com.example.app
```

Run across all platforms:

```bash
helixqa run --banks tests/banks/ --platform all
```

List available test cases without executing:

```bash
helixqa list --banks tests/banks/ --platform android
```

## Step 5: Run an Autonomous QA Session

The autonomous mode uses LLM agents and VisionEngine to navigate applications without predefined steps:

```bash
cp .env.example .env
# Edit .env: set API keys and platform settings

helixqa autonomous --project /path/to/MyApp \
  --platforms android,desktop,web \
  --env .env \
  --timeout 2h \
  --coverage-target 0.9 \
  --output qa-results/ \
  --report markdown,html,json
```

Autonomous sessions run 4 phases:
1. **Setup** -- Select LLMs via LLMsVerifier, build feature map via DocProcessor, spawn agents via LLMOrchestrator
2. **Doc-Driven Verification** -- Verify every documented feature against the running app
3. **Curiosity-Driven Exploration** -- Explore undiscovered areas, test edge cases
4. **Report and Cleanup** -- Aggregate coverage, tickets, and navigation maps

## Step 6: Collect Evidence and Review Results

Evidence is captured at each test step:

```bash
ls qa-results/
# qa-report.md     -- Full report
# tickets/         -- Auto-generated issue tickets (Markdown)
# videos/          -- Screen recordings
# screenshots/     -- Per-step captures
# logcat/          -- Android logcat dumps
```

Each ticket includes: reproduction steps, evidence links, priority, and affected platform.

## Step 7: Generate Reports

Convert existing results to different formats:

```bash
helixqa report --input qa-results --format html
helixqa report --input qa-results --format json
```

Speed modes for different contexts:

| Mode | Use Case |
|------|----------|
| `slow` | Debugging, captures maximum evidence |
| `normal` | Standard QA runs |
| `fast` | CI pipelines, minimal evidence |

## Step 8: Run Tests

```bash
make test       # 235 tests
make test-race  # With race detection
make test-cover # Coverage report
```

## Troubleshooting

- **"ADB device not found"**: Ensure `adb devices` lists your target device or emulator
- **Web tests fail to connect**: Verify the application URL is reachable and the browser binary exists
- **Empty evidence directory**: Check that the platform worker has filesystem write permissions
- **Low coverage score**: Add more test banks or enable curiosity-driven exploration in autonomous mode

## Autonomous Pipeline

HelixQA's autonomous mode runs a 5-phase pipeline that drives the full lifecycle of an unscripted QA session:

| Phase | Name | Description |
|-------|------|-------------|
| 1 | **Learn** | DocProcessor extracts the feature map from documentation. LLMsVerifier selects and ranks the best available LLM agents. LLMOrchestrator spawns the agent pool. |
| 2 | **Plan** | The SessionCoordinator builds an execution plan: prioritizes test cases by criticality, maps documented features to UI screens via VisionEngine, and schedules platform workers. |
| 3 | **Execute** | Platform workers run the test plan in parallel. Each step captures evidence (screenshot, logcat/console, video frame). Crash/ANR detectors run continuously. Failed steps trigger immediate ticket generation. |
| 4 | **Curiosity** | After documented features are covered, curiosity-driven exploration begins. Agents navigate to unvisited screens using the NavigationGraph BFS path planner. Edge cases and undocumented paths are exercised. |
| 5 | **Analyze** | Coverage is aggregated across all platforms. The QA report is generated in the requested formats (Markdown, HTML, JSON). Tickets are filed and evidence is archived. |

Configure the pipeline phases via environment variables:

```bash
HELIXQA_PHASES=learn,plan,execute,curiosity,analyze   # All phases (default)
HELIXQA_PHASES=learn,plan,execute,analyze              # Skip curiosity
HELIXQA_CURIOSITY_TIMEOUT=30m                          # Curiosity phase budget
HELIXQA_COVERAGE_TARGET=0.90                           # Stop when 90% coverage reached
```

## Multi-Device Parallel QA

HelixQA supports running the same test bank simultaneously across multiple Android devices via the `AndroidDevices` configuration. This reduces QA cycle time and validates behavior across different OS versions and screen sizes.

Configure multiple devices in `.env`:

```bash
HELIXQA_ANDROID_DEVICES=emulator-5554,emulator-5556,192.168.1.100:5555
HELIXQA_ANDROID_PACKAGE=com.example.app
HELIXQA_PARALLEL_WORKERS=3
```

Or pass devices at runtime:

```bash
helixqa run --banks tests/banks/ --platform android \
  --devices emulator-5554,emulator-5556,192.168.1.100:5555 \
  --package com.example.app \
  --parallel
```

Each device gets its own `PlatformWorker` instance with isolated evidence directories:

```
qa-results/
  sessions/<session-id>/
    emulator-5554/
      screenshots/
      logcat/
    emulator-5556/
      screenshots/
      logcat/
    192.168.1.100:5555/
      screenshots/
      logcat/
    qa-report.md
    tickets/
```

Test cases marked `platform: android` run on all connected devices. Results are merged into a single report, with per-device breakdowns for each test case.

## Credential Discovery

HelixQA automatically discovers credentials from `.env` files in the project directory, eliminating the need to pass API keys and configuration values on the command line. This is especially useful for autonomous sessions that need LLM provider keys, database URLs, and device credentials.

Discovery order (first match wins):

1. Explicit `--env` flag: `helixqa autonomous --env /path/to/.env`
2. `.env` file in the current working directory
3. `.env` file in the project root (when `--project` is specified)
4. `.env.example` values (safe defaults only, no secrets)

Environment variable precedence (highest to lowest):

```
Shell environment > --env file > .env in CWD > .env in project root
```

Supported credential categories discovered automatically:

| Category | Variables |
|----------|-----------|
| LLM providers | `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `*_API_KEY` |
| Android | `HELIXQA_ANDROID_DEVICE`, `HELIXQA_ANDROID_PACKAGE` |
| Web | `HELIXQA_WEB_URL`, `HELIXQA_WEB_BROWSER` |
| Database | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` |
| Redis | `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` |

To audit which credentials were discovered without running a session:

```bash
helixqa credentials --project /path/to/MyApp --env .env
```

## REST API Integration

HelixQA exposes a REST API under `/v1/qa/*` for programmatic control of QA sessions. This enables integration with CI pipelines, dashboards, and external ticketing systems without using the CLI.

Base URL: `http://localhost:7061/v1/qa`

### Start a QA Session

```http
POST /v1/qa/sessions
Content-Type: application/json

{
  "project": "/path/to/MyApp",
  "platforms": ["android", "web"],
  "banks_dir": "tests/banks/",
  "coverage_target": 0.85,
  "timeout": "2h",
  "report_formats": ["markdown", "json"],
  "output_dir": "qa-results/"
}
```

Response:

```json
{
  "session_id": "helix-1711234567",
  "status": "started",
  "phases": ["learn", "plan", "execute", "curiosity", "analyze"],
  "started_at": "2026-03-30T10:00:00Z"
}
```

### Query Session Status

```http
GET /v1/qa/sessions/{session_id}
```

### List Findings

```http
GET /v1/qa/sessions/{session_id}/findings?severity=critical&platform=android
```

### Update Finding Status

```http
PATCH /v1/qa/sessions/{session_id}/findings/{finding_id}
Content-Type: application/json

{
  "status": "acknowledged",
  "assignee": "dev-team",
  "comment": "Known issue, fix in sprint 12"
}
```

### Stop a Session

```http
DELETE /v1/qa/sessions/{session_id}
```

See [User Manual 44: QA API Guide](44-qa-api-guide.md) for the complete API reference with full request/response schemas and example workflows.

## Playwright Web Testing

HelixQA integrates with Playwright for browser-based web testing. When the `web` platform is selected, HelixQA launches a Playwright-controlled Chromium (or Firefox/WebKit) instance and drives the application under test through a real browser.

Install Playwright browsers before running web tests:

```bash
npx playwright install chromium
```

Configure web testing in `.env`:

```bash
HELIXQA_WEB_URL=http://localhost:4200
HELIXQA_WEB_BROWSER=chromium          # chromium | firefox | webkit
HELIXQA_WEB_HEADLESS=true             # false for visual debugging
HELIXQA_WEB_TIMEOUT=30s               # Per-action timeout
HELIXQA_WEB_SCREENSHOT_ON_FAIL=true
HELIXQA_WEB_VIDEO=retain-on-failure   # always | never | retain-on-failure
```

Playwright capabilities available in test steps:

| Action | Description |
|--------|-------------|
| `navigate` | Load a URL in the browser |
| `click` | Click an element by CSS selector or text content |
| `fill` | Type text into an input field |
| `select` | Choose an option from a dropdown |
| `wait_for` | Wait for an element or network idle |
| `screenshot` | Capture the current browser state |
| `assert_visible` | Assert an element is visible on the page |
| `assert_text` | Assert element text content |

Web test steps in YAML use the same format as other platforms:

```yaml
test_cases:
  - id: WEB-001
    name: "User login flow"
    platform: web
    priority: critical
    steps:
      - name: "Navigate to login page"
        action: "navigate"
        target: "/login"
        expected: "Login form is visible"
      - name: "Enter credentials"
        action: "fill"
        target: "#email"
        value: "user@example.com"
      - name: "Submit form"
        action: "click"
        target: "button[type=submit]"
        expected: "Dashboard is displayed"
```

## Google Gemini Vision Provider

VisionEngine now includes a Google Gemini Vision provider, adding Google's multimodal Gemini models as a first-class vision backend. Gemini Vision offers strong performance on UI screenshot analysis, element detection, and screen identification.

Configure the Gemini Vision provider:

```bash
VISION_GEMINI_API_KEY=your-gemini-api-key
VISION_GEMINI_MODEL=gemini-1.5-flash    # gemini-1.5-flash | gemini-1.5-pro | gemini-2.0-flash
VISION_PROVIDERS=gemini,openai,anthropic # Gemini as primary
```

Use Gemini Vision programmatically:

```go
import "github.com/digital/vasic/visionengine/pkg/llmvision"

provider := llmvision.NewGeminiProvider(llmvision.GeminiConfig{
    APIKey: os.Getenv("VISION_GEMINI_API_KEY"),
    Model:  "gemini-1.5-flash",
})

analysis, err := provider.AnalyzeImage(ctx, screenshotBytes,
    "Identify all interactive UI elements and their positions")
```

Gemini Vision advantages:

- Strong multilingual text recognition (OCR) in screenshots
- Large context window for analyzing complex UIs with many elements
- `gemini-1.5-flash` offers low-latency analysis at reduced cost
- `gemini-1.5-pro` provides the highest accuracy for complex screens
- Seamlessly integrates with the `FallbackProvider` chain alongside OpenAI, Anthropic, and Qwen

## Related Resources

- [User Manual 38: DocProcessor Guide](38-docprocessor-guide.md) -- Feature map extraction
- [User Manual 40: LLMOrchestrator Guide](40-llmorchestrator-guide.md) -- Agent pool management
- [User Manual 41: VisionEngine Guide](41-visionengine-guide.md) -- Screenshot analysis and navigation graphs
- [User Manual 44: QA API Guide](44-qa-api-guide.md) -- REST API reference for programmatic QA control
- Source: `helix_qa/README.md`, `helix_qa/CLAUDE.md`
