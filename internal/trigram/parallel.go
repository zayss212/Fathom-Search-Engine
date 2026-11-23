package trigram

import (
	"io"
	"os"
	"runtime"
	"sync"
)

const parallelThreshold = 64 << 20

func ExtractFile(path string, maxWorkers int) ([]uint32, error) {
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

	if maxWorkers <= 1 || size < parallelThreshold {
		return ExtractReader(f)
	}
	return extractParallel(path, size, maxWorkers)
}

func extractParallel(path string, size int64, maxWorkers int) ([]uint32, error) {
	workers := maxWorkers
	if workers > runtime.NumCPU() {
		workers = runtime.NumCPU()
	}
	if workers < 1 {
		workers = 1
	}
	chunkSize := size / int64(workers)
	if chunkSize < 1<<20 {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return ExtractReader(f)
	}

	sets := make([]*Set, workers)
	errs := make([]error, workers)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		start := int64(w) * chunkSize
		end := start + chunkSize
		if w == workers-1 {
			end = size
		}
		readEnd := end
		if w != workers-1 && readEnd+2 <= size {
			readEnd += 2
		} else if w != workers-1 {
			readEnd = size
		}

		wg.Add(1)
		go func(idx int, start, readEnd int64) {
			defer wg.Done()
			wf, err := os.Open(path)
			if err != nil {
				errs[idx] = err
				return
			}
			defer wf.Close()
			sr := io.NewSectionReader(wf, start, readEnd-start)
			s := NewSet()
			if err := extractInto(s, sr); err != nil {
				errs[idx] = err
				return
			}
			sets[idx] = s
		}(w, start, readEnd)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	merged := sets[0]
	for _, s := range sets[1:] {
		merged.Or(s)
	}
	return merged.ToSlice(), nil
}
