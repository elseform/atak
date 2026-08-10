//go:build linux

package tools

import _ "embed"

//go:embed bin/texconv-linux
var texconvBin []byte

//go:embed bin/7zz
var sevenZipBin []byte

//go:embed bin/compressonator-bc7e-linux
var compressonatorBin []byte

const texconvName = "texconv"
const sevenZipName = "7zz"
const compressonatorName = "compressonatorcli"
