package folding

// The grammars of the languages LeanEdit highlights (leanedit/languages.go)
// that gotreesitter has. Each grammar package embeds its own blob and loads it
// on the first Language() call (and caches it), so only the languages that are
// opened cost memory. Languages without a grammar (batch, sass, systemd,
// apache, crontab, conf, reg, rpmspec, plain text) have no folding.

import (
	"github.com/odvcencio/gotreesitter"
	ts_arduino "github.com/odvcencio/gotreesitter/grammars/arduino"
	ts_bash "github.com/odvcencio/gotreesitter/grammars/bash"
	ts_c "github.com/odvcencio/gotreesitter/grammars/c"
	ts_c_sharp "github.com/odvcencio/gotreesitter/grammars/c_sharp"
	ts_clojure "github.com/odvcencio/gotreesitter/grammars/clojure"
	ts_cmake "github.com/odvcencio/gotreesitter/grammars/cmake"
	ts_cpp "github.com/odvcencio/gotreesitter/grammars/cpp"
	ts_css "github.com/odvcencio/gotreesitter/grammars/css"
	ts_dart "github.com/odvcencio/gotreesitter/grammars/dart"
	ts_desktop "github.com/odvcencio/gotreesitter/grammars/desktop"
	ts_diff "github.com/odvcencio/gotreesitter/grammars/diff"
	ts_dockerfile "github.com/odvcencio/gotreesitter/grammars/dockerfile"
	ts_gitignore "github.com/odvcencio/gotreesitter/grammars/gitignore"
	ts_go "github.com/odvcencio/gotreesitter/grammars/go"
	ts_html "github.com/odvcencio/gotreesitter/grammars/html"
	ts_ini "github.com/odvcencio/gotreesitter/grammars/ini"
	ts_java "github.com/odvcencio/gotreesitter/grammars/java"
	ts_javascript "github.com/odvcencio/gotreesitter/grammars/javascript"
	ts_jinja2 "github.com/odvcencio/gotreesitter/grammars/jinja2"
	ts_json "github.com/odvcencio/gotreesitter/grammars/json"
	ts_json5 "github.com/odvcencio/gotreesitter/grammars/json5"
	ts_kdl "github.com/odvcencio/gotreesitter/grammars/kdl"
	ts_kotlin "github.com/odvcencio/gotreesitter/grammars/kotlin"
	ts_less "github.com/odvcencio/gotreesitter/grammars/less"
	ts_lua "github.com/odvcencio/gotreesitter/grammars/lua"
	ts_make "github.com/odvcencio/gotreesitter/grammars/make"
	ts_markdown "github.com/odvcencio/gotreesitter/grammars/markdown"
	ts_mojo "github.com/odvcencio/gotreesitter/grammars/mojo"
	ts_nginx "github.com/odvcencio/gotreesitter/grammars/nginx"
	ts_nim "github.com/odvcencio/gotreesitter/grammars/nim"
	ts_powershell "github.com/odvcencio/gotreesitter/grammars/powershell"
	ts_proto "github.com/odvcencio/gotreesitter/grammars/proto"
	ts_python "github.com/odvcencio/gotreesitter/grammars/python"
	ts_rust "github.com/odvcencio/gotreesitter/grammars/rust"
	ts_scss "github.com/odvcencio/gotreesitter/grammars/scss"
	ts_sql "github.com/odvcencio/gotreesitter/grammars/sql"
	ts_ssh_config "github.com/odvcencio/gotreesitter/grammars/ssh_config"
	ts_svelte "github.com/odvcencio/gotreesitter/grammars/svelte"
	ts_toml "github.com/odvcencio/gotreesitter/grammars/toml"
	ts_tsx "github.com/odvcencio/gotreesitter/grammars/tsx"
	ts_typescript "github.com/odvcencio/gotreesitter/grammars/typescript"
	ts_xml "github.com/odvcencio/gotreesitter/grammars/xml"
	ts_yaml "github.com/odvcencio/gotreesitter/grammars/yaml"
)

var grammars = map[string]func() *gotreesitter.Language{
	"c":          ts_c.Language,
	"cpp":        ts_cpp.Language,
	"arduino":    ts_arduino.Language,
	"csharp":     ts_c_sharp.Language,
	"java":       ts_java.Language,
	"kotlin":     ts_kotlin.Language,
	"dart":       ts_dart.Language,
	"go":         ts_go.Language,
	"rust":       ts_rust.Language,
	"python":     ts_python.Language,
	"mojo":       ts_mojo.Language,
	"nim":        ts_nim.Language,
	"clojure":    ts_clojure.Language,
	"lua":        ts_lua.Language,
	"bash":       ts_bash.Language,
	"powershell": ts_powershell.Language,
	"html":       ts_html.Language,
	"css":        ts_css.Language,
	"scss":       ts_scss.Language,
	"less":       ts_less.Language,
	"javascript": ts_javascript.Language,
	"jsx":        ts_javascript.Language,
	"typescript": ts_typescript.Language,
	"tsx":        ts_tsx.Language,
	"svelte":     ts_svelte.Language,
	"markdown":   ts_markdown.Language,
	"sql":        ts_sql.Language,
	"json":       ts_json.Language,
	"jsonc":      ts_json5.Language,
	"yaml":       ts_yaml.Language,
	"toml":       ts_toml.Language,
	"xml":        ts_xml.Language,
	"desktop":    ts_desktop.Language,
	"kdl":        ts_kdl.Language,
	"ini":        ts_ini.Language,
	"nginx":      ts_nginx.Language,
	"sshconfig":  ts_ssh_config.Language,
	"dockerfile": ts_dockerfile.Language,
	"makefile":   ts_make.Language,
	"cmake":      ts_cmake.Language,
	"ignore":     ts_gitignore.Language,
	"protobuf":   ts_proto.Language,
	"jinja2":     ts_jinja2.Language,
	"diff":       ts_diff.Language,
}

// Grammar is the grammar name gotreesitter uses for a LeanEdit language id
// ("" when the language has no grammar).
func Grammar(langID string) string {
	if _, ok := grammars[langID]; !ok {
		return ""
	}
	return grammarNames[langID]
}

var grammarNames = map[string]string{
	"c":          "c",
	"cpp":        "cpp",
	"arduino":    "arduino",
	"csharp":     "c_sharp",
	"java":       "java",
	"kotlin":     "kotlin",
	"dart":       "dart",
	"go":         "go",
	"rust":       "rust",
	"python":     "python",
	"mojo":       "mojo",
	"nim":        "nim",
	"clojure":    "clojure",
	"lua":        "lua",
	"bash":       "bash",
	"powershell": "powershell",
	"html":       "html",
	"css":        "css",
	"scss":       "scss",
	"less":       "less",
	"javascript": "javascript",
	"jsx":        "javascript",
	"typescript": "typescript",
	"tsx":        "tsx",
	"svelte":     "svelte",
	"markdown":   "markdown",
	"sql":        "sql",
	"json":       "json",
	"jsonc":      "json5",
	"yaml":       "yaml",
	"toml":       "toml",
	"xml":        "xml",
	"desktop":    "desktop",
	"kdl":        "kdl",
	"ini":        "ini",
	"nginx":      "nginx",
	"sshconfig":  "ssh_config",
	"dockerfile": "dockerfile",
	"makefile":   "make",
	"cmake":      "cmake",
	"ignore":     "gitignore",
	"protobuf":   "proto",
	"jinja2":     "jinja2",
	"diff":       "diff",
}

// Supported reports whether the language can be folded.
func Supported(langID string) bool { return grammars[langID] != nil }

// Languages are the LeanEdit language ids that can be folded.
func Languages() []string {
	out := make([]string, 0, len(grammars))
	for id := range grammars {
		out = append(out, id)
	}
	return out
}
