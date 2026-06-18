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
├── config.json     # user prefs (gamma path, backup path, last used settings)
└── profiles.json   # compression profiles — created on first run from embedded default
```

### Profiles Design

**All compression logic lives in `profiles.json` — nothing is hardcoded in the binary.**
This includes format selection, pattern matching, and mip generation. The binary only
knows how to read and apply profiles, not what they should contain.

**First-run behavior:** if `profiles.json` does not exist in the config dir, the tool
copies the embedded default to `~/.config/stalker-tex/profiles.json` and shows a
one-time notice screen before proceeding to the main menu:

```
┌─────────────────────────────────────────────────────┐
│  Compression profiles created                       │
│                                                     │
│  A default profiles.json has been created at:       │
│  ~/.config/stalker-tex/profiles.json                │
│                                                     │
│  Edit this file to customize which textures get     │
│  compressed and with which format. Changes take     │
│  effect on the next scan.                           │
│                                                     │
│  Press any key to continue                          │
└─────────────────────────────────────────────────────┘
```

This notice is shown exactly once — never again after the file exists.
Implemented as a dedicated screen `internal/tui/screens/firstrun.go`.
The user owns `profiles.json` from this point forward — the tool never
overwrites it on subsequent launches.

**The embedded default** (`configs/compression_profiles.json`) is the seed — it ships
with broadly correct STALKER conventions but users are expected to tune it:

```json
{
  "profiles": [
    {
      "name": "Normal Maps",
      "format": "BC5_UNORM",
      "generateMips": true,
      "patterns": ["*_bump.*", "*_normal.*", "*_nm.*", "*_nmap.*"]
    },
    {
      "name": "UI / Icons",
      "format": "BC3_UNORM",
      "generateMips": false,
      "patterns": ["ui/*", "*_icon.*", "*_hud.*", "*_ui.*"]
    },
    {
      "name": "Diffuse / Color",
      "format": "BC7_UNORM",
      "generateMips": true,
      "patterns": ["*_d.*", "*_diff.*", "*_albedo.*", "*_base.*", "*_col.*"]
    },
    {
      "name": "Specular / Gloss",
      "format": "BC3_UNORM",
      "generateMips": true,
      "patterns": ["*_spec.*", "*_gloss.*"]
    }
  ]
}
```

**Unmatched files** — DDS files that don't match any profile pattern are surfaced in
scan results as a separate "Unmatched" bucket. The user can assign them a format
manually in the Compression Config screen before compressing, or skip them entirely.

**Community sharing** — users can share `profiles.json` files tuned for specific mod
packs. The Settings screen shows the path to `profiles.json` and offers an
"Open in editor" option using `$EDITOR` (Linux) or `notepad.exe` (Windows).

---

## Project Structure

```
stalker-tex/
├── main.go
├── go.mod
├── go.sum
├── SPEC.md
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
│       ├── components/
│       │   ├── modpicker.go    # shared fuzzy mod picker (restore + compress)
│       │   └── operation.go   # shared progress screen (backup/restore/verify/compress)
│       └── screens/
│           ├── welcome.go      # path config, first-run detection
│           ├── firstrun.go     # one-time profiles.json creation notice
│           ├── menu.go         # main menu hub (3 items: Scan, Backup, Settings)
│           ├── about.go        # about + third-party licenses screen
│           ├── backup.go       # backup manager — all archive ops including restore
│           ├── scan.go         # scanning spinner + live counter
│           ├── results.go      # scan results list, per-category breakdown
│           ├── compress.go     # execution screen, progress bar, live log
│           └── summary.go      # completion stats, error list, retry option
│           # restore.go removed — functionality absorbed into backup.go
└── SPEC.md                  # this file
```

---

## Shared Operation Screen

All long-running operations (backup, restore, verify, compress) use a single
shared `OperationScreen` component at `internal/tui/components/operation.go`.

```
┌─────────────────────────────────────────┐
│  <Operation Title>                      │
│                                         │
│  [spinner]                              │
│  [progress bar]                         │
│  Status: <current file or status line>  │
│  Size: <archive or output size>         │
│  Elapsed: <time>                        │
│                                         │
│  ctrl+c to cancel                       │
└─────────────────────────────────────────┘
```

The component accepts:
- A title string
- A channel of `OperationProgressMsg` (percent int, status string, size int64)
- A cancel function

All four operations (backup, restore, verify, compress) feed into this same
component. This ensures consistent progress feedback across all operations and
means verify gets a progress indicator for free.

---

## Screen Flow

```
Welcome / Path Config
        │
        ▼
   Main Menu ◄──────────────────────────┐
   ├── Scan & Compress                  │
   ├── Backup Manager                   │
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

All archive operations live in one screen — the user selects an archive once
and all operations on it are available in place. No separate Restore screen.

```
┌─────────────────────────────────────────────────────┐
│  Backup Manager                                     │
│                                                     │
│  Archive: ~/gamma/backup/gamma_backup.7z            │
│  Size: 50GB  •  Created: 2 days ago                 │
│                                                     │
│  > Create New Backup                                │
│    Restore Single Mod                               │
│    Restore All                                      │
│    Verify Archive                                   │
│    Delete Backup                                    │
└─────────────────────────────────────────────────────┘
```

- **Restore Single Mod** — launches the shared `ModPicker` component to select
  a mod, then runs restore via the shared `OperationScreen` component
- **Restore All** — confirmation dialog, then full restore via `OperationScreen`
- The separate `internal/tui/screens/restore.go` is removed — all restore
  functionality lives in `backup.go`. The `ModPicker` component is reused.
- Main menu has three items: Scan & Compress, Backup Manager, Settings

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
- Supports Ctrl+C cancellation — kills 7zz subprocess, deletes partial archive, returns to main menu

**First-run behavior:** if no backup exists and the user navigates to Scan & Compress,
show a warning screen recommending backup first. Do not block — let them proceed if they
explicitly choose to.

### 2. Restore

Two restore modes accessible from the Restore screen:

```
Restore:
  > Restore Single Mod   ← fuzzy mod picker, restores one mod
    Restore All          ← full restore, no filter
```

**Restore Single Mod:**
- Run `7zz l <archive>` and parse the file listing into a mod name list
- Display as a searchable bubbles/list (fuzzy filter on mod name)
- Confirm dialog showing: mod name, backup date, size on disk
- Restore via:
  ```
  7zz x <archive> -o<parent_of_mods_dir> "mods/<ModName>/*" -r -y
  ```

**Restore All:**
- Confirmation dialog with clear warning: "This will overwrite all mod files
  with backup versions. Continue?"
- Restore via:
  ```
  7zz x <archive> -o<parent_of_mods_dir> -r -y
  ```
- No path filter — extracts everything from the archive

Both modes:
- Stream progress back to UI via shared OperationScreen component
- Support Ctrl+C cancellation — kills 7zz subprocess, returns to main menu

### 3. Scan

- Walk the MO2 mods directory recursively
- For each `.dds` file: read the first 148 bytes, parse the DDS header (including DX10 extended header)
- Skip files where `DDSInfo.Compressed == true` — never re-compress already compressed textures
- Emit `assetFoundMsg` per file (async Cmd) so UI stays live during scan
- Group results by profile for display
- Supports Ctrl+C cancellation — cancels the walk goroutine via context, returns to main menu with message "Scan cancelled"

#### Classification — Two-Pass System

Classification uses two passes. Header data is primary; filename patterns are override.

**Pass 1 — Header-based default (always runs first):**
```
HasAlpha == true  → SuggestedFmt: BC7_UNORM,  ProfileMatch: "Auto (alpha)"
HasAlpha == false → SuggestedFmt: BC1_UNORM,  ProfileMatch: "Auto (no alpha)"
```
Every uncompressed file gets a safe default format from its actual pixel data.
BC7 for alpha textures (high quality, preserves transparency), BC1 for opaque
(smallest footprint).

**Pass 2 — Filename pattern override (runs after, overwrites if matched):**
```
*_bump.*, *_normal.* → BC5_UNORM  "Normal Maps"    (overrides header — normal maps
                                                     may have alpha for gloss data
                                                     but still need BC5)
*/textures/ui/*      → BC3_UNORM  "UI / Icons"     (engine expects BC3 for UI)
*_diff.*, *_base.*   → BC7_UNORM  "Diffuse / Color" (confirms/upgrades header)
```
Filename match always wins over header default. Profiles are fully user-defined
in `profiles.json` — the binary applies whatever profiles are loaded.

**Result buckets in scan results:**
- One bucket per named profile (from profiles.json) — shown regardless of count,
  even zero-hit profiles must render (allows users to verify pattern coverage)
- `Auto (alpha)` — header-classified files with alpha channel, suggested BC7
- `Auto (no alpha)` — header-classified files without alpha, suggested BC1
- `Unknown format` — files with unrecognized FourCC or DXGI format codes that
  cannot be safely classified — skipped from compression by default

**Counters on scan results screen:**
- `___ to compress` — total uncompressed files with a known suggested format
- `___ skipped (compressed)` — files found but skipped because `DDSInfo.Compressed == true`
  This should be ~24,000 for a full GAMMA install. The count must be tracked in
  `scan.Walk` and passed through to the results screen — not calculated from
  the asset list after the fact (already-compressed files are never emitted
  to the assets channel, so they must be counted inside the walker).
- Remove "already done" counter — implies state tracking that doesn't exist
- Remove "unmatched" counter — always 0, misleading

**Unknown format handling:**
Files where the FourCC or DXGI format code is not recognized are marked
`Asset.Unknown = true`. They appear in the "Unknown format" bucket and are
excluded from all compression jobs. Do not attempt to compress unknown formats —
texconv behavior on unrecognized input is undefined and may produce corrupt output.

#### Scanner Exclusions

Directory exclusions are user-configurable via `scanExclusions` in `config.json`.
Value is a list of glob patterns matched against directory **names** (not full paths)
using `filepath.Match`.

Default value shipped in config:
```json
"scanExclusions": [".*", "downloads", "Downloads"]
```

- `.*` — skips all hidden directories (e.g. `.Grok's Modpack Installer`, `.git`)
- `downloads` / `Downloads` — skips the GAMMA downloads folder (both cases for Linux)

Surfaced in the Settings screen as an editable list — users can add or remove patterns.

**Hardcoded exclusions (never user-configurable):**
- Files where `DDSInfo.Compressed == true` — never re-compress already compressed textures
- Files without `.dds` extension — only DDS files are processed

### 4. Compress

#### Compression Config Screen

Format and mip settings are defined entirely in `profiles.json` — they are NOT
overridable in the UI. The compression config screen is a confirmation step only,
not a settings screen.

**What the config screen shows:**
- Summary of files to be compressed (count per profile bucket)
- Worker count (read from config, display only — edit in Settings)
- In-place vs staging directory (read from config, display only — edit in Settings)
- Three run scope options:

```
Run Scope:
  > Run All               ← compress all 2222 files across all profiles
    Run Selected Profile  ← pick one profile bucket (e.g. Normal Maps only)
    Run Selected Mod      ← pick one mod, compress its files across all profiles
```

**Run Selected Mod** is the recommended first-time workflow — compress one small
mod, verify it looks correct in game, then run all. Surface this recommendation
in the UI.

The mod picker for "Run Selected Mod" is identical in behavior to the restore
screen's mod list — a searchable, fuzzy-filtered list of mod names. Extract this
into a shared component at `internal/tui/components/modpicker.go` so both screens
use the same implementation. The mod list is populated from `asset.ModName` values
in the current scan results. On selection, assets are filtered to the chosen mod
before being passed to the compression worker pool:

```go
filtered := []scan.Asset{}
for _, a := range allAssets {
    if a.ModName == selectedMod {
        filtered = append(filtered, a)
    }
}
// pass filtered to worker pool
```

**What the config screen does NOT have:**
- Per-profile format picker (format comes from profiles.json)
- Per-profile mip toggle (generateMips comes from profiles.json)
- In-place toggle (global setting, lives in Settings screen only)

#### Compression Execution

- Worker pool: `max(1, runtime.NumCPU()/2)` concurrent texconv processes
- Per-file texconv invocation:
  ```
  texconv -f <FORMAT> -m 0 -y -o <output_dir> -- <input_file>
  ```
  Note: `--` separator is required before input path — paths starting with `/`
  are interpreted as flags without it.
  `-m 0` generates full mip chain if `generateMips == true` in profile,
  or `-m 1` for no mips if `generateMips == false`
- Capture stdout/stderr per file into `CompressionResult`
- Emit `compressionDoneMsg` per file to update progress bar
- On completion: show summary with success count, error count, estimated VRAM delta
- Error list is navigable; failed files can be retried
- No retry with different settings — if a file failed, fix profiles.json and rescan

#### Cancellation

All long-running operations (backup, restore, compress) must support Ctrl+C cancellation:

- A `context.WithCancel` context is created at operation start and stored in the
  top-level model
- The cancel function is called when Ctrl+C is pressed during an active operation
- Workers receive the context and check `ctx.Done()` between files
- The running subprocess is killed via `cmd.Process.Kill()` on cancellation
- Partial output files are deleted on cancel
- After cancellation, the app returns to the main menu with message: "Operation cancelled"
- Ctrl+C on the main menu or any non-operational screen exits the app normally
- Implemented purely through Bubble Tea key messages — do NOT use `os/signal`

### 5. Settings

- GAMMA mods directory path — auto-detect from common locations:
  - Linux: `~/Games/GAMMA/mods`, `~/GAMMA/mods`, `$MO2_GAME_PATH`
  - Windows: `C:\Games\GAMMA\mods`, `D:\GAMMA\mods`, `%MO2_GAME_PATH%`
  - All path handling via `filepath.Join` — no hardcoded separators anywhere
- Backup archive path
- Backup compression level: integer 1-9, default 6
  - Displayed in Settings as a text input with inline guide:
    ```
    Backup Compression Level (1-9): [6]

    1-3  Fast compression, larger archives
    4-6  Balanced — recommended for most systems
    7-9  Maximum compression, significantly slower
    ```
  - Validated on input — reject values outside 1-9, non-numeric input reverts to previous value
  - All other 7z flags (`-mfb=64 -md=32m -ms=on -xr!downloads -xr!Downloads`) are hardcoded, not user-exposed
- Worker thread count for texture compression (default: `max(1, runtime.NumCPU()/2)`)
- Whether to compress textures in-place or to a staging directory
  (this is the only place this toggle exists — not in the compression config screen)
- Scan exclusions — editable list of glob patterns, default: `[".*", "downloads", "Downloads"]`
- Persist to `os.UserConfigDir()/stalker-tex/config.json`

Full config.json schema:
```json
{
  "modsDir": "/home/user/GAMMA/mods",
  "backupDir": "/home/user/GAMMA/backup",
  "workerCount": 4,
  "compressInPlace": true,
  "backupLevel": 6,
  "scanExclusions": [".*", "downloads", "Downloads"]
}
```

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

## About / Licenses Screen

Accessible from the main menu. Displays:

```
┌─────────────────────────────────────────────────────┐
│  stalker-tex v<version>                             │
│                                                     │
│  A texture compression and backup utility for       │
│  STALKER GAMMA modlists.                            │
│                                                     │
│  github.com/noisethanks/stalker-tex                 │
│                                                     │
│  ── Third-Party Licenses ──────────────────────     │
│                                                     │
│  <scrollable content of THIRD_PARTY_LICENSES.txt>  │
│                                                     │
│  ↑↓ scroll   q/esc back                            │
└─────────────────────────────────────────────────────┘
```

Implementation notes:
- Version string injected at build time via `-ldflags "-X main.version=v0.1.0"`
- License content is hardcoded in `about.go` — no separate file embedding needed
- All licenses displayed in one scrollable section in this order:
  1. texconv (Texconv-Custom-DLL) — MIT + contents of THIRD_PARTY_LICENSES.txt
  2. 7-Zip — LGPL v2.1
  3. Charmbracelet UI dependencies (bubbletea, bubbles, lipgloss) — MIT
- This satisfies matyalatte's redistribution requirement — license notice is
  present in the distributed binary's about screen
- License text is scrollable via `↑↓` / `j k`
- `q` or `esc` returns to main menu
- Implemented as `internal/tui/screens/about.go`

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

## Future / Post-1.0

- **Atomic compression** — compress to staging directory, verify all files
  succeeded, then diff-apply in one pass. Failed jobs leave the mod directory
  untouched. Planned for v1.1.
- **Scan metadata persistence** — store scan results and compression history
  to disk. Enables: "already done" tracking, restore by profile, incremental
  rescans. Requires a simple local database or JSON state file.
- **Restore by profile** — restore only mods containing textures that match
  a given profile. Dependent on scan metadata persistence.
- **stalker-update** — separate binary, same visual identity, handles GAMMA
  mod updates selectively. Dependent on community reception of stalker-tex.

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
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build   -ldflags="-s -w -X main.version=v0.1.0"   -o stalker-tex-linux ./main.go

# Release — Windows x86-64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build   -ldflags="-s -w -X main.version=v0.1.0"   -o stalker-tex-windows.exe ./main.go

# goreleaser handles both targets in CI — version injected from git tag
```

Version is injected at build time via `-X main.version=<tag>`. In development
builds without the flag, version displays as `dev`.

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
