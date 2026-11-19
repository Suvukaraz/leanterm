# Prompts für die restlichen Etappen der Gio-Migration

Jede Etappe ist ein eigener Auftrag. Zuerst den **gemeinsamen Kontext** in den
Prompt kopieren, dann den Block der Etappe. Die Etappen bauen aufeinander auf:
3 → 4 → 5/6 (parallel möglich) → 7 → 8.

---

## Gemeinsamer Kontext (vor jeden Etappen-Prompt setzen)

```
Du arbeitest am Repository Suvukaraz/leanterm (Go). LeanTerm wird von Wails 3 +
Svelte (Web-Oberfläche in WebKitGTK/WebView2) auf eine reine Go-Oberfläche mit
Gio (gioui.org v0.10.x) umgestellt. Die neue Oberfläche soll optisch 1:1 wie die
alte aussehen und sich genauso verhalten.

Lies zuerst docs/gio-migration.md: Dort stehen der Plan, der Stand der Etappen
0–2 und alle bisherigen Erkenntnisse (Gio-Eigenheiten, Engine-Lücken,
Windows-Fallen).

Bereits vorhanden:
- internal/gui/term: Terminal-Engines hinter dem Interface Engine. Ghostty über
  go.mitchellh.com/libghostty (Standard, Build-Tag `ghostty`, braucht
  libghostty-vt, gebaut mit build/gio/build-libghostty.sh und Zig 0.17) und
  charmbracelet/x/vt (Alternative, reines Go). engine_test.go enthält
  Konformitätstests für beide Engines.
- internal/gui/termview: Gio-Terminal-Widget; Session bindet es an den
  SessionManager.
- internal/session: lokale Shells über charmbracelet/x/xpty; Subscribe und
  WriteBytes liefern bzw. nehmen Terminal-Bytes ohne Web-Events.
- internal/events (Event-Bus), internal/services (Host, EditorWindows; die
  Services importieren kein Wails mehr), internal/gui/appcore (baut das Backend
  für eine GUI), internal/gui/dialogs (Prompter für Passwörter, Host-Keys,
  Auswahl).
- internal/gui/gtk: alle GTK-Bausteine der Web-Oberfläche in Gio (Button,
  CheckButton, RadioGroup, Entry, SpinButton, Combo, Frame, ProgressBar, Scale,
  Statusbar, Notebook, Paned, Dialog, MessageDialog, Menu, Tree, ListBox,
  Color-, Font- und FileChooser). Farben aus gtk.css (hell/dunkel), Icons als
  Nerd-Font-Symbole (internal/gui/fonts, fonts.Icon*).
- internal/gui/codeedit: gvcode-Editor mit chroma-Highlighting.
- cmd/leanterm-gio: Prototyp-Fenster. cmd/gtk-gallery: Gio-Galerie (-out
  datei.png rendert headless mit EGL_PLATFORM=surfaceless).
  frontend/gallery.html: die Svelte-Referenzgalerie (nur Vite-Dev-Server).
- .github/workflows/gio.yml: Linux und Windows, beide Engines, Tests, GUI-Lauf
  unter Xvfb, Windows-Drop-Selbsttest, Wächter gegen Wails-Importe in
  internal/.

Vorbild für jedes Fenster ist die Svelte-Komponente unter frontend/src/
(components/, lib/stores, lib/terminal, lib/editor) zusammen mit dem
Go-Service, den sie aufruft. Übernimm Größen, Abstände, Farben, Texte,
Tastenkürzel und Verhalten von dort. Erfinde nichts Neues, lass nichts weg und
baue Widgets aus internal/gui/gtk nicht ein zweites Mal.

Arbeitsweise:
- Neue Gio-Widgets bekommen Interaktionstests über Gios input.Router, wie in
  internal/gui/gtk/widgets_test.go (Harness `ui`: frame/click/press/keys/
  pointer).
- Prüfe jedes neue Fenster im Vergleich: Nimm den Bereich in beide Galerien auf
  (frontend/src/gallery/Gallery.svelte und cmd/gtk-gallery), mach von beiden
  einen Screenshot (Chromium/Playwright bzw. -out) und miss die Abweichung.
  Ziel: höchstens 1–3 px.
- Achte auf die bekannten Gio-Fallen (siehe Doku):
  - key.Filter braucht die Modifier als Optional.
  - Den Fokus gibt es nur mit key.FocusFilter.
  - Die Eingabefläche eines Containers muss vor denen seiner Kinder liegen.
  - Inset zieht die Ränder nicht von Min ab, und Flex reicht die minimale
    Querhöhe an die Kinder weiter.
  - Unter Windows dürfen Win32-Aufrufe, die dem Fenster Nachrichten schicken,
    nicht in der Event-Behandlung laufen.
- Das Backend bleibt frei von GUI-Code: Unter internal/ (außer internal/gui)
  kein Gio und kein Wails. Fehlt der GUI etwas, erweitere services.Host bzw.
  die Services, nicht die GUI-Schicht im Backend.
- Die alte Wails-Oberfläche muss bis Etappe 8 weiter bauen und laufen.
- Vor jedem Commit: gofmt, go vet und go test -race ./internal/... Prüfe nach
  dem Push, ob gio.yml grün ist. libghostty wird nur in der CI gebaut; ohne
  Build-Tag läuft lokal die vt-Engine.
- Kleine, thematisch saubere Commits. Ergänze am Ende jeder Etappe einen
  Abschnitt „Stage N“ mit den Erkenntnissen in docs/gio-migration.md.
- Melde zum Schluss: Was ist fertig, was weicht vom Original ab und warum, was
  ist offen.
```

---

## Etappe 3: Hauptfenster

```
Etappe 3: Baue das Hauptfenster von LeanTerm in Gio nach. Vorbild:
frontend/src/components/main/ (MainWindow, ConnectionPanel, ConnectionTree,
CommandPanel, MainButtonBar, AddRenameNode, DeleteConfirmDialog, QuitConfirm,
About), frontend/src/components/terminal/ (TerminalTab, SplitTab,
TabContextMenu) und die Stores frontend/src/lib/stores/ (ui, uistate,
sessions, splits, config, clusters).

Umfang:
- Neues Programm cmd/leanterm-native (oder cmd/leanterm-gio ausbauen). Es baut
  das Backend mit internal/gui/appcore und implementiert services.Host für Gio
  (Beenden, Dateidialoge; native Dateidialoge z. B. über zenity/xdg-portal
  unter Linux und IFileDialog unter Windows, oder vorerst den Gio-FileChooser).
- Layout wie MainWindow.svelte: linke Seitenleiste mit Verbindungsbaum
  (gtk.Tree mit den echten Daten aus der Config: Gruppen, Favoriten, Suche,
  Drag&Drop im Baum, Kontextmenü mit Verbinden, Bearbeiten, Umbenennen,
  Löschen, Duplizieren usw.), Command-Panel, Button-Leiste, Terminal-Notebook
  rechts, Statusleiste und die Paned-Trennung mit gespeicherter Breite.
- Terminal-Tabs: Status-Icons und -Farben, Schließen, Mittelklick,
  Tab-Kontextmenü, Umbenennen, Tabs verschieben, Split-Ansichten wie
  SplitTab/splits.ts.
- Fenstergeometrie, Seitenleiste und offene Zustände wie uistate speichern und
  wiederherstellen.
- Die Dialoge Hinzufügen/Umbenennen, Löschbestätigung, Beenden-Bestätigung bei
  offenen Sitzungen und Über.
- Globale Tastenkürzel wie in der Web-Oberfläche (Keybindings aus der Config).
- Hell/dunkel nach General.ColorScheme, live umschaltbar.
- internal/gui/dialogs.Prompter auf gtk.Dialog/gtk.MessageDialog umstellen.

Fertig ist die Etappe, wenn man mit dem neuen Programm Verbindungen aus einer
echten Config öffnen kann (lokal, SSH, Telnet), wenn Tabs und Splits wie im
Original funktionieren und wenn der Vergleich des Hauptfensters mit dem
Original passt. Nimm einen GUI-Lauf des neuen Programms unter Xvfb in gio.yml
auf.
```

---

## Etappe 4: Terminal-Widget vervollständigen

```
Etappe 4: Bring internal/gui/termview auf den Funktionsumfang von
frontend/src/components/terminal/TerminalView.svelte und
frontend/src/lib/terminal/ (look.ts, mouseReports.ts, textSearch.ts,
engine.ts) samt der Tests dort.

Umfang:
- Auswahl mit Maus (Zeichen, Wort per Doppelklick, Zeile per Dreifachklick,
  Rechteck mit Alt), Kopieren/Einfügen nach den Config-Optionen
  (copy-on-select, Mittelklick, Rechtsklick-Verhalten), Kontextmenü wie
  TerminalContextMenu.svelte.
- Maus-Reports an die Anwendung (X10, SGR, Motion), genau wie mouseReports.ts;
  übernimm dessen Tests als Go-Tests.
- Suche im Terminal (FindInTerminal.svelte, textSearch.ts) mit Hervorhebung
  und Weiter/Zurück.
- Darstellung nach look.ts: Schrift, Größe, Zeilenhöhe, Farbschema, Cursor-Stil
  und -Blinken, Fett als hell, Transparenz, Padding. Ändert man das in den
  Einstellungen, greift es sofort.
- Hyperlinks (OSC 8 und erkannte URLs, Strg+Klick), Glocke (visuell/akustisch),
  Titel und OSC 7.
- Makro-Leiste (MacrosBar.svelte).
- Fallback-Schriften für CJK und Emoji aus dem System (fontconfig unter Linux,
  DirectWrite-Familien unter Windows), damit kein Tofu erscheint.
- Engine-Wahl pro Verbindung bzw. global (Ghostty Standard, vt als
  Alternative), wie die bisherige Auswahl Ghostty/Rio in der Config; alte
  Config-Werte („rio“) auf vt abbilden.
- Drag&Drop von Dateien aus dem Desktop unter X11 und Wayland: Gio liefert das
  nicht. Baue XDND bzw. wl_data_device-Drop in einem Gio-Fork ein (replace in
  go.mod), Windows ist schon fertig (internal/gui/dnd).
- Leistung: Große Ausgaben (z. B. `yes`, `cat` großer Dateien) dürfen die
  Oberfläche nicht blockieren. Miss und dokumentiere die Bildrate.

Erweitere die Konformitätstests in internal/gui/term, wenn eine Engine Lücken
zeigt, und behandle die Lücke im Adapter.
```

---

## Etappe 5: SFTP-Browser

```
Etappe 5: Baue den SFTP-Browser in Gio nach. Vorbild:
frontend/src/components/sftp/ (SftpPanel, SftpFileList, SftpPathBar,
SftpToolbar, SftpTransferQueue, SftpPermissionsDialog),
frontend/src/lib/stores/sftp.ts und fsBackend.ts sowie die Go-Seite
internal/services/sftpservice.go und localfsservice.go.

Umfang:
- Zweigeteilte Ansicht lokal/entfernt wie im Original, Dateiliste mit
  sortierbaren Spalten, Icons (Nerd Fonts), Mehrfachauswahl, Pfadleiste mit
  Eingabe, Werkzeugleiste, Kontextmenüs.
- Übertragungen per Drag&Drop innerhalb des Fensters und vom Desktop
  (internal/gui/dnd), Warteschlange mit Fortschritt, Abbrechen und
  Konfliktfragen.
- Rechte-Dialog (chmod), Umbenennen, Löschen, neuer Ordner, Öffnen in LeanEdit.
- Die Ereignisse des Services über internal/events, nicht über
  Web-Bridge-Code.
- Die Logik aus sftp.ts mitsamt ihren Tests (sftp.test.ts) nach Go übernehmen.
```

---

## Etappe 6: Dialoge und Nebenfenster

```
Etappe 6: Baue alle übrigen Dialoge und Fenster in Gio nach:
- frontend/src/components/edit/: EditConnectionWindow mit allen Tabs
  (Connection, Terminal, Options, Variables, Macros, Expect, PrePostExec,
  EditableList).
- frontend/src/components/prefs/: PreferencesWindow mit allen Seiten.
- frontend/src/components/dialogs/: BulkEdit, ChainSelection, EditVariables,
  EnterValue, GuiLockOverlay, PipedOutputWindow, PrePostAsk, Prompt,
  QuickConnect, WakeOnLan.
- frontend/src/components/cluster/: ClusterAdminWindow, PCCWindow.

Regeln:
- Fenster, die im Original eigene Fenster sind, werden eigene Gio-Fenster
  (app.NewWindow). Modale Dialoge liegen über dem Hauptfenster. Die
  Dialoggeometrie richtet sich nach frontend/src/lib/dialogGeometry.ts;
  übernimm dessen Tests.
- Validierung und Speichern laufen über dieselben Services wie bisher
  (configservice, clusterservice, dialogservice). Keine Logik doppelt
  schreiben.
- Jedes Formularfeld, jeder Tooltip, jede Fehlermeldung und jede
  Tab-Reihenfolge (Tab/Shift+Tab) wie im Original.
- Für jeden Dialog einen Interaktionstest: öffnen, Werte ändern, speichern,
  abbrechen.
```

---

## Etappe 7: LeanEdit

```
Etappe 7: Baue LeanEdit, den Editor, in Gio nach. Vorbild:
frontend/src/components/editor/ (EditorApp, EditorMenuBar, EditorPane,
EditorStatusBar, QuickOpen, ChoiceDialog, EditorPrefsApp),
frontend/src/lib/editor/ (commands, files, languages, customModes,
navigation, minimap, theme) samt Tests und internal/services/editorservice.go.
Grundlage ist internal/gui/codeedit (gvcode + chroma).

Umfang:
- Implementiere services.EditorWindows für Gio: Editorfenster und
  Einstellungsfenster, Geometrie in UIState speichern, beim Schließen nach
  ungespeicherten Änderungen fragen; Standalone-Modus (Editor als eigene App,
  siehe leanedit.go) beendet beim Schließen die App.
- Menüleiste, Tabs mehrerer Dateien, Split-Panes, Statusleiste (Zeile/Spalte,
  Sprache, Kodierung, Zeilenende, Einrückung), Quick Open, Suchen/Ersetzen,
  alle Befehle und Tastenkürzel aus commands.ts.
- Dateiüberwachung: Beim Fokussieren prüfen, ob sich Dateien auf der Platte
  geändert haben, wie bisher.
- Sprachen und eigene Modi wie languages.ts/customModes.ts mit
  chroma-Lexern.
- Was gvcode nicht kann (Faltung, Blockauswahl, Lesezeichen, Minimap):
  ergänze es in codeedit oder per Fork von gvcode. Wenn etwas nicht geht,
  begründe es und dokumentiere es als Abweichung.
- Öffnen aus SFTP und aus dem Hauptfenster heraus.
```

---

## Etappe 8: Umstellung, Build und Pakete

```
Etappe 8: Mach die Gio-Oberfläche zur einzigen Oberfläche und entferne die
Web-Schicht.

Umfang:
- main.go und leanedit.go starten die Gio-Oberfläche. Entferne Wails
  (wailsgui.go, forwardToWails, Bindings), das frontend/-Verzeichnis, die
  Galerien (oder behalte cmd/gtk-gallery als Entwickler-Werkzeug) und alle
  Wails-Abhängigkeiten aus go.mod. Die Gio-Migration ist dann kein Prototyp
  mehr: Benenne gio.yml um und führe es mit den übrigen Workflows zusammen.
- Build: build/Taskfile.yml, build/config.yml und die Paketskripte
  (package-deb.sh, package-tarball.sh, package-windows.sh) sowie die Workflows
  release.yml, windows.yml und linux-install.yml auf den Gio-Build mit
  libghostty-vt (statisch gelinkt, Build-Tag ghostty) umstellen. Abhängigkeiten
  der .deb-Pakete anpassen (kein WebKitGTK mehr; libEGL, libwayland, libxkb,
  libvulkan je nach Gio-Bedarf).
- App-Icons und Windows-Ressourcen (internal/app/icon_windows.go,
  build/windows), Desktop-Datei und Icons unter Linux.
- Nach Möglichkeit macOS (build/darwin): Gio und libghostty bauen dort.
  Prüfe es in der CI; sonst dokumentieren.
- Migration der Nutzer: Config, UIState und Secrets bleiben kompatibel. Teste
  mit einer Config der letzten Wails-Version.
- README und Doku aktualisieren; docs/gio-migration.md bekommt einen
  Abschluss.
- Zum Schluss: kompletter manueller Durchlauf aller Funktionen (Checkliste in
  der Doku), Paketinstallation in einem frischen Container (Debian/Ubuntu) und
  auf Windows in der CI.
```
