#!/usr/bin/env bash
# Builds the Windows installer (NSIS, amd64) for the Gio GUI by cross-compiling
# from Linux. Needs Go, Zig (ZIG, default: zig; the CI installs it with pip),
# go-winres (GO_WINRES, default: $(go env GOPATH)/bin/go-winres) and makensis
# (Debian/Ubuntu: apt install nsis). PKG_CONFIG_PATH must point to the Windows
# libghostty-vt: build it with
#   build/gio/build-libghostty.sh x86_64-windows-gnu <prefix>
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION="${1:-1.2.1}"
SETUP_NAME="leanterm_${VERSION}_windows_amd64_setup.exe"
ZIG="${ZIG:-zig}"
GO_WINRES="${GO_WINRES:-$(go env GOPATH)/bin/go-winres}"
NSIS_DIR="$ROOT_DIR/build/windows/nsis"

if ! command -v makensis >/dev/null 2>&1; then
    echo "makensis not found. Install NSIS first, e.g.: sudo apt install nsis" >&2
    exit 1
fi
if [ -z "${PKG_CONFIG_PATH:-}" ]; then
    echo "PKG_CONFIG_PATH must point to the Windows libghostty-vt (see build/gio/build-libghostty.sh)" >&2
    exit 1
fi

cd "$ROOT_DIR"
WORK_DIR="$(mktemp -d)"
# The resource object must sit in the package folder while Go links it.
SYSO="$ROOT_DIR/cmd/leanterm-native/rsrc_windows_amd64.syso"
trap 'rm -rf "$WORK_DIR" "$SYSO"' EXIT

# Icon, manifest and version information for Explorer.
echo "Generating Windows resources..."
"$GO_WINRES" simply --arch amd64 --icon build/windows/icon.ico --manifest gui \
    --product-name LeanTerm --file-description "LeanTerm Connection Manager" \
    --product-version "$VERSION" --file-version "$VERSION" \
    --copyright "© 2026 LeanTerm Project, GPL-3.0" --original-filename leanterm.exe \
    --out "$ROOT_DIR/cmd/leanterm-native/rsrc"

echo "Building LeanTerm for Windows (amd64, libghostty)..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
    CC="$ZIG cc -target x86_64-windows-gnu" CXX="$ZIG c++ -target x86_64-windows-gnu" \
    CGO_LDFLAGS_ALLOW='.*[.]lib$' \
    go build -tags ghostty -trimpath -buildvcs=false \
    -ldflags="-w -s -H windowsgui" -o "$WORK_DIR/leanterm.exe" ./cmd/leanterm-native
rm -f "$SYSO"

echo "Creating installer..."
mkdir -p "$ROOT_DIR/bin"
(cd "$NSIS_DIR" && makensis -V2 \
    -DINFO_PRODUCTVERSION="$VERSION" \
    -DARG_BINARY="$WORK_DIR/leanterm.exe" \
    -DOUTFILE="$ROOT_DIR/bin/$SETUP_NAME" \
    gio.nsi)

echo "Windows installer ready: $ROOT_DIR/bin/$SETUP_NAME"
