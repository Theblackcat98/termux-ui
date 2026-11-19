#!/data/data/com.termux/files/usr/bin/bash
# build.sh — build termux-ui on-device (Termux golang) or cross-compile for
# release on a host. Usage:
#
#   ./build.sh              native build -> ./termux-ui
#   ./build.sh release      both arm64+arm artifacts in ./dist/
#
# Builds always target GOOS=android, never GOOS=linux: Android 11's seccomp
# filter kills GOOS=linux binaries with SIGSYS the first time Go's os/exec
# issues faccessat2 (it happens as soon as `core` is on PATH), and
# internal/dnsfix relies on the android GOOS behavior for DNS. CGO stays off:
# internal/dnsfix handles the missing /etc/resolv.conf in pure Go.
set -euo pipefail

cd "$(dirname "$0")"

VERSION="${VERSION:-}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
if [ -z "$VERSION" ]; then
  VERSION="v0.$(date +%Y%m%d)"
fi
# main.version is what `termux-ui --version` prints; internal/app.Version is
# what the dashboard header shows. Set both or the header says "dev".
LDFLAGS="-s -w -X main.version=${VERSION}+${COMMIT} -X github.com/Theblackcat98/termux-ui/internal/app.Version=${VERSION}+${COMMIT}"

if [ "${1:-}" = "release" ]; then
  mkdir -p dist
  echo "==> cross-building ${VERSION} (${COMMIT})"
  GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o dist/termux-ui_linux_arm64 .
  # GOOS=android GOARCH=arm is a Go limitation: 32-bit android requires cgo
  # external linking, so this artifact needs the NDK (CI provides it via
  # ANDROID_NDK_HOME). arm64 stays static above.
  ARM_CC=""
  if [ -n "${ANDROID_NDK_HOME:-}" ]; then
    case "$(uname -s)-$(uname -m)" in
      Darwin-arm64)  HOST_TAG=darwin-aarch64 ;;
      Darwin-*)      HOST_TAG=darwin-x86_64 ;;
      *)             HOST_TAG=linux-x86_64 ;;
    esac
    ARM_CC="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/$HOST_TAG/bin/armv7a-linux-androideabi24-clang"
    [ -x "$ARM_CC" ] || ARM_CC=""
  fi
  if [ -n "$ARM_CC" ]; then
    GOOS=android GOARCH=arm CGO_ENABLED=1 CC="$ARM_CC" go build -trimpath -ldflags "$LDFLAGS" -o dist/termux-ui_linux_arm .
  else
    echo "==> WARNING: ANDROID_NDK_HOME not set — skipping the 32-bit arm build (it needs the NDK for cgo)." >&2
  fi
  sha256sum dist/termux-ui_linux_* | tee dist/sha256sums
  echo "==> artifacts:"
  ls -la dist
else
  echo "==> building ${VERSION} (${COMMIT})"
  GOOS=android CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o termux-ui .
  ./termux-ui --version
fi
