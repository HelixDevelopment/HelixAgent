# HelixAgent Comprehensive Monitoring System

## Overview

The HelixAgent Monitoring System provides comprehensive monitoring capabilities for all challenge executions, including:

- Real-time resource monitoring (CPU, memory, disk, network)
- Log collection from all system components
- Memory leak detection
- Warning/error detection and analysis
- Automatic issue investigation
- Comprehensive HTML/JSON reports

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                 MONITORING SYSTEM ARCHITECTURE               │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────┐    ┌─────────────────┐                │
│  │ monitoring_lib  │    │ report_generator│                │
│  │     .sh         │    │      .sh        │                │
│  └────────┬────────┘    └────────┬────────┘                │
│           │                      │                          │
│           ▼                      ▼                          │
│  ┌─────────────────────────────────────────┐               │
│  │         run_monitored_challenges.sh      │               │
│  │  • Challenge orchestration               │               │
│  │  • Resource sampling                     │               │
│  │  • Log collection                        │               │
│  │  • Issue tracking                        │               │
│  └─────────────────────────────────────────┘               │
│                                                             │
│  Output:                                                    │
│  ┌──────────────┬──────────────┬─────────────┐             │
│  │ JSON Report  │ HTML Report  │ Issue Files │             │
│  └──────────────┴──────────────┴─────────────┘             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Components

### Core Library (`challenges/monitoring/lib/monitoring_lib.sh`)

The core monitoring library provides all monitoring functions:

| Function | Description |
|----------|-------------|
| `mon_init` | Initialize monitoring session |
| `mon_log` | Log messages with severity levels |
| `mon_sample_resources` | Collect CPU/memory/disk/network stats |
| `mon_collect_all_logs` | Gather logs from all components |
| `mon_detect_memory_leaks` | Detect memory leaks against baseline |
| `mon_analyze_log_file` | Analyze logs for errors/warnings |
| `mon_record_issue` | Record detected issues with severity |
| `mon_record_fix` | Record applied fixes with test references |
| `mon_finalize` | Finalize monitoring and generate summary |

### Report Generator (`challenges/monitoring/lib/report_generator.sh`)

Generates comprehensive reports in multiple formats:

- **JSON Report**: Machine-readable format for CI/CD integration
- **HTML Report**: Human-readable format with visualizations

### Main Runner (`challenges/monitoring/run_monitored_challenges.sh`)

The main entry point that:
1. Initializes monitoring
2. Starts infrastructure (HelixAgent, PostgreSQL, Redis)
3. Runs all challenges with monitoring
4. Investigates detected errors
5. Generates final reports

## Usage

### Running Monitored Challenges

```bash
# Run all challenges with monitoring
./challenges/monitoring/run_monitored_challenges.sh

# Run specific challenges
./challenges/monitoring/run_monitored_challenges.sh --challenges "health_monitoring,provider_verification"

# Skip infrastructure checks
./challenges/monitoring/run_monitored_challenges.sh --skip-infra

# Continue on failures
./challenges/monitoring/run_monitored_challenges.sh --continue-on-failure
```

### Using the Monitoring Library

```bash
#!/bin/bash
source "./challenges/monitoring/lib/monitoring_lib.sh"

# Initialize monitoring
mon_init "my_test_session"

# Log events
mon_log "INFO" "Starting test..."

# Sample resources periodically
mon_sample_resources

# Analyze logs
mon_analyze_log_file "/var/log/helixagent.log" "helixagent" || true

# Record issues and fixes
mon_record_issue "high" "Memory usage exceeded threshold"
mon_record_fix "issue_001" "Optimized memory allocation" "TestMemoryOptimization"

# Finalize and generate report
mon_finalize
```

## Error Detection Patterns

### Error Patterns (15 patterns)
```
panic:|fatal error:|FATAL|ERROR|runtime error:|nil pointer|
index out of range|deadlock|timeout|connection refused|
context deadline exceeded|too many open files|out of memory|OOMKilled
```

### Warning Patterns (10 patterns)
```
WARN|WARNING|deprecated|retry|reconnect|circuit breaker|
rate limit|slow query|high latency|token expired|invalid.*format
```

### Ignored Patterns (5 patterns)
```
TestError|error.*test|expected.*error|mock.*error|
PASS|ok.*dev.helix
```

## Memory Leak Detection

The monitoring system detects memory leaks by:

1. **Baseline Collection**: Capture initial memory usage at startup
2. **Periodic Sampling**: Sample memory usage during execution
3. **Threshold Comparison**: Alert if memory exceeds baseline by configured threshold
4. **FD Monitoring**: Track file descriptor count for leak detection

Default threshold: 150% of baseline memory usage

## Resource Monitoring

### Collected Metrics

| Metric | Source | Frequency |
|--------|--------|-----------|
| CPU Usage | `/proc/stat` | Every sample |
| Memory Usage | `/proc/meminfo` | Every sample |
| Disk I/O | `iostat` | Every sample |
| Network I/O | `/proc/net/dev` | Every sample |
| File Descriptors | `/proc/[pid]/fd` | Every sample |
| Goroutine Count | pprof endpoint | Every sample |

### Sample Interval

Default: 5 seconds (configurable via `MON_SAMPLE_INTERVAL`)

## Report Output

### JSON Report Structure

```json
{
  "session_id": "challenges_20260115_013435",
  "start_time": "2026-01-15T01:34:35Z",
  "end_time": "2026-01-15T02:15:22Z",
  "duration_seconds": 2447,
  "challenges": {
    "total": 45,
    "passed": 43,
    "failed": 2,
    "skipped": 0
  },
  "issues": {
    "total": 0,
    "high": 0,
    "medium": 0,
    "low": 0
  },
  "fixes": {
    "total": 0
  },
  "resources": {
    "peak_memory_mb": 512,
    "peak_cpu_percent": 85,
    "peak_disk_io_mb": 150
  }
}
```

### HTML Report Features

- Executive summary with pass/fail counts
- Resource usage graphs
- Issue timeline
- Challenge results table
- Fix history with test references

## Test Coverage

The monitoring system is covered by comprehensive tests:

### Unit Tests (`tests/integration/monitoring_system_test.go`)

- `TestMonitoringLibInit` - Initialization
- `TestMonitoringSampleResources` - Resource sampling
- `TestMonitoringLogCollection` - Log collection
- `TestMonitoringMemoryLeakDetection` - Memory leak detection
- `TestMonitoringLogAnalysis` - Log analysis
- `TestMonitoringIssueTracking` - Issue tracking
- `TestMonitoringFixRecording` - Fix recording
- `TestMonitoringFinalization` - Finalization

### Challenge Script (`challenges/scripts/monitoring_system_challenge.sh`)

21 comprehensive tests covering:
- Library initialization
- Resource sampling
- Log collection
- Memory leak detection
- Log analysis
- Issue tracking
- Fix recording
- Report generation
- Concurrent operations

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MON_LOG_DIR` | Log directory | `/tmp/helixagent_monitoring` |
| `MON_SAMPLE_INTERVAL` | Sample interval (seconds) | `5` |
| `MON_MEMORY_THRESHOLD` | Memory leak threshold (%) | `150` |
| `MON_RESOURCE_MONITOR` | Enable resource monitoring | `true` |

### Output Directories

```
challenges/monitoring/
├── logs/                    # Session logs
│   └── [session_id]/
│       ├── master.log       # Combined log
│       ├── resources/       # Resource samples
│       ├── components/      # Component logs
│       └── issues/          # Issue records
└── reports/                 # Generated reports
    └── [session_id]/
        ├── report.json
        └── report.html
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
- name: Run Monitored Challenges
  run: |
    ./challenges/monitoring/run_monitored_challenges.sh --continue-on-failure

- name: Upload Monitoring Reports
  uses: actions/upload-artifact@v3
  with:
    name: monitoring-reports
    path: challenges/monitoring/reports/

- name: Check for Critical Issues
  run: |
    if grep -q '"high":' challenges/monitoring/reports/*/report.json; then
      echo "Critical issues detected!"
      exit 1
    fi
```

## Troubleshooting

### Common Issues

1. **ANSI color codes in output**
   - Solution: Output colors to stderr, not stdout

2. **Exit codes from analysis functions**
   - Solution: Use `|| true` after `mon_analyze_log_file`

3. **Double counting in issue/fix recording**
   - Solution: Direct file writes instead of `mon_log`

4. **Report generator stdout pollution**
   - Solution: Redirect status messages to stderr

## Future Enhancements

- [ ] Integration with Prometheus for metrics export
- [ ] Real-time dashboard with WebSocket updates
- [ ] Automated issue correlation
- [ ] Machine learning-based anomaly detection
- [ ] Integration with alerting systems (Slack, PagerDuty)
