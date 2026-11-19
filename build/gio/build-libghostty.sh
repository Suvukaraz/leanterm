#!/usr/bin/env bash
# Builds Ghostty's terminal core (libghostty-vt, static) for the Go bindings
# go.mitchellh.com/libghostty, which link it via pkg-config.
#
#   build/gio/build-libghostty.sh <zig-target> <prefix>
#
# e.g. x86_64-linux-gnu.2.34 (glibc 2.34, LeanTerm's minimum) or
# x86_64-windows-gnu. Needs Zig 0.17 (pip install ziglang==0.17.1 provides it
# as `python -m ziglang`; set ZIG to override). Afterwards build LeanTerm with
#
#   PKG_CONFIG_PATH=<prefix>/lib/pkgconfig go build -tags ghostty ./cmd/leanterm-gio
set -euo pipefail

TARGET=${1:?zig target}
PREFIX=${2:?install prefix}
# The Ghostty commit the Go bindings are developed against (their CMakeLists).
GHOSTTY_COMMIT=${GHOSTTY_COMMIT:-5813c9461d5af78b6307a1254d00b1bc190eaccd}
SRC=${GHOSTTY_SRC:-${TMPDIR:-/tmp}/ghostty-src}

if [ -z "${ZIG:-}" ]; then
  if command -v zig >/dev/null; then ZIG=zig; else ZIG="python3 -m ziglang"; fi
fi

if [ ! -d "$SRC/.git" ]; then
  git clone --filter=blob:none https://github.com/ghostty-org/ghostty.git "$SRC"
fi
git -C "$SRC" fetch --quiet --filter=blob:none origin "$GHOSTTY_COMMIT" || true
git -C "$SRC" checkout --quiet "$GHOSTTY_COMMIT"

mkdir -p "$PREFIX"
PREFIX=$(cd "$PREFIX" && pwd)
cd "$SRC"
$ZIG build -Demit-lib-vt -Demit-xcframework=false -Doptimize=ReleaseFast \
  -Dtarget="$TARGET" --prefix "$PREFIX"
ls -la "$PREFIX/lib" "$PREFIX/lib/pkgconfig"
