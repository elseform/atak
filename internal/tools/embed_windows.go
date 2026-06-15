//go:build windows

package tools

import _ "embed"

//go:embed bin/texconv-windows.exe
var texconvBin []byte

//go:embed bin/7za.exe
var sevenZipBin []byte

const texconvName = "texconv.exe"
const sevenZipName = "7za.exe"
