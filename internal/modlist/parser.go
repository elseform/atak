package modlist

import (
	"os"
	"strings"
)

// ParseModList reads an MO2 modlist.txt and returns enabled mod names in
// priority order (highest priority first). MO2 lists low priority first,
// so the slice is reversed before returning.
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

	// Reverse: modlist.txt is low→high, callers want high→low.
	for i, j := 0, len(enabled)-1; i < j; i, j = i+1, j-1 {
		enabled[i], enabled[j] = enabled[j], enabled[i]
	}
	return enabled, nil
}
