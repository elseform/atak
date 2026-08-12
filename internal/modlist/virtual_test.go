package modlist

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBuildVirtualFSHighestPriorityWins is the end-to-end guard for load-order
// correctness: when two mods provide the same relative path, the winning source
// in the virtual filesystem must be the higher-priority mod — the one listed
// first in modlist.txt (highest priority). Before the parser fix, the lowest
// priority mod won, silently compressing the overridden texture.
func TestBuildVirtualFSHighestPriorityWins(t *testing.T) {
	modsDir := t.TempDir()

	rel := filepath.Join("gamedata", "textures", "wpn", "ak74_d.dds")
	writeModFile(t, modsDir, "HighMod", rel, "high")
	writeModFile(t, modsDir, "LowMod", rel, "low")

	// As returned by ParseModList: highest priority first.
	modList := []string{"HighMod", "LowMod"}

	virtual, err := BuildVirtualFS(modsDir, modList)
	if err != nil {
		t.Fatalf("BuildVirtualFS: %v", err)
	}

	winner := virtual[rel]
	want := filepath.Join(modsDir, "HighMod", rel)
	if winner != want {
		t.Fatalf("wrong conflict winner:\n got  %s\n want %s", winner, want)
	}

	data, err := os.ReadFile(winner)
	if err != nil {
		t.Fatalf("read winner: %v", err)
	}
	if string(data) != "high" {
		t.Fatalf("winning file content = %q, want %q (lowest-priority mod won the conflict)", data, "high")
	}
}

// TestBuildVirtualFSAnchorsWrapperFolder covers a mod whose real content sits
// under an extra top-level folder (e.g. an unflattened FOMOD variant folder,
// mods/SomeMod/Hands/gamedata/...) instead of directly at the mod root. The
// virtual path must be anchored to gamedata so it matches the true game path
// — both for correct output structure and so it can merge/override against
// another mod shipping the same file without the wrapper.
func TestBuildVirtualFSAnchorsWrapperFolder(t *testing.T) {
	modsDir := t.TempDir()

	gameRel := filepath.Join("gamedata", "textures", "act", "act_glasses.dds")
	wrapped := filepath.Join("Hands", gameRel)
	writeModFile(t, modsDir, "WrappedMod", wrapped, "wrapped")

	modList := []string{"WrappedMod"}

	virtual, err := BuildVirtualFS(modsDir, modList)
	if err != nil {
		t.Fatalf("BuildVirtualFS: %v", err)
	}

	if _, ok := virtual[wrapped]; ok {
		t.Fatalf("virtual FS keyed the wrapper folder in, want it stripped: %q", wrapped)
	}
	got, ok := virtual[gameRel]
	if !ok {
		t.Fatalf("virtual FS missing anchored path %q; keys: %v", gameRel, virtual)
	}
	want := filepath.Join(modsDir, "WrappedMod", wrapped)
	if got != want {
		t.Fatalf("wrong source for anchored path:\n got  %s\n want %s", got, want)
	}
}

func writeModFile(t *testing.T, modsDir, mod, rel, content string) {
	t.Helper()
	full := filepath.Join(modsDir, mod, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
