#!/bin/bash
#
# add_makefiles.sh - Add Makefile to modules missing one
#
# This script adds a standard Makefile to each module that doesn't already have one.
# Excludes HelixMemory and HelixSpecifier which have custom Makefiles.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE_FILE="${SCRIPT_DIR}/templates/Makefile"

if [[ ! -f "$TEMPLATE_FILE" ]]; then
    echo "Error: Makefile template not found at $TEMPLATE_FILE" >&2
    exit 1
fi

# List of 28 extracted modules
MODULES=(
    "EventBus"
    "Concurrency"
    "Observability"
    "Auth"
    "Storage"
    "Streaming"
    # Security removed — canonical at meta-repo root security/ (P1.5-T03.03)
    "VectorDB"
    "Embeddings"
    "Database"
    "Cache"
    "Messaging"
    "Formatters"
    "MCP_Module"
    "RAG"
    "Memory"
    "Optimization"
    "Plugins"
    # Containers removed — canonical at meta-repo root containers/ (P1.5-T03.02)
    "Challenges"
    "Agentic"
    "LLMOps"
    "SelfImprove"
    "Planning"
    "Benchmark"
    "HelixMemory"
    "HelixSpecifier"
    "BuildCheck"
)

echo "Adding Makefiles to modules missing one..."
echo

for module in "${MODULES[@]}"; do
    module_dir="${SCRIPT_DIR}/${module}"
    makefile_path="${module_dir}/Makefile"
    
    if [[ ! -d "$module_dir" ]]; then
        echo "Skipping $module: directory not found"
        continue
    fi
    
    # Skip HelixMemory and HelixSpecifier (they have custom Makefiles)
    if [[ "$module" == "HelixMemory" || "$module" == "HelixSpecifier" ]]; then
        if [[ -f "$makefile_path" ]]; then
            echo "Skipping $module: already has custom Makefile"
        else
            echo "WARNING: $module should have Makefile but doesn't"
        fi
        continue
    fi
    
    if [[ -f "$makefile_path" ]]; then
        echo "Skipping $module: Makefile already exists"
        continue
    fi
    
    echo "Adding Makefile to $module..."
    
    # Determine module description from README first line (skip heading)
    description="Generic, reusable Go module"
    readme_file="${module_dir}/README.md"
    if [[ -f "$readme_file" ]]; then
        # Extract first non-empty line that doesn't start with '#'
        first_line=$(grep -v '^#' "$readme_file" | grep -v '^$' | head -1 | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
        if [[ -n "$first_line" ]]; then
            description="$first_line"
        fi
    fi
    
    # Convert module name to lowercase for module path
    module_lower=$(echo "$module" | tr '[:upper:]' '[:lower:]')
    
    # Replace placeholders in template
    sed -e "s/{{MODULE_NAME}}/$module/g" \
        -e "s/{{MODULE_DESCRIPTION}}/$description/g" \
        -e "s/{{MODULE_LOWER}}/$module_lower/g" \
        "$TEMPLATE_FILE" > "$makefile_path"
    
    echo "  Created Makefile with description: $description"
done

echo
echo "=== Done ==="