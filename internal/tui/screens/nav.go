package screens

// NavTarget identifies where a NavigateMsg should take the app.
type NavTarget int

const (
	NavMenu NavTarget = iota
	NavScan
	NavResults
	NavCompressConfig
	NavCompress
	NavSummary
	NavBackup
	NavRestore
	NavSettings
	NavSaveConfig
	NavQuit
)

// NavigateMsg is sent by any sub-screen to request a screen transition.
// Data carries screen-specific payload (asset slice, job data, etc.).
type NavigateMsg struct {
	To   NavTarget
	Data any
}

// CompressConfigData is passed from Results → CompressConfig.
type CompressConfigData struct {
	Groups []AssetGroup // grouped by profile
}

// AssetGroup holds assets sharing the same compression profile.
type AssetGroup struct {
	ProfileName  string
	SuggestedFmt string
	Assets       []assetRef
}

type assetRef struct {
	Path         string
	ModName      string
	CurrentFmt   string
	Width        int
	Height       int
	Compressed   bool
}

// CompressJobData is passed from CompressConfig → Compress.
type CompressJobData struct {
	Groups       []ConfiguredGroup
	WorkerCount  int
}

// ConfiguredGroup is a profile group with user-confirmed settings.
type ConfiguredGroup struct {
	ProfileName  string
	Format       string
	GenerateMips bool
	Paths        []string
	OutputDir    string // empty means in-place (filepath.Dir of each asset)
}

// SummaryData is passed from Compress → Summary.
type SummaryData struct {
	Succeeded    int
	Failed       int
	TotalBefore  int64
	TotalAfter   int64
	Errors       []string
	RetryPaths   []string
}
