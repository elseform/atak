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
	Excluded     bool
	Width        int
	Height       int
	HasAlpha     bool
	ProfileMatch string
	SuggestedFmt string
}

// Walk traverses modsDir, emitting Asset values for every uncompressed .dds file found.
// excludePatterns are basename globs from profiles.json — matched files are emitted with
// Excluded:true and ProfileMatch:"Excluded". exclusions are directory-name globs from
// config.json — matched directories are skipped entirely.
// Returns three channels: assets, skipped count (one value sent on completion), and errors.
func Walk(modsDir string, profiles []config.Profile, excludePatterns []string, exclusions []string) (<-chan Asset, <-chan int, <-chan error) {
	assets := make(chan Asset, 256)
	skippedCh := make(chan int, 1)
	errs := make(chan error, 1)

	go func() {
		defer close(errs)
		var skipped int

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
				skipped++
				return nil
			}

			rel, _ := filepath.Rel(modsDir, path)
			modName := modNameFromRel(rel)
			base := strings.ToLower(filepath.Base(path))

			// Global exclude check — runs before profile matching.
			if matchPatterns(base, excludePatterns) {
				assets <- Asset{
					Path:         path,
					ModName:      modName,
					CurrentFmt:   info.Format,
					Width:        info.Width,
					Height:       info.Height,
					HasAlpha:     info.HasAlpha,
					ProfileMatch: "Excluded",
					Excluded:     true,
				}
				return nil
			}

			// Profile matching with per-profile exclusion.
			profileName, suggestedFmt, profileExcluded := matchProfile(path, profiles)
			if profileExcluded {
				return nil // silently skip — matched profile but caught by profile's exclude list
			}
			if profileName == "" {
				profileName = "Unmatched"
			}

			assets <- Asset{
				Path:         path,
				ModName:      modName,
				CurrentFmt:   info.Format,
				Compressed:   info.Compressed,
				Width:        info.Width,
				Height:       info.Height,
				HasAlpha:     info.HasAlpha,
				ProfileMatch: profileName,
				SuggestedFmt: suggestedFmt,
			}
			return nil
		})
		skippedCh <- skipped
		close(assets)
		close(skippedCh)
		if err != nil {
			errs <- err
		}
	}()

	return assets, skippedCh, errs
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
// profileExcluded is true when the file matched a profile's patterns but was caught by
// that profile's exclude list — caller should skip the file entirely (no emit).
func matchProfile(path string, profiles []config.Profile) (name, format string, profileExcluded bool) {
	base := strings.ToLower(filepath.Base(path))
	for _, p := range profiles {
		matched := false
		for _, pattern := range p.Patterns {
			if m, _ := filepath.Match(strings.ToLower(pattern), base); m {
				matched = true
				break
			}
			// Also try matching against last two path components for "ui/*" style patterns.
			if strings.Contains(pattern, "/") || strings.Contains(pattern, string(filepath.Separator)) {
				rel := filepath.ToSlash(path)
				if idx := strings.LastIndex(rel, "/"); idx >= 0 {
					tail := strings.ToLower(rel[max(0, idx-20):])
					if m, _ := filepath.Match(strings.ToLower(filepath.ToSlash(pattern)), tail); m {
						matched = true
						break
					}
				}
			}
		}
		if matched {
			if matchPatterns(base, p.Exclude) {
				return "", "", true
			}
			return p.Name, p.Format, false
		}
	}
	return "", "", false
}

// matchPatterns reports whether base matches any of the given glob patterns.
func matchPatterns(base string, patterns []string) bool {
	for _, pattern := range patterns {
		if m, _ := filepath.Match(strings.ToLower(pattern), base); m {
			return true
		}
	}
	return false
}
