#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION="${1:-0.8.0}"
ARCH="amd64"
PKG_NAME="leanterm"
DEB_NAME="${PKG_NAME}_${VERSION}_${ARCH}.deb"

echo "Building LeanTerm binary..."
cd "$ROOT_DIR"
go build -o "$ROOT_DIR/bin/leanterm" .

DEB_ROOT="$ROOT_DIR/build/deb-root"
rm -rf "$DEB_ROOT"
mkdir -p "$DEB_ROOT/DEBIAN"
mkdir -p "$DEB_ROOT/usr/bin"
mkdir -p "$DEB_ROOT/usr/share/applications"
for size in 16 24 32 48 64 128 256 512; do
    mkdir -p "$DEB_ROOT/usr/share/icons/hicolor/${size}x${size}/apps"
done
mkdir -p "$DEB_ROOT/usr/share/icons/hicolor/scalable/apps"

chmod 0755 "$DEB_ROOT"
chmod 0755 "$DEB_ROOT/DEBIAN"

# 1. Binary
install -m 0755 "$ROOT_DIR/bin/leanterm" "$DEB_ROOT/usr/bin/leanterm"

# 2. Desktop Entry
cat <<EOF > "$DEB_ROOT/usr/share/applications/org.leanterm.LeanTerm.desktop"
[Desktop Entry]
Version=1.0
Type=Application
Name=LeanTerm
GenericName=Connection Manager
Comment=LeanTerm Connection Manager (SSH, Telnet, SFTP)
Exec=/usr/bin/leanterm %u
Icon=leanterm
Terminal=false
Categories=Network;Utility;
StartupWMClass=org.leanterm.LeanTerm
Keywords=ssh;telnet;sftp;terminal;connection;
EOF
chmod 0644 "$DEB_ROOT/usr/share/applications/org.leanterm.LeanTerm.desktop"

# 3. Icons
for size in 16 24 32 48 64 128 256 512; do
    install -m 0644 "$ROOT_DIR/build/linux/icons/leanterm-${size}.png" "$DEB_ROOT/usr/share/icons/hicolor/${size}x${size}/apps/leanterm.png"
done
install -m 0644 "$ROOT_DIR/LeanTerm_Icon.svg" "$DEB_ROOT/usr/share/icons/hicolor/scalable/apps/leanterm.svg"

# 4. Control file
cat <<EOF > "$DEB_ROOT/DEBIAN/control"
Package: ${PKG_NAME}
Version: ${VERSION}
Section: net
Priority: optional
Architecture: ${ARCH}
Maintainer: LeanTerm Project <info@leanterm.org>
Depends: libc6 (>= 2.31), libgtk-4-1, libwebkitgtk-6.0-4
Conflicts: linxterm
Replaces: linxterm
Recommends: xdg-utils, openssh-client, keepassxc
Homepage: https://github.com/Suvukaraz/leanterm
Description: LeanTerm Connection Manager
 Complete rewrite of Ásbrú Connection Manager in Go and Svelte 5.
 Supports native SSH, native Telnet (RFC 854/855/1091/1073/1143), embedded
 MobaXterm-style SFTP browser, automated Expect sequences, variable
 substitutions, clusters & PCC, and KeePassXC integration.
EOF
chmod 0644 "$DEB_ROOT/DEBIAN/control"

# 5. Maintainer scripts
cat <<EOF > "$DEB_ROOT/DEBIAN/postinst"
#!/bin/sh
set -e
if which update-desktop-database >/dev/null 2>&1; then
    update-desktop-database -q || true
fi
if which gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
fi
exit 0
EOF
chmod 0755 "$DEB_ROOT/DEBIAN/postinst"

cat <<EOF > "$DEB_ROOT/DEBIAN/postrm"
#!/bin/sh
set -e
if which update-desktop-database >/dev/null 2>&1; then
    update-desktop-database -q || true
fi
exit 0
EOF
chmod 0755 "$DEB_ROOT/DEBIAN/postrm"

mkdir -p "$ROOT_DIR/bin"
dpkg-deb --build --root-owner-group "$DEB_ROOT" "$ROOT_DIR/bin/$DEB_NAME"
rm -rf "$DEB_ROOT"

echo "Debian package successfully built: $ROOT_DIR/bin/$DEB_NAME"
