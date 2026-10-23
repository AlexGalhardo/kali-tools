#!/usr/bin/env bash
# Idempotent cloud-agent bootstrap. Node 24 is not on the default image; Go 1.27.3 is.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

node_version="24.21.0"
node_prefix="/usr/local/node-v${node_version}"

if [[ ! -x "${node_prefix}/bin/node" ]] || [[ "$("${node_prefix}/bin/node" --version)" != "v${node_version}" ]]; then
  tmp="$(mktemp -d)"
  archive="node-v${node_version}-linux-x64.tar.xz"
  curl -fsSL "https://nodejs.org/dist/v${node_version}/SHASUMS256.txt" -o "${tmp}/SHASUMS256.txt"
  curl -fsSL "https://nodejs.org/dist/v${node_version}/${archive}" -o "${tmp}/${archive}"
  (cd "$tmp" && grep " ${archive}$" SHASUMS256.txt | sha256sum -c -)
  sudo rm -rf "$node_prefix"
  sudo tar -xJf "${tmp}/${archive}" -C /usr/local
  sudo mv "/usr/local/node-v${node_version}-linux-x64" "$node_prefix"
  rm -rf "$tmp"
fi

mkdir -p "${HOME}/.local/bin"
for bin in node npm npx corepack; do
  ln -sfn "${node_prefix}/bin/${bin}" "${HOME}/.local/bin/${bin}"
  sudo ln -sfn "${node_prefix}/bin/${bin}" "/usr/local/bin/${bin}"
done
export PATH="${HOME}/.local/bin:${PATH}"
hash -r

if [[ "$(go env GOVERSION 2>/dev/null || true)" != "go1.27.3" ]]; then
  printf '%s\n' "Go 1.27.3 is required (go.mod and CI). Found: $(go version 2>/dev/null || echo missing)" >&2
  exit 1
fi

go mod download

if ! golangci-lint version 2>/dev/null | grep -q '2.15.0'; then
  GOBIN="${HOME}/.local/bin" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
fi
if ! govulncheck -version 2>&1 | grep -q '1.9.0'; then
  GOBIN="${HOME}/.local/bin" go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
fi
sudo ln -sfn "${HOME}/.local/bin/golangci-lint" /usr/local/bin/golangci-lint
sudo ln -sfn "${HOME}/.local/bin/govulncheck" /usr/local/bin/govulncheck

if [[ "$(bun --version 2>/dev/null || true)" != "1.4.3" ]]; then
  npm install --global --prefix "${HOME}/.local" bun@1.4.3
fi
sudo ln -sfn "${HOME}/.local/bin/bun" /usr/local/bin/bun

npm --prefix frontend ci
(cd frontend && bun install --frozen-lockfile)
export DEBIAN_FRONTEND=noninteractive
(cd frontend && npm exec -- playwright install --with-deps chromium)
hooks_path="$(git config --local --get core.hooksPath || true)"
if [[ -z "$hooks_path" ]]; then
  frontend/node_modules/.bin/lefthook install
else
  printf '%s\n' "core.hooksPath is already ${hooks_path}; lefthook install left it unchanged"
fi
