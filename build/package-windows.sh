#!/usr/bin/env bash
# Builds the Windows installer (NSIS, amd64) by cross-compiling from Linux.
# Needs Go, npm, wails3 (icon, manifest, version info, WebView2 bootstrapper)
# and makensis (Debian/Ubuntu: apt install nsis).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION="${1:-1.0.0}"
ARCH="${2:-amd64}"
SETUP_NAME="leanterm_${VERSION}_windows_${ARCH}_setup.exe"
WAILS3="${WAILS3:-$(command -v wails3 || echo "$HOME/go/bin/wails3")}"
NSIS_DIR="$ROOT_DIR/build/windows/nsis"

if ! command -v makensis >/dev/null 2>&1; then
    echo "makensis not found. Install NSIS first, e.g.: sudo apt install nsis" >&2
    exit 1
fi
case "$ARCH" in
    amd64) ARG_FLAG=AMD64 ;;
    arm64) ARG_FLAG=ARM64 ;;
    *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

cd "$ROOT_DIR"
# The binary embeds frontend/dist, so build the frontend first.
echo "Building frontend..."
npm --prefix frontend run build

WORK_DIR="$(mktemp -d)"
SYSO="$ROOT_DIR/wails_windows_${ARCH}.syso"
trap 'rm -rf "$WORK_DIR" "$SYSO"' EXIT

# Icon, manifest (DPI awareness, Windows 10/11) and version info for Explorer.
echo "Generating Windows resources..."
(cd build && "$WAILS3" generate syso -arch "$ARCH" -icon windows/icon.ico \
    -manifest windows/wails.exe.manifest -info windows/info.json -out "$SYSO")

echo "Building LeanTerm for Windows ($ARCH)..."
GOOS=windows GOARCH="$ARCH" CGO_ENABLED=0 \
    go build -tags production -trimpath -buildvcs=false \
    -ldflags="-w -s -H windowsgui" -o "$WORK_DIR/LeanTerm.exe" .
rm -f "$SYSO"

# The installer installs the WebView2 runtime when it is missing (Windows 10).
echo "Creating installer..."
"$WAILS3" generate webview2bootstrapper -dir "$NSIS_DIR"
mkdir -p "$ROOT_DIR/bin"
(cd "$NSIS_DIR" && makensis -V2 \
    -DINFO_PRODUCTVERSION="$VERSION" \
    -DARG_WAILS_${ARG_FLAG}_BINARY="$WORK_DIR/LeanTerm.exe" \
    -DOUTFILE="$ROOT_DIR/bin/$SETUP_NAME" \
    project.nsi)

echo "Windows installer ready: $ROOT_DIR/bin/$SETUP_NAME"
