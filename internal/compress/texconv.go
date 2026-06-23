package compress

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/noisethanks/atak/internal/scan"
	"github.com/noisethanks/atak/internal/tools"
)

// CompressionResult holds the outcome of a single texconv invocation.
type CompressionResult struct {
	Asset        scan.Asset
	ActualFormat string // format actually used; may differ from requested if BC3_UNORM fallback was triggered
	Success      bool
	Err          error
	Stderr       string
	Before       int64
	After        int64
}

// Run invokes texconv on a single asset and returns the result.
// If format is BC7_UNORM and texconv exits non-zero, automatically retries with BC3_UNORM.
// outputDir should be filepath.Dir(asset.Path) for in-place compression.
func Run(ctx context.Context, texconvPath string, asset scan.Asset, format string, generateMips bool, outputDir string) CompressionResult {
	before, _ := fileSize(asset.Path)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return CompressionResult{Asset: asset, Success: false, Err: err}
	}

	actualFormat := format
	success, stderr, runErr, after := runOnce(ctx, texconvPath, asset.Path, format, outputDir)

	// BC7 fallback — only if ctx is still live (not a cancellation failure).
	if !success && format == "BC7_UNORM" && ctx.Err() == nil {
		actualFormat = "BC3_UNORM"
		success, stderr, runErr, after = runOnce(ctx, texconvPath, asset.Path, "BC3_UNORM", outputDir)
	}

	if ctx.Err() != nil {
		outPath := filepath.Join(outputDir, filepath.Base(asset.Path))
		os.Remove(outPath)
		return CompressionResult{Asset: asset, Success: false, Err: ctx.Err()}
	}
	if !success {
		return CompressionResult{
			Asset:        asset,
			ActualFormat: actualFormat,
			Success:      false,
			Err:          runErr,
			Stderr:       stderr,
			Before:       before,
		}
	}
	return CompressionResult{
		Asset:        asset,
		ActualFormat: actualFormat,
		Success:      true,
		Stderr:       stderr,
		Before:       before,
		After:        after,
	}
}

// runOnce executes a single texconv invocation and reports the outcome.
func runOnce(ctx context.Context, texconvPath, inputPath, format, outputDir string) (success bool, stderr string, err error, after int64) {
	args := []string{
		"-f", format,
		"-m", "0",       // full mip chain
		"-if", "CUBIC",  // cubic interpolation for mip generation
		"-bc", "x",      // quick BCn encoding (major BC7 speedup)
		"-gpu", "0",     // GPU accelerated compression, falls back to CPU if unavailable
		"-y",            // overwrite
		"-nologo",       // suppress header
		"-o", outputDir,
		"--",
		inputPath,
	}

	cmd := exec.Command(texconvPath, args...)
	tools.SetProcAttr(cmd)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	job, _ := tools.NewJob()
	if startErr := cmd.Start(); startErr != nil {
		tools.CloseJob(job)
		return false, "", fmt.Errorf("texconv: %w", startErr), 0
	}
	tools.AssignJob(job, cmd)
	defer tools.CloseJob(job)
	_ = tools.WriteLock(cmd.Process.Pid)

	processExited := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			tools.KillProcess(cmd)
		case <-processExited:
		}
	}()

	waitErr := cmd.Wait()
	close(processExited)
	_ = tools.ClearLock()

	stderrStr := stderrBuf.String()
	if waitErr != nil {
		return false, stderrStr, fmt.Errorf("texconv: %w", waitErr), 0
	}

	outPath := filepath.Join(outputDir, filepath.Base(inputPath))
	sz, _ := fileSize(outPath)
	return true, stderrStr, nil, sz
}

func fileSize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}
