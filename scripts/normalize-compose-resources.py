#!/usr/bin/env python3
"""normalize-compose-resources.py — apply the canonical Compose v2/v3 resource
form to every service in docker-compose.yml using surgical text edits that
preserve every comment, blank line, and unrelated key exactly.

The legacy `mem_limit` / `memswap_limit` / `pids_limit` keys are stripped
(they conflict with `deploy.resources.limits.*` under Docker Compose v2 —
exactly the failure that broke amber.local). Each service's resource block
is rewritten as `${SERVICE_FIELD:-default}` env-var interpolations so the
same compose file scales across dev / staging / production by overriding
env vars (no YAML edits required).

Tier rationale and per-environment scaling profile live in
docs/development/container-resource-policy.md.

Run:
  python3 scripts/normalize-compose-resources.py [--dry-run]
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path
from typing import NamedTuple


class Tier(NamedTuple):
    mem_lim: str
    mem_res: str
    cpu_lim: str
    cpu_res: str
    pids_lim: int


# Tier sizing — keys must cover every service the orchestrator boots.
TIERS: dict[str, Tier] = {
    # --- Tiny: caches, sidecars, dashboards ---
    "redis":             Tier("1G", "256M", "0.50", "0.10", 1024),
    "grafana":           Tier("1G", "256M", "0.50", "0.10", 1024),
    "mock-llm":          Tier("1G", "256M", "0.50", "0.10", 1024),

    # --- Small: services that load light frameworks ---
    "prometheus":        Tier("2G", "512M", "1.00", "0.25", 1024),
    "langchain-server":  Tier("2G", "512M", "1.00", "0.25", 1024),
    "llamaindex-server": Tier("2G", "512M", "1.00", "0.25", 1024),
    "guidance-server":   Tier("2G", "512M", "1.00", "0.25", 1024),
    "lmql-server":       Tier("2G", "512M", "1.00", "0.25", 1024),

    # --- Medium: databases + main gateway + LLM-heavy services ---
    "postgres":          Tier("4G", "1G",   "2.00", "0.50", 1024),
    "chromadb":          Tier("4G", "1G",   "2.00", "0.50", 1024),
    "memgraph":          Tier("4G", "1G",   "2.00", "0.50", 1024),
    "neo4j":             Tier("4G", "1G",   "2.00", "0.50", 2048),
    "cognee":            Tier("4G", "1G",   "2.00", "0.50", 1024),
    "helixagent":        Tier("4G", "1G",   "2.00", "0.50", 2048),
    "sglang":            Tier("4G", "1G",   "2.00", "0.50", 1024),

    # --- XL: local LLM serving with large weights in RAM ---
    "ollama":            Tier("12G", "4G",  "4.00", "1.00", 2048),
}


def env_key(service: str) -> str:
    """`langchain-server` -> `LANGCHAIN_SERVER`."""
    return service.upper().replace("-", "_")


def render_resources_block(service: str, tier: Tier, indent: str = "    ") -> str:
    """Render the canonical deploy.resources block for a service."""
    k = env_key(service)
    return (
        f"{indent}deploy:\n"
        f"{indent}  resources:\n"
        f"{indent}    limits:\n"
        f"{indent}      memory: ${{{k}_MEM_LIMIT:-{tier.mem_lim}}}\n"
        f"{indent}      cpus: \"${{{k}_CPU_LIMIT:-{tier.cpu_lim}}}\"\n"
        f"{indent}      pids: {tier.pids_lim}\n"
        f"{indent}    reservations:\n"
        f"{indent}      memory: ${{{k}_MEM_RESERVE:-{tier.mem_res}}}\n"
        f"{indent}      cpus: \"${{{k}_CPU_RESERVE:-{tier.cpu_res}}}\"\n"
    )


SERVICE_HEADER = re.compile(r"^  ([a-zA-Z][a-zA-Z0-9_-]*):\s*$")
TOP_LEVEL_HEADER = re.compile(r"^[a-zA-Z]")


def find_service_ranges(lines: list[str]) -> list[tuple[str, int, int]]:
    """Return [(name, start_line_idx, end_line_idx_exclusive)] for each service.
    Service body excludes the header line."""
    out: list[tuple[str, int, int]] = []
    in_services = False
    cur_name: str | None = None
    cur_start = -1
    for i, line in enumerate(lines):
        if not in_services:
            if line.rstrip() == "services:":
                in_services = True
            continue
        if TOP_LEVEL_HEADER.match(line) and line.rstrip().endswith(":"):
            # Left services: section
            if cur_name is not None:
                out.append((cur_name, cur_start, i))
                cur_name = None
            break
        m = SERVICE_HEADER.match(line)
        if m:
            if cur_name is not None:
                out.append((cur_name, cur_start, i))
            cur_name = m.group(1)
            cur_start = i + 1  # body starts after header
    if cur_name is not None:
        out.append((cur_name, cur_start, len(lines)))
    return out


# Lines we always strip (legacy resource keys at 4-space indent).
LEGACY_RESOURCE_LINE = re.compile(r"^    (mem_limit|memswap_limit|pids_limit)\s*:.*\n?$")

# Block-detection regex for an existing deploy: block at 4-space indent.
DEPLOY_HEADER = re.compile(r"^    deploy\s*:\s*$")


def strip_existing_deploy_resources(body: list[str]) -> tuple[list[str], list[str]]:
    """Remove the deploy.resources sub-tree from `body`, preserving any other
    keys under deploy: (e.g. devices reservation for GPU services if it lives
    under non-resources keys). Returns (cleaned_body, removed_lines).

    For sglang specifically, the GPU device entry lives under
    deploy.resources.reservations.devices — we want to KEEP that. So we surgical-
    extract the `devices:` sub-list from reservations: before deletion, then
    the caller can re-insert it into the new resources block.

    Strategy: remove ONLY the `resources:` sub-block, preserve everything else
    under deploy:. If after removal `deploy:` has no children, remove it too.
    """
    out: list[str] = []
    removed: list[str] = []
    i = 0
    while i < len(body):
        line = body[i]
        m = DEPLOY_HEADER.match(line)
        if not m:
            out.append(line)
            i += 1
            continue
        # Found deploy: at 4-space indent. Walk children at >=6-space.
        deploy_start = i
        i += 1
        deploy_children: list[str] = []
        while i < len(body):
            nxt = body[i]
            if nxt.strip() == "":
                deploy_children.append(nxt)
                i += 1
                continue
            # Children are indented >= 6 spaces. Anything less ends deploy.
            if not nxt.startswith("      ") and nxt.strip() != "":
                break
            deploy_children.append(nxt)
            i += 1

        # Within deploy_children, find `      resources:` and drop that subtree
        kept: list[str] = []
        j = 0
        while j < len(deploy_children):
            child = deploy_children[j]
            if re.match(r"^      resources\s*:\s*$", child):
                # Skip this line + everything indented >= 8 until we hit a
                # less-indented line.
                removed.append(child)
                j += 1
                while j < len(deploy_children):
                    sub = deploy_children[j]
                    if sub.strip() == "" or sub.startswith("        "):
                        removed.append(sub)
                        j += 1
                    else:
                        break
                continue
            kept.append(child)
            j += 1

        # If anything non-blank remains under deploy:, keep deploy: block.
        non_blank = [k for k in kept if k.strip()]
        if non_blank:
            out.append(line)  # the deploy: header
            out.extend(kept)
        else:
            removed.append(line)
            removed.extend(kept)
    return out, removed


def extract_gpu_devices(removed: list[str]) -> list[str] | None:
    """Pull the `devices:` sub-list from a removed reservations block, if any.
    Indents in the source `removed` block:
      6sp `resources:` / 8sp `reservations:` / 10sp `devices:` / 12sp+ items.
    The new block has the same `reservations:` indent level, so we can copy the
    devices subtree verbatim. Returns lines starting with `          devices:`
    (10sp) and its children at >=12sp, or None if not present."""
    for i, line in enumerate(removed):
        if re.match(r"^          devices\s*:\s*$", line):
            devs: list[str] = [line]
            j = i + 1
            while j < len(removed):
                nxt = removed[j]
                if nxt.strip() == "":
                    j += 1
                    continue
                if nxt.startswith("            "):  # 12+ spaces
                    devs.append(nxt)
                    j += 1
                else:
                    break
            return devs
    return None


def inject_resources_block(
    body: list[str],
    service: str,
    tier: Tier,
    gpu_devices: list[str] | None,
) -> list[str]:
    """Insert the canonical deploy.resources block at a safe position inside
    the service body. Strategy: insert just before the first occurrence of
    `    restart:` (consistent across all services). If no `restart:` line,
    insert at the end of the service body."""
    # Build the new resources block.
    block = render_resources_block(service, tier).splitlines(keepends=True)
    if gpu_devices:
        # Append GPU devices under reservations: by replacing the trailing
        # `cpus: ...` reservations line — actually GPU devices belong as a
        # *sibling* of memory/cpus under reservations:. Easiest: append at end
        # of reservations: block.
        block.extend(gpu_devices)

    # Find insertion point: before the first `^    restart:` line.
    insert_at = None
    for i, line in enumerate(body):
        if re.match(r"^    restart\s*:", line):
            insert_at = i
            break
    if insert_at is None:
        # Fall back: insert before profiles: if present, else at end.
        for i, line in enumerate(body):
            if re.match(r"^    profiles\s*:", line):
                insert_at = i
                break
    if insert_at is None:
        insert_at = len(body)

    return body[:insert_at] + block + body[insert_at:]


def normalize(text: str) -> tuple[str, list[str]]:
    lines = text.splitlines(keepends=True)
    services = find_service_ranges(lines)

    out_lines: list[str] = []
    cursor = 0
    changes: list[str] = []

    for name, start, end in services:
        # Pre-service text (header line and above)
        out_lines.extend(lines[cursor:start])
        cursor = end

        body = lines[start:end]

        if name not in TIERS:
            out_lines.extend(body)
            continue

        # Step 1: strip legacy resource lines.
        before_legacy_count = sum(
            1 for l in body if LEGACY_RESOURCE_LINE.match(l)
        )
        body = [l for l in body if not LEGACY_RESOURCE_LINE.match(l)]
        if before_legacy_count:
            changes.append(
                f"{name}: stripped {before_legacy_count} legacy resource line(s)"
            )

        # Step 2: strip existing deploy.resources sub-tree (preserve any other
        # deploy keys, capture GPU devices for re-injection).
        body, removed = strip_existing_deploy_resources(body)
        if removed:
            changes.append(
                f"{name}: removed existing deploy.resources block ({len(removed)} lines)"
            )

        gpu_devices = extract_gpu_devices(removed)
        if gpu_devices:
            changes.append(f"{name}: preserved GPU devices reservation")

        # Step 3: inject canonical resources block.
        body = inject_resources_block(body, name, TIERS[name], gpu_devices)
        changes.append(f"{name}: inserted canonical deploy.resources block")

        out_lines.extend(body)

    # Tail (volumes:, networks:, etc.)
    out_lines.extend(lines[cursor:])

    return "".join(out_lines), changes


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--file", type=Path, default=Path("docker-compose.yml"))
    p.add_argument("--dry-run", action="store_true")
    args = p.parse_args()

    if not args.file.is_file():
        print(f"FAIL: {args.file} not found", file=sys.stderr)
        return 2

    src = args.file.read_text()
    out, changes = normalize(src)

    print(f"=== {len(changes)} change(s) ===")
    for c in changes:
        print(f"  - {c}")

    if out == src:
        print("\n(no changes — file already at canonical form)")
        return 0

    if args.dry_run:
        print(f"\n--- DRY RUN: file not written ---")
        return 0

    args.file.write_text(out)
    print(f"\nWrote {args.file}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
