package query

import (
	"os"
	"path/filepath"
	"testing"

	"fathom/internal/corpus"
	"fathom/internal/index"
)

func TestCountMatchesSearchAcrossManyFiles(t *testing.T) {
	dir := t.TempDir()
	contents := []string{
		"alpha\nneedle one\nbeta\n",
		"needle two\ngamma\nneedle three\ndelta\n",
		"no match here at all\n",
		"", // empty file
		"needle four\n",
	}
	for i, c := range contents {
		path := filepath.Join(dir, "file"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b := index.NewBuilder()
	if _, err := corpus.BuildIndex(dir, nil, b, nil); err != nil {
		t.Fatal(err)
	}
	idxPath := filepath.Join(dir, "test.idx")
	if err := b.Write(idxPath); err != nil {
		t.Fatal(err)
	}
	idx, err := index.Open(idxPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		pattern            string
		regex, ignoreCase  bool
		wantMatches, wantN int
	}{
		{pattern: "needle", wantMatches: 4, wantN: 4},
		{pattern: "NEEDLE", ignoreCase: true, wantMatches: 4, wantN: 4},
		{pattern: "needle (one|two|three|four)", regex: true, wantMatches: 4, wantN: 4},
		{pattern: "nothing matches this", wantMatches: 0, wantN: 0},
	} {
		matches, err := Search(idx, tc.pattern, tc.regex, tc.ignoreCase)
		if err != nil {
			t.Fatalf("pattern %q: Search error: %v", tc.pattern, err)
		}
		if len(matches) != tc.wantMatches {
			t.Fatalf("pattern %q: Search got %d matches %+v, want %d", tc.pattern, len(matches), matches, tc.wantMatches)
		}

		n, err := Count(idx, tc.pattern, tc.regex, tc.ignoreCase)
		if err != nil {
			t.Fatalf("pattern %q: Count error: %v", tc.pattern, err)
		}
		if n != tc.wantN {
			t.Fatalf("pattern %q: Count got %d, want %d", tc.pattern, n, tc.wantN)
		}
		if n != len(matches) {
			t.Fatalf("pattern %q: Count (%d) disagrees with len(Search(...)) (%d)", tc.pattern, n, len(matches))
		}
	}
}
