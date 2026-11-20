package leanedit

import (
	"testing"

	"github.com/alecthomas/chroma/v2"
)

// The cases of languages.test.ts.
func TestDetectLanguage(t *testing.T) {
	cases := map[string]string{
		"main.go": "go", "/etc/nginx/sites-available/default": "nginx", "x.d.ts": "typescript", "Dockerfile": "dockerfile",
		"/home/u/.ssh/config": "sshconfig", "/etc/systemd/system/x.service": "systemd", "README": "", "CMakeLists.txt": "cmake",
		"/etc/hosts": "conf", ".bashrc": "bash", "a.tsx": "tsx", "/p/.vscode/settings.json": "jsonc", "/x/.env.local": "ini",
	}
	for p, want := range cases {
		if got := detectLanguage(p, ""); got != want {
			t.Errorf("%s: %q, want %q", p, got, want)
		}
	}
	if got := detectLanguage("script", "#!/usr/bin/env python3"); got != "python" {
		t.Errorf("shebang: %q", got)
	}
	if !IsTextFile("README") || !IsTextFile("x.log") || IsTextFile("x.png") {
		t.Error("IsTextFile")
	}
}

// TestLexers lists the languages chroma has no lexer for.
func TestLexers(t *testing.T) {
	for _, l := range languages {
		if l.lexer != "" && chromaLexer(l.id) == nil {
			t.Errorf("%s: chroma has no lexer %q", l.id, l.lexer)
		}
	}
}

func TestCustomModes(t *testing.T) {
	for id, src := range map[string]string{
		"sshconfig": "Host web*\n  User root # admin\n  Port 22\n  ForwardAgent yes\n",
		"crontab":   "MAILTO=a@b\n*/5 * * * * /bin/run \"$HOME\"\n@reboot x\n",
		"ignore":    "# c\n!keep/**/*.go\n",
		"conf":      "127.0.0.1 localhost # loop\n",
	} {
		l := chromaLexer(id)
		if l == nil {
			t.Fatalf("%s: no lexer", id)
		}
		it, err := l.Tokenise(nil, src)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		var got string
		kinds := map[chroma.TokenType]bool{}
		for tok := it(); tok != chroma.EOF; tok = it() {
			got += tok.Value
			kinds[tok.Type] = true
		}
		if got != src {
			t.Errorf("%s: tokens lose text: %q", id, got)
		}
		if !kinds[chroma.Comment] && id != "crontab" {
			t.Errorf("%s: no comment token: %v", id, kinds)
		}
	}
}
