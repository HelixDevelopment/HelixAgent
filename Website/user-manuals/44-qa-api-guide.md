# User Manual 44: QA REST API Guide

## Overview

HelixQA exposes a REST API under `/v1/qa/*` that gives programmatic control over QA sessions, findings, and project knowledge discovery. The API is served by the HelixAgent binary on the same port as the main API (default `7061`) and requires a valid JWT or API key in the `Authorization` header.

Use the QA REST API to:

- Start and stop autonomous QA sessions without using the CLI
- Poll session progress and coverage from dashboards or CI pipelines
- Query, filter, and update findings (bugs, crashes, tickets)
- Trigger project knowledge discovery to refresh the feature map

## Prerequisites

- HelixAgent running: `./bin/helixagent` (or `make run`)
- A valid API key from `.env`: `HELIXAGENT_API_KEY=your-key`
- `curl` 7.x+ or any HTTP client

All examples below use `curl`. Set your key once:

```bash
export HELIX_KEY="your-helixagent-api-key"
export HELIX_URL="http://localhost:7061"
```

---

## Step 1: Start a QA Session

Send a `POST` request to `/v1/qa/sessions` with the session configuration:

```bash
curl -s -X POST "$HELIX_URL/v1/qa/sessions" \
  -H "Authorization: Bearer $HELIX_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "project": "/path/to/MyApp",
    "platforms": ["android", "web"],
    "banks_dir": "tests/banks/",
    "coverage_target": 0.85,
    "timeout": "2h",
    "curiosity_enabled": true,
    "curiosity_timeout": "30m",
    "speed_mode": "normal",
    "report_formats": ["markdown", "json"],
    "output_dir": "qa-results/",
    "devices": {
      "android": ["emulator-5554", "emulator-5556"]
    }
  }'
```

**Request fields:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `project` | string | yes | — | Absolute path to the project directory |
| `platforms` | []string | yes | — | Platforms to test: `android`, `web`, `desktop` |
| `banks_dir` | string | no | `tests/banks/` | Directory containing YAML test bank files |
| `coverage_target` | float | no | `0.90` | Stop when this fraction of features is covered |
| `timeout` | duration | no | `2h` | Hard session timeout (Go duration string) |
| `curiosity_enabled` | bool | no | `true` | Enable curiosity-driven exploration phase |
| `curiosity_timeout` | duration | no | `30m` | Budget for the curiosity phase |
| `speed_mode` | string | no | `normal` | `slow`, `normal`, or `fast` |
| `report_formats` | []string | no | `["markdown"]` | Output formats: `markdown`, `html`, `json` |
| `output_dir` | string | no | `qa-results/` | Directory for reports, evidence, and tickets |
| `devices.android` | []string | no | — | List of ADB device serials for parallel Android testing |

**Response (201 Created):**

```json
{
  "session_id": "helix-1711234567",
  "status": "started",
  "phases": ["learn", "plan", "execute", "curiosity", "analyze"],
  "platforms": ["android", "web"],
  "coverage_target": 0.85,
  "started_at": "2026-03-30T10:00:00Z",
  "timeout_at": "2026-03-30T12:00:00Z"
}
```

Save the `session_id` — you will use it in all subsequent requests.

---

## Step 2: Query Session Status

Poll the session to track progress through the 5-phase pipeline:

```bash
curl -s "$HELIX_URL/v1/qa/sessions/helix-1711234567" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .
```

**Response:**

```json
{
  "session_id": "helix-1711234567",
  "status": "running",
  "current_phase": "execute",
  "phases_completed": ["learn", "plan"],
  "coverage": {
    "overall": 0.62,
    "android": 0.71,
    "web": 0.53
  },
  "findings_count": {
    "critical": 1,
    "high": 3,
    "medium": 7,
    "low": 2
  },
  "elapsed": "38m12s",
  "remaining": "1h21m48s"
}
```

**Status values:**

| Status | Meaning |
|--------|---------|
| `started` | Session accepted, initializing |
| `running` | Actively executing test phases |
| `completed` | All phases finished successfully |
| `stopped` | Manually stopped via DELETE |
| `failed` | Session terminated due to unrecoverable error |
| `timed_out` | Hard timeout reached |

Poll until `status` is `completed`, `stopped`, `failed`, or `timed_out`.

---

## Step 3: Query Findings

List findings generated during the session. Supports filtering by severity, platform, and status:

```bash
# All findings
curl -s "$HELIX_URL/v1/qa/sessions/helix-1711234567/findings" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .

# Only critical findings on Android
curl -s "$HELIX_URL/v1/qa/sessions/helix-1711234567/findings?severity=critical&platform=android" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .

# Open findings only
curl -s "$HELIX_URL/v1/qa/sessions/helix-1711234567/findings?status=open" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .
```

**Query parameters:**

| Parameter | Values | Description |
|-----------|--------|-------------|
| `severity` | `critical`, `high`, `medium`, `low` | Filter by severity level |
| `platform` | `android`, `web`, `desktop` | Filter by platform |
| `status` | `open`, `acknowledged`, `resolved`, `wont_fix` | Filter by finding status |
| `page` | integer | Page number (1-based, default: 1) |
| `per_page` | integer | Results per page (default: 20, max: 100) |

**Response:**

```json
{
  "total": 13,
  "page": 1,
  "per_page": 20,
  "findings": [
    {
      "id": "finding-001",
      "session_id": "helix-1711234567",
      "title": "Login crash on invalid email format",
      "severity": "critical",
      "platform": "android",
      "device": "emulator-5554",
      "status": "open",
      "detected_at": "2026-03-30T10:23:11Z",
      "test_case_id": "TC-001",
      "reproduction_steps": [
        "Open application",
        "Navigate to login screen",
        "Enter 'notanemail' in email field",
        "Tap Login button"
      ],
      "evidence": {
        "screenshot": "qa-results/sessions/helix-1711234567/emulator-5554/screenshots/crash-001.png",
        "logcat": "qa-results/sessions/helix-1711234567/emulator-5554/logcat/logcat-001.txt",
        "stack_trace": "java.lang.NullPointerException at LoginActivity.java:142"
      },
      "ticket_path": "qa-results/sessions/helix-1711234567/tickets/finding-001.md"
    }
  ]
}
```

---

## Step 4: Update Finding Status

Acknowledge, resolve, or close a finding after developer review:

```bash
curl -s -X PATCH \
  "$HELIX_URL/v1/qa/sessions/helix-1711234567/findings/finding-001" \
  -H "Authorization: Bearer $HELIX_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "acknowledged",
    "assignee": "backend-team",
    "comment": "Confirmed crash. Fix scheduled for sprint 12.",
    "priority_override": "critical"
  }'
```

**Request fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `status` | string | yes | New status: `acknowledged`, `resolved`, `wont_fix`, `open` |
| `assignee` | string | no | Team or person responsible for the fix |
| `comment` | string | no | Free-text note attached to the status change |
| `priority_override` | string | no | Override auto-detected severity if incorrect |

**Response (200 OK):**

```json
{
  "id": "finding-001",
  "status": "acknowledged",
  "assignee": "backend-team",
  "comment": "Confirmed crash. Fix scheduled for sprint 12.",
  "updated_at": "2026-03-30T11:05:33Z"
}
```

Bulk update multiple findings in one request:

```bash
curl -s -X PATCH \
  "$HELIX_URL/v1/qa/sessions/helix-1711234567/findings" \
  -H "Authorization: Bearer $HELIX_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "ids": ["finding-003", "finding-004", "finding-005"],
    "status": "resolved",
    "comment": "Fixed in PR #247"
  }'
```

---

## Step 5: Discover Project Knowledge

The knowledge discovery endpoint triggers DocProcessor to (re)scan the project documentation and rebuild the feature map. Run this before starting a session when docs have changed, or on demand to refresh the cached feature map.

```bash
curl -s -X POST \
  "$HELIX_URL/v1/qa/knowledge/discover" \
  -H "Authorization: Bearer $HELIX_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "project": "/path/to/MyApp",
    "docs_dirs": ["docs/", "README.md", "CHANGELOG.md"],
    "force_refresh": true
  }'
```

**Request fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `project` | string | yes | Absolute path to the project root |
| `docs_dirs` | []string | no | Specific docs paths to scan (default: auto-detect) |
| `force_refresh` | bool | no | Bypass the cache and re-scan all docs |

**Response:**

```json
{
  "job_id": "discover-1711234999",
  "status": "running",
  "project": "/path/to/MyApp",
  "started_at": "2026-03-30T10:00:05Z"
}
```

Poll the discovery job:

```bash
curl -s "$HELIX_URL/v1/qa/knowledge/discover/discover-1711234999" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .
```

```json
{
  "job_id": "discover-1711234999",
  "status": "completed",
  "features_found": 47,
  "docs_scanned": 12,
  "completed_at": "2026-03-30T10:00:23Z",
  "feature_map_path": "qa-results/feature-maps/MyApp-20260330.json"
}
```

---

## Complete Workflow Example

The following shell script demonstrates a full automated QA run using only the REST API:

```bash
#!/usr/bin/env bash
set -euo pipefail

HELIX_KEY="${HELIXAGENT_API_KEY}"
HELIX_URL="http://localhost:7061"
PROJECT="/path/to/MyApp"

echo "=== Step 1: Refresh project knowledge ==="
DISCOVER=$(curl -sf -X POST "$HELIX_URL/v1/qa/knowledge/discover" \
  -H "Authorization: Bearer $HELIX_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"project\": \"$PROJECT\", \"force_refresh\": true}")
DISCOVER_JOB=$(echo "$DISCOVER" | jq -r '.job_id')
echo "Discovery job: $DISCOVER_JOB"

# Wait for discovery to complete
while true; do
  STATE=$(curl -sf "$HELIX_URL/v1/qa/knowledge/discover/$DISCOVER_JOB" \
    -H "Authorization: Bearer $HELIX_KEY" | jq -r '.status')
  [[ "$STATE" == "completed" ]] && break
  echo "  Discovery status: $STATE — waiting..."
  sleep 5
done
echo "Discovery complete."

echo "=== Step 2: Start QA session ==="
SESSION=$(curl -sf -X POST "$HELIX_URL/v1/qa/sessions" \
  -H "Authorization: Bearer $HELIX_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"project\": \"$PROJECT\",
    \"platforms\": [\"android\", \"web\"],
    \"coverage_target\": 0.85,
    \"timeout\": \"90m\",
    \"report_formats\": [\"markdown\", \"json\"]
  }")
SESSION_ID=$(echo "$SESSION" | jq -r '.session_id')
echo "Session started: $SESSION_ID"

echo "=== Step 3: Poll until complete ==="
while true; do
  STATUS=$(curl -sf "$HELIX_URL/v1/qa/sessions/$SESSION_ID" \
    -H "Authorization: Bearer $HELIX_KEY")
  STATE=$(echo "$STATUS" | jq -r '.status')
  PHASE=$(echo "$STATUS" | jq -r '.current_phase')
  COVERAGE=$(echo "$STATUS" | jq -r '.coverage.overall')
  echo "  Phase: $PHASE | Coverage: $COVERAGE | Status: $STATE"
  [[ "$STATE" =~ ^(completed|stopped|failed|timed_out)$ ]] && break
  sleep 30
done

echo "=== Step 4: Fetch critical findings ==="
FINDINGS=$(curl -sf \
  "$HELIX_URL/v1/qa/sessions/$SESSION_ID/findings?severity=critical" \
  -H "Authorization: Bearer $HELIX_KEY")
COUNT=$(echo "$FINDINGS" | jq '.total')
echo "Critical findings: $COUNT"
echo "$FINDINGS" | jq -r '.findings[] | "[\(.id)] \(.title) (\(.platform))"'

echo "=== Step 5: Acknowledge all critical findings ==="
IDS=$(echo "$FINDINGS" | jq -r '[.findings[].id]')
curl -sf -X PATCH "$HELIX_URL/v1/qa/sessions/$SESSION_ID/findings" \
  -H "Authorization: Bearer $HELIX_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"ids\": $IDS, \"status\": \"acknowledged\", \"comment\": \"Triaged by CI pipeline\"}"

echo "=== Done. QA session $SESSION_ID complete. ==="
```

---

## Stop a Session Early

To abort a running session and collect any evidence gathered so far:

```bash
curl -s -X DELETE "$HELIX_URL/v1/qa/sessions/helix-1711234567" \
  -H "Authorization: Bearer $HELIX_KEY"
```

The session transitions to `stopped` status. Evidence collected up to that point is preserved in the output directory and a partial report is generated.

---

## Error Responses

All endpoints return standard error shapes:

```json
{
  "error": "session not found",
  "code": "QA_SESSION_NOT_FOUND",
  "session_id": "helix-9999999999"
}
```

Common error codes:

| Code | HTTP Status | Meaning |
|------|------------|---------|
| `QA_SESSION_NOT_FOUND` | 404 | Session ID does not exist |
| `QA_SESSION_ALREADY_RUNNING` | 409 | A session for this project is already active |
| `QA_INVALID_PLATFORM` | 400 | Unknown platform in `platforms` array |
| `QA_PROJECT_NOT_FOUND` | 400 | Project path does not exist on the server |
| `QA_FINDING_NOT_FOUND` | 404 | Finding ID does not exist in this session |
| `QA_INVALID_STATUS` | 400 | Unknown finding status value |
| `UNAUTHORIZED` | 401 | Missing or invalid API key |

---

## Related Resources

- [User Manual 39: HelixQA Guide](39-helixqa-guide.md) -- Full HelixQA setup and CLI reference
- [User Manual 41: VisionEngine Guide](41-visionengine-guide.md) -- Vision backend configuration
- [Video Course 71: HelixQA Orchestration Framework](../video-courses/course-71-helixqa.md) -- In-depth video lessons
- Source: `helix_qa/README.md`, `helix_qa/CLAUDE.md`
