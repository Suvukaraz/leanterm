#!/usr/bin/env bash
# Builds the generic Linux archive used by install.sh on distributions without
# .deb packages (Fedora, openSUSE, Arch, ...): binary, desktop entries
# (LeanTerm and LeanEdit), icons.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION="${1:-1.2.0}"
ARCH="amd64"
NAME="leanterm_${VERSION}_linux_${ARCH}"

cd "$ROOT_DIR"
# The binary embeds frontend/dist, so build the frontend first.
echo "Building frontend..."
npm --prefix frontend run build

echo "Building LeanTerm binary..."
go build -o "$ROOT_DIR/bin/leanterm" .

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
DIR="$STAGE/$NAME"
mkdir -p "$DIR/icons"

install -m 0755 "$ROOT_DIR/bin/leanterm" "$DIR/leanterm"
for size in 16 24 32 48 64 128 256 512; do
    install -m 0644 "$ROOT_DIR/build/linux/icons/leanterm-${size}.png" "$DIR/icons/leanterm-${size}.png"
    install -m 0644 "$ROOT_DIR/build/linux/icons/leanedit-${size}.png" "$DIR/icons/leanedit-${size}.png"
done
install -m 0644 "$ROOT_DIR/LeanTerm_Icon.svg" "$DIR/icons/leanterm.svg"
install -m 0644 "$ROOT_DIR/LeanEdit_Icon.svg" "$DIR/icons/leanedit.svg"
install -m 0644 "$ROOT_DIR/LICENSE" "$DIR/LICENSE"

# Same entry as in the .deb package; install.sh sets the absolute Exec path.
cat <<EOF > "$DIR/org.leanterm.LeanTerm.desktop"
[Desktop Entry]
Version=1.0
Type=Application
Name=LeanTerm
GenericName=Connection Manager
Comment=LeanTerm Connection Manager (SSH, Telnet, SFTP)
Exec=leanterm %u
Icon=leanterm
Terminal=false
Categories=Network;Utility;
StartupWMClass=org.leanterm.LeanTerm
Keywords=ssh;telnet;sftp;terminal;connection;
EOF
chmod 0644 "$DIR/org.leanterm.LeanTerm.desktop"
# LeanEdit, the text editor on its own; install.sh sets the absolute Exec path.
install -m 0644 "$ROOT_DIR/build/linux/org.leanterm.LeanEdit.desktop" "$DIR/org.leanterm.LeanEdit.desktop"

mkdir -p "$ROOT_DIR/bin"
tar -C "$STAGE" --owner=0 --group=0 -czf "$ROOT_DIR/bin/$NAME.tar.gz" "$NAME"

echo "Linux archive ready: $ROOT_DIR/bin/$NAME.tar.gz"
