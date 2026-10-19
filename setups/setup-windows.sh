#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "$0")/.." && pwd)"
command -v node >/dev/null 2>&1 || { printf '%s\n' 'Install Node.js LTS first; see README Windows prerequisites.' >&2; exit 1; }
exec node "$ROOT/scripts/setup.mjs" windows "${1:-setup}"
