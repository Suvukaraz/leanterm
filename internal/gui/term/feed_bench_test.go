package term

import (
	"bytes"
	"fmt"
	"testing"
)

// BenchmarkFeed measures how fast an engine parses plain output (seq).
func BenchmarkFeed(b *testing.B) {
	var buf bytes.Buffer
	for i := 1; i <= 20000; i++ {
		fmt.Fprintf(&buf, "%d\r\n", i)
	}
	data := buf.Bytes()
	for _, name := range Available() {
		b.Run(name, func(b *testing.B) {
			eng, err := New(name, Options{Cols: 120, Rows: 40, Callbacks: Callbacks{Output: func([]byte) {}}})
			if err != nil {
				b.Fatal(err)
			}
			defer eng.Close()
			b.SetBytes(int64(len(data)))
			for range b.N {
				for off := 0; off < len(data); off += 4096 {
					eng.Feed(data[off:min(off+4096, len(data))])
				}
			}
		})
	}
}
