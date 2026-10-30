package session

import (
	"bytes"
	"net/url"
	"regexp"
	"strings"
)

// cwdScanner finds working-directory reports in terminal output: OSC 7
// ("ESC ] 7 ; file://host/path") and OSC 9;9 ("ESC ] 9 ; 9 ; path", Windows
// shells). A sequence split across two reads is kept until it is complete.
type cwdScanner struct {
	pending []byte
}

// maxPendingOSC bounds the carried-over tail of an unterminated sequence.
const maxPendingOSC = 4096

var driveURLPath = regexp.MustCompile(`^/[A-Za-z]:(/|$)`)

// Scan returns the last folder reported in data ("" when there is none).
func (c *cwdScanner) Scan(data []byte) string {
	buf := data
	if len(c.pending) > 0 {
		buf = append(c.pending, data...)
		c.pending = nil
	}
	cwd := ""
	for {
		start := bytes.Index(buf, []byte("\x1b]"))
		if start < 0 {
			return cwd
		}
		body := buf[start+2:]
		end, termLen := oscEnd(body)
		if end < 0 {
			if len(body) < maxPendingOSC {
				c.pending = append([]byte(nil), buf[start:]...)
			}
			return cwd
		}
		if dir := cwdFromOSC(string(body[:end])); dir != "" {
			cwd = dir
		}
		buf = body[end+termLen:]
	}
}

// oscEnd finds BEL or ST (ESC \) and returns its index and length.
func oscEnd(b []byte) (int, int) {
	for i := 0; i < len(b); i++ {
		if b[i] == 0x07 {
			return i, 1
		}
		if b[i] == 0x1b && i+1 < len(b) && b[i+1] == '\\' {
			return i, 2
		}
		if b[i] == 0x1b && i+1 == len(b) {
			return -1, 0
		}
	}
	return -1, 0
}

func cwdFromOSC(body string) string {
	switch {
	case strings.HasPrefix(body, "9;9;"):
		return strings.Trim(body[len("9;9;"):], `"`)
	case strings.HasPrefix(body, "7;file://"):
		rest := body[len("7;file://"):]
		slash := strings.Index(rest, "/")
		if slash < 0 {
			return ""
		}
		p := rest[slash:]
		if decoded, err := url.PathUnescape(p); err == nil {
			p = decoded
		}
		// file://host/C:/Users/me → C:\Users\me
		if driveURLPath.MatchString(p) {
			p = strings.ReplaceAll(p[1:], "/", `\`)
			if len(p) == 2 {
				p += `\`
			}
		}
		return p
	}
	return ""
}
