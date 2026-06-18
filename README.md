# stalker-tex

A texture compression and backup utility for STALKER GAMMA modlists.

Reduces VRAM usage by compressing uncompressed DDS textures using BCn block
compression. Ships as a single binary with no runtime dependencies.

![screenshot placeholder]

---

## Features

- **Backup & Restore** — create a compressed LZMA archive of your full GAMMA
  mod directory, restore individual mods if something goes wrong
- **Texture Scan** — detects uncompressed DDS textures across your modlist,
  classifies them by type using header data and filename patterns
- **Compression** — compresses textures using BC1/BC3/BC5/BC7 via texconv,
  configurable per texture category via `profiles.json`
- **Single-mod compression** — compress one mod at a time to verify results
  before running on your full modlist

---

## Download

Grab the latest binary for your platform from
[Releases](https://github.com/noisethanks/stalker-tex/releases):

- `stalker-tex-linux` — Linux x86-64
- `stalker-tex-windows.exe` — Windows x86-64

No installation required. Just run it.

**Linux:**
```bash
chmod +x stalker-tex-linux
./stalker-tex-linux
```

**Windows:**
```
Double-click stalker-tex-windows.exe, or run from a terminal
```

---

## Usage

On first launch, stalker-tex will:
1. Ask for your GAMMA mods directory path (auto-detected if possible)
2. Create a default `profiles.json` at `~/.config/stalker-tex/profiles.json`
   (Linux) or `%AppData%\stalker-tex\profiles.json` (Windows)

**Recommended workflow:**
1. **Backup first** — always create a backup before compressing anything
2. **Scan** — let the tool find uncompressed textures in your modlist
3. **Test on one mod** — use "Run Selected Mod" to compress a single mod and
   verify it looks correct in game
4. **Run all** — compress your full modlist once you're confident

---

## Configuration

User config lives at:
- Linux: `~/.config/stalker-tex/`
- Windows: `%AppData%\stalker-tex\`

**`config.json`** — paths, worker count, backup level, scan exclusions  
**`profiles.json`** — compression profiles (texture categories and formats)

Edit `profiles.json` to customize which textures get compressed and with which
BCn format. The file is created with sensible defaults on first run.

Community-tuned `profiles.json` files for specific mod packs can be shared and
dropped in directly.

---

## Building from Source

Requires Go 1.21+.

```bash
git clone https://github.com/noisethanks/stalker-tex
cd stalker-tex
go build -o stalker-tex-linux .
```

Release builds (strip debug info, inject version):
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w -X main.version=v0.1.0" \
  -o stalker-tex-linux .
```

---

## Attributions

- [texconv (Texconv-Custom-DLL)](https://github.com/matyalatte/Texconv-Custom-DLL) — MIT — matyalatte
- [7-Zip](https://www.7-zip.org) — LGPL v2.1 — Igor Pavlov
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — MIT — Charmbracelet
- [Bubbles](https://github.com/charmbracelet/bubbles) — MIT — Charmbracelet
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — MIT — Charmbracelet

Full license text available in-app via the About screen.

---

## Support

If stalker-tex saves your VRAM, consider supporting development:

[Patreon](https://patreon.com/yourname) · [GitHub Sponsors](https://github.com/sponsors/noisethanks)
