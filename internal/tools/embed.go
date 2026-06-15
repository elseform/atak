package tools

import (
	"fmt"
	"os"
	"runtime"
)

// EmbeddedTools holds paths to extracted binaries for the current session.
type EmbeddedTools struct {
	TexconvPath  string
	SevenZipPath string
	tmpDir       string
}

// Cleanup removes the temp directory containing extracted binaries.
func (t *EmbeddedTools) Cleanup() {
	if t.tmpDir != "" {
		os.RemoveAll(t.tmpDir)
	}
}

func writeBin(dir, name string, data []byte) (string, error) {
	path := dir + string(os.PathSeparator) + name
	if err := os.WriteFile(path, data, 0755); err != nil {
		return "", fmt.Errorf("write %s: %w", name, err)
	}
	if runtime.GOOS == "linux" {
		if err := os.Chmod(path, 0755); err != nil {
			return "", fmt.Errorf("chmod %s: %w", name, err)
		}
	}
	return path, nil
}

// Extract writes both embedded binaries to a temp dir and returns the tool paths.
func Extract() (*EmbeddedTools, error) {
	dir, err := os.MkdirTemp("", "stalker-tex-*")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp: %w", err)
	}

	tcPath, err := writeBin(dir, texconvName, texconvBin)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	szPath, err := writeBin(dir, sevenZipName, sevenZipBin)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	return &EmbeddedTools{
		TexconvPath:  tcPath,
		SevenZipPath: szPath,
		tmpDir:       dir,
	}, nil
}
