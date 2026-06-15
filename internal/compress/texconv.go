package compress

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/noisethanks/stalker-tex/internal/scan"
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
func Run(texconvPath string, asset scan.Asset, format string, generateMips bool, outputDir string) CompressionResult {
	before, _ := fileSize(asset.Path)

	args := []string{
		"-f", format,
		"-y",           // overwrite
		"-o", outputDir,
	}
	if generateMips {
		args = append(args, "-m", "0") // generate full mip chain
	} else {
		args = append(args, "-m", "1") // no mip generation
	}
	args = append(args, asset.Path)

	cmd := exec.Command(texconvPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
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
