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
		args := texconvArgs("BC3_UNORM", tt.generateMips, 0, "/out", "/in/x.dds")
		got, ok := argValue(args, "-m")
		if !ok {
			t.Fatalf("generateMips=%v: no -m flag in %v", tt.generateMips, args)
		}
		if got != tt.want {
			t.Errorf("generateMips=%v: -m %s, want -m %s", tt.generateMips, got, tt.want)
		}
	}
}

// TestTexconvArgsMaxTextureSize checks that -w appears only when a limit is set, since
// texconv treats -w as an exact width and would otherwise upscale small textures.
func TestTexconvArgsMaxTextureSize(t *testing.T) {
	if _, ok := argValue(texconvArgs("BC7_UNORM", true, 0, "/out", "/in/x.dds"), "-w"); ok {
		t.Error("-w must be omitted when maxTextureSize is 0")
	}
	got, ok := argValue(texconvArgs("BC7_UNORM", true, 2048, "/out", "/in/x.dds"), "-w")
	if !ok || got != "2048" {
		t.Errorf("-w = %q (present=%v), want 2048", got, ok)
	}
}

// TestTexconvArgsInputIsTerminated checks that the input path stays behind "--".
// Mod names in this corpus routinely begin with "-" (e.g. "-Kmack- Rifle Pack"), which
// texconv would otherwise parse as a flag.
func TestTexconvArgsInputIsTerminated(t *testing.T) {
	const in = "/mods/-Kmack- Rifle Pack/gamedata/textures/wpn/ak74.dds"
	args := texconvArgs("BC7_UNORM", true, 0, "/out", in)
	if len(args) < 2 || args[len(args)-2] != "--" || args[len(args)-1] != in {
		t.Errorf("input path must be last and preceded by --, got %v", args[len(args)-3:])
	}
	if strings.Contains(strings.Join(args[:len(args)-2], " "), in) {
		t.Error("input path must not appear before the -- terminator")
	}
}
