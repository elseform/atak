//go:build linux

package tools

import _ "embed"

//go:embed bin/texconv-linux
var texconvBin []byte

//go:embed bin/7zz
var sevenZipBin []byte

const texconvName = "texconv"
const sevenZipName = "7zz"
