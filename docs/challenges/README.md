# HelixAgent Challenge System

## Overview

The HelixAgent challenge system is a comprehensive validation framework that verifies real-life use cases across the entire project. Challenges go beyond traditional tests by validating actual system behavior, code structure, integration points, and deployment readiness. They are a mandatory part of the project's quality gates -- every component must have associated challenges.

The core principle is **no false positives**: challenges must validate actual behavior, not merely check return codes or trivially pass. If a challenge passes, the validated capability genuinely works.

### Key Differences from Tests

| Aspect              | Unit/Integration Tests             | Challenges                                  |
|---------------------|------------------------------------|---------------------------------------------|
| Scope               | Individual functions or packages   | Cross-cutting system capabilities           |
| Data                | Mocks allowed (unit tests only)    | Real data and live services always          |
| Granularity         | Fine-grained assertions            | End-to-end scenario validation              |
| Infrastructure      | May use test doubles               | Requires running containers                 |
| Execution           | `go test`                          | Shell scripts or Go-native orchestrator     |
| Output              | Pass/fail per test function        | Sectioned reports with pass/fail counts     |

---

## How Challenges Validate Real-Life Use Cases

Each challenge script is organized around a specific capability or subsystem. Within the script, individual tests verify that:

1. **Required code structures exist** -- Types, functions, interfaces, and configuration fields are present in the expected source files.
2. **Code compiles and unit tests pass** -- The relevant packages build and their tests succeed.
3. **Integration points are wired correctly** -- Components reference each other as expected (imports, config fields, handler registrations).
4. **Runtime behavior is correct** -- Where possible, the challenge makes HTTP requests to the running HelixAgent server and validates response content.
5. **No regressions** -- Changes to one subsystem have not broken assumptions in another.

For example, the Fallback Mechanism Challenge validates that:
- `FallbackConfig` type is defined in the correct file
- Empty response detection logic is present in the debate service
- Fallback chain iteration loops exist
- Metadata tracking (`fallback_used`, `fallback_index`) is implemented
- Unit tests for fallback scenarios exist and pass
- The running server responds with fallback metadata when providers fail

---

## Types of Challenges

### Shell Script Challenges

Shell script challenges are Bash scripts located in `challenges/scripts/`. Each script is self-contained and follows a standard structure with sections, test counters, and a summary. There are currently 464 challenge scripts.

Shell challenges are the primary form and cover:
- Release build system validation (25 tests)
- Provider integration (47 tests)
- Debate orchestrator (61 tests)
- CLI agent configuration (60 tests)
- HelixSpecifier (138 tests)
- HelixMemory (80+ tests)
- Security scanning, MCP adapters, protocol endpoints, and many more

**Running shell challenges:**

```bash
# Run all challenges
./challenges/scripts/run_all_challenges.sh

# Run a specific challenge
./challenges/scripts/release_build_challenge.sh
./challenges/scripts/fallback_mechanism_challenge.sh

# With custom HelixAgent URL
HELIXAGENT_URL=http://localhost:7061 ./challenges/scripts/fallback_mechanism_challenge.sh
```

### Go-Native Challenges

Go-native challenges live in `internal/challenges/` and use the Challenges module (`digital.vasic.challenges`) framework. They provide 22 user-flow challenges that simulate real users or QA testers interacting with the system through its API endpoints.

Go-native challenges consist of:
- **User flow definitions** (`internal/challenges/userflow/flows.go`) -- API step sequences with assertions
- **Orchestrator** (`internal/challenges/orchestrator.go`) -- Manages parallel execution, stall detection, and result reporting
- **Shell adapter** (`internal/challenges/shell_adapter.go`) -- Discovers and wraps shell scripts as challenge instances
- **Domain-specific challenges** -- Provider verification, debate formation, API quality, infrastructure checks

**Running Go-native challenges:**

```bash
# Via the HelixAgent binary
./bin/helixagent --run-challenges=userflow

# Via go test (challenge tests)
go test -v ./tests/challenge/...
```

Go-native challenges use the `digital.vasic.challenges` module interfaces:
- `challenge.Challenge` -- The core interface every challenge implements
- `challenge.ShellChallenge` -- Wrapper for shell scripts
- `uf.APIFlow` / `uf.APIStep` -- Step-by-step API flow definitions with assertions
- `registry.Registry` -- Central challenge registration
- `runner.Runner` -- Execution engine with concurrency control

---

## How to Run Challenges

### Prerequisites

Infrastructure containers must be running before executing challenges. The HelixAgent binary handles container orchestration automatically during boot:

```bash
# Build the binary
make build

# Run HelixAgent (starts all required infrastructure automatically)
./bin/helixagent
```

The binary reads `containers/.env` and orchestrates all containers (PostgreSQL, Redis, Mock LLM, etc.) either locally or on remote hosts based on configuration.

### Running All Challenges

```bash
./challenges/scripts/run_all_challenges.sh
```

This script:
1. Builds the HelixAgent binary if needed
2. Starts the HelixAgent server (which boots all infrastructure)
3. Runs all challenge scripts in sequence
4. Reports aggregate results
5. Cleans up on exit

### Running Individual Challenges

```bash
# By script name
./challenges/scripts/<challenge_name>.sh

# Examples
./challenges/scripts/release_build_challenge.sh
./challenges/scripts/debate_orchestrator_challenge.sh
./challenges/scripts/helixmemory_challenge.sh
```

### Resource Limits

All challenge execution must stay within 30-40% of host system resources. The `run_all_challenges.sh` runner enforces this. When running individually, apply limits manually:

```bash
nice -n 19 ionice -c 3 ./challenges/scripts/some_challenge.sh
```

---

## How to Interpret Results

### Shell Challenge Output

Each shell challenge prints colored output with `[PASS]` and `[FAIL]` markers per test, organized into sections:

```
=== Section 1: Version Package ===
  [PASS] Version variables with ldflags defaults
  [PASS] Info struct with JSON tags
  [PASS] Get() function exists
  [FAIL] Short() function missing

========================================
Release Build System Challenge Results
========================================
  Total:  25
  Passed: 24
  Failed: 1
```

### Exit Codes

| Exit Code | Meaning                                              |
|-----------|------------------------------------------------------|
| 0         | All tests passed                                     |
| 1         | One or more tests failed                             |

A non-zero exit code means the validated capability has a problem that must be fixed before the component can be considered complete.

### Pass Rate

Some challenges compute a pass rate percentage:

```
Pass Rate: 94%
```

The target is always 100%. Partial passes indicate specific areas that need attention -- review the `[FAIL]` lines to identify exactly which validations did not succeed.

### Aggregate Results (run_all_challenges.sh)

When running all challenges, the runner prints a final summary listing each challenge with its pass/fail status. Any single challenge failure causes the overall run to fail (exit code 1).

### Troubleshooting Failures

1. **Infrastructure not running** -- Most failures when infrastructure is down. Verify the HelixAgent server is running and containers are healthy.
2. **Missing code structures** -- Grep-based checks fail when expected types, functions, or imports are absent. Read the `[FAIL]` message to see which pattern was expected in which file.
3. **Test compilation errors** -- Challenges that run `go test` will fail if the code does not compile. Check the test output for compiler errors.
4. **Runtime failures** -- Challenges that make HTTP requests to the server fail if the server is not responding or returns unexpected status codes. Check `/tmp/helixagent-server.log` for server-side errors.
