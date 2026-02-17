#!/bin/bash
# Build agent binaries for multiple platforms
set -e

cd "$(dirname "$0")"

VERSION=${1:-"dev"}
OUTPUT_DIR="./build"
mkdir -p "$OUTPUT_DIR"

echo "Building ServerSupervisor Agent v$VERSION..."

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "linux/arm"
)

for PLATFORM in "${PLATFORMS[@]}"; do
  OS="${PLATFORM%/*}"
  ARCH="${PLATFORM#*/}"
  OUTPUT="$OUTPUT_DIR/serversupervisor-agent-${OS}-${ARCH}"

  echo "  Building $OS/$ARCH..."
  GOOS=$OS GOARCH=$ARCH CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.Version=$VERSION" \
    -o "$OUTPUT" \
    ./cmd/agent

  echo "  -> $OUTPUT ($(du -h "$OUTPUT" | cut -f1))"
done

echo ""
echo "Build complete! Binaries in $OUTPUT_DIR/"
ls -lh "$OUTPUT_DIR/"
