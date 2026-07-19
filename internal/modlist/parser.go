package modlist

import (
	"os"
	"strings"
)

// ParseModList reads an MO2 modlist.txt and returns enabled mod names in
// priority order (highest priority first).
//
// MO2's modlist.txt is already stored highest priority first: the top line is
// the mod that wins loose-file conflicts, the bottom line is lowest priority.
// This matches the order BuildVirtualFS expects, so file order is preserved.
func ParseModList(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var enabled []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "+") {
			enabled = append(enabled, line[1:])
		}
	}

	return enabled, nil
}
