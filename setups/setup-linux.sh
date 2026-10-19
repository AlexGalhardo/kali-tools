#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "$0")/.." && pwd)"
command -v node >/dev/null 2>&1 || { printf '%s\n' 'Install Node.js LTS first; see README Linux prerequisites.' >&2; exit 1; }
exec node "$ROOT/scripts/setup.mjs" linux "${1:-setup}"
