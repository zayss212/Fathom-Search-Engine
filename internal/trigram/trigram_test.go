package trigram

import (
	"math/rand"
	"sort"
	"testing"
)

func referenceExtract(data []byte) []uint32 {
	seen := make(map[uint32]bool)
	for i := 0; i+2 < len(data); i++ {
		seen[Pack(data[i], data[i+1], data[i+2])] = true
	}
	out := make([]uint32, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func TestExtractMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	sizes := []int{0, 1, 2, 3, 4, 100, 1000, 100_000}
	for _, n := range sizes {
		data := make([]byte, n)
		r.Read(data)
		got := Extract(data)
		want := referenceExtract(data)
		if len(got) != len(want) {
			t.Fatalf("size %d: got %d trigrams, want %d", n, len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("size %d: mismatch at index %d: got %d, want %d", n, i, got[i], want[i])
			}
		}
	}
}
