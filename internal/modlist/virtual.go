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
			if strings.Count(filepath.ToSlash(rel), "gamedata") > 1 {
				return nil // variant folder (e.g. gamedata/Green/gamedata/...) — skip
			}
			virtual[rel] = path
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return virtual, nil
}
