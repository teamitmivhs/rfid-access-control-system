#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
git pull --ff-only
cd backend
go build -o doorlock .
exec ./doorlock
