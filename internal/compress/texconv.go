package compress

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/noisethanks/stalker-tex/internal/scan"
	"github.com/noisethanks/stalker-tex/internal/tools"
)

// CompressionResult holds the outcome of a single texconv invocation.
type CompressionResult struct {
	Asset   scan.Asset
	Success bool
	Err     error
	Stderr  string
	Before  int64
	After   int64
}

// Run invokes texconv on a single asset and returns the result.
// outputDir should be filepath.Dir(asset.Path) for in-place compression.
func Run(ctx context.Context, texconvPath string, asset scan.Asset, format string, generateMips bool, outputDir string) CompressionResult {
	before, _ := fileSize(asset.Path)

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
		asset.Path,
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return CompressionResult{Asset: asset, Success: false, Err: err}
	}
	cmd := exec.Command(texconvPath, args...)
	tools.SetProcAttr(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return CompressionResult{Asset: asset, Success: false, Err: err}
	}
	_ = tools.WriteLock(cmd.Process.Pid)

	processExited := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			tools.KillProcess(cmd)
		case <-processExited:
		}
	}()

	err := cmd.Wait()
	close(processExited)
	_ = tools.ClearLock()

	if ctx.Err() != nil {
		outPath := filepath.Join(outputDir, filepath.Base(asset.Path))
		os.Remove(outPath)
		return CompressionResult{Asset: asset, Success: false, Err: ctx.Err()}
	}
	if err != nil {
		return CompressionResult{
			Asset:   asset,
			Success: false,
			Err:     fmt.Errorf("texconv: %w", err),
			Stderr:  stderr.String(),
			Before:  before,
		}
	}

	// texconv writes to outputDir/<basename>.dds
	outPath := filepath.Join(outputDir, filepath.Base(asset.Path))
	after, _ := fileSize(outPath)

	return CompressionResult{
		Asset:   asset,
		Success: true,
		Stderr:  stderr.String(),
		Before:  before,
		After:   after,
	}
}

func fileSize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}
