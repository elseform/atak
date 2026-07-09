package compress

import (
	"context"
	"sync"

	"github.com/noisethanks/atak/internal/scan"
)

// Job represents a single compression task.
type Job struct {
	Asset           scan.Asset
	Format          string
	GenerateMips    bool
	MaxTextureSize  int
	OutputDir       string // filepath.Dir(asset.Path) for in-place
}

// RunPool executes jobs concurrently using workerCount goroutines.
// Results are sent to the returned channel, which is closed when all jobs complete.
func RunPool(ctx context.Context, texconvPath string, jobs []Job, workerCount int) <-chan CompressionResult {
	results := make(chan CompressionResult, len(jobs))
	work := make(chan Job, len(jobs))

	for _, j := range jobs {
		work <- j
	}
	close(work)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range work {
				select {
				case <-ctx.Done():
					return
				default:
				}
				results <- Run(ctx, texconvPath, job.Asset, job.Format, job.GenerateMips, job.MaxTextureSize, job.OutputDir)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}
