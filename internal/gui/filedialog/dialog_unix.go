//go:build !windows

package filedialog

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

func choose(o Options) (string, error) {
	p, err := portal(o)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, errNoPortal) {
		return "", err
	}
	if z, err := exec.LookPath("zenity"); err == nil {
		return run(exec.Command(z, zenityArgs(o)...))
	}
	if k, err := exec.LookPath("kdialog"); err == nil {
		return run(exec.Command(k, kdialogArgs(o)...))
	}
	return "", ErrUnavailable
}

// run returns the dialog's output; exit code 1 means cancelled.
func run(cmd *exec.Cmd) (string, error) {
	out, err := cmd.Output()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func zenityArgs(o Options) []string {
	args := []string{"--file-selection", "--title=" + o.Title}
	start := o.Directory
	if start != "" && !strings.HasSuffix(start, "/") {
		start += "/"
	}
	if o.Save && o.Filename != "" {
		start = filepath.Join(o.Directory, o.Filename)
	}
	if start != "" {
		args = append(args, "--filename="+start)
	}
	switch {
	case o.Save:
		args = append(args, "--save", "--confirm-overwrite")
	case o.Folder:
		args = append(args, "--directory")
	}
	return args
}

func kdialogArgs(o Options) []string {
	start := o.Directory
	if start == "" {
		start = "."
	}
	switch {
	case o.Save:
		if o.Filename != "" {
			start = filepath.Join(start, o.Filename)
		}
		return []string{"--title", o.Title, "--getsavefilename", start}
	case o.Folder:
		return []string{"--title", o.Title, "--getexistingdirectory", start}
	}
	return []string{"--title", o.Title, "--getopenfilename", start}
}

var errNoPortal = errors.New("no desktop portal")

// portal asks org.freedesktop.portal.FileChooser and waits for the
// request's Response signal.
func portal(o Options) (string, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return "", errNoPortal
	}
	defer conn.Close()
	token := fmt.Sprintf("leanterm%d", time.Now().UnixNano())
	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")
	reqPath := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(reqPath), dbus.WithMatchInterface("org.freedesktop.portal.Request"), dbus.WithMatchMember("Response")); err != nil {
		return "", errNoPortal
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)

	opts := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
		"modal":        dbus.MakeVariant(true),
	}
	if o.Directory != "" {
		opts["current_folder"] = dbus.MakeVariant(append([]byte(o.Directory), 0))
	}
	method := "org.freedesktop.portal.FileChooser.OpenFile"
	if o.Save {
		method = "org.freedesktop.portal.FileChooser.SaveFile"
		if o.Filename != "" {
			opts["current_name"] = dbus.MakeVariant(o.Filename)
		}
	} else if o.Folder {
		opts["directory"] = dbus.MakeVariant(true)
	}
	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	var handle dbus.ObjectPath
	if err := obj.Call(method, 0, "", o.Title, opts).Store(&handle); err != nil {
		return "", errNoPortal
	}
	for sig := range signals {
		if sig.Path != reqPath && sig.Path != handle || len(sig.Body) < 2 {
			continue
		}
		code, _ := sig.Body[0].(uint32)
		if code != 0 {
			return "", nil // cancelled
		}
		results, _ := sig.Body[1].(map[string]dbus.Variant)
		if uris, ok := results["uris"].Value().([]string); ok && len(uris) > 0 {
			return pathFromURI(uris[0]), nil
		}
		return "", nil
	}
	return "", errNoPortal
}
