package folding

import (
	"sort"
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// Range is a foldable block in document lines (0-based, inclusive). Folding
// it keeps StartLine visible and hides StartLine+1 .. EndLine.
type Range struct {
	StartLine, EndLine int
}

// Hidden is the number of lines the range hides when it is folded.
func (r Range) Hidden() int { return r.EndLine - r.StartLine }

// Contains reports whether line is inside the range's hidden part.
func (r Range) Contains(line int) bool { return line > r.StartLine && line <= r.EndLine }

// gotreesitter ships highlight and tags queries but no folds.scm, so the
// foldable nodes are found by one heuristic for all grammars: a named,
// multi-line node whose type names a structure (functions, classes, blocks,
// bodies, imports, literals, comments, markup elements, sections, ...).
// Grammar node types are snake_case; a type is foldable when one of its
// words is in foldWords ("function_declaration", "class_body",
// "object", "block_comment", "fenced_code_block", "block_mapping", ...).
var foldWords = map[string]bool{
	// Definitions.
	"function": true, "method": true, "constructor": true, "lambda": true, "closure": true,
	"class": true, "struct": true, "union": true, "interface": true, "enum": true, "impl": true,
	"trait": true, "protocol": true, "extension": true, "object": true, "record": true,
	"module": true, "namespace": true, "package": true, "macro": true, "template": true,
	"declaration": true, "definition": true, "def": true, "proc": true, "func": true,
	"service": true, "message": true, "oneof": true, "rpc": true,
	// Blocks and bodies.
	"block": true, "body": true, "compound": true, "group": true, "chunk": true,
	"if": true, "else": true, "elif": true, "for": true, "foreach": true, "while": true,
	"do": true, "loop": true, "repeat": true, "switch": true, "case": true, "match": true,
	"select": true, "when": true, "try": true, "catch": true, "except": true, "finally": true,
	"with": true, "using": true, "unsafe": true, "defer": true, "go": true,
	// Imports, arguments, parameters.
	"import": true, "imports": true, "include": true, "use": true, "require": true,
	"arguments": true, "parameters": true, "parameter": true, "argument": true,
	// Literals.
	"array": true, "list": true, "tuple": true, "dictionary": true, "map": true,
	"set": true, "table": true, "initializer": true, "composite": true, "literal": true,
	"string": true, "heredoc": true, "raw": true, "comment": true, "doc": true,
	// Markup and data.
	"element": true, "tag": true, "script": true, "style": true, "section": true,
	"fenced": true, "code": true, "html": true, "mapping": true, "sequence": true,
	"pair": true, "node": true, "children": true, "entry": true, "rule": true,
	"media": true, "keyframes": true, "supports": true, "at": true,
	// Config and other formats.
	"server": true, "location": true, "events": true, "http": true, "upstream": true,
	"directive": true, "host": true, "match_block": true, "hunk": true, "file": true,
	"statement": true, "expression": true, "query": true, "cte": true, "subquery": true,
	"create": true, "insert": true, "values": true, "target": true, "recipe": true,
	"conditional": true, "variable": true, "assignment": true,
}

// Node types that are the wrapping lists of a body: they start on the line
// after their parent's header (Python's "block" under "def f():") and would
// add a second fold for the same block.
var bodyWrappers = map[string]bool{
	"block": true, "statement_list": true, "statements": true, "body": true,
	"declaration_list": true, "field_declaration_list": true, "suite": true,
	"block_node": true, "block_mapping": true, "block_sequence": true, "flow_node": true,
}

// Node types that never fold (the document itself and wrappers of it).
var neverFold = map[string]bool{
	"source_file": true, "program": true, "translation_unit": true, "document": true,
	"stream": true, "compilation_unit": true, "chunk": true, "module": true,
	"source": true, "configuration": true, "makefile": true, "fragment": true,
}

func foldableType(t string) bool {
	if t == "" || neverFold[t] {
		return false
	}
	for _, w := range strings.Split(t, "_") {
		if foldWords[w] {
			return true
		}
	}
	return false
}

// endLine is the last line a node really covers: a node ending at column 0
// ends with the previous line's newline.
func endLine(n *gotreesitter.Node) int {
	end := n.EndPoint()
	row := int(end.Row)
	if end.Column == 0 && row > int(n.StartPoint().Row) {
		row--
	}
	return row
}

// Ranges finds the foldable ranges of a syntax tree: one per start line (the
// longest), sorted by start line. Besides the node type, two rules keep the
// folds where an editor puts them:
//   - A list-like node that starts with a multi-line child (a YAML mapping
//     with its first pair, a statement list with its first if) is not a
//     fold of its own: its first child's fold is the right one.
//   - A body wrapper (Python's "block", Go's "statement_list") that starts on
//     a later line than the node it belongs to adds no second fold.
func Ranges(root *gotreesitter.Node, lang *gotreesitter.Language) []Range {
	if root == nil {
		return nil
	}
	best := map[int]int{} // start line -> end line
	type item struct {
		n         *gotreesitter.Node
		headerRow int // the start line of the node the body belongs to
	}
	stack := []item{{n: root, headerRow: -1}}
	for len(stack) > 0 {
		it := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		n := it.n
		start, end := int(n.StartPoint().Row), endLine(n)
		if end <= start {
			continue // single-line nodes have no multi-line children
		}
		t := ""
		if n.IsNamed() && !n.IsError() {
			t = n.Type(lang)
		}
		headerRow := start
		wrapper := bodyWrappers[t] && it.headerRow >= 0 && start > it.headerRow
		if wrapper {
			headerRow = it.headerRow // its children belong to the same header
		}
		listLike := false
		if n.ChildCount() > 0 {
			if c := n.Child(0); c != nil && c.StartByte() == n.StartByte() && endLine(c) > start {
				listLike = true
			}
		}
		if n != root && !wrapper && !listLike && foldableType(t) {
			if e, ok := best[start]; !ok || end > e {
				best[start] = end
			}
		}
		for i := n.ChildCount() - 1; i >= 0; i-- {
			if c := n.Child(i); c != nil {
				stack = append(stack, item{n: c, headerRow: headerRow})
			}
		}
	}
	out := make([]Range, 0, len(best))
	for s, e := range best {
		out = append(out, Range{StartLine: s, EndLine: e})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartLine < out[j].StartLine })
	return out
}
