package corpus

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"

	"fathom/internal/index"
	"fathom/internal/trigram"
)

type fileJob struct {
	path    string
	size    int64
	modTime int64

	reuse    bool
	oldDocID uint32
}

type fileResult struct {
	job      fileJob
	trigrams []uint32
	err      error
}

type Stats struct {
	Extracted int
	Reused    int
}

func BuildIndex(root string, old *index.Index, b *index.Builder, onSkip func(path string, reason error)) (Stats, error) {
	var oldByPath map[string]index.DocInfo
	var oldDocIDByPath map[string]uint32
	var oldTrigrams map[uint32][]uint32
	if old != nil {
		oldByPath = make(map[string]index.DocInfo, len(old.Docs))
		oldDocIDByPath = make(map[string]uint32, len(old.Docs))
		for i, d := range old.Docs {
			oldByPath[d.Path] = d
			oldDocIDByPath[d.Path] = uint32(i)
		}
		oldTrigrams = old.TrigramsByDoc()
	}

	var jobs []fileJob
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if onSkip != nil {
				onSkip(path, err)
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if onSkip != nil {
				onSkip(path, err)
			}
			return nil
		}

		job := fileJob{path: path, size: info.Size(), modTime: info.ModTime().Unix()}
		if prev, ok := oldByPath[path]; ok && prev.Size == uint64(job.size) && prev.ModTime == job.modTime {
			job.reuse = true
			job.oldDocID = oldDocIDByPath[path]
		}
		jobs = append(jobs, job)
		return nil
	})
	if err != nil {
		return Stats{}, err
	}

	results := make([]fileResult, len(jobs))

	var toExtract []int
	for i, job := range jobs {
		if job.reuse {
			results[i] = fileResult{job: job, trigrams: oldTrigrams[job.oldDocID]}
		} else {
			toExtract = append(toExtract, i)
		}
	}

	cpus := runtime.NumCPU()
	if cpus < 1 {
		cpus = 1
	}
	workers := cpus
	if workers > len(toExtract) {
		workers = len(toExtract)
	}

	perFileWorkers := cpus / max(workers, 1)
	if perFileWorkers < 1 {
		perFileWorkers = 1
	}

	if workers > 0 {
		jobIdx := make(chan int)
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range jobIdx {
					job := jobs[i]
					trigrams, err := trigram.ExtractFile(job.path, perFileWorkers)
					results[i] = fileResult{job: job, trigrams: trigrams, err: err}
				}
			}()
		}
		for _, i := range toExtract {
			jobIdx <- i
		}
		close(jobIdx)
		wg.Wait()
	}

	var stats Stats
	for _, r := range results {
		if r.err != nil {
			if onSkip != nil {
				onSkip(r.job.path, r.err)
			}
			continue
		}
		b.AddDocumentTrigrams(r.job.path, uint64(r.job.size), r.job.modTime, r.trigrams)
		if r.job.reuse {
			stats.Reused++
		} else {
			stats.Extracted++
		}
	}

	return stats, nil
}
