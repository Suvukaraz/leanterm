# Moving LeanTerm to a pure Go GUI (Gio)

LeanTerm's GUI moves from the web layer (Wails, Svelte, WebKitGTK/WebView2)
to [Gio](https://gioui.org) with the same look. The terminal engines change at
the same time:

| | Before | After |
|---|---|---|
| GUI | Wails 3 + Svelte in a web view | Gio, drawn in Go |
| Default terminal engine | Ghostty via ghostty-web (WASM) | Ghostty via [libghostty](https://pkg.go.dev/go.mitchellh.com/libghostty) (libghostty-vt, cgo) |
| Alternative engine | Rio via rioterm (WASM) | [charmbracelet/x/vt](https://pkg.go.dev/github.com/charmbracelet/x/vt) (pure Go) |
| PTY | creack/pty, own ConPTY code | [charmbracelet/x/xpty](https://pkg.go.dev/github.com/charmbracelet/x/xpty) |
| Editor (LeanEdit) | CodeMirror 6 | [gvcode](https://github.com/oligo/gvcode) + chroma |
| Icons | SVG icon themes | [Nerd Fonts](https://github.com/ryanoasis/nerd-fonts) symbols (embedded) |

## Stage 0: prototype

`cmd/leanterm-gio` is a prototype window with a local shell tab per terminal
engine and a LeanEdit tab. It checks the risky parts before the GUI is
rebuilt:

- `internal/gui/term`: the engine-neutral terminal core. `Engine` is the
  interface the GUI talks to; `vt.go` wraps x/vt, `ghostty.go` (build tag
  `ghostty`) wraps libghostty. `engine_test.go` runs the same conformance tests
  against every compiled-in engine: colors, attributes, wide characters, keys
  (incl. application cursor mode), bracketed paste, query answers (DA, CPR),
  title and OSC 7, scrollback and resize.
- `internal/gui/termview`: the Gio terminal widget (cell grid, glyphs snapped to
  the grid, cursor shapes, keyboard and IME text, wheel scrolling, middle-click
  and Ctrl+Shift+V paste, resize).
- `internal/session`: local shells now run on x/xpty (PTY and ConPTY), used by
  the current app as well.
- `internal/gui/codeedit`: gvcode editor with chroma highlighting in LeanTerm's
  editor colors and LeanEdit's status bar fields.
- `internal/gui/fonts`, `internal/gui/theme`: system UI/monospace fonts, the
  embedded Nerd Fonts symbols (`fonts.Icon*`), the GTK color tokens of
  `frontend/src/lib/theme/gtk.css`, notebook tabs and status bar.
- `internal/gui/dnd`: files dropped from the desktop (Windows: `WM_DROPFILES`).

### Building

The vt engine needs nothing special. The Ghostty engine needs libghostty-vt,
built with Zig 0.17 (`pip install ziglang==0.17.1` provides it):

```sh
build/gio/build-libghostty.sh x86_64-linux-gnu.2.34 /tmp/ghostty-linux
PKG_CONFIG_PATH=/tmp/ghostty-linux/lib/pkgconfig go build -tags ghostty ./cmd/leanterm-gio
```

On Linux Gio needs the X11/Wayland/EGL headers (see `.github/workflows/gio.yml`).
Windows builds need no cgo without Ghostty; with Ghostty they are
cross-compiled with `zig cc -target x86_64-windows-gnu` (see the workflow).

### Findings

- **Ghostty engine**: libghostty-vt only depends on libc and links statically;
  Zig cross-compiles it for Windows. Its Go API is not stable yet (the module
  says so), so updates need the conformance tests.
- **x/vt**: works, with three gaps handled in the adapter: its `Close` races
  with `Read` (the pipe is closed instead), pastes do not turn line breaks into
  CR, and the end-of-paste sequence is not filtered.
- **x/xpty**: on Unix it neither makes the shell a session leader nor gives it
  the PTY as controlling terminal, and keeps the child side open; LeanTerm now
  does both (`prepareCmd`, `afterStart`), else Ctrl+C and job control do not
  work and the end of the shell is not noticed.
- **Signals**: a LeanTerm started with SIGINT ignored (`&` from a script,
  `nohup`) passed that on to its shells, so Ctrl+C did nothing there. The
  session package now catches such signals so exec resets them for shells.
- **Gio keyboard**: printable keys come as key events *and* as text input; only
  the text is sent, and Space only counts as a key with Ctrl/Alt. Key filters
  must list the modifiers as optional, else Ctrl/Alt combinations never arrive.
  `termview/view_test.go` covers this through Gio's input router.
- **gvcode**: line numbers appear from the second frame on (the gutter is
  measured from the previous layout); the editor asks for that frame.
  Folding, block selection, bookmarks and the minimap are not part of gvcode.
- **Drag and drop from the desktop**: Gio 0.10 does not deliver dropped files
  on any desktop platform (its X11/Wayland handlers are empty). Windows works
  by subclassing the window (`internal/gui/dnd`, checked in CI with a real
  `WM_DROPFILES`). On Windows Gio's window thread waits while the program
  handles an event, so Win32 calls that message the window (subclassing,
  `DragAcceptFiles`) must run outside event handling, else both block.
  **X11 and Wayland need a change in Gio itself** (XDND client
  messages; `wl_data_device` drop), as a fork via `replace` or upstream.
- **Fonts**: Gio does not use fontconfig; LeanTerm loads the system UI and
  monospace fonts itself. CJK and emoji need fallback faces from the system
  (not done yet). Nerd Font glyphs fall back to the embedded symbols font.

## Stage 1: the backend without the web layer

The backend no longer depends on Wails; the web GUI and the native GUI use it
the same way. The Gio workflow fails if anything under `internal/` imports
Wails again.

- `internal/events`: the event bus and the list of all events with their
  payloads. `main.go` subscribes the web frontend to every event
  (`forwardToWails`); native GUIs subscribe to what they show.
- Terminal I/O for native GUIs: `SessionManager.Subscribe` delivers a
  session's output as bytes (the ring buffer first, then live), without the
  web GUI's base64 events and acknowledgements; `WriteBytes` and `Resize`
  take input and size.
- `services.Host` (dialogs, quit) and `services.EditorWindows` (LeanEdit's
  windows) are what the services need from a GUI; the Wails implementation is
  `wailsgui.go`.
- `internal/gui/appcore` builds the backend for a GUI (configuration,
  secrets, event bus, sessions, clusters, KeePass) as `main.go` does for
  Wails.
- `internal/gui/termview.Session` binds a terminal tab to a session;
  `internal/gui/dialogs.Prompter` answers the backend's questions (passwords,
  host keys, choices) with modal dialogs. The prototype opens local shells
  and saved connections (`-connect name,...`, `-config-dir`) through the real
  session manager; an SSH login with a password dialog was checked against
  a local sshd.
- `internal/gui/theme`: first GTK widgets (button, entry, check button) as
  the base for stage 2.
