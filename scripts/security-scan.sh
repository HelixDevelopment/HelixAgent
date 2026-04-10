#!/bin/bash
# Security Scanning Script for HelixAgent
# Supports: Snyk, SonarQube, Trivy, Gosec
#
# Usage:
#   ./scripts/security-scan.sh [scanner] [options]
#
# Scanners:
#   snyk       - Snyk vulnerability scanner (requires SNYK_TOKEN for full features)
#   sonarqube  - SonarQube code quality and security analysis
#   trivy      - Trivy vulnerability and secret scanner
#   gosec      - Go security checker
#   all        - Run all scanners
#
# Options:
#   --json     - Output in JSON format
#   --html     - Generate HTML report (where supported)
#   --fix      - Attempt to auto-fix issues (where supported)

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
REPORTS_DIR="${PROJECT_DIR}/reports/security"
COMPOSE_FILE="${PROJECT_DIR}/docker-compose.security.yml"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Create reports directory
mkdir -p "$REPORTS_DIR"

# Detect container runtime
detect_runtime() {
    if command -v docker &> /dev/null && docker info &> /dev/null 2>&1; then
        echo "docker"
    elif command -v podman &> /dev/null; then
        echo "podman"
    else
        echo "none"
    fi
}

# Detect compose command
detect_compose() {
    local runtime="$1"
    if [ "$runtime" = "docker" ]; then
        if docker compose version &> /dev/null 2>&1; then
            echo "docker compose"
        elif command -v docker-compose &> /dev/null; then
            echo "docker-compose"
        fi
    elif [ "$runtime" = "podman" ]; then
        if command -v podman-compose &> /dev/null; then
            echo "podman-compose"
        fi
    fi
}

RUNTIME=$(detect_runtime)
COMPOSE_CMD=$(detect_compose "$RUNTIME")

if [ -z "$COMPOSE_CMD" ]; then
    echo -e "${RED}Error: No container runtime found. Install Docker or Podman.${NC}"
    exit 1
fi

echo -e "${BLUE}Using container runtime: ${RUNTIME}${NC}"
echo -e "${BLUE}Using compose command: ${COMPOSE_CMD}${NC}"

# Function to run Gosec (Go Security Checker)
run_gosec() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Running Gosec Security Scanner${NC}"
    echo -e "${BLUE}========================================${NC}"

    local report_file="${REPORTS_DIR}/gosec-${TIMESTAMP}.json"
    local html_report="${REPORTS_DIR}/gosec-${TIMESTAMP}.html"

    # Check if gosec is installed locally
    if command -v gosec &> /dev/null; then
        echo -e "${GREEN}Using local gosec installation${NC}"
        gosec -conf=.gosec.yml -fmt=json -out="$report_file" ./... 2>/dev/null || true
        gosec -conf=.gosec.yml -fmt=html -out="$html_report" ./... 2>/dev/null || true
    else
        echo -e "${YELLOW}Gosec not installed locally, using container${NC}"
        $COMPOSE_CMD -f "$COMPOSE_FILE" --profile scan run --rm gosec-scanner \
            -fmt=json -out=/app/reports/security/gosec-${TIMESTAMP}.json ./... 2>/dev/null || true
    fi

    if [ -f "$report_file" ]; then
        echo -e "${GREEN}Gosec report saved to: ${report_file}${NC}"

        # Parse and display summary
        local issues=$(jq '.Issues | length' "$report_file" 2>/dev/null || echo "0")
        local high=$(jq '[.Issues[] | select(.severity == "HIGH")] | length' "$report_file" 2>/dev/null || echo "0")
        local medium=$(jq '[.Issues[] | select(.severity == "MEDIUM")] | length' "$report_file" 2>/dev/null || echo "0")
        local low=$(jq '[.Issues[] | select(.severity == "LOW")] | length' "$report_file" 2>/dev/null || echo "0")

        echo -e "${YELLOW}Gosec Summary:${NC}"
        echo -e "  Total Issues: ${issues}"
        echo -e "  ${RED}High: ${high}${NC}"
        echo -e "  ${YELLOW}Medium: ${medium}${NC}"
        echo -e "  ${GREEN}Low: ${low}${NC}"
    fi
}

# Function to run Trivy
run_trivy() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Running Trivy Vulnerability Scanner${NC}"
    echo -e "${BLUE}========================================${NC}"

    local report_file="${REPORTS_DIR}/trivy-${TIMESTAMP}.json"
    local html_report="${REPORTS_DIR}/trivy-${TIMESTAMP}.html"

    # Check if trivy is installed locally
    if command -v trivy &> /dev/null; then
        echo -e "${GREEN}Using local trivy installation${NC}"
        trivy fs --format json --output "$report_file" \
            --scanners vuln,secret,misconfig \
            --severity HIGH,CRITICAL \
            "$PROJECT_DIR" 2>/dev/null || true
    else
        echo -e "${YELLOW}Trivy not installed locally, using container${NC}"
        $RUNTIME run --rm -v "${PROJECT_DIR}:/app:ro" \
            -v "${REPORTS_DIR}:/reports" \
            aquasec/trivy:latest fs \
            --format json \
            --output /reports/trivy-${TIMESTAMP}.json \
            --scanners vuln,secret,misconfig \
            --severity HIGH,CRITICAL \
            /app 2>/dev/null || true
    fi

    if [ -f "$report_file" ]; then
        echo -e "${GREEN}Trivy report saved to: ${report_file}${NC}"

        # Parse and display summary
        local vulns=$(jq '.Results[]?.Vulnerabilities // [] | length' "$report_file" 2>/dev/null | awk '{s+=$1} END {print s+0}')
        local secrets=$(jq '.Results[]?.Secrets // [] | length' "$report_file" 2>/dev/null | awk '{s+=$1} END {print s+0}')
        local misconfigs=$(jq '.Results[]?.Misconfigurations // [] | length' "$report_file" 2>/dev/null | awk '{s+=$1} END {print s+0}')

        echo -e "${YELLOW}Trivy Summary:${NC}"
        echo -e "  Vulnerabilities: ${vulns:-0}"
        echo -e "  Secrets: ${secrets:-0}"
        echo -e "  Misconfigurations: ${misconfigs:-0}"
    fi
}

# Function to run Snyk
run_snyk() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Running Snyk Vulnerability Scanner${NC}"
    echo -e "${BLUE}========================================${NC}"

    local report_file="${REPORTS_DIR}/snyk-${TIMESTAMP}.json"

    # Check if SNYK_TOKEN is set
    if [ -z "$SNYK_TOKEN" ]; then
        echo -e "${YELLOW}Warning: SNYK_TOKEN not set. Running in limited OSS mode.${NC}"
        echo -e "${YELLOW}For full features, set SNYK_TOKEN environment variable.${NC}"
    fi

    # Check if snyk is installed locally
    if command -v snyk &> /dev/null; then
        echo -e "${GREEN}Using local snyk installation${NC}"
        cd "$PROJECT_DIR"
        snyk test --json > "$report_file" 2>/dev/null || true
    else
        echo -e "${YELLOW}Snyk not installed locally, using container${NC}"
        $RUNTIME run --rm \
            -v "${PROJECT_DIR}:/app:ro" \
            -v "${REPORTS_DIR}:/reports" \
            -e SNYK_TOKEN="${SNYK_TOKEN:-}" \
            snyk/snyk:golang test --json /app > "$report_file" 2>/dev/null || true
    fi

    if [ -f "$report_file" ] && [ -s "$report_file" ]; then
        echo -e "${GREEN}Snyk report saved to: ${report_file}${NC}"

        # Parse and display summary
        local vulns=$(jq '.vulnerabilities | length' "$report_file" 2>/dev/null || echo "0")
        local high=$(jq '[.vulnerabilities[] | select(.severity == "high")] | length' "$report_file" 2>/dev/null || echo "0")
        local medium=$(jq '[.vulnerabilities[] | select(.severity == "medium")] | length' "$report_file" 2>/dev/null || echo "0")
        local low=$(jq '[.vulnerabilities[] | select(.severity == "low")] | length' "$report_file" 2>/dev/null || echo "0")

        echo -e "${YELLOW}Snyk Summary:${NC}"
        echo -e "  Total Vulnerabilities: ${vulns}"
        echo -e "  ${RED}High: ${high}${NC}"
        echo -e "  ${YELLOW}Medium: ${medium}${NC}"
        echo -e "  ${GREEN}Low: ${low}${NC}"
    else
        echo -e "${YELLOW}Snyk scan completed (no vulnerabilities or limited mode)${NC}"
    fi
}

# Function to start SonarQube server
start_sonarqube() {
    echo -e "${BLUE}Starting SonarQube server...${NC}"
    $COMPOSE_CMD -f "$COMPOSE_FILE" up -d sonarqube sonarqube-db

    echo -e "${YELLOW}Waiting for SonarQube to be ready (this may take 2-3 minutes)...${NC}"
    local max_attempts=60
    local attempt=0

    while [ $attempt -lt $max_attempts ]; do
        if curl -s "http://localhost:9000/api/system/status" | grep -q '"status":"UP"'; then
            echo -e "${GREEN}SonarQube is ready!${NC}"
            return 0
        fi
        attempt=$((attempt + 1))
        echo -n "."
        sleep 5
    done

    echo -e "${RED}SonarQube failed to start within timeout${NC}"
    return 1
}

# Function to run SonarQube analysis
run_sonarqube() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Running SonarQube Code Analysis${NC}"
    echo -e "${BLUE}========================================${NC}"

    # Check if SonarQube is running
    if ! curl -s "http://localhost:9000/api/system/status" | grep -q '"status":"UP"'; then
        echo -e "${YELLOW}SonarQube not running, starting...${NC}"
        start_sonarqube || return 1
    fi

    # Create sonar-project.properties if it doesn't exist
    if [ ! -f "${PROJECT_DIR}/sonar-project.properties" ]; then
        echo -e "${YELLOW}Creating sonar-project.properties...${NC}"
        cat > "${PROJECT_DIR}/sonar-project.properties" << 'EOF'
sonar.projectKey=helixagent
sonar.projectName=HelixAgent
sonar.projectVersion=1.0

# Source directories
sonar.sources=.
sonar.exclusions=vendor/**,**/testdata/**,**/*_test.go,**/mock_*.go,reports/**,bin/**,docs/**

# Go settings
sonar.go.coverage.reportPaths=coverage.out
sonar.go.tests.reportPaths=test-report.json

# Encoding
sonar.sourceEncoding=UTF-8

# Quality Gate
sonar.qualitygate.wait=true
EOF
    fi

    # Generate coverage report for SonarQube
    echo -e "${YELLOW}Generating coverage report...${NC}"
    cd "$PROJECT_DIR"
    go test -coverprofile=coverage.out ./internal/... 2>/dev/null || true

    # Run sonar-scanner
    local sonar_token="${SONAR_TOKEN:-}"
    if [ -z "$sonar_token" ]; then
        # Try to generate token with admin credentials from env (fallback to default)
        local sonar_user="${SONARQUBE_ADMIN_USER:-admin}"
        local sonar_pass="${SONARQUBE_ADMIN_PASSWORD:-admin}"
        echo -e "${YELLOW}No SONAR_TOKEN set, generating token with admin credentials${NC}"
        sonar_token=$(curl -s -u "${sonar_user}:${sonar_pass}" -X POST "http://localhost:9000/api/user_tokens/generate" -d "name=scan-${TIMESTAMP}" 2>/dev/null | jq -r '.token // empty')
        if [ -z "$sonar_token" ]; then
            echo -e "${RED}Failed to generate SonarQube token. Please set SONAR_TOKEN environment variable.${NC}"
            return 1
        fi
        echo -e "${GREEN}Generated temporary scan token${NC}"
    fi

    if command -v sonar-scanner &> /dev/null; then
        echo -e "${GREEN}Using local sonar-scanner${NC}"
        sonar-scanner \
            -Dsonar.host.url=http://localhost:9000 \
            -Dsonar.token="$sonar_token" \
            -Dsonar.projectBaseDir="$PROJECT_DIR"
    else
        echo -e "${YELLOW}Using containerized sonar-scanner${NC}"
        # Use :Z for SELinux relabeling and --userns=keep-id for proper permissions
        local mount_opts=":Z"
        if [ "$RUNTIME" = "docker" ]; then
            mount_opts=""
        fi
        $RUNTIME run --rm \
            -v "${PROJECT_DIR}:/usr/src${mount_opts}" \
            -w /usr/src \
            --network=host \
            --userns=keep-id \
            -e SONAR_HOST_URL="http://localhost:9000" \
            -e SONAR_TOKEN="$sonar_token" \
            docker.io/sonarsource/sonar-scanner-cli \
            -Dsonar.projectKey=helixagent \
            -Dsonar.projectName=HelixAgent \
            -Dsonar.projectVersion=1.0 \
            -Dsonar.sources=internal,cmd,Toolkit,LLMsVerifier \
            -Dsonar.sourceEncoding=UTF-8 \
            -Dsonar.qualitygate.wait=false
    fi

    echo -e "${GREEN}SonarQube analysis complete!${NC}"
    echo -e "${BLUE}View results at: http://localhost:9000/dashboard?id=helixagent${NC}"
}

# Function to run Go static analysis tools
run_go_analysis() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Running Go Static Analysis${NC}"
    echo -e "${BLUE}========================================${NC}"

    local report_file="${REPORTS_DIR}/go-analysis-${TIMESTAMP}.txt"

    cd "$PROJECT_DIR"

    echo -e "${YELLOW}Running go vet...${NC}"
    go vet ./... > "$report_file" 2>&1 || true

    echo -e "${YELLOW}Running staticcheck...${NC}"
    if command -v staticcheck &> /dev/null; then
        staticcheck ./... >> "$report_file" 2>&1 || true
    fi

    echo -e "${YELLOW}Running golangci-lint...${NC}"
    if command -v golangci-lint &> /dev/null; then
        golangci-lint run --out-format json > "${REPORTS_DIR}/golangci-lint-${TIMESTAMP}.json" 2>/dev/null || true
    fi

    echo -e "${GREEN}Go analysis report saved to: ${report_file}${NC}"
}

# Function to generate combined report
generate_combined_report() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Generating Combined Security Report${NC}"
    echo -e "${BLUE}========================================${NC}"

    local combined_report="${REPORTS_DIR}/security-summary-${TIMESTAMP}.md"

    cat > "$combined_report" << EOF
# Security Scan Report
**Date:** $(date '+%Y-%m-%d %H:%M:%S')
**Project:** HelixAgent

## Executive Summary

This report contains the results of comprehensive security and code quality scans.

EOF

    # Add Gosec results
    local gosec_file=$(ls -t "${REPORTS_DIR}"/gosec-*.json 2>/dev/null | head -1)
    if [ -f "$gosec_file" ]; then
        local gosec_issues=$(jq '.Issues | length' "$gosec_file" 2>/dev/null || echo "0")
        cat >> "$combined_report" << EOF
### Gosec (Go Security Checker)
- **Total Issues:** ${gosec_issues}
- **Report:** $(basename "$gosec_file")

EOF
    fi

    # Add Trivy results
    local trivy_file=$(ls -t "${REPORTS_DIR}"/trivy-*.json 2>/dev/null | head -1)
    if [ -f "$trivy_file" ]; then
        cat >> "$combined_report" << EOF
### Trivy (Vulnerability Scanner)
- **Report:** $(basename "$trivy_file")

EOF
    fi

    # Add Snyk results
    local snyk_file=$(ls -t "${REPORTS_DIR}"/snyk-*.json 2>/dev/null | head -1)
    if [ -f "$snyk_file" ]; then
        local snyk_vulns=$(jq '.vulnerabilities | length' "$snyk_file" 2>/dev/null || echo "0")
        cat >> "$combined_report" << EOF
### Snyk (Dependency Scanner)
- **Vulnerabilities:** ${snyk_vulns}
- **Report:** $(basename "$snyk_file")

EOF
    fi

    cat >> "$combined_report" << EOF
## Recommendations

1. Review all HIGH severity issues immediately
2. Address MEDIUM severity issues in the next sprint
3. Track LOW severity issues in backlog
4. Run scans regularly as part of CI/CD pipeline

## Reports Location
All detailed reports are available in: \`reports/security/\`
EOF

    echo -e "${GREEN}Combined report saved to: ${combined_report}${NC}"
}

# Main execution
main() {
    local scanner="${1:-all}"
    shift || true

    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}HelixAgent Security Scanner${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""

    cd "$PROJECT_DIR"

    case "$scanner" in
        gosec)
            run_gosec
            ;;
        trivy)
            run_trivy
            ;;
        snyk)
            run_snyk
            ;;
        sonarqube|sonar)
            run_sonarqube
            ;;
        go|goanalysis)
            run_go_analysis
            ;;
        all)
            run_gosec
            echo ""
            run_trivy
            echo ""
            run_go_analysis
            echo ""
            generate_combined_report
            ;;
        start-sonar)
            start_sonarqube
            ;;
        stop)
            echo -e "${YELLOW}Stopping security scanning services...${NC}"
            $COMPOSE_CMD -f "$COMPOSE_FILE" down
            ;;
        *)
            echo "Usage: $0 [scanner] [options]"
            echo ""
            echo "Scanners:"
            echo "  gosec       - Go security checker"
            echo "  trivy       - Vulnerability and secret scanner"
            echo "  snyk        - Snyk vulnerability scanner"
            echo "  sonarqube   - SonarQube code analysis"
            echo "  go          - Go static analysis (vet, staticcheck)"
            echo "  all         - Run all scanners (except SonarQube)"
            echo ""
            echo "Commands:"
            echo "  start-sonar - Start SonarQube server"
            echo "  stop        - Stop all security services"
            exit 1
            ;;
    esac

    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}Security scan complete!${NC}"
    echo -e "${GREEN}Reports saved to: ${REPORTS_DIR}${NC}"
    echo -e "${GREEN}========================================${NC}"
}

main "$@"
