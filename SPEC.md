# stalker-tex — Project Specification

## What This Is

A single compiled Go binary that wraps 7-zip and texconv with a Bubble Tea TUI.
It helps STALKER GAMMA players back up their mod directory and compress textures
to reduce VRAM usage. The target user is non-technical — someone who followed a
YouTube guide to install GAMMA and wants better performance without breaking anything.

**This is not a platform. It is a focused utility with a small, permanent feature set.**

---

## Embedded Binaries

Both tools are embedded into the Go binary via `//go:embed` and build tags, so
the correct platform binary is baked in at compile time. Extracted to a temp
directory on startup, cleaned up on exit.

```
bin/
├── texconv-linux       # community Linux port of Microsoft's texconv
├── texconv-windows.exe # official Microsoft build
├── 7zz                 # 7-Zip standalone Linux binary
└── 7zz.exe             # 7-Zip standalone Windows binary
```

Build tag pattern — same variable name on both platforms, different file:

```go
//go:build linux

//go:embed bin/texconv-linux
var texconvBin []byte

//go:embed bin/7zz
var sevenZipBin []byte
```

```go
//go:build windows

//go:embed bin/texconv-windows.exe
var texconvBin []byte

//go:embed bin/7zz.exe
var sevenZipBin []byte
```

On startup:
1. Extract both binaries to `os.MkdirTemp`
2. `chmod 0755` both (no-op on Windows, harmless)
3. Store paths in an `EmbeddedTools` struct passed through the app
4. `defer tools.Cleanup()` in main

No other runtime dependencies. The binary must run on any x86-64 Linux or
Windows machine without the user installing anything.

---

## Configuration

Config directory is platform-aware via `os.UserConfigDir()` — no hardcoded paths:

```
Linux:   ~/.config/stalker-tex/
Windows: %AppData%\stalker-tex\
```

Each contains:
```
├── config.json          # user prefs (gamma path, backup path, last used settings)
└── profiles.json        # optional: overrides embedded compression profiles
```

Embedded default profiles (in `configs/compression_profiles.json`):

```json
{
  "profiles": [
    {
      "name": "Normal Maps",
      "format": "BC5_UNORM",
      "patterns": ["*_bump.*", "*_normal.*", "*_nm.*"],
      "generateMips": true
    },
    {
      "name": "UI / Icons",
      "format": "BC3_UNORM",
      "patterns": ["ui/*", "*_icon.*", "*_hud.*"]
    },
    {
      "name": "Diffuse / Color",
      "format": "BC7_UNORM",
      "patterns": ["*_d.*", "*_diff.*", "*_albedo.*"],
      "generateMips": true
    }
  ]
}
```

User-provided `~/.config/stalker-tex/profiles.json` takes precedence if present.
This is the primary contribution surface for community tuning — no Go required.

---

## Project Structure

```
stalker-tex/
├── main.go
├── go.mod
├── go.sum
├── bin/
│   ├── texconv
│   └── 7zz
├── configs/
│   └── compression_profiles.json
├── internal/
│   ├── tools/
│   │   └── embed.go         # binary extraction, EmbeddedTools struct
│   ├── config/
│   │   └── config.go        # load/save user config and profiles
│   ├── scan/
│   │   ├── walker.go        # walk mod directory, enumerate assets
│   │   └── dds.go           # parse DDS headers, classify format
│   ├── compress/
│   │   ├── texconv.go       # exec.Command wrapper, arg builder
│   │   └── worker.go        # goroutine pool, N concurrent jobs
│   ├── archive/
│   │   └── sevenzip.go      # backup, restore, list, verify via 7zz
│   └── tui/
│       ├── model.go         # top-level AppModel, screen enum, Init/Update/View
│       ├── styles.go        # lipgloss theme (one place, no scattered styling)
│       └── screens/
│           ├── welcome.go   # path config, first-run detection
│           ├── menu.go      # main menu hub
│           ├── backup.go    # backup manager screen
│           ├── restore.go   # mod picker + confirm + progress
│           ├── scan.go      # scanning spinner + live counter
│           ├── results.go   # scan results list, per-category breakdown
│           ├── compress.go  # execution screen, progress bar, live log
│           └── summary.go   # completion stats, error list, retry option
└── SPEC.md                  # this file
```

---

## Screen Flow

```
Welcome / Path Config
        │
        ▼
   Main Menu ◄──────────────────────────┐
   ├── Scan & Compress                  │
   ├── Backup Manager                   │
   ├── Restore Mod                      │
   └── Settings                         │
        │                               │
   ┌────┴────────────────────────┐      │
   │                             │      │
   ▼                             ▼      │
COMPRESS FLOW              BACKUP/RESTORE FLOW
                                        │
Scan (async, live counter)         Backup Manager
        │                          ├── list existing backups
        ▼                          ├── create new backup
Scan Results                       └── delete old backups
(grouped by profile)                        │
        │                          Restore Mod
        ▼                          ├── fuzzy search mod list
Compression Config                 ├── confirm dialog
(per-category override)            └── progress → done
        │
        ▼
Executing
(progress bar, live log, error counter)
        │
        ▼
Summary
(stats, errors, retry option)
        │
        └──────────────────────────────►  Main Menu
```

---

## Feature Spec

### 1. Backup Manager

- List existing backups in the GAMMA directory archive with size and date
- Create a new LZMA solid archive of the full GAMMA mods directory via:
  ```
  7zz a -t7z -m0=lzma2 -mx=6 -mfb=64 -md=32m -ms=on -bsp1 <output.7z> <mods_dir> -xr!downloads -xr!Downloads
  ```
  - `-mx=6` — balanced compression, reasonable RAM usage
  - `-mfb=64` — 64 fast bytes, well suited for binary/texture data
  - `-md=32m` — 32MB dictionary, keeps RAM usage sane on large mod lists
  - `-ms=on` — auto solid block sizing, let 7z decide
  - `-xr!downloads`, `-xr!Downloads` — always exclude downloads folder, both cases for Linux case-sensitivity
  - Compression level (`-mx`) is the only user-exposed knob (see Settings)
- Parse 7zz `-bsp1` stderr progress into a Bubble Tea progress bar
- Delete old backups with confirmation
- Verify archive integrity via `7zz t`

**First-run behavior:** if no backup exists and the user navigates to Scan & Compress,
show a warning screen recommending backup first. Do not block — let them proceed if they
explicitly choose to.

### 2. Restore Mod

- Run `7zz l <archive>` and parse the file listing into a mod name list
- Display as a searchable bubbles/list (fuzzy filter on mod name)
- Confirm dialog showing: mod name, backup date, size on disk
- Restore via:
  ```
  7zz e <archive> -o<mod_dir> "Mods/<ModName>/*" -y
  ```
- Stream progress back to UI, show completion or error

### 3. Scan

- Walk the MO2 mods directory recursively
- For each `.dds` file: read the first 128 bytes, parse the DDS header
- Classify as: already compressed (BC1/BC3/BC5/BC7), uncompressed, unknown
- Match filename against compression profiles to determine suggested format
- Emit `assetFoundMsg` per file (async Cmd) so UI stays live during scan
- Group results by matched profile for display

### 4. Compress

- User reviews scan results grouped by profile, can override per-category format
- Configurable worker count (default: `runtime.NumCPU() / 2`)
- Per-file texconv invocation:
  ```
  texconv -f <FORMAT> -m 0 -y -o <output_dir> <input_file>
  ```
- Capture stdout/stderr per file into `CompressionResult`
- Emit `compressionDoneMsg` per file to update progress bar
- On completion: show summary with success count, error count, estimated VRAM delta
- Error list is navigable; failed files can be retried with different settings

### 5. Settings

- GAMMA mods directory path — auto-detect from common locations:
  - Linux: `~/Games/GAMMA/mods`, `~/GAMMA/mods`, `$MO2_GAME_PATH`
  - Windows: `C:\Games\GAMMA\mods`, `D:\GAMMA\mods`, `%MO2_GAME_PATH%`
  - All path handling via `filepath.Join` — no hardcoded separators anywhere
- Backup archive path
- Backup compression level: Fast (`-mx=3`), Balanced (`-mx=6`, default), Maximum (`-mx=9`)
  - All other 7z flags (`-mfb=64 -md=32m -ms=on -xr!downloads`) are hardcoded, not user-exposed
- Worker thread count for texture compression (default: `max(1, runtime.NumCPU()/2)`)
- Whether to compress textures in-place or to a staging directory
- Persist to `os.UserConfigDir()/stalker-tex/config.json`

---

## Data Structures

```go
// Core asset record produced by scan
type Asset struct {
    Path        string
    ModName     string
    CurrentFmt  string   // from DDS header: "DXT1", "DXT5", "R8G8B8", etc.
    Compressed  bool
    Width       int
    Height      int
    HasAlpha    bool
    ProfileMatch string  // which profile matched, "" if none
    SuggestedFmt string  // "BC1_UNORM", "BC3_UNORM", etc.
}

// Result of one texconv invocation
type CompressionResult struct {
    Asset   Asset
    Success bool
    Err     error
    Stderr  string
    Before  int64  // bytes
    After   int64
}

// Bubble Tea messages
type assetFoundMsg      struct{ asset Asset }
type scanCompleteMsg    struct{ total int; skipped int }
type compressionDoneMsg struct{ result CompressionResult }
type archiveProgressMsg struct{ percent int; currentFile string }
type archiveDoneMsg     struct{ err error }
```

---

## Bubble Tea Conventions

These must be followed consistently or the architecture drifts:

- `Update` is pure. No I/O, no side effects, no blocking calls.
- All I/O happens in `Cmd` functions that return a `Msg`.
- Sub-screens each have their own `Model`, `Update`, and `View`.
- Top-level `AppModel` delegates to the active screen's Update/View.
- All styling is in `tui/styles.go` via lipgloss. No inline color strings elsewhere.
- Screen transitions happen by returning a new screen enum value from Update.
  The top-level model swaps the active screen on the next render cycle.

---

## What This Is Not

To keep maintenance footprint small, the following are explicitly out of scope:

- macOS support (not a GAMMA platform)
- 32-bit builds (x86-64 only)
- Plugin or extension system
- Network features (no auto-update, no telemetry, no download)
- Support for archive formats other than 7z
- Texture formats other than DDS input / BCn output
- MO2 integration beyond reading the mods directory path

---

## Dependencies

```
github.com/charmbracelet/bubbletea   # TUI runtime
github.com/charmbracelet/bubbles     # list, textinput, progress, spinner
github.com/charmbracelet/lipgloss    # styling
```

No other external dependencies. Standard library only for everything else.

---

## Build

```bash
# Development
go run ./main.go

# Release — Linux x86-64, static
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o stalker-tex-linux ./main.go

# Release — Windows x86-64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o stalker-tex-windows.exe ./main.go

# goreleaser handles both targets in CI
```

## Build Targets

- `linux/amd64` — primary, tested by maintainer
- `windows/amd64` — supported, community-tested

The `-s -w` flags strip debug info. Final binaries should be under 25MB including
all embedded tools (texconv + 7zz per platform).

## Cross-Platform Rules

These must be followed in every file or Windows support silently breaks:

- **Never** use `/` as a path separator. Always `filepath.Join`.
- **Never** hardcode `~/.config`. Always `os.UserConfigDir()`.
- **Never** assume execute permissions need setting on Windows — `chmod` calls
  must be gated behind a `//go:build linux` file or a runtime `runtime.GOOS` check.
- All subprocess invocations via `exec.Command` use the extracted binary path
  from `EmbeddedTools` — never a hardcoded binary name.
