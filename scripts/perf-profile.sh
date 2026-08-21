#!/usr/bin/env bash
set -e
echo "Running go test -bench and UI perf"
go test ./... -bench=. -benchmem
echo "Profiling done"
