#!/data/data/com.termux/files/usr/bin/bash
# build.sh — build termux-ui on-device (Termux golang) or cross-compile for
# release on a host. Usage:
#
#   ./build.sh              native build -> ./termux-ui
#   ./build.sh release      both arm64+arm artifacts in ./dist/
set -euo pipefail

cd "$(dirname "$0")"

VERSION="${VERSION:-}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
if [ -z "$VERSION" ]; then
  VERSION="v0.$(date +%Y%m%d)"
fi
LDFLAGS="-s -w -X main.version=${VERSION}+${COMMIT}"

if [ "${1:-}" = "release" ]; then
  mkdir -p dist
  echo "==> cross-building ${VERSION} (${COMMIT})"
  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o dist/termux-ui_linux_arm64 .
  GOOS=linux GOARCH=arm CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o dist/termux-ui_linux_arm .
  sha256sum dist/termux-ui_linux_* | tee dist/sha256sums
  echo "==> artifacts:"
  ls -la dist
else
  echo "==> building ${VERSION} (${COMMIT})"
  CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o termux-ui .
  ./termux-ui --version
fi
