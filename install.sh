#!/data/data/com.termux/files/usr/bin/bash
# install.sh — one-liner installer for termux-ui.
#
#   curl -fsSL <raw-url>/install.sh | bash
#
# Detects the device arch, downloads the matching prebuilt binary from
# GitHub Releases and installs it to $PREFIX/bin/termux-ui.
set -euo pipefail

REPO="Theblackcat98/termux-ui"

if [ -z "${TERMUX_VERSION:-}" ] || [ -z "${PREFIX:-}" ]; then
  echo "error: this installer must run inside Termux (\$PREFIX/$TERMUX_VERSION missing)." >&2
  exit 1
fi

ARCH="$(uname -m)"
case "$ARCH" in
  aarch64) ASSET="termux-ui_linux_arm64" ;;
  armv7l|armv8l|arm) ASSET="termux-ui_linux_arm" ;;
  *) echo "error: unsupported arch: $ARCH (need aarch64 or arm)" >&2; exit 1 ;;
esac

mkdir -p "$PREFIX/bin"
URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
TMP="$PREFIX/bin/termux-ui.download"

echo "==> downloading $URL"
FAILED=0
if command -v curl >/dev/null 2>&1; then
  curl -fL --progress-bar "$URL" -o "$TMP" || FAILED=1
elif command -v wget >/dev/null 2>&1; then
  wget -q --show-progress "$URL" -O "$TMP" || FAILED=1
else
  echo "error: need curl or wget to download." >&2
  exit 1
fi

if [ "$FAILED" = 1 ]; then
  rm -f "$TMP"
  echo "error: download failed — no published release provides asset $ASSET." >&2
  echo "       Build on-device instead (Termux's own golang, no root needed):" >&2
  echo "         pkg install golang git" >&2
  echo "         git clone https://github.com/${REPO}.git && cd termux-ui && ./build.sh" >&2
  echo "         cp termux-ui $PREFIX/bin/" >&2
  exit 1
fi

chmod +x "$TMP"
mv -f "$TMP" "$PREFIX/bin/termux-ui"

echo "==> installed: $($PREFIX/bin/termux-ui --version)"
echo "==> run it with: termux-ui"
