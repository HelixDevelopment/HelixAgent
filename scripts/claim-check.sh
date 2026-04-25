#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# claim-check.sh — Stop hook that reminds about evidence-based DoD.
#
# Reads the session's transcript, scans the most recent assistant message for
# unverified claims of "done / verified / passing / working / complete / fixed
# / validated / confirmed" without an adjacent fenced output block, and emits a
# systemMessage advisory when the pattern matches.
#
# Wired via .claude/settings.json as a Stop hook in this project. Advisory
# only — never blocks termination. A broken hook must never hang sessions.
#
# Receives on stdin (Stop hook payload):
#   { "session_id": "...", "tool_name": "...", ... }
#
# Returns on stdout:
#   {} for no advisory, OR
#   {"systemMessage": "DoD reminder: ..."} when an unverified claim is detected.
#
# Always exits 0.

# Defensive: any failure in this script is silent. The hook must never
# interfere with normal session termination. If the transcript can't be
# found, the patterns can't be parsed, etc., we just exit silently.
{
    payload=$(cat) || exit 0

    session_id=$(printf '%s' "$payload" | jq -r '.session_id // empty' 2>/dev/null) || exit 0
    [ -z "$session_id" ] && exit 0

    # Locate the transcript file. Claude Code stores per-project session
    # transcripts under <claude-base>/projects/<sanitized-cwd>/<session>.jsonl.
    # The base directory varies by install (~/.claude or an aliased equivalent).
    # We probe both common locations and stop at the first hit.
    sanitized=$(printf '%s' "$PWD" | sed 's|/|-|g')
    transcript=""
    for base in "$HOME/.claude" "$HOME/.claude-milos85vasic2nd" "${CLAUDE_CONFIG_DIR:-}"; do
        [ -z "$base" ] && continue
        candidate="$base/projects/$sanitized/$session_id.jsonl"
        if [ -f "$candidate" ]; then
            transcript="$candidate"
            break
        fi
    done
    [ -z "$transcript" ] && exit 0

    # Pull the last assistant message's plain-text content. Claude transcripts
    # are JSONL with one record per turn; assistant text content lives at
    # `.message.content[].text` for type=="text" entries on role=="assistant"
    # records. We tail the file to keep this fast on long sessions.
    last_text=$(tail -200 "$transcript" 2>/dev/null \
        | jq -r 'select(.message? and .message.role=="assistant") | .message.content[]? | select(.type=="text") | .text' 2>/dev/null \
        | tail -400) || exit 0
    [ -z "$last_text" ] && exit 0

    # Claim-word detection: case-insensitive whole-word match.
    # The list mirrors docs/development/definition-of-done.md rule #1.
    if ! printf '%s' "$last_text" | grep -qiwE 'verified|tested|working|complete|fixed|passing|done|validated|confirmed'; then
        exit 0
    fi

    # Evidence heuristic: a fenced code block within the same message
    # (>= 2 ``` fence markers means the message includes at least one
    # complete code block, plausibly the demanded "pasted output").
    fence_count=$(printf '%s' "$last_text" | grep -c '^```' || printf '0')
    if [ "$fence_count" -ge 2 ]; then
        exit 0
    fi

    # Emit advisory. The wording mirrors the DoD vocabulary so the message
    # is recognisable from the policy docs.
    cat <<'JSON'
{"systemMessage":"DoD reminder: the last assistant message contains claim words (verified / done / passing / working / complete / fixed / validated / confirmed) without an adjacent fenced output block. Per docs/development/definition-of-done.md rule #1, such claims need pasted terminal output from a real run in this session. If you actually have the evidence, paste it in the next message; if you don't, retract the claim."}
JSON
} 2>/dev/null

exit 0
