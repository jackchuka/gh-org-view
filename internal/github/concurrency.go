package github

import "sync"

// defaultWorkers bounds per-repo API fan-out (collaborators, CODEOWNERS).
const defaultWorkers = 8

// forEachConcurrent runs fn over items with at most `workers` concurrent calls.
// fn must write results into caller-owned pre-indexed storage (keyed by i);
// it must not append to a shared slice. The first non-nil error is returned.
func forEachConcurrent[T any](items []T, workers int, fn func(i int, item T) error) error {
	if workers < 1 {
		workers = 1
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, item T) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := fn(i, item); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(i, items[i])
	}
	wg.Wait()
	return firstErr
}
