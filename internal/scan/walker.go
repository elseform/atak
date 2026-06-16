package scan

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/noisethanks/stalker-tex/internal/config"
)

// Asset is a single texture file discovered during a scan.
type Asset struct {
	Path         string
	ModName      string
	CurrentFmt   string
	Compressed   bool
	Width        int
	Height       int
	HasAlpha     bool
	ProfileMatch string
	SuggestedFmt string
}

// Walk traverses modsDir, emitting Asset values for every .dds file found.
// Each asset is sent on the returned channel; the channel is closed when done.
// Errors during individual file parsing are non-fatal; the asset is skipped.
func Walk(modsDir string, profiles []config.Profile, exclusions []string) (<-chan Asset, <-chan error) {
	assets := make(chan Asset, 256)
	errs := make(chan error, 1)

	go func() {
		defer close(assets)
		defer close(errs)

		err := filepath.WalkDir(modsDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries, keep walking
			}
			if d.IsDir() {
				for _, pattern := range exclusions {
					if matched, err := filepath.Match(pattern, d.Name()); err == nil && matched {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".dds") {
				return nil
			}

			info, err := ParseDDS(path)
			if err != nil {
				return nil // skip unparseable files silently
			}
			if info.Compressed {
				return nil
			}

			rel, _ := filepath.Rel(modsDir, path)
			modName := modNameFromRel(rel)

			// Pass 1: header-based default.
			profileMatch := "Auto (no alpha)"
			suggestedFmt := "BC1_UNORM"
			if info.HasAlpha {
				profileMatch = "Auto (alpha)"
				suggestedFmt = "BC7_UNORM"
			}
			// Pass 2: filename pattern override wins if matched.
			if p, f := matchProfile(path, profiles); p != "" {
				profileMatch = p
				suggestedFmt = f
			}

			assets <- Asset{
				Path:         path,
				ModName:      modName,
				CurrentFmt:   info.Format,
				Compressed:   info.Compressed,
				Width:        info.Width,
				Height:       info.Height,
				HasAlpha:     info.HasAlpha,
				ProfileMatch: profileMatch,
				SuggestedFmt: suggestedFmt,
			}
			return nil
		})
		if err != nil {
			errs <- err
		}
	}()

	return assets, errs
}

// modNameFromRel extracts the top-level mod directory name from a relative path.
func modNameFromRel(rel string) string {
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return rel
}

// matchProfile returns the profile name and suggested format for the given file path.
func matchProfile(path string, profiles []config.Profile) (string, string) {
	base := strings.ToLower(filepath.Base(path))
	for _, p := range profiles {
		for _, pattern := range p.Patterns {
			// filepath.Match handles glob patterns.
			matched, err := filepath.Match(strings.ToLower(pattern), base)
			if err == nil && matched {
				return p.Name, p.Format
			}
			// Also try matching against the last two path components for "ui/*" style patterns.
			if strings.Contains(pattern, "/") || strings.Contains(pattern, string(filepath.Separator)) {
				rel := filepath.ToSlash(path)
				if idx := strings.LastIndex(rel, "/"); idx >= 0 {
					tail := strings.ToLower(rel[max(0, idx-20):])
					if matched, err := filepath.Match(strings.ToLower(filepath.ToSlash(pattern)), tail); err == nil && matched {
						return p.Name, p.Format
					}
				}
			}
		}
	}
	return "", ""
}
