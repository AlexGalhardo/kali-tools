#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "$0")/.." && pwd)"
cd -- "$ROOT"
ACTION="${1:-test}"
if [ "$ACTION" = help ]; then printf '%s\n' 'Usage: bash setups/setup-windows-docker.sh [test|build|doctor|down|help]' 'Start Docker Desktop with its Linux engine first. Docker tests the CLI, not the native Windows UI or Wi-Fi.'; exit 0; fi
command -v docker >/dev/null 2>&1 || { printf '%s\n' 'Install Docker Desktop with Linux containers; see README.' >&2; exit 1; }
docker info >/dev/null 2>&1 || { printf '%s\n' 'Docker is unavailable. Start Docker Desktop, select Linux containers, and check engine permissions.' >&2; exit 1; }
compose() { docker compose --project-name kali-tools-lab -f docker/compose.yml "$@"; }
case "$ACTION" in
  test) trap 'status=$?; compose down --volumes --remove-orphans || true; exit "$status"' EXIT; compose up --build --abort-on-container-exit --exit-code-from test ;;
  build) compose build ;;
  doctor) docker version; docker compose version; compose config --quiet ;;
  down) compose down --volumes --remove-orphans ;;
  *) printf '%s\n' 'Unknown action; use help.' >&2; exit 2 ;;
esac
