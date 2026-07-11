#!/bin/bash
# Build agent binaries for multiple platforms
set -e

cd "$(dirname "$0")"

VERSION=${1:-"dev"}
OUTPUT_DIR="./build"
mkdir -p "$OUTPUT_DIR"

echo "Building ServerSupervisor Agent v$VERSION..."

# Mirrors .github/workflows/release.yml's agent-binaries matrix exactly (4
# targets: amd64, arm64, armv7, armv6). armv7 and armv6 are both
# GOARCH=arm and only differ by GOARM, which a plain "linux/arch" pair can't
# express — hence the third "/GOARM" segment, empty for non-arm targets.
PLATFORMS=(
  "linux/amd64/"
  "linux/arm64/"
  "linux/arm/7"
  "linux/arm/6"
)

for PLATFORM in "${PLATFORMS[@]}"; do
  OS="${PLATFORM%%/*}"
  REST="${PLATFORM#*/}"
  ARCH="${REST%%/*}"
  ARM="${REST#*/}"

  SUFFIX="$ARCH"
  if [ "$ARCH" = "arm" ] && [ -n "$ARM" ]; then
    SUFFIX="armv${ARM}"
  fi
  OUTPUT="$OUTPUT_DIR/serversupervisor-agent-${OS}-${SUFFIX}"

  echo "  Building $OS/$SUFFIX..."
  GOOS=$OS GOARCH=$ARCH GOARM=$ARM CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.Version=$VERSION" \
    -o "$OUTPUT" \
    ./cmd/agent

  echo "  -> $OUTPUT ($(du -h "$OUTPUT" | cut -f1))"
done

echo ""
echo "Build complete! Binaries in $OUTPUT_DIR/"
ls -lh "$OUTPUT_DIR/"
