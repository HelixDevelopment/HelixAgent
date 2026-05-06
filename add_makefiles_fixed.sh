#!/bin/bash
#
# add_makefiles_fixed.sh - Add Makefile to modules missing one (robust version)
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE_FILE="${SCRIPT_DIR}/templates/Makefile"

if [[ ! -f "$TEMPLATE_FILE" ]]; then
    echo "Error: Makefile template not found at $TEMPLATE_FILE" >&2
    exit 1
fi

# List of modules (excluding HelixMemory and HelixSpecifier)
MODULES=(
    "EventBus"
    "Concurrency"
    "Observability"
    "Auth"
    "Storage"
    "Streaming"
    "Security"
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
    # Containers removed — canonical at meta-repo root Containers/ (P1.5-T03.02)
    "Challenges"
    "Agentic"
    "LLMOps"
    "SelfImprove"
    "Planning"
    "Benchmark"
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
        first_line=$(grep -v '^#' "$readme_file" | grep -v '^$' | head -1)
        if [[ -n "$first_line" ]]; then
            # Trim leading/trailing whitespace
            first_line=$(echo "$first_line" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
            description="$first_line"
        fi
    fi
    
    # Convert module name to lowercase for module path
    module_lower=$(echo "$module" | tr '[:upper:]' '[:lower:]')
    
    # Use awk to replace placeholders (awk handles special characters better)
    awk -v module="$module" \
        -v description="$description" \
        -v module_lower="$module_lower" \
        '
        { 
            gsub("{{MODULE_NAME}}", module)
            gsub("{{MODULE_DESCRIPTION}}", description)
            gsub("{{MODULE_LOWER}}", module_lower)
            print
        }
        ' "$TEMPLATE_FILE" > "$makefile_path"
    
    echo "  Created Makefile with description: $description"
done

echo
echo "=== Done ==="