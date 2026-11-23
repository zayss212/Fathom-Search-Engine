package query

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"runtime"
	"sync"
)

const parallelSearchThreshold = 64 << 20

type matcher func(line []byte) bool

func searchFile(path string, m matcher) ([]Match, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()

	if size < parallelSearchThreshold {
		matches, _, err := scanSegment(io.NewSectionReader(f, 0, size), path, m)
		return matches, err
	}
	return searchParallel(f, size, path, m)
}

func scanSegment(r io.Reader, path string, m matcher) ([]Match, int, error) {
	var matches []Match
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10<<20)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if m(line) {
			matches = append(matches, Match{Path: path, Line: lineNo, Text: string(line)})
		}
	}
	return matches, lineNo, scanner.Err()
}

type segmentResult struct {
	matches   []Match
	lineCount int
	err       error
}

func searchParallel(f *os.File, size int64, path string, m matcher) ([]Match, error) {
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	if size/int64(workers) < 1<<20 {
		matches, _, err := scanSegment(io.NewSectionReader(f, 0, size), path, m)
		return matches, err
	}
	return searchParallelN(f, size, path, m, workers)
}

func searchParallelN(f *os.File, size int64, path string, m matcher, workers int) ([]Match, error) {
	naiveChunk := size / int64(workers)
	if naiveChunk < 1 {
		naiveChunk = 1
	}

	bounds := make([]int64, workers+1)
	bounds[workers] = size
	for i := 1; i < workers; i++ {
		aligned, err := nextLineStart(f, int64(i)*naiveChunk, size)
		if err != nil {
			return nil, err
		}
		bounds[i] = aligned
	}
	for i := 1; i <= workers; i++ {
		if bounds[i] < bounds[i-1] {
			bounds[i] = bounds[i-1]
		}
	}

	results := make([]segmentResult, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		start, end := bounds[i], bounds[i+1]
		if start >= end {
			continue
		}
		wg.Add(1)
		go func(idx int, start, end int64) {
			defer wg.Done()
			wf, err := os.Open(path)
			if err != nil {
				results[idx] = segmentResult{err: err}
				return
			}
			defer wf.Close()
			matches, lineCount, err := scanSegment(io.NewSectionReader(wf, start, end-start), path, m)
			results[idx] = segmentResult{matches: matches, lineCount: lineCount, err: err}
		}(i, start, end)
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
	}

	var all []Match
	lineOffset := 0
	for _, r := range results {
		for _, mt := range r.matches {
			mt.Line += lineOffset
			all = append(all, mt)
		}
		lineOffset += r.lineCount
	}
	return all, nil
}

func countFile(path string, m matcher) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	size := info.Size()

	if size < parallelSearchThreshold {
		count, _, err := countSegment(io.NewSectionReader(f, 0, size), m)
		return count, err
	}
	return countParallel(f, size, path, m)
}

func countSegment(r io.Reader, m matcher) (count int, lineCount int, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10<<20)
	for scanner.Scan() {
		lineCount++
		if m(scanner.Bytes()) {
			count++
		}
	}
	return count, lineCount, scanner.Err()
}

func countParallel(f *os.File, size int64, path string, m matcher) (int, error) {
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	if size/int64(workers) < 1<<20 {
		count, _, err := countSegment(io.NewSectionReader(f, 0, size), m)
		return count, err
	}
	return countParallelN(f, size, path, m, workers)
}

func countParallelN(f *os.File, size int64, path string, m matcher, workers int) (int, error) {
	naiveChunk := size / int64(workers)
	if naiveChunk < 1 {
		naiveChunk = 1
	}

	bounds := make([]int64, workers+1)
	bounds[workers] = size
	for i := 1; i < workers; i++ {
		aligned, err := nextLineStart(f, int64(i)*naiveChunk, size)
		if err != nil {
			return 0, err
		}
		bounds[i] = aligned
	}
	for i := 1; i <= workers; i++ {
		if bounds[i] < bounds[i-1] {
			bounds[i] = bounds[i-1]
		}
	}

	counts := make([]int, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		start, end := bounds[i], bounds[i+1]
		if start >= end {
			continue
		}
		wg.Add(1)
		go func(idx int, start, end int64) {
			defer wg.Done()
			wf, err := os.Open(path)
			if err != nil {
				errs[idx] = err
				return
			}
			defer wf.Close()
			count, _, err := countSegment(io.NewSectionReader(wf, start, end-start), m)
			counts[idx], errs[idx] = count, err
		}(i, start, end)
	}
	wg.Wait()

	total := 0
	for i, err := range errs {
		if err != nil {
			return 0, err
		}
		total += counts[i]
	}
	return total, nil
}

func nextLineStart(f *os.File, offset, size int64) (int64, error) {
	if offset <= 0 {
		return 0, nil
	}
	if offset >= size {
		return size, nil
	}

	buf := make([]byte, 64*1024)
	pos := offset
	for pos < size {
		n, err := f.ReadAt(buf, pos)
		if n > 0 {
			if idx := bytes.IndexByte(buf[:n], '\n'); idx >= 0 {
				return pos + int64(idx) + 1, nil
			}
		}
		if err != nil && err != io.EOF {
			return 0, err
		}
		pos += int64(n)
		if err == io.EOF || n == 0 {
			break
		}
	}
	return size, nil
}
