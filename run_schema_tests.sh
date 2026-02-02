#!/usr/bin/env sh
set -eu

# Run schema package tests.
# Usage:
#   ./run_schema_tests.sh
#   ./run_schema_tests.sh Deduplicate

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

if [ "${1:-}" = "" ]; then
  echo "Running: go test ./schema -v"
  go test ./schema -v
else
  echo "Running: go test ./schema -run $1 -v"
  go test ./schema -run "$1" -v
fi

