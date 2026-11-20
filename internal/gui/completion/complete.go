package completion

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/sahilm/fuzzy"
)

// Kind is where a candidate comes from.
type Kind int

const (
	KindDocumentWord Kind = iota
	KindKeyword
)

// Badge is the short label the popup shows for a kind.
func (k Kind) Badge() string {
	if k == KindKeyword {
		return "kw"
	}
	return "abc"
}

// Candidate is a completion candidate.
type Candidate struct {
	Text string
	Kind Kind
}

// Item is a ranked completion.
type Item struct {
	Candidate
	// Matched are the rune indexes of Text that match the prefix.
	Matched []int
	Score   int
}

// MaxItems is the most completions Complete returns.
const MaxItems = 20

// candidates implements fuzzy.Source.
type candidates []Candidate

func (c candidates) String(i int) string { return c[i].Text }
func (c candidates) Len() int            { return len(c) }

var _ fuzzy.Source = candidates(nil)

// Candidates merges the document's words and the language's keywords
// (a word that is also a keyword is one keyword candidate). words are the
// indexed words without the cursor line, line the cursor line's words
// without the word at the cursor.
func Candidates(words, line map[string]int, kw []string) []Candidate {
	out := make([]Candidate, 0, len(words)+len(line)+len(kw))
	seen := make(map[string]bool, cap(out))
	for _, k := range kw {
		if !seen[k] {
			seen[k] = true
			out = append(out, Candidate{Text: k, Kind: KindKeyword})
		}
	}
	for _, m := range []map[string]int{words, line} {
		for w, n := range m {
			if n > 0 && !seen[w] {
				seen[w] = true
				out = append(out, Candidate{Text: w, Kind: KindDocumentWord})
			}
		}
	}
	return out
}

// Complete ranks the candidates for prefix with sahilm/fuzzy: prefix
// matches first, then the fuzzy score, then shorter words, then
// alphabetically. A candidate equal to the prefix is left out. It returns
// at most limit items (MaxItems when limit <= 0).
func Complete(prefix string, cands []Candidate, limit int) []Item {
	if prefix == "" {
		return nil
	}
	if limit <= 0 {
		limit = MaxItems
	}
	matches := fuzzy.FindFromNoSort(prefix, candidates(cands))
	lower := strings.ToLower(prefix)
	type ranked struct {
		Item
		isPrefix bool
	}
	out := make([]ranked, 0, len(matches))
	for _, m := range matches {
		c := cands[m.Index]
		if c.Text == prefix {
			continue
		}
		out = append(out, ranked{
			Item:     Item{Candidate: c, Matched: runeIndexes(c.Text, m.MatchedIndexes), Score: m.Score},
			isPrefix: strings.HasPrefix(strings.ToLower(c.Text), lower),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.isPrefix != b.isPrefix {
			return a.isPrefix
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if la, lb := utf8.RuneCountInString(a.Text), utf8.RuneCountInString(b.Text); la != lb {
			return la < lb
		}
		if a.Kind != b.Kind {
			return a.Kind == KindDocumentWord
		}
		return a.Text < b.Text
	})
	if len(out) > limit {
		out = out[:limit]
	}
	items := make([]Item, len(out))
	for i, r := range out {
		items[i] = r.Item
	}
	return items
}

// runeIndexes turns sahilm/fuzzy's byte offsets into rune indexes.
func runeIndexes(s string, bytes []int) []int {
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

// RuneIndexes is runeIndexes for other users of sahilm/fuzzy matches.
func RuneIndexes(s string, byteOffsets []int) []int { return runeIndexes(s, byteOffsets) }
