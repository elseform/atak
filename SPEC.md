# atak — Project Specification

## What This Is

A single compiled Go binary that wraps 7-zip and texconv with a Bubble Tea TUI.
It helps S.T.A.L.K.E.R. Anomaly players back up their mod directory and compress textures
to reduce VRAM usage. The target user is non-technical — someone who followed a
YouTube guide to install an Anomaly-based modpack and wants better performance without breaking anything.

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
├── texconv-macos       # matyalatte macOS universal binary (Intel + Apple Silicon)
├── 7zz                 # 7-Zip standalone Linux binary
├── 7zz.exe             # 7-Zip standalone Windows binary
└── 7zz-macos           # 7-Zip standalone macOS universal binary (Intel + Apple Silicon)
```

macOS universal binaries contain both x86-64 and ARM64 slices — one binary covers
all Mac hardware. No need to split darwin/amd64 and darwin/arm64 build tags.

Build tag pattern — same variable name on all platforms, different file:

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

```go
//go:build darwin

//go:embed bin/texconv-macos
var texconvBin []byte

//go:embed bin/7zz-macos
var sevenZipBin []byte
```

On startup:
1. Extract both binaries to `os.MkdirTemp`
2. `chmod 0755` both (no-op on Windows, harmless)
3. Store paths in an `EmbeddedTools` struct passed through the app
4. `defer tools.Cleanup()` in main

No other runtime dependencies. The binary must run on any supported platform
without the user installing anything.

---

## Configuration

Config directory is platform-aware via `os.UserConfigDir()` — no hardcoded paths:

```
Linux:   ~/.config/atak/
Windows: %AppData%\atak\
macOS:   ~/Library/Application Support/atak/
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
copies the embedded default to `~/.config/atak/profiles.json` and shows a
one-time notice screen before proceeding to the main menu:

```
┌─────────────────────────────────────────────────────┐
│  Compression profiles created                       │
│                                                     │
│  A default profiles.json has been created at:       │
│  ~/.config/atak/profiles.json                │
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

## profiles.json Format Reference

```json
{
  "minFileSizeBytes": 1024,
  "excludePatterns": ["fx_sun*", "fx_*"],
  "profiles": [
    {
      "name": "Profile Name",
      "format": "BC3_UNORM",
      "generateMips": true,
      "patterns": ["*_bump.*", "*/textures/sky/*"],
      "exclude": ["*_bump_detail.*"]
    }
  ]
}
```

### Fields

**Top level:**
- `minFileSizeBytes` — integer, default 1024. Files smaller than this value are
  skipped silently. Protects against compressing stub/placeholder textures which
  produce garbage output (e.g. leopard print artifacts).
- `excludePatterns` — array of glob patterns. Matching rules:
  - Patterns **without** `/` match against the filename (basename) only
  - Patterns **containing** `/` match against the full relative path from the
    mod root, and cover the whole subtree — `*/textures/ui/SquareDOV/*` excludes
    every file under that directory at any depth, regardless of which mod
    provides them. See [Pattern matching rules](#pattern-matching-rules)
  - Matching is case-insensitive on all platforms
  - Evaluated **before** profile matching, so an entry here always beats a
    profile pattern. A pattern listed both here and in a profile makes that
    profile's copy unreachable
  - Files matching any pattern are never compressed regardless of profile match,
    and are reported as `Excluded`

**Per profile:**
- `name` — display name shown in scan results UI
- `format` — BCn compression format. Valid values:
  - `BC1_UNORM` — opaque textures, no alpha. Smallest file size (0.5 bytes/texel).
    Best for: opaque diffuse, environment textures without transparency
  - `BC3_UNORM` — color + alpha (DXT5). Good quality, wide engine support (1 byte/texel).
    Best for: UI, textures with alpha, general purpose safe default
  - `BC4_UNORM` — single channel grayscale. Good for masks, AO maps (0.5 bytes/texel)
  - `BC5_UNORM` — two channel XY normal map data. Required for bump/normal maps (1 byte/texel).
    Do not use BC3 for normal maps — it will produce incorrect lighting
  - `BC7_UNORM` — high quality color + alpha. Best visual quality (1 byte/texel).
    GPU-accelerated on Windows, CPU-only on Linux (~40-60 min for large jobs).
    Best for: high quality diffuse, detailed character/weapon textures
- `generateMips` — mip-chain **policy** for this profile, not an unconditional switch.
  The effective per-file decision is resolved from each source DDS header by
  `ShouldGenerateMips` in `internal/compress/texconv.go`. With the default settings it
  is `profile.generateMips || sourceMipCount > 1`; the `stripMipsWhenDisabled` setting
  (below) changes what `generateMips:false` means:
  - `true` — **always** generate a full chain. Use for world textures (diffuse,
    normal, weapon, terrain, sky) — they are minified with distance and need mips
    even if a careless source shipped without them. Unaffected by `stripMipsWhenDisabled`.
  - `false` — **preserve the source's own choice** (default): keep a full chain when
    the source already had one, generate none when it did not. This is *not* a blanket
    "strip mips" — a flare or scope reticle that ships with mips keeps them; flat UI art
    without mips stays single-level. When the global `stripMipsWhenDisabled` setting is
    on, `false` instead becomes an authoritative "strip": even a mipped source is
    flattened to a single level (smaller output, at the cost of source fidelity).

  This is why one profile can cover a directory whose sources disagree. In a real
  GAMMA install ~1/3 of `anamflares` ship a mip chain (the moon flare has 11 levels
  and renders as a distant billboard that *needs* them) while the rest ship one;
  86% of scope reticles ship mips, 3% of UI icons do. No single static boolean is
  correct for such a folder — the source decides. Stripping a mipped flare or
  reticle to a single level makes it alias and read as a visibly wrong shade when
  minified in-game, since the GPU can no longer pre-average the bright core.
- `patterns` — array of glob patterns matched against filename OR full relative
  path. Path patterns must contain `/`. Order matters — first match wins.
  Examples:
  - `*_bump.*` — matches any file with `_bump` before the extension
  - `*/textures/sky/*` — matches any file under a `textures/sky/` directory
  - `*_d.*` — matches files ending in `_d` before the extension
- `maxTextureSize` — optional integer, default 0 (no limit). When set, caps
  the output texture's maximum dimension while preserving aspect ratio.
  Only applied when at least one dimension exceeds the limit — textures
  smaller than maxTextureSize are never upscaled.

  Implementation notes:
  - texconv's `-w -h` set exact dimensions — passing both forces a square
    output, distorting non-square textures. Pass only `-w` — texconv scales
    height proportionally to maintain aspect ratio.
  - Guard against upscaling: only add `-w` when at least one dimension exceeds
    maxTextureSize (`||` not `&&`)

  ```go
  // correct implementation
  if maxTextureSize > 0 && (asset.Width > maxTextureSize || asset.Height > maxTextureSize) {
      args = append(args, "-w", strconv.Itoa(maxTextureSize))
      // do NOT pass -h — texconv maintains aspect ratio from -w alone
  }
  ```

  Recommended use: set on Sky, Terrain, Detail profiles for 4GB VRAM cards.
  Do NOT use on Weapon or Character textures — quality loss is visible up close.
  Do NOT use on cubemap/LOD textures (`*#small*`, `*cube#*`) — texconv handles
  these incorrectly with resize flags, producing files 30x larger than the input.
  ```json
  "maxTextureSize": 1024
  ```
- `exclude` — optional array of glob patterns. A file matching this profile's
  `patterns` **and** its `exclude` is **declined by this profile**, and matching
  continues with the profiles after it. The file is not dropped.

  This is the mechanism that routes exceptions to a better-suited profile:
  Normal Maps declines `*scope*bump*` and `*lens_bump*`, so scope lens bumps fall
  through to Scope Textures (BC7) rather than being flattened to two-channel BC5.
  It only works because Normal Maps appears *before* Scope Textures — a decline
  with no later match leaves the file `Unmatched`, which is surfaced in scan
  results rather than silently discarded.

  To drop a file outright, use the top-level `excludePatterns`. Those run before
  profile matching and report the file as `Excluded`, so it stays visible in the
  scan; a profile decline is silent by comparison.

### Pattern matching rules
- `*` matches any sequence of characters except `/`
- `?` matches a single character except `/`
- Patterns without `/` are matched against the filename only (basename)
- Patterns containing `/` are matched against the full relative path from the mod root
- **A trailing `/*` is recursive** — it matches everything below that directory at
  any depth, not just direct children. A bare trailing `/` is shorthand for the
  same thing, so `*/textures/ui/` and `*/textures/ui/*` are equivalent
- Patterns are anchored at both ends — a full match, not a substring search
- Matching is case-insensitive on all platforms
- More specific patterns must appear before general ones (first match wins)

The recursive trailing `/*` (equivalently, a bare trailing `/`) is the one
deliberate departure from plain `filepath.Match`, and it is what makes a
path-based exclude usable: `*/textures/ui/SquareDOV/*` has to cover every file
under that directory at any depth, regardless of which mod ships it. Everywhere
else a star stays inside one path segment, so `*/textures/wpn/scope_*` matches
`scope_30mm.dds` but not `scope_reticles/lens.dds`.

All three pattern lists — a profile's `patterns`, a profile's `exclude`, and the
top-level `excludePatterns` — share one implementation (`matchesPattern` in
`internal/scan/walker.go`) so inclusion and exclusion cannot diverge. Previously
inclusion matched by substring while exclusion used `filepath.Match`, which is
how path excludes came to silently miss subtrees.

Patterns are matched against the **mod-root-relative** path, so both walkers
must normalize before matching: `WalkVirtual`'s `relPath` already has that shape,
while `Walk`'s path is relative to `modsDir` and carries a leading mod-name
segment that `modRelPath` strips. Skipping that step leaves an extra segment
that no path pattern can match, silently routing everything to `Unmatched`.

A backslash in a pattern always means a separator and is normalized to `/`,
since patterns are authored rather than observed.

### Compression format quick reference

| Format | Quality | Size | Alpha | GPU accel Linux | Use for |
|--------|---------|------|-------|-----------------|---------|
| BC1 | Good | 0.5 bpt | No | Yes | Opaque diffuse |
| BC3 | Good | 1 bpt | Yes | Yes | UI, general alpha |
| BC4 | Good | 0.5 bpt | No | Yes | Grayscale/masks |
| BC5 | Excellent | 1 bpt | No | Yes | Normal maps only |
| BC7 | Excellent | 1 bpt | Yes | No (CPU only) | High quality diffuse |

bpt = bytes per texel

---

**Profile ordering matters** — profiles are matched in order, first match wins.
More specific path patterns must come before more general ones:
- `*/textures/ui/readables/*` must appear before `*/textures/ui/*`
- `*/textures/sky/night/*` must appear before `*/textures/sky/*`
- Filename suffix patterns (`*_bump.*`) are order-independent since they don't overlap

**The `_bump` suffix is not a reliable indicator of BC5 compatibility.**
Some mods use `_bump` naming for textures that carry more than XY normal data —
scope lens reflection textures in particular often use all 4 RGBA channels for
reflection intensity, gloss, and specular data. Compressing these with BC5 (which
discards B and A channels) causes visual artifacts — loss of reflections, banding
on metallic surfaces.

Mitigation:
- Add `"exclude": ["*scope*bump*", "*lens_bump*"]` to the Normal Maps profile
- Add a dedicated Scope Textures profile using BC7_UNORM before Weapon Textures
- BC7 correctly handles all 4 channels and is appropriate for complex
  metallic/reflective surfaces regardless of naming convention

The two steps are one mechanism, not two independent ones: the `exclude` makes
Normal Maps *decline* those files so they keep matching downward, and the Scope
Textures profile is what catches them. Ordering is load-bearing — Scope Textures
must come after Normal Maps to receive the declines, and the patterns must not
also appear in the top-level `excludePatterns`, which would drop the files before
any profile is consulted.

When in doubt about a texture's channel usage, check with dds_analyze —
`UNCOMPRESSED_RGBA` with a `_bump` suffix means BC5 is wrong for that texture.

**The embedded default** (`configs/compression_profiles.json`) is the seed — it ships
with broadly correct STALKER conventions but users are expected to tune it:

```json
{
    "_note": "Profile order matters — first match wins. Specific path patterns (ui/readables, flares) must precede general ones (ui/*, fx_*).",
    "minFileSizeBytes": 1024,
    "excludePatterns": [
        "fx_sun*",
        "fx_*",
        "*_lm.*",
        "*_cm.*",
        "*_nm2.*",
        "*detail_map*",
        "*_hm.*",
        "lut_*",
        "*#small*",
        "*cube#*",
        "*_cube#*",
        "*/textures/ui/SquareDOV/*"
    ],
    "profiles": [
        {
            "name": "UI Readables",
            "format": "BC3_UNORM",
            "generateMips": false,
            "patterns": [
                "*/textures/ui/readables/*",
                "*/textures/ui/npe/*"
            ]
        },
        {
            "name": "Flares / FX",
            "format": "BC3_UNORM",
            "generateMips": false,
            "patterns": [
                "*/textures/anamflares/*",
                "*/textures/flares/*"
            ]
        },
        {
            "name": "Particle / FX",
            "format": "BC3_UNORM",
            "generateMips": false,
            "patterns": [
                "*/textures/semitone/*"
            ]
        },
        {
            "name": "Normal Maps",
            "format": "BC5_UNORM",
            "generateMips": true,
            "exclude": [
                "*scope*bump*",
                "*lens_bump*"
            ],
            "patterns": [
                "*_bump.*",
                "*_bump#.*",
                "*_normal.*",
                "*_nm.*",
                "*_nmap.*",
                "*_nrm.*",
                "*_norm.*",
                "*_norm_*",
                "*nbump*",
                "*_normalbump.*",
                "*_nm_*"
            ]
        },
        {
            "name": "Custom UI",
            "format": "BC3_UNORM",
            "generateMips": false,
            "patterns": [
                "*/textures/catsy/*"
            ]
        },
        {
            "name": "UI / Icons",
            "format": "BC3_UNORM",
            "generateMips": false,
            "patterns": [
                "*/textures/ui/*",
                "*_icons.*"
            ]
        },
        {
            "name": "Diffuse / Color",
            "format": "BC3_UNORM",
            "generateMips": true,
            "exclude": [
                "*scope*diff*"
            ],
            "patterns": [
                "*_d.*",
                "*_diff.*",
                "*_diffuse.*",
                "*_albedo.*",
                "*_base.*",
                "*_col.*",
                "*_color.*",
                "*_co.*",
                "*_c.*",
                "*_b.*",
                "*_rgb.*",
                "*_details.*"
            ]
        },
        {
            "name": "Specular / Gloss",
            "format": "BC3_UNORM",
            "generateMips": true,
            "patterns": [
                "*_spec.*",
                "*_gloss.*"
            ]
        },
        {
            "name": "Masks / Alpha",
            "format": "BC3_UNORM",
            "generateMips": true,
            "patterns": [
                "*_mask.*",
                "*_alpha.*"
            ]
        },
        {
            "name": "Sky Textures",
            "format": "BC3_UNORM",
            "generateMips": true,
            "patterns": [
                "*/textures/sky/*"
            ]
        },
        {
            "name": "Detail / Terrain",
            "format": "BC3_UNORM",
            "generateMips": true,
            "patterns": [
                "*/textures/detail/*",
                "*/textures/terrain/*"
            ]
        },
        {
            "name": "Sights / Reticles",
            "format": "BC3_UNORM",
            "generateMips": false,
            "patterns": [
                "*/textures/wpn/scope_reticles/*",
                "*/textures/bonus_sights/*",
                "*crosshair*",
                "*reticle*",
                "*_reticle.*",
                "*_crosshair.*"
            ]
        },
        {
            "name": "Scope Textures",
            "format": "BC7_UNORM",
            "generateMips": true,
            "patterns": [
                "*/textures/wpn/scope_*",
                "*scope*diff*",
                "*scope*bump*",
                "*lens_bump*"
            ]
        },
        {
            "name": "Weapon Textures",
            "format": "BC3_UNORM",
            "generateMips": true,
            "patterns": [
                "*/textures/wpn/*",
                "*/textures/rwap/*"
            ]
        },
        {
            "name": "Character / Hands",
            "format": "BC3_UNORM",
            "generateMips": true,
            "patterns": [
                "*/textures/act/*",
                "*/textures/MK/*"
            ]
        },
        {
            "name": "Items",
            "format": "BC3_UNORM",
            "generateMips": true,
            "patterns": [
                "*/textures/items/*",
                "*/textures/item/*",
                "*/textures/usable_items/*",
                "*/textures/farcry4/*",
                "*/textures/artifact/*",
                "*/textures/gwr/*"
            ]
        }
    ]
}
```

Note on format choices:
- **UI/Icons uses BC3** — BC7 was tried here and reverted. UI textures do use
  smooth alpha gradients that BC7 handles better in principle, but the category
  is large (~350 files in a GAMMA install) and BC7 is CPU-only on Linux, so it
  dominated compression time for a difference that is not visible on flat icon
  art with hard alpha edges. Users who want it can set BC7 in their own
  `profiles.json`; there is no runtime cost either way, since BCn decompresses in
  hardware at the same speed regardless of format.
- **Scope Textures use BC7** — scope bump textures often use all 4 RGBA channels
  for reflection/gloss data, not just XY normals. BC5 would destroy B and A.
  This is the only BC7 profile in the defaults, and it stays affordable on Linux
  because it covers a small number of files.
- **Weapon Textures and Character/Hands use BC3** in `default.json` for Linux
  CPU performance. The `quality.json` profile upgrades these to BC7.

Notes:
- All profiles default to BC3_UNORM except Normal Maps (BC5, required for
  two-channel normal data) and Scope Textures (BC7, required for 4-channel lens
  data) — BC3 is safe, fast, and well-supported across all XRay engine versions
- BC7_UNORM produces better quality for diffuse textures but is CPU-intensive on Linux
  (no GPU acceleration) and caused issues in testing — power users can change
  Diffuse/Color to BC7_UNORM in their profiles.json
- The BC7 → BC3 automatic fallback in texconv.go remains as a safety net for any
  profile that uses BC7
- Normal map patterns expanded to match bash script proven conventions

**Unmatched files** — DDS files that don't match any profile pattern are surfaced in
scan results as a separate "Unmatched" bucket. The user can assign them a format
manually in the Compression Config screen before compressing, or skip them entirely.

**Community sharing** — users can share `profiles.json` files tuned for specific mod
packs. The Settings screen shows the path to `profiles.json` and offers an
"Open in editor" option using `$EDITOR` (Linux) or `notepad.exe` (Windows).

---

## Project Structure

```
atak/
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
│   │   ├── walker.go        # walk mod directory or virtual FS, enumerate assets
│   │   └── dds.go           # parse DDS headers, classify format
│   ├── modlist/
│   │   ├── parser.go        # parseModList() — reads MO2 modlist.txt
│   │   └── virtual.go       # buildVirtualFS() — assembles virtual filesystem map
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
│           ├── results.go      # scan results + compression launcher (enter/r/m)
│           ├── compress.go     # execution screen using OperationScreen component
│           └── summary.go      # completion stats, error list
│           # restore.go removed — functionality absorbed into backup.go
│           # compress_config.go removed — replaced by results.go keybindings
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
   Main Menu ◄──────────────────────────────────────┐
   ├── Scan & Compress                               │
   ├── Backup Manager                                │
   ├── Settings                                      │
   └── About                                         │
        │
        ▼
   Scanning... (async, live counter)
        │
        ▼
   Scan Results ────────────────────────────────────►─┐
   [enter] Run Selected Profile                        │
   [r]     Run All                                     │
   [m]     Run Selected Mod → ModPicker → Compress     │
   [q]     Main Menu                                   │
        │                                              │
        ▼                                              │
   Compressing... (OperationScreen)                    │
        │                                              │
        ▼                                              │
   Summary ─────────────────────────────────────────►─┘
        │
        └──────────────────────────────► Main Menu
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

All archive operations live in one screen. No separate Restore screen.

```
┌─────────────────────────────────────────────────────┐
│  Backup Manager                                     │
│                                                     │
│  Backups in ~/gamma/backup/:                        │
│                                                     │
│  gamma_backup.7z        50.2 GB   Jun 08 14:23      │
│  gamma_backup_old.7z    48.7 GB   May 15 09:41      │
│                                                     │
│  > Create New Backup                                │
│    Restore Single Mod                               │
│    Restore All                                      │
│    Verify Archive                                   │
│    Delete Backup                                    │
└─────────────────────────────────────────────────────┘
```

**Backup list:**
- Scan backup directory with `os.ReadDir` on screen init, filter for `*.7z` files
- Stat each file for size and modification time — no 7z invocation needed
- Display filename, human-readable size, and date above the action menu
- If no backups exist show "No backups found" in that section
- Refresh list after Create New Backup or Delete completes

**Archive selection:**
- When user selects any option except Create New Backup:
  - If only one archive exists — auto-select it, proceed directly
  - If multiple archives exist — show a picker to select which to operate on

**Actions:**
- **Create New Backup** — runs compression via shared `OperationScreen`, no
  archive selection needed
- **Restore Single Mod** — archive picker (if needed) → `ModPicker` component
  → confirm → restore via `OperationScreen`
- **Restore All** — archive picker (if needed) → confirmation dialog with
  warning → full restore via `OperationScreen`
- **Verify Archive** — archive picker (if needed) → verify via `OperationScreen`
- **Delete Backup** — archive picker (if needed) → confirmation → `os.Remove`

- The separate `internal/tui/screens/restore.go` is removed — all restore
  functionality lives in `backup.go`. The `ModPicker` component is reused.
- Main menu has four items: Scan & Compress, Backup Manager, Settings, About

- List existing backups in the Anomaly mods directory archive with size and date
- Create a new LZMA solid archive of the full Anomaly mods directory via:
  ```
  7zz a -t7z -m0=lzma2 -mx=6 -mfb=64 -md=32m -ms=on -mmt=<backupThreads> -bsp1 <output.7z> <mods_dir> -xr!downloads -xr!Downloads
  ```
  - `-mx=6` — balanced compression, reasonable RAM usage
  - `-mfb=64` — 64 fast bytes, well suited for binary/texture data
  - `-md=32m` — 32MB dictionary, keeps RAM usage sane on large mod lists
  - `-ms=on` — auto solid block sizing, let 7z decide
  - `-mmt=<n>` — thread count from `cfg.BackupThreads`, default `max(2, NumCPU/2)`
  - `-xr!downloads`, `-xr!Downloads` — always exclude downloads folder, both cases for Linux case-sensitivity
  - Compression level (`-mx`) and thread count (`-mmt`) are user-exposed (see Settings)
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

#### Classification Philosophy

**If it's not explicitly in a profile, don't compress it.**

The tool never blindly compresses unrecognized textures. Texture formats in Anomaly
mods are highly inconsistent across mod authors — engine-specific textures, unusual
formats, and edge cases are common. Auto-compressing unknown textures risks game
crashes and visual corruption.

**Classification is pattern-match only:**
- Files matched by a profile pattern → queued for compression with that profile's format
- Files matched by global `excludePatterns` → always skipped, counted as "Excluded"
- Files not matched by any profile → shown as "Unmatched" (informational only, never compressed)

**No Auto buckets.** The previous Auto (alpha) / Auto (no alpha) header-based
fallback has been removed — it caused engine crashes by compressing engine-specific
textures to unsupported formats.

**Result buckets in scan results:**
- One bucket per named profile (from profiles.json) — compressible, selectable
- `Unmatched` — files with no profile match, shown with count but greyed out and
  not selectable for compression. Label: "Add patterns to profiles.json to compress these."
- `Excluded` — files matching global excludePatterns, shown for transparency
- All buckets shown regardless of count (zero-hit profiles still render)

#### Profile-Level Exclusions

Profiles support an optional `exclude` array — patterns that match the profile's
`patterns` but which this profile should **decline**. A declined file is not
dropped; matching continues with the profiles below it, and only a file that
reaches the end with no match becomes `Unmatched`.

```json
{
  "name": "Normal Maps",
  "format": "BC5_UNORM",
  "generateMips": true,
  "patterns": ["*_bump.*", "*_normal.*"],
  "exclude": ["*scope*bump*", "*lens_bump*"]
}
```

Here Normal Maps claims bump maps generally but hands scope and lens bumps to
whichever later profile wants them — Scope Textures, at BC7. Declining is
therefore a routing decision, not a skip. Use the top-level `excludePatterns`
when the intent is genuinely "never compress this file".

#### Global Exclusion Patterns

`profiles.json` supports a top-level `excludePatterns` array. Patterns without
`/` match against the filename (basename). Patterns containing `/` match against
the full relative path from the mod root — useful for excluding specific texture
directories regardless of which mod provides them:

```json
{
  "excludePatterns": [
    "fx_sun*",
    "*_lm.*",
    "*/textures/ui/SquareDOV/*"
  ],
  "profiles": [...]
}
```

Evaluated before any profile matching — against the basename for patterns without
`/`, against the full mod-root-relative path for patterns containing one. If a
file matches `excludePatterns`, it is skipped and counted as "Excluded".

The embedded default `profiles.json` ships with conservative `excludePatterns`
covering known engine-specific texture naming conventions in Anomaly.

**Counters on scan results screen:**
- `___ to compress` — total files matched by profiles (excluding excluded files)
- `___ skipped (compressed)` — files already compressed, skipped by scanner
- `___ unmatched` — uncompressed files with no profile match (informational)
- `___ excluded` — files matching global excludePatterns

**Unknown format handling:**
Files where the FourCC or DXGI format code is not recognized are treated as
unmatched — shown in the Unmatched bucket, never compressed.


#### Scanner Exclusions

Directory exclusions are user-configurable via `scanExclusions` in `config.json`.
They share the profiles.json pattern syntax: a pattern **without** a separator matches a
directory (or mod) **name** anywhere, and a **path** pattern matches a directory's
mod-root-relative path, so an exclusion can target a nested subtree and not only a
top-level folder. The matched directory and everything beneath it is skipped — in the
directory walk via `filepath.SkipDir`, and in mod-output mode by dropping the whole mod
(name patterns) or every file under the directory (path patterns), so the two scan modes
agree. Name matching is case-insensitive, as elsewhere in the pattern system. A path
pattern names its directory in any of three equivalent forms: `dir`, `dir/`, `dir/*`.

Default value shipped in config:
```json
"scanExclusions": [".*", "downloads", "Downloads", "G.A.M.M.A. UI"]
```

- `.*` — skips all hidden directories (e.g. `.Grok's Modpack Installer`, `.git`)
- `downloads` / `Downloads` — skips the Anomaly/GAMMA downloads folder (both entries are
  now redundant since matching is case-insensitive, but kept for clarity)
- `G.A.M.M.A. UI` — skips the GAMMA UI mod directory. Compressing main menu assets
  causes excessive loading times — confirmed by community testing

Note: SquareDOV minimap textures are excluded via `*/textures/ui/SquareDOV/*` in
`excludePatterns` in `profiles.json` — path-based exclusion covers all mods that
ship those textures regardless of mod name or number prefix. The same directory could
instead be pruned entirely with `*/textures/ui/SquareDOV` in `scanExclusions`; the
difference is that a profile exclude reports the files as Excluded, while a scan
exclusion skips them silently before classification.

Surfaced in the Settings screen as an editable list — users can add or remove patterns.

**Hardcoded exclusions (never user-configurable):**
- Files where `DDSInfo.Compressed == true` — never re-compress already compressed textures
- Files without `.dds` extension — only DDS files are processed

**Configurable exclusions (in profiles.json):**
- `minFileSizeBytes` — top-level field in profiles.json, default 1024. Files smaller
  than this value are skipped. Stub/placeholder textures are typically under 200 bytes;
  the smallest real usable texture (16x16 uncompressed RGBA) is ~1KB. Counted in the
  skipped total, not surfaced as errors. Users can lower this if they have legitimate
  tiny textures, or raise it to skip small textures entirely.
  ```json
  {
    "minFileSizeBytes": 1024,
    "excludePatterns": [...],
    "profiles": [...]
  }
  ```

### 4. Compress

#### Compression — No Config Screen

There is no separate compression config screen. The scan results screen is the
compression launcher. All compression is initiated directly from scan results
via keybindings:

```
Scan Results keybindings:
  [enter]   run selected profile (whichever profile row is highlighted)
  [r]       run all profiles
  [m]       run selected mod — opens ModPicker, then compresses that mod only
  [q]       back to main menu
```

**Run Selected Mod** is the recommended first-time workflow — surface this in
the scan results screen as a hint: "Press [m] to compress a single mod first".

The mod picker for [m] uses the shared `ModPicker` component. On selection,
assets are filtered to the chosen mod before passing to the worker pool:

```go
filtered := []scan.Asset{}
for _, a := range allAssets {
    if a.ModName == selectedMod {
        filtered = append(filtered, a)
    }
}
```

`internal/tui/screens/compress_config.go` is deleted — its functionality is
absorbed into `results.go` keybindings. The `compress.go` execution screen
remains — it is still needed to show the OperationScreen during compression.

#### Compression Execution

- Worker pool: `max(1, runtime.NumCPU()/2)` concurrent texconv processes
- Per-file texconv invocation:
  ```
  texconv -f <FORMAT> -m 0 -if CUBIC -gpu 0 -y -nologo [-w <maxTextureSize>] -o <output_dir> -- <input_file>
  ```
  Note: `--` separator is required before input path — paths starting with `/`
  are interpreted as flags without it.
  `-m 0` full mip chain / `-m 1` top level only. The choice is resolved **per file**,
  not per profile: a profile's `generateMips:true` always yields `-m 0`, while
  `generateMips:false` yields `-m 0` only when the source DDS already had a chain
  (`mipMapCount > 1`) and `-m 1` otherwise. So a mipped source is never flattened and
  a mipless world texture still gets a chain forced by its profile
  (`ShouldGenerateMips`, resolved when jobs are built and threaded parallel to each
  group's paths). The `stripMipsWhenDisabled` setting overrides the `generateMips:false`
  branch to always yield `-m 1` (authoritative strip); `generateMips:true` is unaffected.
  See the profiles.json `generateMips` field for the policy rationale.
  `-if CUBIC` cubic interpolation for mip generation (better quality)
  `-gpu 0` GPU accelerated compression (DirectX GPU on Windows, CPU fallback on Linux)
  `-nologo` suppress Microsoft header output
  `-w <n>` only added when `maxTextureSize > 0` — caps width, height scales
  proportionally. Never applied when input is smaller than the limit.

  Note: `-bc x` (quick BC7 encoder) intentionally removed. The exhaustive BC7
  encoder produces significantly better quality on metallic and reflective surfaces
  (scopes, weapons). On Windows with GPU acceleration there is no meaningful speed
  impact. Linux users running BC7 profiles should expect longer compression times.

- **BC7 → BC3 automatic fallback:** If texconv exits non-zero with BC7_UNORM,
  automatically retry with BC3_UNORM. Matches proven bash script behavior.
  CompressionResult records the actual format used after fallback.



- **Extension case preservation:** texconv lowercases the output extension by
  default — `texture.DDS` becomes `texture.dds`. On Linux (case-sensitive
  filesystem) this creates a second file, leaving the original uncompressed
  `.DDS` file untouched. Fix: after successful texconv run, if the output path
  differs from the original asset path in case only, rename the output to match
  the original filename exactly via `os.Rename`. This ensures the original file
  is always overwritten regardless of extension case.
  ```go
  texconvOut := filepath.Join(outputDir,
      strings.TrimSuffix(filepath.Base(asset.Path), ext) + ".dds")
  if !strings.EqualFold(texconvOut, asset.Path) || texconvOut != asset.Path {
      os.Rename(texconvOut, asset.Path)
  }
  ```
- Capture stdout/stderr per file into `CompressionResult`
- Emit `compressionDoneMsg` per file — adapted to feed shared `OperationScreen`
  component with:
  ```go
  OperationProgressMsg{
      Percent: (doneCount * 100) / totalCount,
      Status:  filepath.Base(asset.Path),  // current file
      Size:    totalBytesSaved,            // accumulated bytes saved
      Done:    allWorkersFinished,
  }
  ```
- Per-file errors accumulate separately and are shown on the summary screen —
  individual file failures do not set `Err` on `OperationProgressMsg`
- Uses shared `internal/tui/components/operation.go` for progress display —
  same spinner/size/elapsed UI as backup and restore
- On completion: transition to summary screen with success count, error count,
  estimated VRAM delta
- Error list is navigable; failed files can be retried
- No retry with different settings — if a file failed, fix profiles.json and rescan

#### Cancellation

All long-running operations (backup, restore, compress) must support Ctrl+C cancellation:

- A `context.WithCancel` context is created at operation start and stored in the
  top-level model
- The cancel function is called when Ctrl+C is pressed during an active operation
- Workers receive the context and check `ctx.Done()` between files
- Subprocess kill on cancellation — two steps required:
  1. Set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` when creating
     the command — puts the subprocess in its own process group
  2. On cancel: `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)` — kills the
     entire process group including any children 7zz or texconv may have spawned
  3. `cmd.Wait()` after kill will return an error — swallow it as expected
- Partial output files are deleted on cancel
- After cancellation, the app returns to the main menu with message: "Operation cancelled"
- Ctrl+C on the main menu or any non-operational screen exits the app normally
- Implemented purely through Bubble Tea key messages — do NOT use `os/signal`

#### Crash / Orphan Process Mitigation

Platform-specific process management is split into build-tag files:
- `internal/tools/process_linux.go` — `//go:build linux`
- `internal/tools/process_windows.go` — `//go:build windows`

Both expose the same interface:
```go
func killProcess(cmd *exec.Cmd)   // kill subprocess on cancel
func setProcAttr(cmd *exec.Cmd)   // set process attributes before Start()
```

**Linux:**
- `setProcAttr` sets `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`
- `killProcess` uses `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)` to kill
  the entire process group
- Crash mitigation via lockfile:
  - On operation start: write `~/.config/atak/atak.lock` with PID
  - On clean end: delete lockfile
  - On startup: check for stale lockfile, kill stale PID, log warning
  - Lockfile lives in `internal/tools/lockfile.go` (Linux build tag only)

**Windows:**
- `setProcAttr` creates a Windows Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`
- After `cmd.Start()`, assign subprocess to the Job Object via `AssignProcessToJobObject`
- When Go process exits (clean or crash), Windows automatically kills all job members
- `killProcess` calls `cmd.Process.Kill()` directly for the cancel case
- `CheckStaleLock()` is a no-op on Windows — Job Objects make lockfile unnecessary
- Uses `golang.org/x/sys/windows` package (~40 lines total)

```go
// process_windows.go outline
func setProcAttr(cmd *exec.Cmd) (windows.Handle, error) {
    job, err := windows.CreateJobObject(nil, nil)
    if err != nil {
        return 0, err
    }
    info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
    info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
    windows.SetInformationJobObject(job,
        windows.JobObjectExtendedLimitInformation,
        uintptr(unsafe.Pointer(&info)),
        uint32(unsafe.Sizeof(info)))
    return job, nil
}

func assignToJob(job windows.Handle, cmd *exec.Cmd) {
    handle, _ := windows.OpenProcess(
        windows.PROCESS_ALL_ACCESS, false,
        uint32(cmd.Process.Pid))
    windows.AssignProcessToJobObject(job, handle)
    windows.CloseHandle(handle)
}

func killProcess(cmd *exec.Cmd) {
    cmd.Process.Kill()
}
```

Job handle is created before `cmd.Start()`, subprocess assigned after. Handle
is stored in the operation context and closed on operation completion — the
`KILL_ON_JOB_CLOSE` flag means closing the handle kills the subprocess if
the Go process exits unexpectedly.

### 5. Settings

- Anomaly mods directory path — auto-detect from common locations:
  - Linux: `~/Games/Anomaly/mods`, `~/Anomaly/mods`, `$MO2_GAME_PATH`
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
- **Compression workers** (`workerCount`) — concurrent texconv processes.
  Default: `max(1, runtime.NumCPU()/4)`. Each worker pegs one CPU core.
  This setting applies to texconv only, not 7-Zip.
  Settings screen label: "Compression workers (texconv)"

- **Backup threads** (`backupThreads`) — 7-Zip internal thread count via `-mmt`.
  Default: `max(2, runtime.NumCPU()/2)`. 7-Zip is I/O-bound and benefits from
  more threads than texconv. Without this setting, 7-Zip uses all available
  threads by default which can cause high CPU during backups.
  Settings screen label: "Backup threads (7-Zip)"
  Passed to 7-Zip as `-mmt=<n>` in all archive operations (backup, restore, verify).
- Compression is always in-place — no staging directory option
  - The backup system is the safety net; restore from backup if needed
  - Removes user confusion and config complexity
- Scan exclusions — editable list of glob patterns, default: `[".*", "downloads", "Downloads"]`
- **Strip mips when disabled** (`stripMipsWhenDisabled`) — bool toggle, default `false`.
  When off, a profile's `generateMips:false` preserves a mipped source's chain (the
  source decides). When on, `generateMips:false` becomes authoritative and strips the
  chain to a single level regardless of source — smaller output at the cost of fidelity
  for flares/reticles. Never affects `generateMips:true`. Settings screen label:
  "Strip Mips When Disabled". Resolved in `compress.ShouldGenerateMips`.
- Persist to `os.UserConfigDir()/atak/config.json`

Full config.json schema:
```json
{
  "modsDir": "/home/user/Anomaly/mods",
  "backupDir": "/home/user/Anomaly/backup",
  "workerCount": 1,
  "backupThreads": 4,
  "backupLevel": 6,
  "scanExclusions": [".*", "downloads", "Downloads", "G.A.M.M.A. UI"],
  "modOutputMode": false,
  "modOutputName": "ATAK",
  "modlistPath": "",
  "stripMipsWhenDisabled": false
}
```

Default compression is in-place. The backup system is the safety net.
When `modOutputMode` is true and `modlistPath` is set, ATAK uses the
virtual filesystem approach — see Mod Output Mode section.

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
    SourceMipCount int   // mip levels in the source DDS (1 = no chain); drives per-file mip policy
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

## Persistent Scan State

Scan results must persist in `AppModel`, not in `ResultsModel`. This prevents
state loss when navigating away from and back to the results screen.

```go
// AppModel holds scan state at the top level
type AppModel struct {
    // ...
    scanAssets  []scan.Asset  // persisted after scan completes
    scanSkipped int           // persisted after scan completes
}
```

When navigating back to results, reconstruct `ResultsModel` from `AppModel.scanAssets`
and `AppModel.scanSkipped` — never lose scan data on screen transition.

`ResultsModel` is a view over the data, not the owner of it.

---

## About / Licenses Screen

Accessible from the main menu. Displays:

```
┌─────────────────────────────────────────────────────┐
│  atak v<version>                             │
│                                                     │
│  A texture compression and backup utility for       │
│  S.T.A.L.K.E.R. Anomaly modlists.                            │
│                                                     │
│  github.com/noisethanks/atak                 │
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

- 32-bit builds (x86-64 and ARM64 only)
- Plugin or extension system
- Network features (no auto-update, no telemetry, no download)
- Support for archive formats other than 7z
- Texture formats other than DDS input / BCn output
- MO2 integration beyond reading the mods directory path

---

## Mod Output Mode (v0.2.0)

An optional non-destructive compression mode that outputs compressed textures
to a single flat mod folder compatible with MO2, rather than compressing in-place.

### Overview

When a MO2 `modlist.txt` is provided, ATAK builds a virtual filesystem representing
the final merged modlist — the same view MO2 presents to the game. Only the winning
file for each texture path is compressed. Output goes to a single flat mod folder
(`ATAK/` by default) that the user adds as the highest-priority mod in MO2.

```
Before:                          After (MO2 load order):
mods/                            mods/
├── 001- Mod A/                  ├── 001- Mod A/          ← originals untouched
│   └── gamedata/tex/ak74.dds   ├── 002- Mod B/          ← originals untouched
├── 002- Mod B/                  └── ATAK/               ← add as highest priority
│   └── gamedata/tex/ak74.dds       └── gamedata/
└── ATAK/  ← new                         └── tex/
    └── gamedata/                             └── ak74.dds ← compressed winner
        └── tex/
            └── ak74.dds  (002- Mod B wins)
```

### Configuration

New fields in `config.json`:

```json
{
  "modOutputMode": false,
  "modOutputName": "ATAK",
  "modlistPath": "/path/to/MO2/profiles/Default/modlist.txt"
}
```

- `modOutputMode` — enable/disable. Default false (in-place mode)
- `modOutputName` — name of the output mod folder. Default "ATAK".
  Created as `<modsDir>/<modOutputName>/`
- `modlistPath` — path to MO2 `modlist.txt`. Optional — if not set,
  Mod Output Mode falls back to in-place behavior

### modlist.txt parsing

MO2's modlist.txt format:
```
+High Priority Mod
+Medium Priority Mod  
-Disabled Mod
+Low Priority Mod
```

- `+` prefix = enabled
- `-` prefix = disabled
- Order = priority (FIRST line = highest priority in MO2)

Parsing:
1. Read file, split on newlines, trim whitespace
2. Filter to lines starting with `+`
3. Strip `+` prefix to get mod names
4. Do NOT reverse — modlist.txt already lists high priority first
5. Return `[]string` of enabled mod names in priority order (high → low)

### Virtual filesystem

```go
func buildVirtualFS(modsDir string, modList []string) map[string]string {
    // map[relPath]absoluteSourcePath
    // iterate high→low priority, lower priority overwrites
    // (so high priority mods win — they are processed last, overwriting lower ones)
    virtual := map[string]string{}
    for i := len(modList)-1; i >= 0; i-- {
        modPath := filepath.Join(modsDir, modList[i])
        filepath.WalkDir(modPath, func(path string, d fs.DirEntry, err error) error {
            if !d.IsDir() {
                rel, _ := filepath.Rel(modPath, path)
                virtual[rel] = path
            }
            return nil
        })
    }
    return virtual
}
```

Result: flat map of `relPath → winning source file`. Passed to the scanner
instead of walking the mods directory directly.

### Output structure

Compressed files are written to `<modsDir>/<modOutputName>/gamedata/...`:

```
mods/ATAK/
└── gamedata/
    └── textures/
        └── wpn/
            └── ak74_d.dds  ← compressed version of whichever mod won
```

Standard flat MO2 mod structure. User adds `modsDir/ATAK/` as a mod in MO2
and places it at the top of the load order.

### Scanner behavior in Mod Output Mode

- The output folder (`ATAK/` or custom name) is automatically added to
  `scanExclusions` — never scanned, never compressed
- Scan operates on the virtual filesystem, not the raw mods directory
- File counts reflect unique files (duplicates across mods deduplicated)
- Already-compressed files in the virtual filesystem are skipped as normal

### Incremental runs

On subsequent runs, ATAK checks if the output file already exists in the
output folder before compressing. If it exists, the file is skipped.

This allows incremental updates — add new mods, rerun, only new files
are compressed. Existing compressed files in the output folder are untouched.

**User communication — incremental skips must be surfaced clearly:**

The scan results and execution screens show a separate counter:
`X already in output folder` — distinct from `skipped (compressed)` which
refers to already-compressed source files.

If ALL files are skipped because the output folder already has everything:
- Summary screen shows: "Nothing to compress — all files already exist
  in output folder. Delete <outputDir> to force recompression."
- This is displayed prominently, not buried

Settings screen shows the output folder path with a hint:
```
Output folder: mods/ATAK/   [Delete to recompress all]
```

**To force full recompression:** delete the output folder and rerun.
ATAK does not provide a built-in "force recompress" button — deleting
the folder is the explicit user action.

### Settings screen

New fields in Settings:
- **Mod Output Mode** toggle (on/off)
- **Output mod name** text field (default "ATAK"), shown when toggle is on
- **MO2 modlist.txt path** text field, shown when toggle is on
  - "Browse" or manual path entry
  - Shows warning if file not found

### New files

```
internal/
└── modlist/
    ├── parser.go      # parseModList() — reads and parses modlist.txt
    └── virtual.go     # buildVirtualFS() — assembles virtual filesystem map
```

### Fallback behavior

If `modlistPath` is empty or the file cannot be read:
- Log a warning
- Fall back to scanning the raw mods directory (current behavior)
- Mod Output Mode still writes to the output folder, but may include
  duplicate files (one per mod that has them)

---

## Community Profiles Directory

A `profiles/` directory in the repo root serves as a community resource for
curated profile configurations. Ships with two official profiles:

```
profiles/
├── default.json   # conservative BC3 defaults — safe for all hardware
└── quality.json   # BC7 for diffuse — better quality, slower on Linux CPU
```

Users drop these into `~/.config/atak/profiles.json` to switch configurations.
Community members can contribute profiles for specific mod packs as PRs —
low barrier to contribution, high value for the ecosystem.

`quality.json` differs from `default.json` in these profiles (BC7 instead of BC3):
- Weapon Textures — players look at these up close constantly
- Character / Hands — high detail, visible at close range
- Diffuse / Color — general quality upgrade for named diffuse textures

Recommended for: Windows users with discrete GPUs (BC7 is GPU-accelerated on Windows).
Not recommended for: Linux users doing large compression jobs (BC7 is CPU-only on Linux).

A future `lowvram.json` profile preset could set `maxTextureSize: 1024` on Sky,
Terrain, and Detail profiles for users with 4GB VRAM cards who need maximum
VRAM reduction beyond what BCn compression alone provides.

---

## Future / Post-1.0

- **Atomic compression** — compress to staging directory, verify all files
  succeeded, then diff-apply in one pass. Failed jobs leave the mod directory
  untouched. Planned for v1.1.
- **Scan metadata persistence** — store scan results and compression history
  to disk. Enables: "already done" tracking, incremental rescans. Requires a
  simple local database or JSON state file.
- **stalker-update** — separate binary, same visual identity, handles Anomaly modpack updates selectively. Dependent on community reception of atak.

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
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build   -ldflags="-s -w -X main.version=v0.1.0"   -o atak-linux ./main.go

# Release — Windows x86-64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build   -ldflags="-s -w -X main.version=v0.1.0"   -o atak-windows.exe ./main.go

# Release — macOS (universal embedded tools, Go binary is amd64)
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build   -ldflags="-s -w -X main.version=v0.1.0"   -o atak-macos ./main.go

# goreleaser handles all targets in CI — version injected from git tag
```

Version is injected at build time via `-X main.version=<tag>`. In development
builds without the flag, version displays as `dev`.

## Build Targets

- `linux/amd64` — primary, tested by maintainer
- `windows/amd64` — supported, community-tested
- `darwin/amd64` — macOS universal binary (Intel + Apple Silicon), community-tested

Note: goreleaser only needs one darwin target since the embedded binaries are
universal. The Go binary itself is architecture-specific but the embedded tools
work on both Intel and Apple Silicon.

The `-s -w` flags strip debug info. Final binaries should be under 25MB including
all embedded tools (texconv + 7zz per platform).

## Cross-Platform Rules

These must be followed in every file or platform support silently breaks:

- **Never** use `/` as a path separator. Always `filepath.Join`.
- **Never** hardcode `~/.config`. Always `os.UserConfigDir()`.
- **Never** assume execute permissions need setting on Windows — `chmod` calls
  must be gated behind a build tag or `runtime.GOOS` check.
- All subprocess invocations via `exec.Command` use the extracted binary path
  from `EmbeddedTools` — never a hardcoded binary name.
- **Extension case:** Never assume `.dds` — always preserve the original file's
  extension case when writing output. Use `os.Rename` to match original case.
- **macOS:** `os.UserConfigDir()` returns `~/Library/Application Support` —
  no special handling needed, already correct via the stdlib.
- **macOS process management:** Same as Linux — `syscall.SysProcAttr{Setpgid: true}`
  and `syscall.Kill(-pid, syscall.SIGKILL)` work on Darwin. `process_linux.go`
  build tag should be changed to `//go:build linux || darwin`.
