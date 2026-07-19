package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/noisethanks/atak/internal/config"
)

// Asset is a single texture file discovered during a scan.
type Asset struct {
	Path           string
	ModName        string
	CurrentFmt     string
	Compressed     bool
	Excluded       bool
	Width          int
	Height         int
	HasAlpha       bool
	ProfileMatch   string
	SuggestedFmt   string
	VirtualRelPath string // set by WalkVirtual: path relative to mod root (e.g. gamedata/textures/wpn/ak74.dds)
}

// Walk traverses modsDir, emitting Asset values for every uncompressed .dds file found.
// excludePatterns are globs from profiles.json — patterns without '/' match the basename,
// patterns with '/' match the full path relative to the mod root. Matched files are emitted
// with Excluded:true. exclusions are directory-name globs from config.json — matched
// directories are skipped entirely.
// Returns three channels: assets, skipped count (one value sent on completion), and errors.
func Walk(modsDir string, profiles []config.Profile, excludePatterns []string, exclusions []string, minFileSizeBytes int) (<-chan Asset, <-chan int, <-chan error) {
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
			if fi, fiErr := d.Info(); fiErr == nil && fi.Size() < int64(minFileSizeBytes) {
				skipped++
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
			if strings.Count(filepath.ToSlash(rel), "gamedata") > 1 {
				skipped++
				return nil // variant folder with double gamedata path
			}
			modName := modNameFromRel(rel)
			base := strings.ToLower(filepath.Base(path))

			// mod-root-relative slash path for path-based excludePattern matching.
			relSlash := filepath.ToSlash(rel)
			var modRelSlash string
			if idx := strings.Index(relSlash, "/"); idx >= 0 {
				modRelSlash = relSlash[idx+1:]
			} else {
				modRelSlash = relSlash
			}

			// Global exclude check — runs before profile matching.
			if matchExcludePatterns(base, modRelSlash, excludePatterns) {
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
			profileName, suggestedFmt, profileExcluded := matchProfile(path, rel, profiles)
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

// WalkVirtual scans a pre-built virtual filesystem map (relPath→absPath) instead of
// walking a directory. Same classification logic as Walk. excludePatterns follow the
// same path-aware rules as Walk — relPath (already mod-root-relative) is used directly.
// Mod-level exclusions must be applied before calling by filtering the modList passed to BuildVirtualFS.
func WalkVirtual(virtualFS map[string]string, modsDir string, profiles []config.Profile, excludePatterns []string, minFileSizeBytes int) (<-chan Asset, <-chan int, <-chan error) {
	assets := make(chan Asset, 256)
	skippedCh := make(chan int, 1)
	errs := make(chan error, 1)

	go func() {
		defer close(errs)
		var skipped int

		for relPath, absPath := range virtualFS {
			if strings.Count(filepath.ToSlash(relPath), "gamedata") > 1 {
				skipped++
				continue // variant folder with double gamedata path
			}
			if !strings.EqualFold(filepath.Ext(absPath), ".dds") {
				continue
			}

			fi, err := os.Stat(absPath)
			if err != nil {
				continue
			}
			if fi.Size() < int64(minFileSizeBytes) {
				skipped++
				continue
			}

			relToMods, relErr := filepath.Rel(modsDir, absPath)
			if relErr != nil {
				continue
			}
			modName := modNameFromRel(relToMods)

			info, err := ParseDDS(absPath)
			if err != nil {
				continue
			}
			if info.Compressed {
				skipped++
				continue
			}

			base := strings.ToLower(filepath.Base(absPath))

			// Global exclude check — runs before profile matching.
			if matchExcludePatterns(base, filepath.ToSlash(relPath), excludePatterns) {
				assets <- Asset{
					Path:           absPath,
					ModName:        modName,
					CurrentFmt:     info.Format,
					Width:          info.Width,
					Height:         info.Height,
					HasAlpha:       info.HasAlpha,
					ProfileMatch:   "Excluded",
					Excluded:       true,
					VirtualRelPath: relPath,
				}
				continue
			}

			// Profile matching uses relPath (relative to mod root, not modsDir).
			profileName, suggestedFmt, profileExcluded := matchProfile(absPath, relPath, profiles)
			if profileExcluded {
				continue
			}
			if profileName == "" {
				profileName = "Unmatched"
			}

			assets <- Asset{
				Path:           absPath,
				ModName:        modName,
				CurrentFmt:     info.Format,
				Width:          info.Width,
				Height:         info.Height,
				HasAlpha:       info.HasAlpha,
				ProfileMatch:   profileName,
				SuggestedFmt:   suggestedFmt,
				VirtualRelPath: relPath,
			}
		}

		skippedCh <- skipped
		close(assets)
		close(skippedCh)
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

// matchProfile returns the profile name and suggested format for the given file.
// rel is the path relative to modsDir, used for path-based patterns like */textures/ui/*.
// profileExcluded is true when the file matched a profile's patterns but was caught by
// that profile's exclude list — caller should skip the file entirely (no emit).
func matchProfile(path, rel string, profiles []config.Profile) (name, format string, profileExcluded bool) {
	base := strings.ToLower(filepath.Base(path))
	relSlash := strings.ToLower(filepath.ToSlash(rel))
	for _, p := range profiles {
		matched := false
		for _, pattern := range p.Patterns {
			if m, _ := filepath.Match(strings.ToLower(pattern), base); m {
				matched = true
				break
			}
			if strings.Contains(pattern, "/") || strings.Contains(pattern, string(filepath.Separator)) {
				stripped := strings.ToLower(strings.Trim(filepath.ToSlash(pattern), "*"))
				if strings.Contains(relSlash, stripped) {
					matched = true
					break
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

// matchesExcludePattern matches a single excludePattern entry against a file.
// Patterns containing '/' are matched against the full mod-root-relative slash path;
// patterns without '/' are matched against the basename only.
func matchesExcludePattern(pattern, basename, relSlash string) bool {
	if strings.Contains(pattern, "/") {
		matched, _ := filepath.Match(
			strings.ToLower(pattern),
			strings.ToLower(filepath.ToSlash(relSlash)),
		)
		return matched
	}
	matched, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(basename))
	return matched
}

// matchExcludePatterns reports whether the file matches any excludePattern entry.
func matchExcludePatterns(basename, relSlash string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchesExcludePattern(pattern, basename, relSlash) {
			return true
		}
	}
	return false
}
