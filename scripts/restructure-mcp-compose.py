#!/usr/bin/env python3
"""restructure-mcp-compose.py — convert every service in
docker/mcp/docker-compose.mcp-servers.yml from project-root build contexts
(`context: ../..`, `dockerfile: docker/mcp/Dockerfile.<type>`) to focused
per-service sub-contexts that the orchestrator can ship to remote workers.

The orchestrator's main adapter (internal/adapters/containers/adapter.go)
intentionally SKIPS project-root build contexts to avoid scp'ing the 27 GB
project root to every remote host. That breaks MCP servers on remote
distribution. Per-service sub-contexts (the specific mcp_servers/ folder or
MCP/submodules/<name>/) are small and ship correctly.

This script makes the rewrite text-surgically (preserves comments / blank
lines) and is idempotent — re-running on already-restructured services
is a no-op. Pair-edit `docker/mcp/Dockerfile.mcp-server`,
`Dockerfile.mcp-submodule`, `Dockerfile.mcp-python`, `Dockerfile.mcp-go`,
and `Dockerfile.mcp-playwright` to use `COPY . .` instead of `COPY
${SOURCE_DIR} .` since the context IS the source dir now.

Run:
  python3 scripts/restructure-mcp-compose.py [--dry-run]
"""
from __future__ import annotations

import argparse
import os
import re
import sys
from pathlib import Path


SERVICE_HEADER = re.compile(r"^  ([a-zA-Z][a-zA-Z0-9_-]*):\s*$")
BUILD_HEADER   = re.compile(r"^    build:\s*$")
CONTEXT_LINE   = re.compile(r"^      context:\s*(\S+.*?)\s*$")
DOCKERFILE_LN  = re.compile(r"^      dockerfile:\s*(\S+.*?)\s*$")
ARGS_HEADER    = re.compile(r"^      args:\s*$")
SRC_DIR_LINE   = re.compile(r"^        SOURCE_DIR:\s*(\S+.*?)\s*$")


def find_service_blocks(lines: list[str]):
    """Yield (name, start_idx, end_idx) for every service block."""
    in_services = False
    cur_name = None
    cur_start = -1
    blocks = []
    for i, line in enumerate(lines):
        stripped = line.rstrip("\n")
        if not in_services:
            if stripped == "services:":
                in_services = True
            continue
        if line[:1].strip() and line[:1] != " ":
            if cur_name is not None:
                blocks.append((cur_name, cur_start, i))
                cur_name = None
            break
        m = SERVICE_HEADER.match(line)
        if m:
            if cur_name is not None:
                blocks.append((cur_name, cur_start, i))
            cur_name = m.group(1)
            cur_start = i
    if cur_name is not None:
        blocks.append((cur_name, cur_start, len(lines)))
    return blocks


def rewrite_service(lines: list[str], start: int, end: int) -> tuple[list[str], list[str]]:
    """Rewrite the service block in-place. Returns (new_block, change_log)."""
    block = lines[start:end]
    changes: list[str] = []

    # Locate build:, context:, dockerfile:, args.SOURCE_DIR.
    build_idx = None
    ctx_idx   = None
    df_idx    = None
    src_idx   = None
    src_value = None
    ctx_value = None

    for i, line in enumerate(block):
        if BUILD_HEADER.match(line):
            build_idx = i
        m = CONTEXT_LINE.match(line)
        if m:
            ctx_idx = i
            ctx_value = m.group(1).strip()
        m = DOCKERFILE_LN.match(line)
        if m:
            df_idx = i
        m = SRC_DIR_LINE.match(line)
        if m:
            src_idx = i
            src_value = m.group(1).strip()

    if build_idx is None or ctx_idx is None or df_idx is None:
        return block, changes  # not a build-with-context-and-dockerfile service

    # Skip if already restructured (context is not project root).
    if ctx_value not in ("../..",):
        return block, changes

    if not src_value:
        return block, changes  # cannot determine source dir

    # New context: ../../<SOURCE_DIR>  (compose lives at docker/mcp/, sub-
    # context at <project>/<SOURCE_DIR>/ → ../../<SOURCE_DIR>)
    new_ctx = f"../../{src_value}"

    # New dockerfile: relative to the new context. Compose file lives at
    # docker/mcp/, dockerfile lives at docker/mcp/Dockerfile.<type>. From
    # context <project>/<SOURCE_DIR>/, the dockerfile path back to
    # docker/mcp/Dockerfile.<type> is N + 'docker/mcp/Dockerfile.<type>'
    # where N = number of "../" needed to climb from <SOURCE_DIR> back to
    # the project root.
    depth = len(src_value.strip("/").split("/"))  # MCP-Servers=1, MCP/submodules/X=3
    df_match = DOCKERFILE_LN.match(block[df_idx])
    df_path = df_match.group(1).strip()  # original 'docker/mcp/Dockerfile.<type>'
    new_df = ("../" * depth) + df_path

    # Apply the rewrite.
    new_block = list(block)
    new_block[ctx_idx] = re.sub(
        r"context:.*", f"context: {new_ctx}", block[ctx_idx]
    )
    new_block[df_idx] = re.sub(
        r"dockerfile:.*", f"dockerfile: {new_df}", block[df_idx]
    )

    # Drop the now-redundant SOURCE_DIR arg (the Dockerfile uses COPY . .
    # so it doesn't need SOURCE_DIR anymore). Leaving it would be harmless
    # but stale; removing it keeps the file self-documenting. If args:
    # becomes empty (no SERVER_NAME or other arg remains), drop the
    # `args:` header too — an empty mapping `args:` followed by a
    # less-indented sibling key is invalid YAML / Compose v2 rejects.
    if src_idx is not None:
        new_block.pop(src_idx)
        # Re-locate args: header in the mutated block.
        args_hdr_idx = None
        for j, ln in enumerate(new_block):
            if ARGS_HEADER.match(ln):
                args_hdr_idx = j
                break
        if args_hdr_idx is not None:
            # Look ahead: are there any 8-space-indented children left?
            has_remaining = False
            for ln in new_block[args_hdr_idx + 1:]:
                if not ln.strip():
                    continue
                # children are indented >= 8 spaces; siblings/parents
                # less-indented end the block.
                if ln.startswith("        "):
                    has_remaining = True
                    break
                break
            if not has_remaining:
                new_block.pop(args_hdr_idx)
                changes.append("dropped empty args: block")

    changes.append(f"context: '../..' -> {new_ctx!r}")
    changes.append(f"dockerfile: -> {new_df!r}")
    if src_idx is not None:
        changes.append(f"removed args.SOURCE_DIR={src_value!r}")
    return new_block, changes


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--file", type=Path,
                   default=Path("docker/mcp/docker-compose.mcp-servers.yml"))
    p.add_argument("--dry-run", action="store_true")
    args = p.parse_args()

    if not args.file.is_file():
        print(f"FAIL: {args.file} not found", file=sys.stderr)
        return 2

    lines = args.file.read_text().splitlines(keepends=True)
    blocks = find_service_blocks(lines)
    print(f"=== {len(blocks)} service blocks ===")

    out: list[str] = []
    cursor = 0
    total_changes = 0
    for name, start, end in blocks:
        out.extend(lines[cursor:start])
        new_block, changes = rewrite_service(lines, start, end)
        out.extend(new_block)
        cursor = end
        if changes:
            print(f"  {name}:")
            for c in changes:
                print(f"    - {c}")
            total_changes += len(changes)
    out.extend(lines[cursor:])

    if total_changes == 0:
        print("\n(no changes — file already restructured)")
        return 0

    if args.dry_run:
        print(f"\n--- DRY RUN: {total_changes} change(s); file not written ---")
        return 0

    args.file.write_text("".join(out))
    print(f"\nWrote {args.file} ({total_changes} change(s))")
    return 0


if __name__ == "__main__":
    sys.exit(main())
