//go:build darwin

package tools

import _ "embed"

//go:embed bin/texconv-macos
var texconvBin []byte

//go:embed bin/7zz-macos
var sevenZipBin []byte

// compressonator-bc7e is not built for macOS (upstream fork is Linux/Windows only).
// Stub the vars so callers can check len(compressonatorBin) == 0 to detect absence.
var compressonatorBin []byte
var _ = compressonatorBin

const texconvName = "texconv"
const sevenZipName = "7zz"
const compressonatorName = ""
