#!/bin/bash
#
# setup_module_upstreams.sh - Set up Upstreams directory and install_upstreams script for all modules
#
# This script ensures each module has:
# 1. Upstreams/ directory with GitHub.sh, GitLab.sh, GitFlic.sh, GitVerse.sh
# 2. install_upstreams.sh script in module root
#
# Usage: ./setup_module_upstreams.sh [--dry-run]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATES_DIR="${SCRIPT_DIR}/templates/Upstreams"
INSTALL_SCRIPT="${SCRIPT_DIR}/install_upstreams.sh"

if [[ ! -d "${TEMPLATES_DIR}" ]]; then
    echo "Error: templates directory not found at ${TEMPLATES_DIR}" >&2
    exit 1
fi

if [[ ! -f "${INSTALL_SCRIPT}" ]]; then
    echo "Error: install_upstreams.sh not found at ${INSTALL_SCRIPT}" >&2
    exit 1
fi

DRY_RUN=false
if [[ "$1" == "--dry-run" ]]; then
    DRY_RUN=true
    echo "=== DRY RUN MODE ==="
fi

# List of 28 extracted modules
MODULES=(
    "EventBus"
    "Concurrency"
    "Observability"
    "Auth"
    "Storage"
    "Streaming"
    # Security removed — canonical at meta-repo root Security/ (P1.5-T03.03)
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
    "HelixMemory"
    "HelixSpecifier"
    "BuildCheck"
)

echo "Setting up upstreams for ${#MODULES[@]} modules..."
echo

for module in "${MODULES[@]}"; do
    echo "Processing module: $module"
    module_dir="${SCRIPT_DIR}/${module}"
    
    if [[ ! -d "$module_dir" ]]; then
        echo "  Warning: Module directory not found, skipping"
        continue
    fi
    
    # 1. Ensure Upstreams directory exists
    upstreams_dir="${module_dir}/Upstreams"
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "  [DRY RUN] Would create directory: $upstreams_dir"
    else
        mkdir -p "$upstreams_dir"
    fi
    
    # 2. Create/update the four upstream scripts
    for platform in GitHub GitLab GitFlic GitVerse; do
        template_file="${TEMPLATES_DIR}/${platform}.sh.template"
        output_file="${upstreams_dir}/${platform}.sh"
        
        if [[ ! -f "$template_file" ]]; then
            echo "  Error: Template not found: $template_file" >&2
            continue
        fi
        
        if [[ "$DRY_RUN" == "true" ]]; then
            echo "  [DRY RUN] Would create $output_file with module name $module"
            continue
        fi
        
        # Replace {MODULE_NAME} with actual module name
        sed "s/{MODULE_NAME}/$module/g" "$template_file" > "$output_file"
        chmod +x "$output_file"
        echo "  Created/updated: $platform.sh"
    done
    
    # 3. Copy install_upstreams.sh to module root (if not present)
    module_install_script="${module_dir}/install_upstreams.sh"
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "  [DRY RUN] Would copy install_upstreams.sh to $module_dir/"
    else
        cp "$INSTALL_SCRIPT" "$module_install_script"
        chmod +x "$module_install_script"
        echo "  Installed: install_upstreams.sh"
    fi
    
    echo
done

echo "=== Done ==="
echo "Next steps:"
echo "1. For each module, cd into directory and run ./install_upstreams.sh"
echo "2. This will add git remotes for all upstream repositories"
echo "3. Push to upstreams with: ./install_upstreams.sh --push"
echo ""
echo "Note: Repository URLs assume vasic-digital organization exists on each platform."
echo "      You may need to create repositories on GitLab, GitFlic, GitVerse."