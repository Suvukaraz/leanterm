package symbols

import (
	"reflect"
	"testing"

	"github.com/odvcencio/gotreesitter"
	ts_go "github.com/odvcencio/gotreesitter/grammars/go"
	ts_md "github.com/odvcencio/gotreesitter/grammars/markdown"
	ts_py "github.com/odvcencio/gotreesitter/grammars/python"
	ts_rs "github.com/odvcencio/gotreesitter/grammars/rust"
	ts_ts "github.com/odvcencio/gotreesitter/grammars/typescript"
)

func extract(t *testing.T, lang *gotreesitter.Language, src string) []Symbol {
	t.Helper()
	tree, err := gotreesitter.NewParser(lang).Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Release()
	return Extract(tree.RootNode(), lang, []byte(src))
}

type sym struct {
	name      string
	kind      Kind
	line, col int
	container string
}

func brief(ss []Symbol) []sym {
	var out []sym
	for _, s := range ss {
		out = append(out, sym{s.Name, s.Kind, s.Line, s.Col, s.Container})
	}
	return out
}

func TestGo(t *testing.T) {
	src := `package x

const Answer = 42

var (
	größe int
)

type Server struct{ port int }

type Handler interface{ Serve() }

type ID string

func (s *Server) Start() error {
	var local int
	_ = local
	return nil
}

func main() {}
`
	got := brief(extract(t, ts_go.Language(), src))
	want := []sym{
		{"Answer", KindConst, 2, 6, ""},
		{"größe", KindVar, 5, 1, ""},
		{"Server", KindStruct, 8, 5, ""},
		{"Handler", KindInterface, 10, 5, ""},
		{"Serve", KindMethod, 10, 24, "Handler"},
		{"ID", KindType, 12, 5, ""},
		{"Start", KindMethod, 14, 17, "Server"},
		{"main", KindFunc, 20, 5, ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("go:\n got %v\nwant %v", got, want)
	}
}

func TestPythonRustTSMarkdown(t *testing.T) {
	py := brief(extract(t, ts_py.Language(), "LIMIT = 10\n\nclass Shape:\n    sides = 0\n    def area(self):\n        tmp = 1\n        return tmp\n\ndef helper():\n    pass\n"))
	wantPy := []sym{
		{"LIMIT", KindVar, 0, 0, ""},
		{"Shape", KindClass, 2, 6, ""},
		{"sides", KindVar, 3, 4, "Shape"},
		{"area", KindMethod, 4, 8, "Shape"},
		{"helper", KindFunc, 8, 4, ""},
	}
	if !reflect.DeepEqual(py, wantPy) {
		t.Errorf("python:\n got %v\nwant %v", py, wantPy)
	}

	rs := brief(extract(t, ts_rs.Language(), "struct Point { x: i32 }\n\nimpl Point {\n    fn len(&self) -> i32 { 0 }\n}\n\ntrait Draw { fn draw(&self); }\n\nfn main() {}\n"))
	wantRs := []sym{
		{"Point", KindStruct, 0, 7, ""},
		{"Point", KindImpl, 2, 5, ""},
		{"len", KindMethod, 3, 7, "Point"},
		{"Draw", KindTrait, 6, 6, ""},
		{"draw", KindMethod, 6, 16, "Draw"},
		{"main", KindFunc, 8, 3, ""},
	}
	if !reflect.DeepEqual(rs, wantRs) {
		t.Errorf("rust:\n got %v\nwant %v", rs, wantRs)
	}

	ts := brief(extract(t, ts_ts.Language(), "interface Props { a: number }\nexport class App {\n  render(): void {}\n}\nfunction util() { const inner = 1; }\nconst VERSION = '1';\n"))
	wantTs := []sym{
		{"Props", KindInterface, 0, 10, ""},
		{"App", KindClass, 1, 13, ""},
		{"render", KindMethod, 2, 2, "App"},
		{"util", KindFunc, 4, 9, ""},
		{"VERSION", KindVar, 5, 6, ""},
	}
	if !reflect.DeepEqual(ts, wantTs) {
		t.Errorf("typescript:\n got %v\nwant %v", ts, wantTs)
	}

	md := extract(t, ts_md.Language(), "# Title\n\ntext\n\n## Usage\n\nmore\n")
	if len(md) != 2 || md[0].Name != "Title" || md[1].Name != "Usage" || md[1].Line != 4 || md[0].Kind != KindHeading {
		t.Errorf("markdown: %v", brief(md))
	}
}

func TestEmpty(t *testing.T) {
	if Extract(nil, nil, nil) != nil {
		t.Error("nil tree")
	}
	if got := Filter(nil, "x"); len(got) != 0 {
		t.Error("filter of nothing")
	}
	if got := Filter(nil, ""); len(got) != 0 {
		t.Error("all of nothing")
	}
}

func TestFilter(t *testing.T) {
	syms := []Symbol{
		{Name: "NewServer", Kind: KindFunc, Line: 1},
		{Name: "Start", Kind: KindMethod, Line: 5, Container: "Server"},
		{Name: "stop", Kind: KindMethod, Line: 9, Container: "Server"},
		{Name: "main", Kind: KindFunc, Line: 20},
	}
	all := Filter(syms, "")
	if len(all) != 4 || all[0].Name != "NewServer" || all[3].Name != "main" {
		t.Error("empty query", all)
	}
	got := Filter(syms, "st")
	if len(got) != 2 || got[0].Name+got[1].Name != "stopStart" && got[0].Name+got[1].Name != "Startstop" || !reflect.DeepEqual(got[0].Matched, []int{0, 1}) {
		t.Errorf("st: %+v", got)
	}
	// "Server.stop" through the container.
	got = Filter(syms, "srvstop")
	if len(got) != 1 || got[0].Name != "stop" || !reflect.DeepEqual(got[0].Matched, []int{0, 1, 2, 3}) || len(got[0].MatchedContainer) != 3 {
		t.Errorf("srvstop: %+v", got)
	}
	if got := Filter(syms, "zzz"); len(got) != 0 {
		t.Error("zzz", got)
	}
	if Nearest(syms, 7) != 1 || Nearest(syms, 0) != 0 || Nearest(syms, 100) != 3 {
		t.Error("nearest")
	}
}
