package compress

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/noisethanks/atak/internal/scan"
)

// Job represents a single compression task.
type Job struct {
	Asset          scan.Asset
	Format         string
	GenerateMips   bool
	MaxTextureSize int
	OutputDir      string // filepath.Dir(asset.Path) for in-place
	// Mod output mode: when ModOutputDir is non-empty, output goes to
	// filepath.Join(ModOutputDir, RelPath) instead of OutputDir.
	ModOutputDir string // e.g. /mods/ATAK
	RelPath      string // e.g. gamedata/textures/wpn/ak74.dds
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
					if job.ModOutputDir != "" && job.RelPath != "" {
					outPath := filepath.Join(job.ModOutputDir, job.RelPath)
					if _, err := os.Stat(outPath); err == nil {
						// Already compressed on a previous run — skip incrementally.
						results <- CompressionResult{Asset: job.Asset, Success: true, OutputSkipped: true}
						continue
					}
					if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
						results <- CompressionResult{Asset: job.Asset, Success: false, Err: err}
						continue
					}
					job.OutputDir = filepath.Dir(outPath)
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
