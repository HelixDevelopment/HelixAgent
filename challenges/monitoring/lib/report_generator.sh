#!/bin/bash
# HelixAgent Challenges - Comprehensive Report Generator
# Generates detailed HTML and JSON reports from monitoring data

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/monitoring_lib.sh" 2>/dev/null || true

#===============================================================================
# REPORT CONFIGURATION
#===============================================================================

REPORT_VERSION="1.0.0"

#===============================================================================
# JSON REPORT GENERATION
#===============================================================================

generate_json_report() {
    local session_dir="$1"
    local output_file="$2"

    if [ ! -d "$session_dir" ]; then
        echo "Error: Session directory not found: $session_dir" >&2
        return 1
    fi

    local session_id=$(basename "$session_dir")
    local summary_file="$session_dir/session_summary.json"

    # Read session summary
    local start_time=""
    local end_time=""
    local duration=0
    local exit_code=0
    local total_issues=0
    local total_errors=0
    local total_warnings=0
    local total_fixes=0

    if [ -f "$summary_file" ]; then
        start_time=$(jq -r '.start_time // ""' "$summary_file" 2>/dev/null)
        end_time=$(jq -r '.end_time // ""' "$summary_file" 2>/dev/null)
        duration=$(jq -r '.duration_seconds // 0' "$summary_file" 2>/dev/null)
        exit_code=$(jq -r '.exit_code // 0' "$summary_file" 2>/dev/null)
        total_issues=$(jq -r '.issues.total // 0' "$summary_file" 2>/dev/null)
        total_errors=$(jq -r '.issues.errors // 0' "$summary_file" 2>/dev/null)
        total_warnings=$(jq -r '.issues.warnings // 0' "$summary_file" 2>/dev/null)
        total_fixes=$(jq -r '.issues.fixes_applied // 0' "$summary_file" 2>/dev/null)
    fi

    # Collect all errors
    local errors_json="[]"
    if [ -f "$session_dir/issues/errors.log" ]; then
        errors_json=$(cat "$session_dir/issues/errors.log" | while IFS= read -r line; do
            echo "$line" | sed 's/"/\\"/g'
        done | jq -Rs 'split("\n") | map(select(length > 0))')
    fi

    # Collect all warnings
    local warnings_json="[]"
    if [ -f "$session_dir/issues/warnings.log" ]; then
        warnings_json=$(cat "$session_dir/issues/warnings.log" | while IFS= read -r line; do
            echo "$line" | sed 's/"/\\"/g'
        done | jq -Rs 'split("\n") | map(select(length > 0))')
    fi

    # Collect all fixes
    local fixes_json="[]"
    if [ -f "$session_dir/issues/fixes.log" ]; then
        fixes_json=$(cat "$session_dir/issues/fixes.log" | while IFS= read -r line; do
            echo "$line" | sed 's/"/\\"/g'
        done | jq -Rs 'split("\n") | map(select(length > 0))')
    fi

    # Collect memory leak data
    local memory_leaks_json="{}"
    if [ -f "$session_dir/issues/memory_leaks.json" ]; then
        memory_leaks_json=$(cat "$session_dir/issues/memory_leaks.json")
    fi

    # Collect resource samples (last 100)
    local resource_samples="[]"
    if [ -f "$session_dir/resources/samples.jsonl" ]; then
        resource_samples=$(tail -100 "$session_dir/resources/samples.jsonl" | jq -s '.')
    fi

    # Collect component analysis
    local component_analysis="[]"
    for analysis_file in "$session_dir"/issues/analysis_*.json; do
        if [ -f "$analysis_file" ]; then
            local analysis=$(cat "$analysis_file")
            component_analysis=$(echo "$component_analysis" | jq --argjson item "$analysis" '. + [$item]')
        fi
    done

    # Generate challenge results
    local challenge_results="[]"
    local challenges_dir="$(dirname "$(dirname "$session_dir")")/results"
    if [ -d "$challenges_dir" ]; then
        for result_dir in "$challenges_dir"/*/; do
            if [ -d "$result_dir" ]; then
                local challenge_name=$(basename "$result_dir")
                local passed=$(ls "$result_dir"/*PASSED* 2>/dev/null | wc -l)
                local failed=$(ls "$result_dir"/*FAILED* 2>/dev/null | wc -l)
                if [ "$passed" -gt 0 ] || [ "$failed" -gt 0 ]; then
                    challenge_results=$(echo "$challenge_results" | jq --arg name "$challenge_name" --argjson passed "$passed" --argjson failed "$failed" '. + [{"name": $name, "passed": ($passed > 0), "pass_count": $passed, "fail_count": $failed}]')
                fi
            fi
        done
    fi

    # Generate final JSON report
    cat > "$output_file" << EOF
{
    "report_version": "$REPORT_VERSION",
    "generated_at": "$(date -Iseconds)",
    "session": {
        "id": "$session_id",
        "start_time": "$start_time",
        "end_time": "$end_time",
        "duration_seconds": $duration,
        "exit_code": $exit_code
    },
    "summary": {
        "total_issues": $total_issues,
        "errors": $total_errors,
        "warnings": $total_warnings,
        "fixes_applied": $total_fixes,
        "status": $([ "$exit_code" -eq 0 ] && [ "$total_errors" -eq 0 ] && echo '"PASS"' || echo '"FAIL"')
    },
    "issues": {
        "errors": $errors_json,
        "warnings": $warnings_json,
        "fixes": $fixes_json
    },
    "memory_analysis": $memory_leaks_json,
    "resource_samples": $resource_samples,
    "component_analysis": $component_analysis,
    "challenge_results": $challenge_results
}
EOF

    echo "JSON report generated: $output_file" >&2
}

#===============================================================================
# HTML REPORT GENERATION
#===============================================================================

generate_html_report() {
    local session_dir="$1"
    local output_file="$2"

    if [ ! -d "$session_dir" ]; then
        echo "Error: Session directory not found: $session_dir" >&2
        return 1
    fi

    local session_id=$(basename "$session_dir")
    local summary_file="$session_dir/session_summary.json"

    # Read session summary
    local start_time="N/A"
    local end_time="N/A"
    local duration=0
    local exit_code=0
    local total_issues=0
    local total_errors=0
    local total_warnings=0
    local total_fixes=0

    if [ -f "$summary_file" ]; then
        start_time=$(jq -r '.start_time // "N/A"' "$summary_file" 2>/dev/null)
        end_time=$(jq -r '.end_time // "N/A"' "$summary_file" 2>/dev/null)
        duration=$(jq -r '.duration_seconds // 0' "$summary_file" 2>/dev/null)
        exit_code=$(jq -r '.exit_code // 0' "$summary_file" 2>/dev/null)
        total_issues=$(jq -r '.issues.total // 0' "$summary_file" 2>/dev/null)
        total_errors=$(jq -r '.issues.errors // 0' "$summary_file" 2>/dev/null)
        total_warnings=$(jq -r '.issues.warnings // 0' "$summary_file" 2>/dev/null)
        total_fixes=$(jq -r '.issues.fixes_applied // 0' "$summary_file" 2>/dev/null)
    fi

    # Determine status color
    local status_class="success"
    local status_text="PASS"
    if [ "$exit_code" -ne 0 ] || [ "$total_errors" -gt 0 ]; then
        status_class="danger"
        status_text="FAIL"
    elif [ "$total_warnings" -gt 0 ]; then
        status_class="warning"
        status_text="PASS (with warnings)"
    fi

    # Generate errors table rows
    local errors_rows=""
    if [ -f "$session_dir/issues/errors.log" ]; then
        while IFS= read -r line; do
            local escaped_line=$(echo "$line" | sed 's/</\&lt;/g; s/>/\&gt;/g')
            errors_rows+="<tr><td class=\"error-row\">$escaped_line</td></tr>"
        done < "$session_dir/issues/errors.log"
    fi

    # Generate warnings table rows
    local warnings_rows=""
    if [ -f "$session_dir/issues/warnings.log" ]; then
        while IFS= read -r line; do
            local escaped_line=$(echo "$line" | sed 's/</\&lt;/g; s/>/\&gt;/g')
            warnings_rows+="<tr><td class=\"warning-row\">$escaped_line</td></tr>"
        done < "$session_dir/issues/warnings.log"
    fi

    # Generate fixes table rows
    local fixes_rows=""
    if [ -f "$session_dir/issues/fixes.log" ]; then
        while IFS= read -r line; do
            local escaped_line=$(echo "$line" | sed 's/</\&lt;/g; s/>/\&gt;/g')
            fixes_rows+="<tr><td class=\"fix-row\">$escaped_line</td></tr>"
        done < "$session_dir/issues/fixes.log"
    fi

    # Generate memory analysis section
    local memory_section=""
    if [ -f "$session_dir/issues/memory_leaks.json" ]; then
        local leaks_detected=$(jq -r '.leaks_detected // 0' "$session_dir/issues/memory_leaks.json" 2>/dev/null)
        if [ "$leaks_detected" -eq 1 ]; then
            memory_section="<div class=\"alert alert-danger\"><strong>Memory Leaks Detected!</strong></div>"
            memory_section+=$(jq -r '.details[] | "<div class=\"leak-item\">Process: \(.process), Baseline: \(.baseline_mb // "N/A")MB, Current: \(.current_mb)MB, Increase: \(.increase_mb // "N/A")MB</div>"' "$session_dir/issues/memory_leaks.json" 2>/dev/null)
        else
            memory_section="<div class=\"alert alert-success\">No memory leaks detected</div>"
        fi
    fi

    # Generate HTML
    cat > "$output_file" << 'HTMLHEADER'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>HelixAgent Challenge Monitoring Report</title>
    <style>
        :root {
            --bg-color: #1a1a2e;
            --card-bg: #16213e;
            --text-color: #eee;
            --accent-color: #0f3460;
            --success-color: #00b894;
            --warning-color: #fdcb6e;
            --danger-color: #e74c3c;
            --info-color: #74b9ff;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: 'Segoe UI', system-ui, sans-serif;
            background: var(--bg-color);
            color: var(--text-color);
            line-height: 1.6;
            padding: 20px;
        }
        .container { max-width: 1400px; margin: 0 auto; }
        h1, h2, h3 { margin-bottom: 15px; }
        h1 { text-align: center; color: var(--info-color); font-size: 2.5em; margin-bottom: 30px; }
        .card {
            background: var(--card-bg);
            border-radius: 12px;
            padding: 20px;
            margin-bottom: 20px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.3);
        }
        .card h2 { color: var(--info-color); border-bottom: 2px solid var(--accent-color); padding-bottom: 10px; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; }
        .stat-box {
            background: var(--accent-color);
            border-radius: 8px;
            padding: 20px;
            text-align: center;
        }
        .stat-box .value { font-size: 2.5em; font-weight: bold; }
        .stat-box .label { font-size: 0.9em; opacity: 0.8; }
        .stat-box.success .value { color: var(--success-color); }
        .stat-box.warning .value { color: var(--warning-color); }
        .stat-box.danger .value { color: var(--danger-color); }
        .stat-box.info .value { color: var(--info-color); }
        .status-badge {
            display: inline-block;
            padding: 8px 20px;
            border-radius: 20px;
            font-weight: bold;
            font-size: 1.2em;
        }
        .status-badge.success { background: var(--success-color); color: #000; }
        .status-badge.warning { background: var(--warning-color); color: #000; }
        .status-badge.danger { background: var(--danger-color); color: #fff; }
        table { width: 100%; border-collapse: collapse; margin-top: 15px; }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid var(--accent-color);
        }
        th { background: var(--accent-color); color: var(--info-color); }
        .error-row { color: var(--danger-color); }
        .warning-row { color: var(--warning-color); }
        .fix-row { color: var(--success-color); }
        .alert {
            padding: 15px;
            border-radius: 8px;
            margin: 15px 0;
        }
        .alert-success { background: rgba(0, 184, 148, 0.2); border-left: 4px solid var(--success-color); }
        .alert-warning { background: rgba(253, 203, 110, 0.2); border-left: 4px solid var(--warning-color); }
        .alert-danger { background: rgba(231, 76, 60, 0.2); border-left: 4px solid var(--danger-color); }
        .leak-item { padding: 10px; margin: 5px 0; background: var(--accent-color); border-radius: 4px; }
        code { background: var(--accent-color); padding: 2px 6px; border-radius: 4px; font-family: monospace; }
        .meta { font-size: 0.85em; opacity: 0.7; }
        .section-header { display: flex; justify-content: space-between; align-items: center; }
        .timestamp { color: var(--info-color); }
        footer { text-align: center; margin-top: 40px; opacity: 0.6; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔬 HelixAgent Challenge Monitoring Report</h1>
HTMLHEADER

    cat >> "$output_file" << EOF
        <div class="card">
            <div class="section-header">
                <h2>📊 Session Overview</h2>
                <span class="status-badge $status_class">$status_text</span>
            </div>
            <div class="grid" style="margin-top: 20px;">
                <div class="stat-box info">
                    <div class="value">$session_id</div>
                    <div class="label">Session ID</div>
                </div>
                <div class="stat-box info">
                    <div class="value">${duration}s</div>
                    <div class="label">Duration</div>
                </div>
                <div class="stat-box info">
                    <div class="value">$exit_code</div>
                    <div class="label">Exit Code</div>
                </div>
            </div>
            <p class="meta" style="margin-top: 15px;">
                <strong>Start:</strong> <span class="timestamp">$start_time</span> |
                <strong>End:</strong> <span class="timestamp">$end_time</span>
            </p>
        </div>

        <div class="card">
            <h2>📈 Issue Summary</h2>
            <div class="grid">
                <div class="stat-box danger">
                    <div class="value">$total_errors</div>
                    <div class="label">Errors</div>
                </div>
                <div class="stat-box warning">
                    <div class="value">$total_warnings</div>
                    <div class="label">Warnings</div>
                </div>
                <div class="stat-box success">
                    <div class="value">$total_fixes</div>
                    <div class="label">Fixes Applied</div>
                </div>
                <div class="stat-box info">
                    <div class="value">$total_issues</div>
                    <div class="label">Total Issues</div>
                </div>
            </div>
        </div>

        <div class="card">
            <h2>🧠 Memory Analysis</h2>
            $memory_section
        </div>
EOF

    # Errors section
    if [ -n "$errors_rows" ]; then
        cat >> "$output_file" << EOF
        <div class="card">
            <h2>❌ Errors ($total_errors)</h2>
            <table>
                <thead><tr><th>Error Details</th></tr></thead>
                <tbody>$errors_rows</tbody>
            </table>
        </div>
EOF
    fi

    # Warnings section
    if [ -n "$warnings_rows" ]; then
        cat >> "$output_file" << EOF
        <div class="card">
            <h2>⚠️ Warnings ($total_warnings)</h2>
            <table>
                <thead><tr><th>Warning Details</th></tr></thead>
                <tbody>$warnings_rows</tbody>
            </table>
        </div>
EOF
    fi

    # Fixes section
    if [ -n "$fixes_rows" ]; then
        cat >> "$output_file" << EOF
        <div class="card">
            <h2>✅ Fixes Applied ($total_fixes)</h2>
            <table>
                <thead><tr><th>Fix Details</th></tr></thead>
                <tbody>$fixes_rows</tbody>
            </table>
        </div>
EOF
    fi

    # Footer
    cat >> "$output_file" << 'HTMLFOOTER'
        <footer>
            <p>Generated by HelixAgent Challenge Monitoring System v1.0.0</p>
            <p>Report generated at: <span id="gen-time"></span></p>
            <script>document.getElementById('gen-time').textContent = new Date().toISOString();</script>
        </footer>
    </div>
</body>
</html>
HTMLFOOTER

    echo "HTML report generated: $output_file" >&2
}

#===============================================================================
# COMPREHENSIVE REPORT (ALL FORMATS)
#===============================================================================

generate_comprehensive_report() {
    local session_dir="$1"
    local report_dir="${2:-$(dirname "$session_dir")/reports/$(basename "$session_dir")}"

    mkdir -p "$report_dir"

    echo "Generating comprehensive report..."

    # Generate JSON report
    generate_json_report "$session_dir" "$report_dir/report.json"

    # Generate HTML report
    generate_html_report "$session_dir" "$report_dir/report.html"

    # Copy raw logs
    cp -r "$session_dir"/* "$report_dir/" 2>/dev/null || true

    # Generate summary text file
    cat > "$report_dir/SUMMARY.txt" << EOF
===============================================================================
                    HELIXAGENT CHALLENGE MONITORING REPORT
===============================================================================

Session ID: $(basename "$session_dir")
Generated:  $(date)

SUMMARY
-------
EOF

    if [ -f "$session_dir/session_summary.json" ]; then
        jq -r '
            "Status:       \(if .exit_code == 0 and .issues.errors == 0 then "PASS" else "FAIL" end)",
            "Duration:     \(.duration_seconds)s",
            "Exit Code:    \(.exit_code)",
            "",
            "ISSUES",
            "------",
            "Errors:       \(.issues.errors)",
            "Warnings:     \(.issues.warnings)",
            "Fixes:        \(.issues.fixes_applied)",
            "Total:        \(.issues.total)"
        ' "$session_dir/session_summary.json" >> "$report_dir/SUMMARY.txt"
    fi

    echo ""
    echo "FILES"
    echo "-----" >> "$report_dir/SUMMARY.txt"
    echo "- report.html (Interactive HTML report)" >> "$report_dir/SUMMARY.txt"
    echo "- report.json (Machine-readable JSON)" >> "$report_dir/SUMMARY.txt"
    echo "- session_summary.json (Session metadata)" >> "$report_dir/SUMMARY.txt"
    echo "- master.log (Complete monitoring log)" >> "$report_dir/SUMMARY.txt"
    echo "" >> "$report_dir/SUMMARY.txt"
    echo "===============================================================================" >> "$report_dir/SUMMARY.txt"

    echo ""
    echo "Comprehensive report generated in: $report_dir"
    echo "  - report.html (Interactive HTML report)"
    echo "  - report.json (Machine-readable JSON)"
    echo "  - SUMMARY.txt (Quick summary)"
}

#===============================================================================
# MAIN ENTRY POINT
#===============================================================================

if [ "${BASH_SOURCE[0]}" == "${0}" ]; then
    if [ $# -lt 1 ]; then
        echo "Usage: $0 <session_dir> [output_dir]"
        echo ""
        echo "Generates comprehensive monitoring reports from session data."
        echo ""
        echo "Arguments:"
        echo "  session_dir  Path to the monitoring session directory"
        echo "  output_dir   Optional: Output directory for reports (default: session_dir/../reports)"
        exit 1
    fi

    generate_comprehensive_report "$1" "$2"
fi
