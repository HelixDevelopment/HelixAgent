#!/bin/bash
# Capture performance baselines
set -euo pipefail

BASEDIR="benchmarks/baselines"
mkdir -p "$BASEDIR"

echo "=== Capturing Performance Baselines ==="
echo ""

echo "--- Handler benchmarks ---"
GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=3 \
  -run=^$ ./internal/handlers/ -p 1 -timeout=300s 2>/dev/null \
  | tee "$BASEDIR/handlers.txt"

echo ""
echo "--- Pool benchmarks ---"
GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=3 \
  -run=^$ ./internal/clis/ -p 1 -timeout=300s 2>/dev/null \
  | tee "$BASEDIR/pool.txt"

echo ""
echo "--- HTTP benchmarks ---"
GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=3 \
  -run=^$ ./internal/http/ -p 1 -timeout=300s 2>/dev/null \
  | tee "$BASEDIR/http.txt"

echo ""
echo "--- Ensemble benchmarks ---"
GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=3 \
  -run=^$ ./internal/ensemble/... -p 1 -timeout=300s 2>/dev/null \
  | tee "$BASEDIR/ensemble.txt"

echo ""
echo "=== Baselines captured at $BASEDIR/ ==="
date > "$BASEDIR/CAPTURED_AT.txt"
