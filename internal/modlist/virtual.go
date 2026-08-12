package modlist

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// BuildVirtualFS assembles a virtual filesystem representing the merged MO2
// modlist. Returns map[relPath]absoluteSourcePath where relPath is relative to
// the mod's own root (e.g. "gamedata/textures/wpn/ak74.dds").
//
// modList must be ordered high→low priority (as returned by ParseModList).
// We iterate low→high so higher-priority mods overwrite lower-priority entries.
func BuildVirtualFS(modsDir string, modList []string) (map[string]string, error) {
	virtual := make(map[string]string)

	for i := len(modList) - 1; i >= 0; i-- {
		modPath := filepath.Join(modsDir, modList[i])
		err := filepath.WalkDir(modPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if d.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(modPath, path)
			if relErr != nil {
				return nil
			}
			relSlash := filepath.ToSlash(rel)
			if idx := strings.Index(relSlash, "gamedata/"); idx > 0 {
				// Mod's real content sits under a wrapper folder (e.g. an
				// unflattened FOMOD variant folder) rather than at the mod
				// root — anchor to gamedata so this file's virtual path
				// matches the true game path and merges/overrides correctly
				// against other mods that ship the same file without the
				// wrapper.
				relSlash = relSlash[idx:]
			}
			if strings.Count(relSlash, "gamedata") > 1 {
				return nil // variant folder (e.g. gamedata/Green/gamedata/...) — skip
			}
			virtual[filepath.FromSlash(relSlash)] = path
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return virtual, nil
}
