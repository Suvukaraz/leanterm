package services

// FileDialog describes a native file chooser.
type FileDialog struct {
	Title string
	// Directory is the folder the dialog starts in ("" = the GUI's choice).
	Directory string
	// Filename is the suggested name of a save dialog.
	Filename string
	// Save asks for a file to write (with "replace?" confirmation).
	Save bool
	// Folder chooses a folder instead of a file (open dialogs).
	Folder bool
	// ShowHidden lists hidden files.
	ShowHidden bool
	// Window names the window the dialog belongs to: "" for the current
	// window, EditorWindowName for the editor.
	Window string
}

// Host is the GUI the services run in: it shows native dialogs and ends the
// application. The web GUI (Wails) and the native GUI implement it.
type Host interface {
	// ChooseFile shows a native file chooser and returns the chosen path, or
	// "" when the user cancelled.
	ChooseFile(d FileDialog) (string, error)
	// Quit ends the application.
	Quit()
}

// EditorWindows shows LeanEdit's windows; the GUI implements it. When the
// editor window has closed, the GUI calls EditorService.WindowClosed.
type EditorWindows interface {
	// ShowEditor creates the editor window or raises it.
	ShowEditor() error
	// CloseEditor closes the editor window without asking again.
	CloseEditor()
	// EditorShown reports whether the editor window is open.
	EditorShown() bool
	// ShowEditorPrefs opens LeanEdit's settings window or raises it.
	ShowEditorPrefs() error
	// CloseEditorPrefs closes the settings window.
	CloseEditorPrefs()
}

// EditorWindowName names the editor window (see FileDialog.Window).
const EditorWindowName = "editor"
