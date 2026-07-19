package modlist

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestParseModListPriorityOrder locks in the direction of the priority order.
// MO2's modlist.txt lists highest priority first (top line wins loose-file
// conflicts), and ParseModList must preserve that order — the first enabled
// line must be index 0. Regression guard against re-introducing a reversal.
func TestParseModListPriorityOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "modlist.txt")
	content := "+HighMod\n+MedMod\n-DisabledMod\n+LowMod\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ParseModList(path)
	if err != nil {
		t.Fatalf("ParseModList: %v", err)
	}

	// Highest priority first, disabled entries dropped.
	want := []string{"HighMod", "MedMod", "LowMod"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wrong priority order:\n got  %v\n want %v", got, want)
	}
}

func TestParseModListMissingFile(t *testing.T) {
	if _, err := ParseModList(filepath.Join(t.TempDir(), "nope.txt")); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
