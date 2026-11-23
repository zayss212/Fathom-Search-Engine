package query

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, data []byte) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func literalMatcher(needle string) matcher {
	b := []byte(needle)
	return func(line []byte) bool { return bytes.Contains(line, b) }
}

func TestSearchParallelNAgainstSequential(t *testing.T) {
	cases := []string{
		"",
		"\n",
		"needle\n",
		"needle",     // no trailing newline
		"a\nb\nc\n",
		"a\nb\nc",    // last line no newline
		"\n\n\n",     // blank lines only
		"needle\nneedle\nneedle\n",
		"no match here\nor here\n",
	}

	for _, c := range cases {
		for _, workers := range []int{1, 2, 3, 4, 8} {
			f := writeTempFile(t, []byte(c))
			size := int64(len(c))

			want, _, err := scanSegment(bytes.NewReader([]byte(c)), f.Name(), literalMatcher("needle"))
			if err != nil {
				t.Fatalf("case %q: reference error: %v", c, err)
			}
			got, err := searchParallelN(f, size, f.Name(), literalMatcher("needle"), workers)
			if err != nil {
				t.Fatalf("case %q workers=%d: %v", c, workers, err)
			}
			if len(got) != len(want) {
				t.Fatalf("case %q workers=%d: got %d matches %v, want %d %v", c, workers, len(got), got, len(want), want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("case %q workers=%d: match %d = %+v, want %+v", c, workers, i, got[i], want[i])
				}
			}
		}
	}
}

func TestSearchParallelNRandomized(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	alphabet := []byte("ab needle\n")
	for trial := 0; trial < 150; trial++ {
		n := r.Intn(2000)
		data := make([]byte, n)
		for i := range data {
			data[i] = alphabet[r.Intn(len(alphabet))]
		}
		f := writeTempFile(t, data)
		size := int64(len(data))

		want, _, err := scanSegment(bytes.NewReader(data), f.Name(), literalMatcher("needle"))
		if err != nil {
			t.Fatalf("trial %d: reference error: %v", trial, err)
		}

		workers := 1 + r.Intn(8)
		got, err := searchParallelN(f, size, f.Name(), literalMatcher("needle"), workers)
		if err != nil {
			t.Fatalf("trial %d workers=%d: %v", trial, workers, err)
		}
		if len(got) != len(want) {
			t.Fatalf("trial %d workers=%d data=%q: got %d matches %v, want %d %v", trial, workers, data, len(got), got, len(want), want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("trial %d workers=%d data=%q: match %d = %+v, want %+v", trial, workers, data, i, got[i], want[i])
			}
		}
	}
}

func TestSearchFileEndToEnd(t *testing.T) {
	data := []byte("alpha\nneedle one\nbeta\nneedle two\ngamma\n")
	f := writeTempFile(t, data)
	path := f.Name()
	f.Close()

	matches, err := searchFile(path, literalMatcher("needle"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || matches[0].Line != 2 || matches[1].Line != 4 {
		t.Fatalf("unexpected matches: %+v", matches)
	}
}
