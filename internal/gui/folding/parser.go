package folding

import (
	"sync"
	"time"

	"github.com/odvcencio/gotreesitter"
)

// Result is a parse's foldable ranges for the text of generation Gen.
type Result struct {
	Gen    uint64
	Ranges []Range
}

// Parser parses a document in the background and finds its foldable
// ranges. The parser and its syntax tree live on the worker goroutine only;
// the editor hands it texts with Update and picks up results with Result.
// Updates in quick succession (typing) are coalesced: the worker waits
// Delay after the first one and parses the latest. After the first parse,
// it edits the previous tree with the changed bytes and parses
// incrementally.
type Parser struct {
	langID string
	load   func() *gotreesitter.Language
	notify func()
	delay  time.Duration

	mu      sync.Mutex
	pending *request
	result  *Result
	closed  bool

	wake chan struct{}
	quit chan struct{}
	done chan struct{}
}

type request struct {
	gen  uint64
	text string
}

// NewParser starts a parser for a LeanEdit language. It returns nil when
// the language has no grammar (no folding). notify is called (from the
// worker goroutine) when a result is ready.
func NewParser(langID string, notify func()) *Parser {
	load := grammars[langID]
	if load == nil {
		return nil
	}
	p := &Parser{langID: langID, load: load, notify: notify, delay: 200 * time.Millisecond,
		wake: make(chan struct{}, 1), quit: make(chan struct{}), done: make(chan struct{})}
	go p.run()
	return p
}

// Language is the LeanEdit language id of the parser.
func (p *Parser) Language() string { return p.langID }

// Update asks for a parse of text, the document's generation gen.
func (p *Parser) Update(gen uint64, text string) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.pending = &request{gen: gen, text: text}
	p.mu.Unlock()
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// Result takes the latest result, if there is a new one.
func (p *Parser) Result() (Result, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.result == nil {
		return Result{}, false
	}
	r := *p.result
	p.result = nil
	return r, true
}

// Close stops the worker and frees the tree.
func (p *Parser) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	close(p.quit)
	<-p.done
}

func (p *Parser) run() {
	defer close(p.done)
	var (
		lang    *gotreesitter.Language
		parser  *gotreesitter.Parser
		tree    *gotreesitter.Tree
		oldText []byte
		broken  bool
	)
	defer func() {
		if tree != nil {
			tree.Release()
		}
	}()
	first := true
	for {
		select {
		case <-p.quit:
			return
		case <-p.wake:
		}
		if !first {
			// Coalesce the updates of typing.
			select {
			case <-p.quit:
				return
			case <-time.After(p.delay):
			}
		}
		first = false
		p.mu.Lock()
		req := p.pending
		p.pending = nil
		p.mu.Unlock()
		if req == nil || broken {
			continue
		}
		if parser == nil {
			lang = loadLanguage(p.load)
			if lang == nil {
				broken = true // the grammar failed to load: no folding
				p.publish(Result{Gen: req.gen})
				continue
			}
			parser = gotreesitter.NewParser(lang)
			parser.SetTimeoutMicros(5_000_000)
		}
		src := []byte(req.text)
		next := parse(parser, tree, oldText, src)
		if tree != nil {
			tree.Release()
		}
		tree, oldText = next, src
		var ranges []Range
		if tree != nil {
			ranges = rangesOf(tree, lang)
		}
		p.publish(Result{Gen: req.gen, Ranges: ranges})
	}
}

func (p *Parser) publish(r Result) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.result = &r
	p.mu.Unlock()
	if p.notify != nil {
		p.notify()
	}
}

// loadLanguage loads a grammar; a grammar that panics while loading counts
// as missing.
func loadLanguage(load func() *gotreesitter.Language) (lang *gotreesitter.Language) {
	defer func() {
		if recover() != nil {
			lang = nil
		}
	}()
	return load()
}

// rangesOf walks a tree; a panic in a grammar's tree means no ranges.
func rangesOf(tree *gotreesitter.Tree, lang *gotreesitter.Language) (out []Range) {
	defer func() {
		if recover() != nil {
			out = nil
		}
	}()
	return Ranges(tree.RootNode(), lang)
}

// parse parses src, incrementally when there is a previous tree: the tree
// is edited with the changed bytes first. It returns nil when parsing fails.
func parse(parser *gotreesitter.Parser, old *gotreesitter.Tree, oldSrc, src []byte) (tree *gotreesitter.Tree) {
	defer func() {
		if recover() != nil {
			tree = nil
		}
	}()
	if old != nil {
		old.Edit(inputEdit(oldSrc, src))
		if t, err := parser.ParseIncremental(src, old); err == nil && t != nil {
			return t
		}
	}
	t, err := parser.Parse(src)
	if err != nil {
		if t != nil {
			t.Release()
		}
		return nil
	}
	return t
}

// inputEdit describes the change from a to b as a tree-sitter edit (byte
// offsets, rows and byte columns).
func inputEdit(a, b []byte) gotreesitter.InputEdit {
	n := min(len(a), len(b))
	p := 0
	for p < n && a[p] == b[p] {
		p++
	}
	s := 0
	for s < n-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	start := point(a, p)
	return gotreesitter.InputEdit{
		StartByte:   uint32(p),
		OldEndByte:  uint32(len(a) - s),
		NewEndByte:  uint32(len(b) - s),
		StartPoint:  start,
		OldEndPoint: pointFrom(a, p, len(a)-s, start),
		NewEndPoint: pointFrom(b, p, len(b)-s, start),
	}
}

func point(s []byte, off int) gotreesitter.Point {
	return pointFrom(s, 0, off, gotreesitter.Point{})
}

// pointFrom is the point of off, counting from the point at of from.
func pointFrom(s []byte, from, off int, at gotreesitter.Point) gotreesitter.Point {
	pt := at
	for i := from; i < off; i++ {
		if s[i] == '\n' {
			pt.Row++
			pt.Column = 0
		} else {
			pt.Column++
		}
	}
	return pt
}
