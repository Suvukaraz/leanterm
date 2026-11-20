package leanedit

import "testing"

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
