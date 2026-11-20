// Package symbols finds the declarations of a document (Go to Symbol) in
// its gotreesitter syntax tree and filters them with github.com/sahilm/fuzzy.
// It has no UI and no parser of its own: the editor's background parser
// (package folding) hands it the tree it keeps for folding. Languages
// without a grammar have no symbols; there is no fallback.
package symbols

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/odvcencio/gotreesitter"
	"github.com/sahilm/fuzzy"
)

// Kind is what a symbol declares.
type Kind string

const (
	KindFunc      Kind = "func"
	KindMethod    Kind = "method"
	KindClass     Kind = "class"
	KindStruct    Kind = "struct"
	KindInterface Kind = "interface"
	KindTrait     Kind = "trait"
	KindImpl      Kind = "impl"
	KindEnum      Kind = "enum"
	KindType      Kind = "type"
	KindConst     Kind = "const"
	KindVar       Kind = "var"
	KindModule    Kind = "module"
	KindMacro     Kind = "macro"
	KindHeading   Kind = "heading"
)

// Badge is the short label of a kind in the picker.
func (k Kind) Badge() string {
	switch k {
	case KindFunc:
		return "fn"
	case KindMethod:
		return "meth"
	case KindClass:
		return "cls"
	case KindInterface:
		return "intf"
	case KindModule:
		return "mod"
	case KindHeading:
		return "h"
	case KindMacro:
		return "mac"
	}
	return string(k)
}

// Symbol is one declaration. Line and Col are 0-based; Col counts runes.
type Symbol struct {
	Name      string
	Kind      Kind
	Line, Col int
	// Container is the enclosing class, type or module, or a method's
	// receiver type.
	Container string
}

// definitions maps node types (across the grammars) to the kind they
// declare. Functions inside classes, impls and traits become methods.
var definitions = map[string]Kind{
	"function_declaration": KindFunc, "function_definition": KindFunc, "function_item": KindFunc,
	"function_signature_item": KindFunc, "generator_function_declaration": KindFunc,
	"method_declaration": KindMethod, "method_definition": KindMethod, "method_elem": KindMethod,
	"method_signature": KindMethod, "constructor_declaration": KindMethod,
	"class_declaration": KindClass, "class_definition": KindClass, "class_specifier": KindClass,
	"abstract_class_declaration": KindClass, "object_declaration": KindClass, "record_declaration": KindClass,
	"struct_item": KindStruct, "struct_specifier": KindStruct, "struct_declaration": KindStruct,
	"interface_declaration": KindInterface, "protocol_declaration": KindInterface,
	"trait_item": KindTrait, "impl_item": KindImpl,
	"enum_item": KindEnum, "enum_declaration": KindEnum, "enum_specifier": KindEnum,
	"type_spec": KindType, "type_alias_declaration": KindType, "type_item": KindType,
	"type_definition": KindType, "type_alias": KindType,
	"const_spec": KindConst, "const_item": KindConst, "static_item": KindConst,
	"var_spec": KindVar, "variable_declarator": KindVar, "assignment": KindVar,
	"mod_item": KindModule, "namespace_definition": KindModule, "module_declaration": KindModule,
	"preproc_def": KindMacro, "preproc_function_def": KindMacro, "macro_definition": KindMacro,
	"atx_heading": KindHeading, "setext_heading": KindHeading,
	// Protocol Buffers.
	"message": KindStruct, "service": KindInterface, "rpc": KindMethod, "enum": KindEnum,
}

// Kinds that are containers of the symbols inside them.
var containers = map[Kind]bool{
	KindClass: true, KindStruct: true, KindInterface: true, KindTrait: true, KindImpl: true,
	KindEnum: true, KindModule: true, KindType: true,
}

// Variables count only outside of functions (no locals).
var localScope = map[Kind]bool{KindFunc: true, KindMethod: true}

// Extract walks a syntax tree of src for its symbols, in document order.
func Extract(root *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []Symbol {
	if root == nil || lang == nil {
		return nil
	}
	type frame struct {
		n         *gotreesitter.Node
		container string
		inFunc    bool
		inType    bool // inside a class, impl, trait, ... (functions are methods)
	}
	var out []Symbol
	stack := []frame{{n: root}}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		n := f.n
		container, inFunc, inType := f.container, f.inFunc, f.inType
		if n.IsNamed() && !n.IsError() {
			t := n.Type(lang)
			if kind, ok := definitions[t]; ok {
				if sym, ok := symbolOf(n, t, kind, lang, src, f.container, f.inFunc, f.inType); ok {
					out = append(out, sym)
					if containers[sym.Kind] {
						container = sym.Name
						inType = true
					}
					if localScope[sym.Kind] {
						inFunc = true
					}
				}
			}
		}
		for i := n.ChildCount() - 1; i >= 0; i-- {
			if c := n.Child(i); c != nil {
				stack = append(stack, frame{n: c, container: container, inFunc: inFunc, inType: inType})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Col < out[j].Col
	})
	return out
}

func symbolOf(n *gotreesitter.Node, t string, kind Kind, lang *gotreesitter.Language, src []byte,
	container string, inFunc, inType bool) (Symbol, bool) {
	switch kind {
	case KindVar, KindConst:
		if inFunc {
			return Symbol{}, false // locals
		}
	case KindFunc:
		if inFunc {
			return Symbol{}, false // nested functions are not listed
		}
		if inType {
			kind = KindMethod
		}
	case KindMethod:
		if inFunc {
			return Symbol{}, false
		}
	}
	name := nameNode(n, t, lang)
	if name == nil {
		return Symbol{}, false
	}
	text := strings.TrimSpace(name.Text(src))
	if t == "atx_heading" || t == "setext_heading" {
		text = strings.TrimSpace(strings.TrimLeft(text, "#"))
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	if text == "" || utf8.RuneCountInString(text) > 200 {
		return Symbol{}, false
	}
	// A Go type spec says what it declares.
	if t == "type_spec" {
		if ty := n.ChildByFieldName("type", lang); ty != nil {
			switch ty.Type(lang) {
			case "struct_type":
				kind = KindStruct
			case "interface_type":
				kind = KindInterface
			}
		}
	}
	// A Go method's container is its receiver type.
	if t == "method_declaration" {
		if r := n.ChildByFieldName("receiver", lang); r != nil {
			if id := firstOfType(r, lang, "type_identifier"); id != nil {
				container = id.Text(src)
			}
		}
	}
	start := name.StartPoint()
	return Symbol{Name: text, Kind: kind, Line: int(start.Row), Col: runeCol(src, name.StartByte(), int(start.Column)),
		Container: container}, true
}

// nameNode is the node holding a declaration's name.
func nameNode(n *gotreesitter.Node, t string, lang *gotreesitter.Language) *gotreesitter.Node {
	switch t {
	case "impl_item":
		return n.ChildByFieldName("type", lang)
	case "assignment":
		if l := n.ChildByFieldName("left", lang); l != nil && l.Type(lang) == "identifier" {
			return l
		}
		return nil
	case "atx_heading", "setext_heading":
		if c := firstOfType(n, lang, "inline"); c != nil {
			return c
		}
		return n
	}
	if c := n.ChildByFieldName("name", lang); c != nil {
		return c
	}
	// C and C++: the name is inside the declarator chain.
	d := n
	for i := 0; i < 8; i++ {
		next := d.ChildByFieldName("declarator", lang)
		if next == nil {
			break
		}
		d = next
		switch d.Type(lang) {
		case "identifier", "field_identifier", "type_identifier", "qualified_identifier", "destructor_name", "operator_name":
			return d
		}
	}
	return nil
}

// firstOfType is the first descendant of n (depth first) of type typ.
func firstOfType(n *gotreesitter.Node, lang *gotreesitter.Language, typ string) *gotreesitter.Node {
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil {
			continue
		}
		if c.Type(lang) == typ {
			return c
		}
		if d := firstOfType(c, lang, typ); d != nil {
			return d
		}
	}
	return nil
}

// runeCol turns a byte column into a rune column.
func runeCol(src []byte, at uint32, byteCol int) int {
	end := int(at)
	start := end - byteCol
	if start < 0 || end > len(src) {
		return byteCol
	}
	return utf8.RuneCount(src[start:end])
}

// Match is a symbol that matches a query, with the rune indexes of its name
// (and container) that match.
type Match struct {
	Symbol
	Matched          []int
	MatchedContainer []int
	Score            int
}

type names []string

func (s names) String(i int) string { return s[i] }
func (s names) Len() int            { return len(s) }

// Filter matches the symbols against query with sahilm/fuzzy, by name and
// by "Container.Name": the best of both counts. Results are ranked by
// score, then document order. An empty query keeps all symbols in
// document order.
func Filter(syms []Symbol, query string) []Match {
	query = strings.TrimSpace(query)
	if query == "" {
		out := make([]Match, len(syms))
		for i, s := range syms {
			out[i] = Match{Symbol: s}
		}
		return out
	}
	best := map[int]Match{}
	plain := make(names, len(syms))
	qualified := make(names, len(syms))
	for i, s := range syms {
		plain[i] = s.Name
		qualified[i] = s.Name
		if s.Container != "" {
			qualified[i] = s.Container + "." + s.Name
		}
	}
	for _, m := range fuzzy.FindFromNoSort(query, plain) {
		best[m.Index] = Match{Symbol: syms[m.Index], Matched: runes(m.Str, m.MatchedIndexes), Score: m.Score}
	}
	for _, m := range fuzzy.FindFromNoSort(query, qualified) {
		s := syms[m.Index]
		if s.Container == "" {
			continue
		}
		if old, ok := best[m.Index]; ok && old.Score >= m.Score {
			continue
		}
		var inName, inCont []int
		cl := utf8.RuneCountInString(s.Container) + 1
		for _, r := range runes(m.Str, m.MatchedIndexes) {
			if r >= cl {
				inName = append(inName, r-cl)
			} else if r < cl-1 {
				inCont = append(inCont, r)
			}
		}
		best[m.Index] = Match{Symbol: s, Matched: inName, MatchedContainer: inCont, Score: m.Score}
	}
	out := make([]Match, 0, len(best))
	idx := make([]int, 0, len(best))
	for i := range best {
		idx = append(idx, i)
	}
	sort.Slice(idx, func(a, b int) bool {
		ma, mb := best[idx[a]], best[idx[b]]
		if ma.Score != mb.Score {
			return ma.Score > mb.Score
		}
		return idx[a] < idx[b]
	})
	for _, i := range idx {
		out = append(out, best[i])
	}
	return out
}

// Nearest is the index of the last symbol at or before line (0 when none).
func Nearest(syms []Symbol, line int) int {
	best := 0
	for i, s := range syms {
		if s.Line <= line {
			best = i
		}
	}
	return best
}

// runes turns sahilm/fuzzy's byte offsets into rune indexes.
func runes(s string, bytes []int) []int {
	if len(bytes) == 0 {
		return nil
	}
	out := make([]int, 0, len(bytes))
	r, j := 0, 0
	for b := range s {
		for j < len(bytes) && bytes[j] == b {
			out = append(out, r)
			j++
		}
		r++
	}
	return out
}
