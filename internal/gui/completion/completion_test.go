package completion

import (
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPrefixAndWord(t *testing.T) {
	for _, c := range []struct {
		text   string
		pos    int
		prefix string
		start  int
	}{
		{"foo", 3, "foo", 0},
		{"a.bar", 5, "bar", 2},
		{"x = myVar", 6, "my", 4},    // mid-word: only the part before the cursor
		{"größe übel", 4, "größ", 0}, // umlauts are letters (rune offsets)
		{"übel", 2, "üb", 0},
		{"a(", 2, "", 2},
		{"", 0, "", 0},
		{"snake_case", 6, "snake_", 0},
	} {
		p, s := PrefixAt([]rune(c.text), c.pos)
		if p != c.prefix || s != c.start {
			t.Errorf("%q@%d: %q %d, want %q %d", c.text, c.pos, p, s, c.prefix, c.start)
		}
	}
	if s, e := WordAt([]rune("x myVar y"), 4); s != 2 || e != 7 {
		t.Error("word", s, e)
	}
}

func TestIndexWords(t *testing.T) {
	text := "alpha beta 1234 ab x1y2\nbeta gamma_1 übergröße\nthird line"
	got := IndexWords(text, -1)
	want := map[string]int{"alpha": 1, "beta": 2, "x1y2": 1, "gamma_1": 1, "übergröße": 1, "third": 1, "line": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%v", got)
	}
	if got := IndexWords(text, 1); got["gamma_1"] != 0 || got["beta"] != 1 {
		t.Errorf("skip line: %v", got)
	}
}

// A word typed for the first time does not suggest itself; one that is
// elsewhere in the buffer does.
func TestNoSelfMatch(t *testing.T) {
	doc := "func handler() {}\n\nhandleRequest(x)\nhandl"
	lines := strings.Split(doc, "\n")
	cur := 3
	line := []rune(lines[cur])
	col := len(line)
	words := IndexWords(doc, cur)
	lw := LineWords(line, col)
	prefix, _ := PrefixAt(line, col)
	items := Complete(prefix, Candidates(words, lw, nil), 0)
	var got []string
	for _, it := range items {
		got = append(got, it.Text)
	}
	if !reflect.DeepEqual(got, []string{"handler", "handleRequest"}) {
		t.Errorf("handl: %v", got)
	}
	// Typing a new word "zebra": nothing, not "zebra" itself.
	doc2 := "one two\nzebra"
	line2 := []rune("zebra")
	if items := Complete("zebra", Candidates(IndexWords(doc2, 1), LineWords(line2, 5), nil), 0); len(items) != 0 {
		t.Errorf("self match: %v", items)
	}
	// In the middle of a word only the part before the cursor counts, and
	// the word itself is not a candidate.
	line3 := []rune("call newThing here")
	lw3 := LineWords(line3, 7) // "new|Thing"
	if lw3["newThing"] != 0 || lw3["call"] != 1 || lw3["here"] != 1 {
		t.Errorf("line words: %v", lw3)
	}
	// The same word elsewhere on the line is a candidate.
	line4 := []rune("zebra zeb")
	if items := Complete("zeb", Candidates(nil, LineWords(line4, 9), nil), 0); len(items) != 1 || items[0].Text != "zebra" {
		t.Errorf("same line: %v", items)
	}
}

func TestDedupAndRanking(t *testing.T) {
	words := map[string]int{"format": 2, "fortune": 1, "for": 3, "platform": 1, "info": 1}
	kw := Keywords("go")
	cands := Candidates(words, nil, kw)
	n := 0
	for _, c := range cands {
		if c.Text == "for" {
			n++
			if c.Kind != KindKeyword {
				t.Error("for is a keyword")
			}
		}
	}
	if n != 1 {
		t.Fatalf("for %d times", n)
	}
	items := Complete("for", cands, 0)
	if len(items) == 0 {
		t.Fatal("no items")
	}
	for _, it := range items {
		if it.Text == "for" {
			t.Error("the prefix itself is suggested")
		}
	}
	// Prefix matches first, shorter first on equal scores; "platform"
	// (fuzzy only) after them.
	idx := map[string]int{}
	for i, it := range items {
		idx[it.Text] = i
	}
	if !(idx["format"] < idx["fortune"] && idx["fortune"] < idx["platform"]) {
		t.Errorf("order: %v", items)
	}
	if f := items[idx["format"]]; !reflect.DeepEqual(f.Matched, []int{0, 1, 2}) || f.Kind != KindDocumentWord || f.Kind.Badge() != "abc" {
		t.Errorf("format: %+v", f)
	}
	// Rune indexes for non-ASCII words.
	items = Complete("gr", Candidates(map[string]int{"übergröße": 1}, nil, nil), 0)
	if len(items) != 1 || !reflect.DeepEqual(items[0].Matched, []int{4, 5}) {
		t.Errorf("runes: %+v", items)
	}
	if Keywords("text") != nil || Keywords("") != nil {
		t.Error("plain text has keywords")
	}
	if len(Keywords("python")) == 0 || len(Keywords("rust")) == 0 || len(Keywords("sql")) == 0 {
		t.Error("missing keywords")
	}
	// The limit.
	many := map[string]int{}
	for i := 0; i < 50; i++ {
		many["word"+string(rune('a'+i%26))+string(rune('a'+i/26))] = 1
	}
	if got := Complete("wor", Candidates(many, nil, nil), 0); len(got) != MaxItems {
		t.Errorf("%d items", len(got))
	}
}

func TestIndexer(t *testing.T) {
	x := NewIndexer(nil)
	defer x.Close()
	x.delay = 5 * time.Millisecond
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= 30; i++ {
			x.Update(uint64(i), strings.Repeat("alpha beta ", i), -1)
		}
	}()
	wg.Wait()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if s := x.Snapshot(); s != nil && s.Gen == 30 {
			if s.Words["alpha"] != 30 {
				t.Fatal(s.Words)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no snapshot")
		}
		time.Sleep(2 * time.Millisecond)
	}
}
