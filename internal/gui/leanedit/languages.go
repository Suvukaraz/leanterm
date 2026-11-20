package leanedit

import (
	"regexp"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/Suvukaraz/leanterm/internal/gui/codeedit"
)

type lexer = chroma.Lexer

// language is languages.ts's LanguageDef; lexer names the chroma lexer
// that highlights it ("" = none in chroma: custom, see customModes).
type language struct {
	id, name, group string
	exts            []string
	names           []string
	paths           []*regexp.Regexp
	shebang         *regexp.Regexp
	lexer           string
}

func re(s string) *regexp.Regexp { return regexp.MustCompile(s) }

// languages is the table of languages.ts (same ids, names, groups,
// extensions, file names, path patterns and shebangs).
var languages = []language{
	// ---- Programming languages
	{id: "c", name: "C", group: "code", exts: []string{"c", "h"}, lexer: "c"},
	{id: "cpp", name: "C++", group: "code", exts: []string{"cpp", "cc", "cxx", "c++", "hpp", "hh", "hxx", "h++", "ipp", "tpp", "inl"}, lexer: "cpp"},
	{id: "arduino", name: "Arduino", group: "code", exts: []string{"ino", "pde"}, lexer: "arduino"},
	{id: "csharp", name: "C#", group: "code", exts: []string{"cs", "csx"}, lexer: "csharp"},
	{id: "java", name: "Java", group: "code", exts: []string{"java"}, lexer: "java"},
	{id: "kotlin", name: "Kotlin", group: "code", exts: []string{"kt", "kts"}, lexer: "kotlin"},
	{id: "dart", name: "Dart / Flutter", group: "code", exts: []string{"dart"}, lexer: "dart"},
	{id: "go", name: "Go", group: "code", exts: []string{"go"}, lexer: "go"},
	{id: "rust", name: "Rust", group: "code", exts: []string{"rs"}, lexer: "rust"},
	{id: "python", name: "Python", group: "code", exts: []string{"py", "pyw", "pyi"}, names: []string{"SConstruct", "SConscript"},
		shebang: re(`python[\d.]*$`), lexer: "python"},
	{id: "mojo", name: "Mojo", group: "code", exts: []string{"mojo", "🔥"}, lexer: "mojo"},
	{id: "nim", name: "Nim", group: "code", exts: []string{"nim", "nims", "nimble"}, lexer: "nim"},
	{id: "clojure", name: "Clojure", group: "code", exts: []string{"clj", "cljs", "cljc", "edn", "bb"}, shebang: re(`^(bb|clojure|clj)$`), lexer: "clojure"},
	{id: "lua", name: "Lua", group: "code", exts: []string{"lua", "rockspec"}, shebang: re(`^lua[\d.]*$`), lexer: "lua"},
	{id: "bash", name: "Bash / Shell", group: "code", exts: []string{"sh", "bash", "zsh", "ksh", "ash", "dash", "ebuild", "eclass", "bats", "command"},
		names: []string{".bashrc", ".bash_profile", ".bash_login", ".bash_logout", ".bash_aliases", ".profile", ".zshrc",
			".zshenv", ".zprofile", ".zlogin", ".kshrc", ".xprofile", ".xinitrc", ".xsessionrc", "PKGBUILD",
			"APKBUILD", "bashrc", "profile", "zshrc"},
		shebang: re(`^(ba|z|k|da|a)?sh$`), lexer: "bash"},
	{id: "powershell", name: "PowerShell", group: "code", exts: []string{"ps1", "psm1", "psd1"}, shebang: re(`^pwsh$`), lexer: "powershell"},
	{id: "batch", name: "Windows Batch", group: "code", exts: []string{"bat", "cmd"}, lexer: "batchfile"},

	// ---- Web
	{id: "html", name: "HTML", group: "web", exts: []string{"html", "htm", "xhtml", "shtml"}, lexer: "html"},
	{id: "css", name: "CSS", group: "web", exts: []string{"css"}, lexer: "css"},
	{id: "scss", name: "SCSS", group: "web", exts: []string{"scss"}, lexer: "scss"},
	{id: "sass", name: "Sass", group: "web", exts: []string{"sass"}, lexer: "sass"},
	{id: "less", name: "Less", group: "web", exts: []string{"less"}, lexer: "scss"},
	{id: "javascript", name: "JavaScript", group: "web", exts: []string{"js", "mjs", "cjs"}, shebang: re(`^(node|nodejs|deno|bun)$`), lexer: "javascript"},
	{id: "jsx", name: "React (JSX)", group: "web", exts: []string{"jsx"}, lexer: "react"},
	{id: "typescript", name: "TypeScript", group: "web", exts: []string{"ts", "mts", "cts"}, lexer: "typescript"},
	{id: "tsx", name: "React (TSX)", group: "web", exts: []string{"tsx"}, lexer: "tsx"},
	{id: "svelte", name: "Svelte", group: "web", exts: []string{"svelte"}, lexer: "svelte"},
	{id: "markdown", name: "Markdown", group: "web", exts: []string{"md", "markdown", "mdown", "mkd"}, lexer: "markdown"},
	{id: "sql", name: "SQL", group: "web", exts: []string{"sql"}, lexer: "sql"},

	// ---- Data and configuration formats
	{id: "json", name: "JSON", group: "config", exts: []string{"json", "webmanifest", "har", "geojson", "ipynb"},
		names: []string{"composer.lock", ".prettierrc"}, lexer: "json"},
	{id: "jsonc", name: "JSON with comments / JSON5", group: "config", exts: []string{"jsonc", "json5"},
		names: []string{"tsconfig.json", "jsconfig.json", ".eslintrc", ".eslintrc.json", ".babelrc", ".jshintrc", "devcontainer.json"},
		paths: []*regexp.Regexp{re(`/\.vscode/[^/]+\.json$`)}, lexer: "json"},
	{id: "yaml", name: "YAML", group: "config", exts: []string{"yaml", "yml"}, names: []string{".clang-format", ".clang-tidy", ".yamllint"}, lexer: "yaml"},
	{id: "toml", name: "TOML", group: "config", exts: []string{"toml"}, names: []string{"Cargo.lock", "Pipfile", "poetry.lock", "uv.lock"}, lexer: "toml"},
	{id: "xml", name: "XML", group: "config", exts: []string{
		"xml", "xsd", "xsl", "xslt", "svg", "plist", "csproj", "vbproj", "fsproj", "vcxproj", "props",
		"targets", "xaml", "resx", "config", "wsdl", "rss", "atom", "kml", "gpx", "xul", "ui", "glade",
		"manifest", "nuspec", "pom"}, lexer: "xml"},
	{id: "systemd", name: "systemd Unit", group: "config", exts: []string{
		"service", "socket", "device", "mount", "automount", "swap", "target", "path", "timer", "slice", "scope",
		"network", "netdev", "link", "nspawn"},
		paths: []*regexp.Regexp{re(`/systemd/.+\.(conf|d/[^/]+)$`), re(`/containers/systemd/.+\.(container|volume|network|kube|pod|image|build|artifact)$`)},
		lexer: "systemd"},
	{id: "desktop", name: "Desktop Entry", group: "config", exts: []string{"desktop", "directory", "kdelnk"},
		names: []string{"mimeapps.list", "defaults.list", "mimeinfo.cache"}, lexer: "desktop"},
	{id: "kdl", name: "KDL", group: "config", exts: []string{"kdl"}, lexer: "kdl"},
	{id: "ini", name: "INI / Conf", group: "config", exts: []string{
		"ini", "cfg", "conf", "cnf", "properties", "prefs", "inf", "url", "lnk", "env", "npmrc", "cfg.example",
		"conf.example", "ini.example"},
		names: []string{".editorconfig", ".gitconfig", ".gitmodules", ".gitattributes", ".npmrc", ".yarnrc", ".pypirc",
			"pip.conf", "my.cnf", "php.ini", "setup.cfg", "tox.ini", "pytest.ini", "mypy.ini", "desktop.ini",
			".flake8", ".pylintrc", ".coveragerc", "smb.conf", "pacman.conf", "yum.conf", "dnf.conf"},
		paths: []*regexp.Regexp{re(`/\.env(\.[^/]*)?$`), re(`/\.git/config$`)}, lexer: "ini"},
	{id: "nginx", name: "nginx", group: "config", exts: []string{"nginx", "nginx.conf", "vhost"},
		names: []string{"nginx.conf", "mime.types", "fastcgi_params", "fastcgi.conf", "proxy_params", "uwsgi_params", "scgi_params"},
		paths: []*regexp.Regexp{re(`/nginx/(sites-(available|enabled)|conf\.d|snippets)/`)}, lexer: "nginx"},
	{id: "apache", name: "Apache httpd", group: "config",
		names: []string{"httpd.conf", "apache2.conf", "apache.conf", ".htaccess", ".htpasswd", "ports.conf"},
		paths: []*regexp.Regexp{re(`/(apache2?|httpd)/(sites-(available|enabled)|conf(-available|-enabled|\.d|\.modules\.d)?|mods-(available|enabled))/`)},
		lexer: "apacheconf"},
	{id: "sshconfig", name: "SSH config", group: "config", names: []string{"ssh_config", "sshd_config"},
		paths: []*regexp.Regexp{re(`/\.ssh/config$`), re(`/ssh/(ssh|sshd)_config\.d/`)}},
	{id: "crontab", name: "crontab", group: "config", names: []string{"crontab", "anacrontab"},
		paths: []*regexp.Regexp{re(`/cron\.d/`), re(`/crontabs/`)}},
	{id: "conf", name: "Config (whitespace separated)", group: "config", names: []string{
		"hosts", "fstab", "crypttab", "resolv.conf", "exports", "hosts.allow", "hosts.deny", "networks",
		"protocols", "services", "nsswitch.conf", "sudoers", "login.defs", "limits.conf", "modules",
		"environment", "inittab", "shells", "go.mod", "go.work", "requirements.txt"},
		paths: []*regexp.Regexp{re(`/sudoers\.d/`), re(`/modprobe\.d/`), re(`/security/[^/]+\.conf$`)}},
	{id: "dockerfile", name: "Dockerfile", group: "config", exts: []string{"dockerfile", "containerfile"},
		names: []string{"Dockerfile", "Containerfile"}, paths: []*regexp.Regexp{re(`/(Dockerfile|Containerfile)\.[^/]+$`)}, lexer: "docker"},
	{id: "makefile", name: "Makefile", group: "config", exts: []string{"mk", "mak", "make"},
		names: []string{"Makefile", "GNUmakefile", "makefile", "Kbuild", "Makefile.am", "Makefile.in"}, lexer: "makefile"},
	{id: "cmake", name: "CMake", group: "config", exts: []string{"cmake"}, names: []string{"CMakeLists.txt"}, lexer: "cmake"},
	{id: "ignore", name: "Ignore file", group: "config",
		names: []string{".gitignore", ".dockerignore", ".npmignore", ".hgignore", ".prettierignore", ".eslintignore", ".stignore"},
		paths: []*regexp.Regexp{re(`/\.git/info/exclude$`)}},
	{id: "reg", name: "Windows Registry", group: "config", exts: []string{"reg"}, lexer: "registry"},
	{id: "protobuf", name: "Protocol Buffers", group: "config", exts: []string{"proto"}, lexer: "protobuf"},
	{id: "jinja2", name: "Jinja2 / Ansible template", group: "config", exts: []string{"j2", "jinja", "jinja2"}, lexer: "jinja"},
	{id: "rpmspec", name: "RPM spec", group: "config", exts: []string{"spec"}, lexer: "spec"},
	{id: "diff", name: "Diff / Patch", group: "config", exts: []string{"diff", "patch", "rej"}, lexer: "diff"},
}

// plainTextExts and plainTextNames open in the editor without highlighting.
var plainTextExts = setOf("txt", "text", "log", "out", "csv", "tsv", "nfo", "asc", "pem", "crt", "csr", "key", "pub", "srt", "sub",
	"vtt", "lst", "list", "rst", "adoc", "asciidoc", "org", "tex", "bib", "sum", "sha256", "md5", "gpg",
	"lock", "example", "sample", "template", "tmpl", "tpl", "dist", "bak", "orig", "old", "in")

var plainTextNames = setOf("readme", "license", "licence", "copying", "authors", "changelog", "changes", "news", "todo", "install",
	"notice", "contributors", "maintainers", "version", "codeowners", "motd", "issue", "hostname",
	"machine-id", "passwd", "group", "shadow", "authorized_keys", "known_hosts", "id_rsa", "id_ed25519",
	"id_ecdsa", "timezone", "locale.gen", "vconsole.conf", "os-release", "mailname", "procfile", "brewfile",
	"gemfile", "rakefile", "vagrantfile", "jenkinsfile", "caddyfile", "justfile")

func setOf(s ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range s {
		m[x] = true
	}
	return m
}

func languageByID(id string) *language {
	for i := range languages {
		if languages[i].id == id {
			return &languages[i]
		}
	}
	return nil
}

// extensionsOf lists a name's extensions, longest first ("x.d.ts" → d.ts, ts).
func extensionsOf(name string) []string {
	lower := strings.TrimLeft(strings.ToLower(name), ".")
	parts := strings.Split(lower, ".")
	var out []string
	for i := 1; i < len(parts); i++ {
		out = append(out, strings.Join(parts[i:], "."))
	}
	return out
}

var shebangRe = regexp.MustCompile(`^#!\s*(\S+)(?:\s+(\S+))?`)

// detectLanguage picks the language of a file from its path and first line
// ("" = plain text).
func detectLanguage(path, firstLine string) string {
	unix := strings.ReplaceAll(path, `\`, "/")
	name := baseName(unix)
	lower := strings.ToLower(name)
	for _, l := range languages {
		for _, f := range l.names {
			if strings.ToLower(f) == lower {
				return l.id
			}
		}
	}
	for _, l := range languages {
		for _, p := range l.paths {
			if p.MatchString(unix) {
				return l.id
			}
		}
	}
	for _, ext := range extensionsOf(name) {
		for _, l := range languages {
			for _, e := range l.exts {
				if e == ext {
					return l.id
				}
			}
		}
	}
	if m := shebangRe.FindStringSubmatch(firstLine); m != nil {
		interp := baseName(m[1])
		if interp == "env" && m[2] != "" {
			interp = baseName(m[2])
		}
		for _, l := range languages {
			if l.shebang != nil && l.shebang.MatchString(interp) {
				return l.id
			}
		}
	}
	return ""
}

// IsTextFile reports whether a double click opens the file in the editor
// (known text formats only).
func IsTextFile(path string) bool {
	if detectLanguage(path, "") != "" {
		return true
	}
	name := baseName(strings.ReplaceAll(path, `\`, "/"))
	if plainTextNames[strings.ToLower(name)] {
		return true
	}
	for _, e := range extensionsOf(name) {
		if plainTextExts[e] {
			return true
		}
	}
	return false
}

// languageName is the name shown for a language id.
func languageName(id string) string {
	if l := languageByID(id); l != nil {
		return l.name
	}
	return "Plain Text"
}

// chromaLexer is the highlighter of a language (nil = none).
func chromaLexer(id string) chroma.Lexer {
	l := languageByID(id)
	if l == nil {
		return nil
	}
	if cm := customMode(id); cm != nil {
		return cm
	}
	if l.lexer == "" {
		return nil
	}
	return codeedit.LexerByName(l.lexer)
}

// customMode is a lexer of customModes.ts (TODO 7c).
func customMode(id string) chroma.Lexer { return nil }

type langGroup struct {
	label string
	langs []*language
}

// languageGroups are the submenus of Tools → Highlighting.
func languageGroups() []langGroup {
	groups := []langGroup{{label: "Programming"}, {label: "Web"}, {label: "Configuration & Data"}}
	for i := range languages {
		l := &languages[i]
		switch l.group {
		case "code":
			groups[0].langs = append(groups[0].langs, l)
		case "web":
			groups[1].langs = append(groups[1].langs, l)
		default:
			groups[2].langs = append(groups[2].langs, l)
		}
	}
	for _, g := range groups {
		sort.Slice(g.langs, func(i, j int) bool { return strings.ToLower(g.langs[i].name) < strings.ToLower(g.langs[j].name) })
	}
	return groups
}

var _ = lexers.Get
