//go:build darwin

package tools

import _ "embed"

//go:embed bin/texconv-macos
var texconvBin []byte

//go:embed bin/7zz-macos
var sevenZipBin []byte

//go:embed bin/compressonator-bc7e-macos-arm64
var compressonatorBin []byte

const texconvName = "texconv"
const sevenZipName = "7zz"
const compressonatorName = "compressonator-bc7e"
