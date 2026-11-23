package query

import (
	"bytes"
	"regexp"
	"runtime"
	"sync"

	"fathom/internal/bitset"
	"fathom/internal/index"
)

type Match struct {
	Path string
	Line int
	Text string
}

func plan(idx *index.Index, pattern string, isRegex bool, ignoreCase bool) (matcher, []uint32, error) {
	var re *regexp.Regexp
	if isRegex || ignoreCase {
		p := pattern
		if !isRegex {
			p = regexp.QuoteMeta(pattern)
		}
		if ignoreCase {
			p = "(?i)" + p
		}
		compiled, err := regexp.Compile(p)
		if err != nil {
			return nil, nil, err
		}
		re = compiled
	}

	required := RequiredTrigrams(pattern, isRegex, ignoreCase)
	candidates := candidateDocIDs(idx, required)

	var match matcher
	if re != nil {
		match = re.Match
	} else {
		needle := []byte(pattern)
		match = func(line []byte) bool { return bytes.Contains(line, needle) }
	}
	return match, candidates, nil
}

func Search(idx *index.Index, pattern string, isRegex bool, ignoreCase bool) ([]Match, error) {
	match, candidates, err := plan(idx, pattern, isRegex, ignoreCase)
	if err != nil {
		return nil, err
	}

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	if workers > len(candidates) {
		workers = len(candidates)
	}

	results := make([][]Match, len(candidates))
	if workers > 0 {
		jobs := make(chan int)
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range jobs {
					doc := idx.Docs[candidates[i]]
					m, _ := searchFile(doc.Path, match)
					results[i] = m
				}
			}()
		}
		for i := range candidates {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
	}

	var all []Match
	for _, m := range results {
		all = append(all, m...)
	}
	return all, nil
}

func Count(idx *index.Index, pattern string, isRegex bool, ignoreCase bool) (int, error) {
	match, candidates, err := plan(idx, pattern, isRegex, ignoreCase)
	if err != nil {
		return 0, err
	}

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	if workers > len(candidates) {
		workers = len(candidates)
	}

	counts := make([]int, len(candidates))
	if workers > 0 {
		jobs := make(chan int)
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range jobs {
					doc := idx.Docs[candidates[i]]
					c, _ := countFile(doc.Path, match)
					counts[i] = c
				}
			}()
		}
		for i := range candidates {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
	}

	total := 0
	for _, c := range counts {
		total += c
	}
	return total, nil
}

func candidateDocIDs(idx *index.Index, required []uint32) []uint32 {
	if len(required) == 0 {
		all := make([]uint32, len(idx.Docs))
		for i := range all {
			all[i] = uint32(i)
		}
		return all
	}

	var bitmaps []*bitset.Bitmap
	seen := make(map[uint32]bool, len(required))
	for _, t := range required {
		if seen[t] {
			continue
		}
		seen[t] = true
		bm := idx.PostingsBitmap(t)
		if bm == nil {
			return nil
		}
		bitmaps = append(bitmaps, bm)
	}

	result := bitmaps[0]
	for _, bm := range bitmaps[1:] {
		result = bitset.And(result, bm)
	}
	return result.ToSlice()
}
