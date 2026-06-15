package compress

import (
	"path/filepath"
	"sync"

	"github.com/noisethanks/stalker-tex/internal/config"
	"github.com/noisethanks/stalker-tex/internal/scan"
)

// Job represents a single compression task.
type Job struct {
	Asset        scan.Asset
	Format       string
	GenerateMips bool
	OutputDir    string // filepath.Dir(asset.Path) for in-place
}

// RunPool executes jobs concurrently using workerCount goroutines.
// Results are sent to the returned channel, which is closed when all jobs complete.
func RunPool(texconvPath string, jobs []Job, workerCount int, cfg *config.Config) <-chan CompressionResult {
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
				outDir := job.OutputDir
				if outDir == "" {
					outDir = outputDir(job.Asset, cfg)
				}
				results <- Run(texconvPath, job.Asset, job.Format, job.GenerateMips, outDir)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

func outputDir(asset scan.Asset, cfg *config.Config) string {
	if cfg.CompressInPlace {
		return filepath.Dir(asset.Path)
	}
	return cfg.StagingDir
}
