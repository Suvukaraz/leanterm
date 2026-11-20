// Package completion finds word completions for the editor (KWrite's
// automatic word completion): the words of the document and the keywords
// of its language, ranked with github.com/sahilm/fuzzy. It has no UI; all
// positions are rune offsets (as in the editor's buffer).
package completion

import (
	"strings"
	"unicode"
)

// MinWordLen is the shortest word that is indexed and the prefix length
// that opens the completion by itself.
const MinWordLen = 3

// IsWordRune reports whether r belongs to a word: letters, digits and "_".
func IsWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// WordAt is the word around the rune offset pos of text: [start, end).
// start == end when pos is not in or at the end of a word.
func WordAt(text []rune, pos int) (start, end int) {
	pos = max(0, min(pos, len(text)))
	start, end = pos, pos
	for start > 0 && IsWordRune(text[start-1]) {
		start--
	}
	for end < len(text) && IsWordRune(text[end]) {
		end++
	}
	return start, end
}

// PrefixAt is the part of the word before pos and where it starts: what the
// completion replaces.
func PrefixAt(text []rune, pos int) (prefix string, start int) {
	pos = max(0, min(pos, len(text)))
	start = pos
	for start > 0 && IsWordRune(text[start-1]) {
		start--
	}
	return string(text[start:pos]), start
}

// indexable reports whether a word goes into the index: long enough and not
// a number.
func indexable(w []rune) bool {
	if len(w) < MinWordLen {
		return false
	}
	for _, r := range w {
		if !unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// CountWords adds the words of s to counts.
func CountWords(s string, counts map[string]int) {
	var w []rune
	flush := func() {
		if indexable(w) {
			counts[string(w)]++
		}
		w = w[:0]
	}
	for _, r := range s {
		if IsWordRune(r) {
			w = append(w, r)
			continue
		}
		flush()
	}
	flush()
}

// IndexWords counts the words of text, leaving out line skip (0-based; -1
// for none): the line being edited is counted live instead (LineWords), so
// a word typed for the first time never suggests itself.
func IndexWords(text string, skip int) map[string]int {
	counts := map[string]int{}
	line := 0
	for len(text) > 0 {
		l, rest, found := strings.Cut(text, "\n")
		if line != skip {
			CountWords(l, counts)
		}
		text = rest
		line++
		if !found {
			break
		}
	}
	return counts
}

// LineWords counts the words of one line except the word around the
// cursor (col, a rune offset in the line).
func LineWords(line []rune, col int) map[string]int {
	start, end := WordAt(line, col)
	counts := map[string]int{}
	CountWords(string(line[:start]), counts)
	CountWords(string(line[end:]), counts)
	return counts
}
