#!/usr/bin/env bash
# ANNAVE PDF Engine — cross-compile release script
#
# Usage: ./scripts/release.sh [version]
#
# Builds static binaries for all four targets, computes SHA256 checksums,
# and writes them to dist/checksums.txt.
#
# The version argument defaults to the value in internal/engine/config.go.
# Override: ./scripts/release.sh 1.2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

VERSION="${1:-$(grep 'EngineVersion' internal/engine/config.go | grep -o '"[^"]*"' | tr -d '"')}"
DIST="dist/v${VERSION}"

echo "Building ANNAVE PDF Engine v${VERSION}"
mkdir -p "$DIST"

build() {
  local os="$1" arch="$2"
  local name="annave-pdf-engine_${VERSION}_${os}_${arch}"
  local out="$DIST/${name}"

  echo "  → ${os}/${arch}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build \
    -ldflags="-s -w -X annave.tech/pdf-engine/internal/engine.EngineVersion=${VERSION}" \
    -o "${out}" \
    ./cmd/cli

  # Also build the server binary for the same target.
  local server_out="$DIST/annave-pdf-engine-server_${VERSION}_${os}_${arch}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build \
    -ldflags="-s -w -X annave.tech/pdf-engine/internal/engine.EngineVersion=${VERSION}" \
    -o "${server_out}" \
    ./cmd/server
}

build darwin  amd64
build darwin  arm64
build linux   amd64
build linux   arm64

echo ""
echo "Computing checksums…"
cd "$DIST"
shasum -a 256 annave-* > checksums.txt
cat checksums.txt

echo ""
echo "Release artifacts in $DIST"
echo ""
echo "Next steps:"
echo "  1. Create a GitHub release tagged v${VERSION}"
echo "  2. Upload all files in $DIST as release assets"
echo "  3. Update homebrew/annave-pdf-engine.rb with the sha256 values from checksums.txt"
echo "     (darwin_arm64 line uses the darwin_arm64 CLI binary checksum)"
