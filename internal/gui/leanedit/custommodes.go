package leanedit

import (
	"sync"

	"github.com/alecthomas/chroma/v2"
)

// Lexers for the formats of customModes.ts that chroma has none for (the
// others — Nim, Mojo, Makefile, Batch, registry, Apache, systemd, desktop
// entries, KDL — use chroma's lexers).

var (
	customOnce  sync.Once
	customModes map[string]chroma.Lexer
)

func customMode(id string) chroma.Lexer {
	customOnce.Do(func() {
		customModes = map[string]chroma.Lexer{
			"sshconfig": sshConfigLexer(),
			"crontab":   crontabLexer(),
			"ignore":    ignoreLexer(),
			"conf":      genericConfLexer(),
		}
	})
	return customModes[id]
}

func ruleLexer(name string, rules chroma.Rules) chroma.Lexer {
	return chroma.Coalesce(chroma.MustNewLexer(&chroma.Config{Name: name}, func() chroma.Rules { return rules }))
}

// sshConfigLexer: keywords at the line start, Host/Match lines as headings
// with their patterns as definitions, yes/no/... atoms, numbers, %x and
// ${VAR} variables.
func sshConfigLexer() chroma.Lexer {
	return ruleLexer("sshconfig", chroma.Rules{
		"root": {
			{Pattern: `^\s*#.*$`, Type: chroma.Comment},
			{Pattern: `^(\s*)((?i:host|match))(\b)`, Type: chroma.ByGroups(chroma.Text, chroma.KeywordType, chroma.Text), Mutator: chroma.Push("block")},
			{Pattern: `^(\s*)([A-Za-z][A-Za-z0-9]*)`, Type: chroma.ByGroups(chroma.Text, chroma.Keyword)},
			{Pattern: `#.*$`, Type: chroma.Comment},
			{Pattern: `"(\\.|[^"\\])*"`, Type: chroma.String},
			{Pattern: `(?i)\b(yes|no|any|none|auto|ask)\b`, Type: chroma.KeywordConstant},
			{Pattern: `\b\d+[smhdwKMG]?\b`, Type: chroma.Number},
			{Pattern: `%[a-zA-Z%]|\$\{[^}]*\}`, Type: chroma.NameVariable},
			{Pattern: `=`, Type: chroma.Operator},
			{Pattern: `\s+|.`, Type: chroma.Text},
		},
		"block": {
			{Pattern: `\n`, Type: chroma.Text, Mutator: chroma.Pop(1)},
			{Pattern: `#.*`, Type: chroma.Comment},
			{Pattern: `"(\\.|[^"\\])*"`, Type: chroma.String},
			{Pattern: `\S+`, Type: chroma.NameFunction},
			{Pattern: `[ \t]+`, Type: chroma.Text},
		},
	})
}

// crontabLexer: comments, VAR= assignments, @reboot etc., the five time
// fields as numbers, quoted strings and $variables in the command.
func crontabLexer() chroma.Lexer {
	return ruleLexer("crontab", chroma.Rules{
		"root": {
			{Pattern: `^\s*#.*$`, Type: chroma.Comment},
			{Pattern: `^(\s*)([A-Za-z_]\w*)(\s*)(=)`, Type: chroma.ByGroups(chroma.Text, chroma.NameProperty, chroma.Text, chroma.Operator)},
			{Pattern: `^(\s*)(@(?:reboot|yearly|annually|monthly|weekly|daily|midnight|hourly)\b)`, Type: chroma.ByGroups(chroma.Text, chroma.Keyword)},
			{Pattern: `^(\s*)(\S+)(\s+)(\S+)(\s+)(\S+)(\s+)(\S+)(\s+)(\S+)`, Type: chroma.ByGroups(chroma.Text, chroma.Number, chroma.Text, chroma.Number,
				chroma.Text, chroma.Number, chroma.Text, chroma.Number, chroma.Text, chroma.Number)},
			{Pattern: `"(\\.|[^"\\])*"|'[^']*'`, Type: chroma.String},
			{Pattern: `\$\{?\w+\}?`, Type: chroma.NameVariable},
			{Pattern: `\s+|.`, Type: chroma.Text},
		},
	})
}

// ignoreLexer: comments, negation, glob characters and slashes.
func ignoreLexer() chroma.Lexer {
	return ruleLexer("ignore", chroma.Rules{
		"root": {
			{Pattern: `^#.*$`, Type: chroma.Comment},
			{Pattern: `^!`, Type: chroma.Operator},
			{Pattern: `\*\*|\*|\?|\[[^\]]*\]`, Type: chroma.Operator},
			{Pattern: `/`, Type: chroma.Punctuation},
			{Pattern: `[^*?\[/\n]+|\n`, Type: chroma.Text},
		},
	})
}

// genericConfLexer (hosts, fstab, resolv.conf, ...): the first word of a
// line as keyword, comments, strings, addresses and numbers.
func genericConfLexer() chroma.Lexer {
	return ruleLexer("conf", chroma.Rules{
		"root": {
			{Pattern: `[#;].*$`, Type: chroma.Comment},
			{Pattern: `^(\s*)([^\s#;"'=]+)`, Type: chroma.ByGroups(chroma.Text, chroma.Keyword)},
			{Pattern: `"(\\.|[^"\\])*"|'[^']*'`, Type: chroma.String},
			{Pattern: `(\d{1,3}\.){3}\d{1,3}(/\d+)?|[0-9a-fA-F]*:[0-9a-fA-F:]+(/\d+)?`, Type: chroma.Number},
			{Pattern: `\b\d+(\.\d+)?\b`, Type: chroma.Number},
			{Pattern: `[^\s#;"'=]+`, Type: chroma.Text},
			{Pattern: `\s+`, Type: chroma.Text},
			{Pattern: `.`, Type: chroma.Operator},
		},
	})
}
