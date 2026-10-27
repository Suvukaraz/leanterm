# LeanTerm – Modern Connection Manager

<p align="center"><img src="LeanTerm_Icon.svg" alt="LeanTerm logo" width="128"></p>

LeanTerm is a fast, native Linux connection and terminal manager built with **Go** and **Svelte 5** (via Wails v3). It is a complete, clean rewrite of the legacy Perl-based *Ásbrú Connection Manager*, preserving complete visual and functional parity with the GTK3 interface while eliminating security flaws, shell injections, and slow script wrappers.

> **Status:** 0.8.0 – usable day to day on Linux (tested on Debian 13 / KDE Plasma, Wayland). Windows and macOS build files exist but are untested.

---

## 🌟 Key Features

### Lean, Configurable UI
- Main button bar at the top (default) or bottom of the window (*Preferences → Main Options*).
- Toggles in the button bar: **Connections** tree, **File Browser** panels (SFTP and local), **Clusters** functions (hidden by default to keep the UI lean).
- SFTP panel on the left (default) or right side of the terminal (*Preferences → Terminal Options → SFTP browser*).
- New connections only need a name; the title is derived from it.
- Closing the window asks for confirmation while connections or SFTP transfers are active.

### Native Protocols (No CLI wrappers)
- **Native SSH (Go `x/crypto/ssh`):**
  - Direct connection, SOCKS5 proxies, and multi-hop Jump Host chains (`ProxyJump`).
  - Full authentication: Password, Public Key (+ passphrase), Keyboard-Interactive (OTP), SSH Agent forwarding (`SSH_AUTH_SOCK`), and automatic fallback.
  - Port forwarding: Local (`-L`), Remote (`-R`), and Dynamic SOCKS5 server (`-D`).
  - X11 Forwarding (`x11-req` with MIT-MAGIC-COOKIE from local display).
  - `NoRemoteCommand` support ("Port forwarding only").
  - Host key verification with strict `known_hosts` matching and auto-accept options.
- **Native Telnet (`internal/telnet`):**
  - RFC 854/855/1091/1073/1143 IAC option negotiation engine.
  - Window size negotiation (NAWS) and Terminal Type (`TTYPE`).
  - Automatic prompt login state machine via Perl-compatible regexes (`regexp2`).
  - Interactive escape character prompt (`^]`).
- **Local PTY Shell:**
  - High-performance local terminal using native Unix pseudo-terminals (`creack/pty`).

### Embedded SFTP Browser (MobaXterm Style)
- Slides out on the left or right side of every SSH terminal tab.
- Operates directly over the session's active SSH client (no second login).
- Real-time directory synchronization via **OSC 7** terminal sequences ("Follow terminal folder").
- Full file operations: Upload (drag-and-drop or file dialog), download, rename (F2), delete (Del), mkdir (F7), empty file creation, and chmod (rwx matrix + octal).
- **"Open with default application":** Downloads to a temporary folder, opens with `xdg-open`, watches for changes via `fsnotify`, and prompts to upload modifications back to the remote server.
- Transfer queue with progress, speed, ETA, parallel transfer throttling, and resume support.
- Optional periodic folder polling (`poll_seconds`).

### Local File Browser
- Local shell tabs show the same browser for the local file system (title "File Browser").
- Starts in the shell's working directory; "Follow terminal folder" tracks the shell via `/proc/<pid>/cwd` (no shell integration or OSC 7 needed).
- Double-click opens files directly in their default application; folders are entered.
- The Home button opens your home folder; free space is shown on Linux, macOS, FreeBSD and Windows.
- Copy to… / Move to… instead of upload/download; files dropped from the desktop are copied into the current folder, asking before existing files are overwritten.
- File operations use the Go standard library directly – no shell commands.

### Automation & Productivity
- **Variable Substitution Engine:**
  - Built-in tokens: `<UUID>`, `<SOCKS5_PORT>`, `<TIMESTAMP>`, `<DATE_Y>`, `<DATE_M>`, `<DATE_D>`, `<TIME_H>`, `<TIME_M>`, `<TIME_S>`, `<NAME>`, `<TITLE>`, `<IP>`, `<PORT>`, `<USER>`, `<PASS>`.
  - Global variables (`<GV:name>`) and local variables (`<V:0>` through `<V:9>`).
  - Process environment (`<ENV:NAME>`).
  - Interactive value prompts (`<ASK:n>`, `<ASK:desc|opt1|opt2>`, `<ASK:text>`).
  - Local command evaluation (`<CMD:command>`).
  - Control tags (`<CTRL_X:cmd>`, `<TEE:file>`, `<PIPE:cmd>`).
  - KeePassXC masks (`<field|entry>` or `<Password|entry>`).
- **Expect Engine (`internal/session/expect.go`):**
  - Multiline regex matching with lookahead/backreferences (`github.com/dlclark/regexp2`).
  - Actions on match/fail: `next`, `stop`, `goto` (with 1000-jump loop guard).
  - Delay support and progress tracking.
- **Connection Chains:**
  - Drag and drop any connection from the tree directly onto an active terminal.
  - Interactive Chain Selection Dialog to review and selectively execute the chained Expect commands.
  - Preference to automatically skip initial login prompts (`skip_first_chain_expect`).
- **Clusters & Power Cluster Controller (PCC):**
  - Group active terminals into ad-hoc, configured, or regex-based auto-clusters.
  - Broadcast keystrokes in real time to all cluster members.
  - PCC window to send commands (with variable substitution) across all clustered terminals.
- **KeePassXC Integration:**
  - Direct querying via `keepassxc-cli` with master password passed over secure stdin.
  - Transparent tag replacement for `<Password|entry>`, `<UserName|entry>`, etc.
  - In-memory credential caching with automatic TTL expiration.
- **Pipe & Tee Output:**
  - `<TEE:filepath>` mirrors all terminal output to a local log file.
  - `<PIPE:command>` pipes terminal output to an external process and streams output to a dedicated Piped Output Window.
- **Wake on LAN & Bulk Edit:**
  - UDP magic packet generator and `/proc/net/arp` reader.
  - Safe bulk search-and-replace across connections without `eval`.

### Security First
- Secrets are encrypted using **`filippo.io/age`** (X25519 & scrypt) and stored in `secrets.age`.
- Supports OS Keyring (Secret Service / libsecret via D-Bus) or Master Password mode.
- Plain text passwords never appear in `config.toml`, process arguments, or terminal logs.
- Master password GUI lock overlay.

---

## 🛠️ Installation & Building

### Debian / Ubuntu Package (.deb)
Download `leanterm_0.8.0_amd64.deb` from the [Releases](https://github.com/Suvukaraz/leanterm/releases) page and install it:
```bash
sudo apt install ./leanterm_0.8.0_amd64.deb
```

Runtime dependencies: GTK4 and WebKitGTK 6.0 (`libgtk-4-1`, `libwebkitgtk-6.0-4`, e.g. Debian 13 / Ubuntu 24.04 or newer); `apt` installs them automatically.

> **Upgrading from LinXTerm:** LeanTerm was previously called LinXTerm. The package replaces `linxterm`, and on first start `~/.config/linxterm` is moved to `~/.config/leanterm` (including the keyring entry), so all connections are kept.

To build the `.deb` package from source:
```bash
./build/package-deb.sh 0.8.0
```

### Building from Source
**Prerequisites:**
- Go ≥ 1.22
- Node.js ≥ 18 & npm
- GTK4 / WebKitGTK 6.0 development headers (`libgtk-4-dev`, `libwebkitgtk-6.0-dev`)

**Build steps:**
```bash
# 1. Install frontend dependencies
npm --prefix frontend install

# 2. Build frontend production bundle
npm --prefix frontend run build

# 3. Build standalone binary
go build -o bin/leanterm .
```

---

## 💻 CLI Usage

```
leanterm [options] [uuid:cluster ...]

Options:
  --config-dir <dir>      Path to configuration directory (default: ~/.config/leanterm)
  --start-uuid <uuid>     Start connection by UUID (can be repeated or use uuid:cluster)
  --edit-uuid <uuid>      Open connection editor for UUID
  --dump-uuid <uuid>      Dump connection configuration in TOML format
  --list-uuids            List all configured UUIDs and names in tabular format
  --quick-conn            Open Quick Connect dialog
  --start-shell           Launch local terminal shell
  --preferences           Open Preferences dialog
  --readonly              Start in read-only mode (modifications disabled)
  --no-backup             Disable automatic configuration backups
  --password <pwd>        Master password for vault decryption
  --help, -h              Display help and options
```

---

## ⚙️ Configuration Reference (`config.toml`)

LeanTerm stores its configuration in human-readable TOML in `~/.config/leanterm/config.toml`. All secrets are stored separately in `secrets.age`.

### Section `[general]`
| Key | Type | Default | Description |
|---|---|---|---|
| `main_window_layout` | string | `"traditional"` | `"traditional"` (tree on left) or `"compact"` |
| `icons_theme` | string | `"default"` | `"default"`, `"asbru-color"`, `"asbru-dark"`, `"system"` |
| `start_main_maximized` | bool | `false` | Start main window maximized |
| `start_iconified` | bool | `false` | Start minimized to system tray |
| `close_to_tray` | bool | `false` | Minimize to tray instead of closing |
| `confirm_exit` | bool | `true` | Show confirmation prompt before exiting |
| `auto_save` | bool | `true` | Automatically save configuration changes |
| `protect_with_password` | bool | `false` | Master password protection for GUI and secrets |
| `auto_accept_host_keys`| bool | `false` | Automatically accept and store new SSH host keys |

### Section `[terminal]`
| Key | Type | Default | Description |
|---|---|---|---|
| `show_status_bar` | bool | `true` | Show terminal status bar with port forwarding and state |
| `macros_bar` | string | `"buttons"` | Toolbar mode for macros: `"never"`, `"combo"`, `"buttons"` |
| `show_global_macros` | bool | `false` | Display global macros alongside connection macros |
| `confirm_chains` | bool | `true` | Prompt for confirmation when dropping connection chain |
| `skip_first_chain_expect` | bool | `true` | Skip the first Expect entry during chained execution |
| `max_reconnect_attempts` | int | `3` | Max automated reconnection attempts on drop |

### Section `[sftp_browser]`
| Key | Type | Default | Description |
|---|---|---|---|
| `auto_open` | bool | `true` | Automatically open SFTP browser on SSH connections |
| `width` | int | `340` | Default width of the SFTP panel in pixels |
| `follow_terminal_folder`| bool | `false` | Follow terminal remote folder changes via OSC 7 |
| `show_hidden` | bool | `false` | Show hidden (dot) files in browser |
| `confirm_delete` | bool | `true` | Prompt for confirmation before deleting files |
| `parallel_transfers` | int | `3` | Maximum concurrent SFTP file transfers (1–8) |
| `poll_seconds` | int | `0` | Polling interval in seconds (0 = disabled) |

### Section `[keepass]`
| Key | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Enable KeePassXC password manager integration |
| `database` | string | `""` | Path to `.kdbx` database file |
| `key_file` | string | `""` | Optional path to key file |

### Connection Definition `[[connections]]`
```toml
[[connections]]
id = "c1b2c3d4-..."
name = "Production Web Server"
title = "Prod Web 01"
method = "ssh" # "ssh" | "telnet"
host = "192.168.1.100"
port = 22
auto_reconnect = true

[connections.auth]
type = "password" # "password" | "publickey" | "manual"
user = "deploy"

[connections.ssh]
forward_x11 = true
forward_agent = true
gateway_ports = false

[[connections.expect]]
pattern = "[#$] "
send = "uptime"
append_return = true
on_match = "next"
```

---

## 🧪 Testing

To execute all Go unit tests:
```bash
go test ./internal/...
```

To run frontend Vitest suites and TypeScript checks:
```bash
npm --prefix frontend test -- --run
npm --prefix frontend run check
```

---

## 📄 License
LeanTerm is licensed under the **GNU General Public License v3.0 (GPLv3)**.
