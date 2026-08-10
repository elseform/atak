//go:build windows

package tools

import _ "embed"

//go:embed bin/texconv-windows.exe
var texconvBin []byte

//go:embed bin/7za.exe
var sevenZipBin []byte

//go:embed bin/compressonator-bc7e-windows.exe
var compressonatorBin []byte

const texconvName = "texconv.exe"
const sevenZipName = "7za.exe"
const compressonatorName = "compressonatorcli.exe"
