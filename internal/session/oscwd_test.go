package session

import "testing"

func TestCwdScanner(t *testing.T) {
	var c cwdScanner
	if got := c.Scan([]byte("hello \x1b]9;9;C:\\Users\\me\x1b\\PS C:\\Users\\me> ")); got != `C:\Users\me` {
		t.Fatalf("OSC 9;9 = %q", got)
	}
	if got := c.Scan([]byte("\x1b]9;9;\"D:\\with space\"\x07")); got != `D:\with space` {
		t.Fatalf("quoted OSC 9;9 = %q", got)
	}
	if got := c.Scan([]byte("\x1b]7;file://host/home/u/my%20dir\x07$ ")); got != "/home/u/my dir" {
		t.Fatalf("OSC 7 = %q", got)
	}
	if got := c.Scan([]byte("\x1b]7;file://pc/C:/Users/me\x07")); got != `C:\Users\me` {
		t.Fatalf("OSC 7 with drive = %q", got)
	}
	// Other OSC sequences (window title) are ignored.
	if got := c.Scan([]byte("\x1b]0;title\x07plain text")); got != "" {
		t.Fatalf("title = %q", got)
	}
	// A sequence split across reads.
	if got := c.Scan([]byte("out \x1b]9;9;C:\\Pro")); got != "" {
		t.Fatalf("partial = %q", got)
	}
	if got := c.Scan([]byte("gram Files\x1b\\> ")); got != `C:\Program Files` {
		t.Fatalf("joined = %q", got)
	}
	// The last report in a chunk wins.
	if got := c.Scan([]byte("\x1b]9;9;C:\\a\x07\x1b]9;9;C:\\b\x07")); got != `C:\b` {
		t.Fatalf("last = %q", got)
	}
}
