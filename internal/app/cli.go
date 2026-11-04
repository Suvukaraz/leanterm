package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/pelletier/go-toml/v2"
)

type CLIFlags struct {
	Help       bool
	ConfigDir  string
	TempDir    string
	NoBackup   bool
	StartShell bool
	Password   string
	StartUUIDs []string // id or id:cluster
	EditUUID   string
	// EditFiles are local files to open in the built-in editor (--edit).
	EditFiles []string
	// LeanEdit starts only the text editor, as its own application (--leanedit).
	LeanEdit bool
	// NewWindow makes LeanEdit open its own window even when it runs as a single
	// instance (--new-window, File → New Window).
	NewWindow bool
	// Files are the arguments without a leading dash; LeanEdit opens them.
	Files       []string
	DumpUUID    string
	ListUUIDs   bool
	Preferences bool
	QuickConn   bool
	NoSplash    bool
	ReadOnly    bool
	Verbose     bool
	UnknownArgs []string
}

func ParseCLI(args []string) *CLIFlags {
	flags := &CLIFlags{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			flags.Files = append(flags.Files, arg)
			continue
		}
		if arg == "--" {
			// Everything after "--" is a file name, even when it starts with a dash.
			flags.Files = append(flags.Files, args[i+1:]...)
			break
		}

		// Normalize single leading dash to double dash if not already
		clean := strings.TrimLeft(arg, "-")
		parts := strings.SplitN(clean, "=", 2)
		name := strings.ToLower(parts[0])
		hasVal := len(parts) == 2
		val := ""
		if hasVal {
			val = parts[1]
		}

		switch name {
		case "h", "help", "?":
			flags.Help = true
		case "config-dir":
			if hasVal {
				flags.ConfigDir = val
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags.ConfigDir = args[i]
			}
		case "temp-dir":
			if hasVal {
				flags.TempDir = val
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags.TempDir = args[i]
			}
		case "no-backup":
			flags.NoBackup = true
		case "start-shell":
			flags.StartShell = true
		case "password":
			if hasVal {
				flags.Password = val
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags.Password = args[i]
			}
		case "start-uuid":
			if hasVal {
				flags.StartUUIDs = append(flags.StartUUIDs, val)
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags.StartUUIDs = append(flags.StartUUIDs, args[i])
			}
		case "edit-uuid":
			if hasVal {
				flags.EditUUID = val
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags.EditUUID = args[i]
			}
		case "dump-uuid":
			if hasVal {
				flags.DumpUUID = val
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags.DumpUUID = args[i]
			}
		case "edit":
			if hasVal {
				flags.EditFiles = append(flags.EditFiles, val)
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags.EditFiles = append(flags.EditFiles, args[i])
			}
		case "leanedit":
			flags.LeanEdit = true
		case "new-window":
			flags.NewWindow = true
		case "list-uuids":
			flags.ListUUIDs = true
		case "preferences", "settings":
			flags.Preferences = true
		case "quick-conn":
			flags.QuickConn = true
		case "no-splash":
			flags.NoSplash = true
		case "readonly", "read-only":
			flags.ReadOnly = true
		case "verbose", "v":
			flags.Verbose = true
		default:
			flags.UnknownArgs = append(flags.UnknownArgs, arg)
			fmt.Fprintf(os.Stderr, "INFO: bad start up parameter %s\n", arg)
		}
	}

	return flags
}

func PrintUsage() {
	usage := `LeanTerm - Advanced Connection Manager
Usage: leanterm [OPTIONS]

Options:
  --help, -h                  Show this help message and exit
  --config-dir=<path>         Path to configuration directory
                              (default: ~/.config/leanterm, Windows: %APPDATA%\LeanTerm)
  --temp-dir=<path>           Path to temporary directory
  --no-backup                 Do not create configuration backups on startup
  --start-shell               Open a local shell connection on startup
  --password=<pwd>            Master / GUI password to unlock stored credentials
  --start-uuid=<id>[:<clust>] Start the connection with the given UUID (can be repeated)
  --edit-uuid=<id>            Open the edit window for the connection with UUID <id>
  --edit=<file>               Open a local file in the built-in editor (can be repeated)
  --leanedit [<file>...]      Start only the text editor LeanEdit, as its own application
                              (independent of a running LeanTerm); with "single instance"
                              in its settings, files open as tabs of the running LeanEdit
  --new-window                LeanEdit: open a new window even in single instance mode
  --dump-uuid=<id>            Print configuration for UUID <id> as TOML and exit
  --list-uuids                Print table of all connections and groups (UUID/Type/Method/Name)
  --settings                  Open the Settings window on startup (also --preferences)
  --quick-conn                Open the Quick Connect window on startup
  --no-splash                 Disable splash/greeting on startup
  --readonly                  Start in read-only mode (no configuration changes saved)
  --verbose, -v               Enable debug logging
`
	fmt.Print(usage)
}

func DumpUUID(cfg *config.Config, id string) error {
	for _, conn := range cfg.Connections {
		if conn.ID == id {
			data, err := toml.Marshal(conn)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}
	}
	return fmt.Errorf("connection '%s' not found", id)
}

func ListUUIDs(cfg *config.Config) {
	fmt.Printf("%-36s|%-36s|%-10s|%-8s|%s\n", "UUID", "PARENT_UUID", "TYPE", "METHOD", "NAME")
	fmt.Println("------------------------------------+------------------------------------+----------+--------+--------------------")
	for _, g := range cfg.Groups {
		fmt.Printf("%-36s|%-36s|%-10s|%-8s|%s\n", g.ID, g.Parent, "group", "", g.Name)
	}
	for _, c := range cfg.Connections {
		fmt.Printf("%-36s|%-36s|%-10s|%-8s|%s\n", c.ID, c.Parent, "connection", c.Method, c.Name)
	}
}
