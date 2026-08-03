package compress

import (
	"strings"
	"testing"
)

// argValue returns the value following flag in an argv slice.
func argValue(args []string, flag string) (string, bool) {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

// TestShouldGenerateMips covers the per-file mip policy: a profile's generateMips:true
// forces a chain regardless of source (world textures are always minified), while
// generateMips:false defers to the source's own choice by default — keeping a chain when
// the source shipped one and none when it didn't. The stripWhenDisabled preference flips
// generateMips:false into an authoritative "strip", dropping even a mipped source's chain,
// but never overrides generateMips:true. A source with a single level (mipMapCount 1, or a
// malformed 0) counts as "no chain".
func TestShouldGenerateMips(t *testing.T) {
	tests := []struct {
		name        string
		profileMips bool
		sourceMips  int
		strip       bool
		want        bool
	}{
		{"profile forces on, source flat", true, 1, false, true},     // world texture without source mips
		{"profile forces on, source mipped", true, 11, false, true},  // world texture, source already mipped
		{"profile forces on, strip ignored", true, 11, true, true},   // strip never overrides generateMips:true
		{"profile off, source mipped", false, 11, false, true},       // moon flare: 11 mips preserved
		{"profile off, source flat", false, 1, false, false},         // flat UI/flare: stays single-level
		{"profile off, malformed zero", false, 0, false, false},      // 0 mip count treated as no chain
		{"profile off, strip drops mipped", false, 11, true, false},  // setting on: authoritative strip
		{"profile off, strip on, flat", false, 1, true, false},       // already flat, stays flat
	}
	for _, tt := range tests {
		if got := ShouldGenerateMips(tt.profileMips, tt.sourceMips, tt.strip); got != tt.want {
			t.Errorf("%s: ShouldGenerateMips(%v, %d, %v) = %v, want %v",
				tt.name, tt.profileMips, tt.sourceMips, tt.strip, got, tt.want)
		}
	}
}

// TestTexconvArgsMips is the regression guard for a profile's generateMips reaching
// texconv. The setting was plumbed correctly from profiles.json all the way to Run,
// which accepted it as a parameter and never read it — runOnce hardcoded "-m 0", so the
// six profiles declaring generateMips:false silently got full mip chains anyway. Go does
// not warn on unused parameters, so only an assertion on the built argv catches this.
func TestTexconvArgsMips(t *testing.T) {
	tests := []struct {
		generateMips bool
		want         string
	}{
		{true, "0"},  // full chain down to 1x1
		{false, "1"}, // top level only
	}
	for _, tt := range tests {
		args := texconvArgs("BC3_UNORM", tt.generateMips, 0, 0, "/out", "/in/x.dds")
		got, ok := argValue(args, "-m")
		if !ok {
			t.Fatalf("generateMips=%v: no -m flag in %v", tt.generateMips, args)
		}
		if got != tt.want {
			t.Errorf("generateMips=%v: -m %s, want -m %s", tt.generateMips, got, tt.want)
		}
	}
}

// TestTexconvArgsMaxTextureSize checks that -w/-h appear only when explicit targets are
// set. Both are passed together so texconv resizes to an exact pre-computed size that
// preserves aspect ratio (Run computes targetW/targetH before calling here).
func TestTexconvArgsMaxTextureSize(t *testing.T) {
	noResize := texconvArgs("BC7_UNORM", true, 0, 0, "/out", "/in/x.dds")
	if _, ok := argValue(noResize, "-w"); ok {
		t.Error("-w must be omitted when targetW is 0")
	}
	if _, ok := argValue(noResize, "-h"); ok {
		t.Error("-h must be omitted when targetH is 0")
	}
	withResize := texconvArgs("BC7_UNORM", true, 1024, 512, "/out", "/in/x.dds")
	if w, ok := argValue(withResize, "-w"); !ok || w != "1024" {
		t.Errorf("-w = %q (present=%v), want 1024", w, ok)
	}
	if h, ok := argValue(withResize, "-h"); !ok || h != "512" {
		t.Errorf("-h = %q (present=%v), want 512", h, ok)
	}
}

// TestTexconvArgsInputIsTerminated checks that the input path stays behind "--".
// Mod names in this corpus routinely begin with "-" (e.g. "-Kmack- Rifle Pack"), which
// texconv would otherwise parse as a flag.
func TestTexconvArgsInputIsTerminated(t *testing.T) {
	const in = "/mods/-Kmack- Rifle Pack/gamedata/textures/wpn/ak74.dds"
	args := texconvArgs("BC7_UNORM", true, 0, 0, "/out", in)
	if len(args) < 2 || args[len(args)-2] != "--" || args[len(args)-1] != in {
		t.Errorf("input path must be last and preceded by --, got %v", args[len(args)-3:])
	}
	if strings.Contains(strings.Join(args[:len(args)-2], " "), in) {
		t.Error("input path must not appear before the -- terminator")
	}
}
