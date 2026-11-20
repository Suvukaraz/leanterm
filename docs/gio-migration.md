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
- `internal/gui/fonts`, `internal/gui/gtk` (then `theme`): system UI/monospace fonts, the
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
- First GTK widgets (button, entry, check button) as the base for stage 2.

## Stage 2: the GTK widgets in Gio

`internal/gui/gtk` draws the web GUI's GTK components
(`frontend/src/components/gtk`) with Gio, in the colors of `gtk.css` (dark and
light; `palette_test.go` checks the tokens against the stylesheet):

- Basics: Button (normal, flat, primary, toggle, with icon), CheckButton,
  RadioGroup, Entry (placeholder, password, read-only), SpinButton, Combo,
  Frame, ProgressBar, Scale, Statusbar, Notebook with closable tabs, Paned.
- Complex widgets: Dialog and MessageDialog with action bar, context Menu
  with submenus, shortcuts and keyboard navigation, Tree (expanders, icons,
  keyboard), ListBox, ColorButton/ColorChooser, FontButton/FontChooser,
  FileChooserButton.
- Icons are Nerd Fonts symbols; expander triangles are drawn as paths.

### Side-by-side check

`frontend/gallery.html` (Vite dev server only, not part of the build) shows
the Svelte components; `cmd/gtk-gallery` shows the same sections with the Gio
widgets (`-out file.png` renders headless, with `EGL_PLATFORM=surfaceless`).
Measured on screenshots of both, heights match and positions differ by at
most 1–3 px.

### Findings

- Gio shapes text at whole pixel sizes (10pt = 13.33px becomes 13px), so text
  was 2.5% narrower than in the web view. `gtk.Text` shapes at a multiple of
  the size that is (nearly) whole and scales the glyphs down.
- Layout differs from CSS in ways that matter for a 1:1 look: an Inset does
  not subtract its padding from the minimum size, a Flex passes its minimum
  cross size on (which stretched labels), flexed children must fill their
  width themselves. CSS `line-height` is emulated for single lines.
- Input: a widget only gets the keyboard focus with a `key.FocusFilter`;
  pointer events go to the topmost area, so a container's own area must come
  before its children's. `widgets_test.go` drives every interactive widget
  through Gio's input router and found several such bugs (disabled buttons
  still clickable, a spin button's keys eaten by its editor, a dialog's Escape
  one frame late, menus' hover overriding the keyboard).

## Stage 3: the main window

`cmd/leanterm-native` is the main window on the backend of
`internal/gui/appcore`: button bar, connection panel (tree, favourites,
history, clusters, search and connect row), the notebook with the Info tab
and terminal tabs (splits, tab menu, reordering), the terminal status bar,
the global shortcuts, the add/rename/delete/quit/about/keybindings dialogs,
the lock overlay and the UI state (window, panel width, expanded groups).
`gio.yml` builds it for Linux and Windows and runs it: under Xvfb with
`build/gio/demo-config` (expand a group, open the tree menu, a local shell,
screenshots as artifacts, `-quit-after`), on Windows start and quit.

### Side-by-side check

The Wails build and `leanterm-native` ran with the same configuration under
Xvfb (the Wails one needs `dbus-run-session`, `GTK_A11Y=none` and
`WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1` there). Button bar, tree,
welcome page, tabs, context menu and terminal status bar now match in
layout, sizes and colors. Remaining differences:

- Icons are Nerd Font glyphs in the main color of the web GUI's colored
  SVGs; glyphs fill their 16px box a little less.
- The web GUI shows a scrollbar under the left notebook's tabs when they do
  not fit (a browser artifact); the native one does not.
- Terminal cells: same width, but the line height and glyph weight differ
  (stage 4 renders the terminal after `look.ts`).

### Findings

- **A system font was shadowed**: the Go fonts were registered under the
  same names as the fontconfig faces, and Gio picked them. The GUI used Go
  instead of Inter (visible in the wide hyphen) and Go Mono instead of
  DejaVu Sans Mono in terminals. `fonts` now only adds a fallback where no
  system face exists; a test checks that no face is registered twice.
- **Redraws**: Gio draws a frame only after an event. A widget that changes
  its state during Layout after drawing, or queues events that the program
  reads before Layout, must ask for another frame
  (`gtx.Execute(op.InvalidateCmd{})`), else the change shows only on the
  next pointer move. `gtk.Tree` and `gtk.Notebook` do so.
- **Widget state must survive frames**: recreating `gtk.Tab`s every frame
  lost their clicks (the left notebook could not switch tabs).
- **Pointer position**: input areas are siblings in Gio, so an area below
  the others never sees the pointer above them. The window-wide pointer
  position for context menus comes from a pass-through area
  (`pointer.PassOp`) on top. Menus close on a click outside through such an
  area too, so the click still reaches its target, as in the web GUI.
- **Windows**: the main window enabled file drops inside event handling,
  which hangs as described in stage 0 (the window never drew). The CI run
  on Windows found it; `-quit-after` now logs all goroutines and exits
  with code 3 instead of hanging.
- **Synthetic clicks**: a press that arrives in the same frame as the
  pointer move onto a widget whose filter was not read yet is lost (xdotool
  `mousemove x y click`); the CI run moves first and presses after a pause.

## Stage 4: the terminal widget

The terminal of the native window now does what TerminalView.svelte and
the web GUI's engine wrappers did:

- **Engines** (`internal/gui/term`): `Modes` (mouse tracking, SGR,
  alternate screen, followed in the output for both engines like
  `mouseReports.ts`), `Lines`/`Line` (any line of history and screen),
  `LinkAt` (OSC 8), `SendRaw`, `Reset`, `Clear`; `EncodeMouse` encodes
  X10 and SGR reports. `Normalize` maps the engine setting: `rio` (the web
  GUI's alternative) is now `vt`. Conformance tests cover both engines.
- **View** (`internal/gui/termview`): selection by characters, words
  (double click, with the look's word characters) and lines (triple click),
  Alt for a rectangle; Ctrl+Shift+C/V, middle click pastes, selection to
  clipboard; Ctrl+= / - / 0 and Ctrl+wheel zoom (6–40pt); mouse reports
  incl. the wheel (Shift still selects), arrow keys for the wheel on the
  alternate screen; Ctrl+click opens OSC 8 links and URLs, underlined
  while Ctrl is held; blinking cursor; the web GUI's inset.
- **Search** (`search.go`, `textSearch.ts` with its tests) and the find bar
  (Ctrl+Shift+F, Ctrl+F3; search as you type, Enter / Shift+Enter).
- **Look** (`look.go`, `look.ts` with its tests): the merged look (local
  shell or connection over `[terminal.look]`) sets colors, palette, history,
  font family and size and the cursor; font, size and cursor follow
  settings changes, colors apply to new terminals (as in the web GUI).
- **Native window**: the terminal context menu, the macros bar (buttons or
  combo, with confirmation), notices with "Open folder", screenshots
  (drawn in software: Gio cannot read its window back) and session logs.
- **Fonts**: CJK and emoji come from the system (fontconfig; on Windows
  MS Gothic, Microsoft YaHei, Malgun Gothic, Segoe UI Emoji); color emoji
  render.

### Findings

- **One face per name**: Gio keeps one face per typeface name, weight and
  style. The fallback faces first shared one name and hid each other (no
  CJK); each now has its own. `TestCollectionHasNoDuplicateFaces` guards it.
- **Key order within a frame**: a filter without a focus tag that is read
  earlier in the frame takes keys the focused widget wanted (the window's
  Escape took the find bar's). Global keys skip while a text field has
  the focus.
- **No Insert key**: Gio has no key name for Insert, so Shift+Insert and
  Ctrl+Insert (paste/copy in the web GUI) cannot be supported without a
  change to Gio.
- **x/vt throughput**: x/vt parses plain output at about 0.2 MB/s
  (`BenchmarkFeed`; `seq 1 2000000` takes minutes): every line feed moves
  the whole screen (`DeleteLineArea`) and a full history shifts all its
  lines (`slices.Delete`). The window stays responsive (about 25 frames/s
  with software rendering under Xvfb, `-stats` logs it). Ghostty, the
  default, parses the same output at about 95 MB/s in CI.
- **Drops on X11/Wayland** are still missing: Gio 0.10 has no drop
  handling there (stage 0). It needs XDND client messages and the
  `wl_data_device` drop in a Gio fork plus an event for dropped files;
  Windows works.
- Colors of running terminals do not follow settings changes (ghostty-web
  did not either); the selection keeps the text colors under the
  translucent tint like ghostty-web.

## Stage 5: the file browser

`internal/gui/filebrowser` is the panel beside a terminal
(`frontend/src/components/sftp`): SFTP for SSH tabs, the local file system
for local shells, with one UI.

- `fspath.go` is `fspath.ts` (POSIX, Windows drive and UNC paths, the drive
  list) with its tests.
- `browser.go` is the state of `stores/sftp.ts` over a `Backend` (SFTP or
  local, as `fsBackend.ts`): history, hidden files, following the terminal
  folder, actions, downloads/uploads, local copy/move, conflicts and the
  transfer queue. Backend calls run in goroutines; tests use a fake backend.
- `panel.go`: header, toolbar (back, forward, up, home, refresh, new
  folder, upload, download, ≡ menu), path bar with history and "Follow
  terminal folder", the sortable list (folders first, natural names,
  selection with Ctrl/Shift, double click, type-ahead, F2/F5/F7/Delete/
  Enter/Backspace, context menus), the transfer queue, the footer; dialogs
  for new folder/file, rename, delete, permissions (bits, octal,
  recursive with scope), name conflicts, errors and uploading a file
  changed in its application back.
- The native window shows it left or right of the terminal with its status
  bar, as wide as saved (`sftp_panel_width`), toggled from the button bar,
  the status bar, the terminal menu or Ctrl+Alt+S; files dropped on Windows
  go to the shown browser. Checked against a local sshd: listing, download
  into the download folder, the queue and the footer.

### Findings

- SFTP needs the SSH connection: the browser opens once the session is
  connected (else "no active SSH client").
- Gio's click gesture counts clicks by the events' time: test events need a
  `Time`, else every click is a double click.
- The web GUI's path history button falls back to its CSS default colors
  (#e8e8e8) in the dark theme; the native one draws the same.
- Without a native file dialog yet (stage 6), "Upload files...", "Copy
  to..." and "Move to..." report the missing dialog; drops and downloads
  work.

## Stage 6: dialogs and side windows

- `internal/gui/filedialog` asks the system for files and folders: the
  XDG desktop portal over D-Bus (GNOME, KDE, Wayland), else zenity or
  kdialog; on Windows comdlg32 and the folder browser. The file browser's
  upload, copy to and move to use it, as do all file fields.
- `internal/gui/form` builds the settings dialogs from gtk widgets bound to
  Go fields (entries, checks, spins, combos, radios, colors, fonts, files,
  editable lists with Ctrl+Z, a CSS-like grid, two columns and wrapping
  rows), so a dialog edits a config struct directly.
- `internal/gui/editconn` is the Edit Connection dialog with all its tabs;
  the tree toolbar opens it and adds, renames and deletes nodes.
- `internal/gui/prefs` is the Settings window (`PreferencesWindow.svelte`):
  the ten pages on a tab column on the left (`Notebook.LayoutSide`),
  "Reset to defaults", "Export…", "Close without saving", "Save and Close"
  with the restart hint for options marked (*), and the master password
  dialog. It edits a copy of the settings; saving reloads the window
  through `settings:changed` (colour scheme, looks, tree), and a changed
  panel width replaces the dragged one.
- `internal/gui/dialogs`: Wake On LAN (ARP lookup, magic packet), Quick
  Connect (an unsaved SSH/Telnet/SFTP connection), Edit Local Variables
  (`<V:0>`..`<V:9>`), Bulk Edit (field rules with regex replacement), the
  cluster administration (running clusters, saved clusters, auto cluster
  rules with a preview) and the Power Cluster Controller (send one line or
  a script to a cluster or all terminals, restart/close all). The button
  bar, the tree menu, the cluster buttons and the terminal menu open them.
  Results of blocking service calls reach the dialogs through a channel
  drained on the UI goroutine (`go test -race` checks it).
- Not ported: the chain dialog opens when a connection is dragged from the
  tree onto a terminal, which the native tree does not offer yet; the
  piped output window is never opened by the web GUI either; Enter Value
  belongs to LeanEdit (stage 7).

### Findings

- The web window's sub notebooks (Main/Terminal Options) stack their tabs
  vertically: the left notebook's `.pos-left .notebook-header` rule also
  matches the nested top notebooks. The native window shows them on top,
  as `position="top"` asks.
- WebKitGTK draws `<input type=number>` with an up/down stepper; the native
  spin buttons have it too (clickable). Check labels wrap in narrow grid
  columns, select boxes and font buttons stretch in grid cells, like CSS.
- Gio input areas that are siblings take events from each other: a hover
  area must be inside the button's (else the tab column ignores clicks).
- The terminal engine choice offers Ghostty and the pure Go VT engine; the
  web GUI's "rio" value means VT.
- Gio blends alpha in linear light: text at CSS opacity 0.5 came out much
  brighter than in the browser (173 instead of 140 on the dark theme).
  Disabled buttons and menu items mix their text with the background in
  sRGB instead (`gtk.dim`). Check labels stay unfaded, as in the web GUI.
- `form.Entry` did not pass `Password` on: password fields (also in Edit
  Connection) showed their text. Fixed, with a test.
- The web cluster window's "Saved clusters" tab reads connections from the
  settings, which have none, and "Save Rule" puts the rule into settings
  that `SaveSettings` drops: the native window lists the connections with
  their clusters and saves rules to `auto_clusters`.
- A dialog's Escape set the close request after the caller had read it;
  the dialog now asks for another frame, so it closes at once.

## Stage 7: LeanEdit

`internal/gui/leanedit` is the editor (`frontend/src/components/editor` and
`lib/editor`) on gvcode. `third_party/gvcode` is a copy of gvcode v0.8.2
(MIT) used through a `replace` in go.mod; LeanTerm's changes are marked
"LeanTerm:" (exported Undo/Redo, the gutter's background and border,
whitespace markers).

- Tabs, loading and saving through `EditorService` in chunks (encodings,
  line endings, mixed line endings, large/huge and binary files, saving
  over a change by another program, a closed SFTP connection), checking
  the open files when the window gets the focus, the unsaved-changes
  questions on closing tabs and the window (`ClosingEvent.Abort`).
- Menu bar, tool bar, tab bar, status bar (position, INS/LINE, indent,
  encoding and language popups, line endings) as in EditorApp.svelte.
- `commands.ts` as pure Go functions with its test cases, applied as one
  undoable edit; the search panel (case, regexp, by word, replace with
  groups, highlighted matches), go to line, quick open, Align On,
  bookmarks, modified lines, the jump and edit histories.
- `languages.ts` (detection by name, path, extension and shebang) with
  chroma lexers; custom lexers for SSH config, crontab, ignore files and
  whitespace-separated config.
- View: word wrap, whitespace, the minimap (`minimap.ts`: 80px, 3px per
  line, syntax colors, slider, click/drag/wheel) or a scrollbar, zoom.
- The settings window ("LeanEdit Settings", the Editor page with Cancel,
  Apply, OK) in its own window; `-leanedit [files]` runs LeanEdit on its
  own (closing quits, settings go straight to config.toml).
- Files open from `EditorService.Open`, the file browser's "Open in
  Editor" and double click on text files, and files dropped on the main
  window.

### KWrite defaults

LeanEdit's defaults follow KWrite (KTextEditor's `kateconfig.cpp`): the
fixed system font at 10pt, tab width 4 with spaces, indentation detected
from the file, dynamic word wrap on, tabs shown ("»") but not spaces, the
scrollbar minimap (60px), trailing spaces removed on modified lines and a
line break added at the end of the file when saving, and a window per
start (no single instance). `show_tabs`, `auto_detect_indent`,
`remove_trailing_spaces` and `newline_at_eof` are new in `[editor]`; only
the native GUI uses them (the web GUI ignores them). Existing
configuration files keep the values they have written.

### Deviations

- gvcode has one cursor: no block selection, no extra carets (Add Caret
  above/below, Cursors to Line Ends, Select All Occurrences are disabled);
  "Find and Select Next Occurrence" moves the selection.
- No code folding (Fold All/Unfold All are disabled) and no overwrite mode
  (the status bar shows INS/LINE).
- Syntax highlighting has colors only: gvcode shapes a line with one face,
  so keywords are not bold and comments not italic.
- Modified lines are the lines between the common start and end of the
  saved and the current text (CodeMirror tracks each edit); for single
  edits that is the same.
- LeanEdit on its own does not yet hand files to a running instance
  (single instance) and has no own icon; that is stage 8 packaging.

### Findings

- A menu that closes in the frame an entry is chosen lost the choice: the
  open menu's index was already reset; the menu bar remembers the menu it
  showed.
- Windows opened before the main window is mapped end up below it on X11
  without a window manager; `-edit` opens its file a second later.
