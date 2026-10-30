//go:build !windows

package keepass

// cliPath is the KeePassXC command line tool, found through PATH.
func cliPath() string { return "keepassxc-cli" }
