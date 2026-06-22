# ATAK — Anomaly Texture Analysis Kit

> Texture compression and backup utility for STALKER GAMMA modlists.

![screenshot placeholder — gameplay screenshot showing performance overlay]

STALKER GAMMA represents a labor of love by hundreds of modders, culminating in a unique and memorable gaming experience. However, the ecosystem ships many texture assets uncompressed. On hardware with limited VRAM, this causes stuttering, hitching, and outright crashes during gameplay. ATAK compresses those textures to BCn block compression formats, dramatically reducing VRAM pressure with minimal visual difference.

---

## Download

Grab the latest binary for your platform from [Releases](https://github.com/noisethanks/atak/releases):

| Platform | File |
|---|---|
| Linux x86-64 | `atak-vX.X.X-linux-x64.tar.gz` |
| Windows x86-64 | `atak-vX.X.X-windows-x64.zip` |

No installation required.

**Linux:**
```bash
tar -xzf atak-vX.X.X-linux-x64.tar.gz
chmod +x atak-linux
./atak-linux
```

**Windows:** Extract the zip, run `atak-windows.exe` in Windows Terminal or PowerShell.

---

## First time? Start here.

**Back up before you compress. Always.**

1. Launch ATAK and go to **Backup Manager**
2. Create a backup of your GAMMA mods directory — this is your safety net
3. Go to **Scan & Compress**
4. Let it scan your modlist
5. Press **[m]** to compress a single mod first — verify it looks right in game
6. If happy, press **[r]** to compress everything
7. If something looks wrong, restore from backup and try again

Compression is always in-place. Your originals are gone after compression — that's what the backup is for.

---

## What it does

### Backup & Restore
- Create compressed LZMA archives of your full GAMMA mods directory
- Restore individual mods or your entire modlist from backup
- Verify archive integrity

### Scan & Compress
ATAK scans your modlist for uncompressed DDS textures and classifies them by type using filename patterns and directory paths. Only textures explicitly matched by a profile are compressed — nothing is touched blindly.

| Profile | Format | Detection |
|---|---|---|
| Normal / bump maps | BC5 | `_bump`, `_normal`, `_nrm`, `_norm` |
| UI / icons | BC3 | `textures/ui/` path |
| Diffuse / color | BC7 (BC3 fallback) | `_diff`, `_base`, `_col`, `_d` |
| Specular / gloss | BC3 | `_spec`, `_gloss` |
| Masks / alpha | BC3 | `_mask`, `_alpha` |

Already-compressed textures are detected and skipped automatically (~24,000 files in a typical GAMMA install).

**Unmatched textures are never touched.** Files that don't match any profile are shown in the scan results but excluded from compression. Add patterns to `profiles.json` to include them.

---

## Configuration

Config lives at:
- **Linux:** `~/.config/atak/`
- **Windows:** `%AppData%\atak\`

**`config.json`** — mods path, backup path, worker count, compression level, scan exclusions

**`profiles.json`** — compression profiles created on first run. Edit freely to customize which textures get compressed and with which format. Community-tuned profiles for specific mod packs can be dropped in directly.

---

## Performance

**Note on BC7 compression:** BC7 GPU acceleration is not available on Linux — compression is CPU-only and takes approximately 40-60 minutes for large jobs. Windows users benefit from DirectX GPU acceleration. For faster Linux compression, change BC7 profiles to BC3 in `profiles.json` — minimal quality difference for most textures.

Benchmarks and feedback welcome — open a GitHub issue or post in the GAMMA Discord.

---

## Known limitations

- BC7 GPU acceleration Linux-only via DirectX — CPU fallback used on Linux
- Cancelling a compression job deletes the in-progress file — originals untouched
- 7-Zip backup progress reporting is sparse on large solid archives — the archive is growing even when the progress bar appears stuck

---

## Building from source

Requires Go 1.21+.

```bash
git clone https://github.com/noisethanks/atak
cd atak
go build -o atak-linux .
```

Release build:
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w -X main.version=v0.1.0" \
  -o atak-linux .
```

---

## GAMMA

[S.T.A.L.K.E.R. GAMMA](https://www.stalkergamma.com/) is a large modpack
for S.T.A.L.K.E.R. Anomaly maintained by Grok. Join the community on
[Discord](https://discord.com/invite/stalker-gamma).

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

If ATAK saved your playthrough, consider supporting development:

**[noisethanks.com/support](https://noisethanks.com/support)**

Made by [mrchocolate](https://github.com/noisethanks)
