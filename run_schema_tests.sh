#!/usr/bin/env sh
set -eu

# Run endpoint package tests.
# Usage:
#   ./run_endpoint_tests.sh
#   ./run_endpoint_tests.sh Deduplicate

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

if [ "${1:-}" = "" ]; then
  echo "Running: go test ./endpoint -v"
  go test ./endpoint -v
else
  echo "Running: go test ./endpoint -run $1 -v"
  go test ./endpoint -run "$1" -v
fi

