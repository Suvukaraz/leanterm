#!/bin/sh
# LeanTerm installer for Linux (x86_64).
#
#   curl -fsSL https://raw.githubusercontent.com/Suvukaraz/leanterm/main/install.sh | sh
#
# Options (pass them after "sh -s --"):
#   --version X.Y.Z   install this release instead of the latest one
#   --uninstall       remove LeanTerm (the configuration in ~/.config/leanterm is kept)
#   --help            show this help
#
# Debian, Ubuntu and derivatives get the .deb package through apt. Fedora/RHEL,
# openSUSE and Arch get the required libraries from their package manager and
# LeanTerm itself in /usr/local. Running the script again updates LeanTerm.
#
# Environment: LEANTERM_VERSION (like --version), LEANTERM_PREFIX (default
# /usr/local), LEANTERM_BASE_URL (download folder, for tests).

set -eu

REPO="Suvukaraz/leanterm"
PREFIX="${LEANTERM_PREFIX:-/usr/local}"
MANIFEST="$PREFIX/share/leanterm/installed-files"
TMP=""
SUDO=""

say() { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mWarning:\033[0m %s\n' "$*" >&2; }
die() {
	printf '\033[1;31mError:\033[0m %s\n' "$*" >&2
	exit 1
}
has() { command -v "$1" >/dev/null 2>&1; }

usage() {
	cat <<'USAGE'
LeanTerm installer for Linux (x86_64)

  curl -fsSL https://raw.githubusercontent.com/Suvukaraz/leanterm/main/install.sh | sh
  curl -fsSL https://raw.githubusercontent.com/Suvukaraz/leanterm/main/install.sh | sh -s -- [options]

Options:
  --version X.Y.Z   install this release instead of the latest one
  --uninstall       remove LeanTerm (the configuration in ~/.config/leanterm is kept)
  --help            show this help
USAGE
}

cleanup() {
	if [ -n "$TMP" ]; then rm -rf "$TMP"; fi
}

# ---- Privileges and downloads ------------------------------------------------------

setup_sudo() {
	if [ "$(id -u)" -eq 0 ]; then
		SUDO=""
	elif has sudo; then
		SUDO="sudo"
	elif has doas; then
		SUDO="doas"
	else
		die "root privileges are needed: run as root or install sudo"
	fi
}

as_root() {
	if [ -n "$SUDO" ]; then $SUDO "$@"; else "$@"; fi
}

fetch() { # url file
	if has curl; then
		curl -fsSL --retry 3 -o "$2" "$1"
	elif has wget; then
		wget -q -O "$2" "$1"
	else
		die "curl or wget is required"
	fi
}

latest_version() {
	url="https://github.com/$REPO/releases/latest"
	if has curl; then
		final=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$url")
	else
		final=$(wget -q -S --max-redirect=0 -O /dev/null "$url" 2>&1 | sed -n 's/^ *[Ll]ocation: *//p' | tail -n 1)
	fi
	version=${final##*/tag/v}
	case "$version" in
	[0-9]*.[0-9]*) echo "$version" ;;
	*) die "could not determine the latest release (got '$final')" ;;
	esac
}

# Downloads a release file into $TMP and checks it against SHA256SUMS.
download() { # file
	say "Downloading $1"
	fetch "$BASE_URL/$1" "$TMP/$1" || die "download of $BASE_URL/$1 failed"
	if [ ! -f "$TMP/SHA256SUMS" ]; then
		fetch "$BASE_URL/SHA256SUMS" "$TMP/SHA256SUMS" || die "download of the checksums failed"
	fi
	sum=$(grep " \*\{0,1\}$1\$" "$TMP/SHA256SUMS" | cut -d ' ' -f 1)
	[ -n "$sum" ] || die "$1 is not listed in SHA256SUMS"
	actual=$(sha256sum "$TMP/$1" | cut -d ' ' -f 1)
	[ "$sum" = "$actual" ] || die "checksum mismatch for $1"
}

# ---- System checks ------------------------------------------------------------------

check_system() {
	[ "$(uname -s)" = "Linux" ] || die "this installer is for Linux; for Windows use the installer from https://github.com/$REPO/releases"
	case "$(uname -m)" in
	x86_64 | amd64) ;;
	*) die "LeanTerm is built for x86_64 only (this system: $(uname -m))" ;;
	esac
	# The binary needs glibc 2.34 or newer (no musl, e.g. Alpine).
	glibc=$(getconf GNU_LIBC_VERSION 2>/dev/null | cut -d ' ' -f 2 || true)
	[ -n "$glibc" ] || die "glibc is required (musl based systems such as Alpine are not supported)"
	major=${glibc%%.*}
	minor=${glibc#*.}
	minor=${minor%%.*}
	if [ "$major" -lt 2 ] || { [ "$major" -eq 2 ] && [ "$minor" -lt 34 ]; }; then
		die "glibc $glibc is too old, 2.34 or newer is required"
	fi
}

# Prints the family of the distribution: deb, dnf, zypper, pacman or unknown.
distro_family() {
	[ -r /etc/os-release ] || {
		echo unknown
		return
	}
	ids=$(. /etc/os-release && echo "${ID:-} ${ID_LIKE:-}")
	for id in $ids; do
		case "$id" in
		debian | ubuntu) echo deb && return ;;
		fedora | rhel | centos) echo dnf && return ;;
		opensuse* | suse | sles) echo zypper && return ;;
		arch | archlinux) echo pacman && return ;;
		esac
	done
	echo unknown
}

distro_name() {
	(. /etc/os-release 2>/dev/null && echo "${PRETTY_NAME:-${NAME:-Linux}}") || echo Linux
}

# Lists the shared library cache; ldconfig is in /sbin on Debian (not in a user's PATH).
ldconfig_p() {
	for c in ldconfig /sbin/ldconfig /usr/sbin/ldconfig; do
		if has "$c"; then
			LC_ALL=C "$c" -p 2>/dev/null
			return
		fi
	done
}

missing_libraries() {
	cache=$(ldconfig_p)
	for lib in libgtk-4.so.1 libwebkitgtk-6.0.so.4 libjavascriptcoregtk-6.0.so.1 libsoup-3.0.so.0; do
		if [ -n "$cache" ]; then
			echo "$cache" | grep -q "$lib" || echo "$lib"
		elif ! ls /usr/lib*/"$lib" /usr/lib/*/"$lib" >/dev/null 2>&1; then
			echo "$lib"
		fi
	done
}

# LeanTerm uses the file dialogs of GTK 4.10 (gtk_file_dialog_*); older GTK 4
# versions load, but the program would crash at the first file dialog.
check_gtk() {
	lib=$(ldconfig_p | sed -n 's/.*libgtk-4\.so\.1 .*=> //p' | head -n 1)
	[ -n "$lib" ] || return 0 # missing libraries are reported by verify
	# -a and LC_ALL=C: search the binary file byte-wise in any locale.
	LC_ALL=C grep -a -q gtk_file_dialog_new "$lib" ||
		die "the GTK 4 of $(distro_name) is older than 4.10, which LeanTerm needs"
}

# ---- Installation -------------------------------------------------------------------

install_deb() {
	say "Updating the package lists"
	as_root apt-get update -qq
	if ! apt-cache show libwebkitgtk-6.0-4 >/dev/null 2>&1; then
		die "$(distro_name) has no WebKitGTK 6.0 (libwebkitgtk-6.0-4); Debian 13, Ubuntu 24.04 or a newer derivative is required"
	fi
	gtk=$(LC_ALL=C apt-cache policy libgtk-4-1 | sed -n 's/^ *Candidate: *//p')
	if [ -z "$gtk" ] || [ "$gtk" = "(none)" ] || ! dpkg --compare-versions "$gtk" ge 4.10; then
		die "$(distro_name) has GTK ${gtk:-(none)}, but LeanTerm needs GTK 4.10 or newer; Debian 13, Ubuntu 24.04 or a newer derivative is required"
	fi
	file="leanterm_${VERSION}_amd64.deb"
	download "$file"
	# apt reads the file as user _apt.
	chmod 0755 "$TMP"
	chmod 0644 "$TMP/$file"
	say "Installing $file"
	as_root env DEBIAN_FRONTEND=noninteractive apt-get install -y "$TMP/$file"
	BIN=/usr/bin/leanterm
}

install_dependencies() { # family
	case "$1" in
	dnf)
		say "Installing GTK 4 and WebKitGTK 6.0 (dnf)"
		as_root dnf install -y gtk4 webkitgtk6.0 ||
			die "could not install gtk4 and webkitgtk6.0 (RHEL/Alma/Rocky need version 10 or newer)"
		;;
	zypper)
		say "Installing GTK 4 and WebKitGTK 6.0 (zypper)"
		as_root zypper --non-interactive install libgtk-4-1 libwebkitgtk-6_0-4 ||
			die "could not install libgtk-4-1 and libwebkitgtk-6_0-4 (openSUSE Tumbleweed or Leap 16 is required)"
		;;
	pacman)
		say "Installing GTK 4 and WebKitGTK 6.0 (pacman)"
		# Without a package database (fresh system) a full update is needed first.
		as_root pacman -S --needed --noconfirm gtk4 webkitgtk-6.0 2>/dev/null ||
			as_root pacman -Syu --needed --noconfirm gtk4 webkitgtk-6.0 ||
			die "could not install gtk4 and webkitgtk-6.0"
		;;
	*)
		missing=$(missing_libraries)
		if [ -n "$missing" ]; then
			die "$(distro_name) is not supported automatically. Install GTK 4 and WebKitGTK 6.0 so that these libraries exist, then run the installer again: $(echo $missing)"
		fi
		;;
	esac
}

install_tarball() {
	name="leanterm_${VERSION}_linux_amd64"
	download "$name.tar.gz"
	tar -xzf "$TMP/$name.tar.gz" -C "$TMP"
	src="$TMP/$name"

	say "Installing LeanTerm to $PREFIX"
	files="$TMP/installed-files"
	: >"$files"
	put() { # mode source target
		as_root install -D -m "$1" "$2" "$3"
		echo "$3" >>"$files"
	}
	put 0755 "$src/leanterm" "$PREFIX/bin/leanterm"
	sed "s|^Exec=leanterm|Exec=$PREFIX/bin/leanterm|" "$src/org.leanterm.LeanTerm.desktop" >"$TMP/org.leanterm.LeanTerm.desktop"
	put 0644 "$TMP/org.leanterm.LeanTerm.desktop" "$PREFIX/share/applications/org.leanterm.LeanTerm.desktop"
	for size in 16 24 32 48 64 128 256 512; do
		put 0644 "$src/icons/leanterm-$size.png" "$PREFIX/share/icons/hicolor/${size}x${size}/apps/leanterm.png"
	done
	put 0644 "$src/icons/leanterm.svg" "$PREFIX/share/icons/hicolor/scalable/apps/leanterm.svg"
	echo "$MANIFEST" >>"$files"
	as_root install -D -m 0644 "$files" "$MANIFEST"
	refresh_caches
	BIN="$PREFIX/bin/leanterm"
}

refresh_caches() {
	if has update-desktop-database; then as_root update-desktop-database -q "$PREFIX/share/applications" 2>/dev/null || true; fi
	if has gtk-update-icon-cache; then as_root gtk-update-icon-cache -q -t -f "$PREFIX/share/icons/hicolor" 2>/dev/null || true; fi
}

# The binary must find all libraries; --help loads them without opening a window.
verify() {
	if has ldd; then
		missing=$(LC_ALL=C ldd "$BIN" 2>/dev/null | sed -n 's/^[[:space:]]*\([^ ]*\) => not found.*/\1/p')
		[ -z "$missing" ] || die "LeanTerm is installed, but these libraries are missing: $(echo $missing)"
	fi
	"$BIN" --help >/dev/null 2>&1 || die "LeanTerm is installed, but $BIN does not start"
}

# ---- Uninstall ----------------------------------------------------------------------

uninstall() {
	setup_sudo
	removed=0
	if has dpkg-query && dpkg-query -W -f='${Status}' leanterm 2>/dev/null | grep -q "install ok installed"; then
		say "Removing the leanterm package"
		as_root env DEBIAN_FRONTEND=noninteractive apt-get remove -y leanterm
		removed=1
	fi
	if [ -f "$MANIFEST" ]; then
		say "Removing LeanTerm from $PREFIX"
		while IFS= read -r f; do
			case "$f" in
			"$PREFIX"/*) as_root rm -f "$f" ;;
			esac
		done <"$MANIFEST"
		as_root rmdir "$PREFIX/share/leanterm" 2>/dev/null || true
		refresh_caches
		removed=1
	fi
	[ "$removed" -eq 1 ] || die "LeanTerm is not installed (no package and no $MANIFEST)"
	say "LeanTerm was removed. Your configuration in ~/.config/leanterm was kept."
}

# ---- Main ---------------------------------------------------------------------------

main() {
	VERSION="${LEANTERM_VERSION:-}"
	action=install
	while [ $# -gt 0 ]; do
		case "$1" in
		--version)
			[ $# -ge 2 ] || die "--version needs a value"
			VERSION=$2
			shift
			;;
		--version=*) VERSION=${1#--version=} ;;
		--uninstall) action=uninstall ;;
		-h | --help)
			usage
			exit 0
			;;
		*) die "unknown option: $1 (see --help)" ;;
		esac
		shift
	done
	VERSION=${VERSION#v}

	if [ "$action" = uninstall ]; then
		uninstall
		return
	fi

	check_system
	setup_sudo
	has sha256sum || die "sha256sum is required"
	has tar || die "tar is required"

	[ -n "$VERSION" ] || VERSION=$(latest_version)
	BASE_URL="${LEANTERM_BASE_URL:-https://github.com/$REPO/releases/download/v$VERSION}"
	TMP=$(mktemp -d)
	trap cleanup EXIT
	# dash keeps running after a trapped signal, so leave explicitly.
	trap 'cleanup; exit 130' INT TERM

	family=$(distro_family)
	say "Installing LeanTerm $VERSION on $(distro_name)"
	if [ "$family" = deb ]; then
		install_deb
	else
		install_dependencies "$family"
		check_gtk
		install_tarball
	fi
	verify
	say "LeanTerm $VERSION is installed. Start it from the application menu or with: leanterm"
}

main "$@"
