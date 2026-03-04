# RageQuit

> Rage-slam your desk near MacBook (or MacBook itself). Get a surprise image.

A macOS CLI tool for Apple Silicon that detects physical impacts on your laptop via the built-in accelerometer and responds by showing a fullscreen image on **all connected displays**. Inspired by [spank](https://github.com/taigrr/spank).

![macOS](https://img.shields.io/badge/macOS-Apple%20Silicon-black?logo=apple)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-blue)

---

## How it works

1. Reads the Bosch BMI286 IMU (accelerometer) via IOKit HID — same sensor used by `spank`
2. Detects impacts above a configurable g-force threshold
3. Spawns a fullscreen window on every connected display showing a random image from your folder
4. Click or press any key to dismiss

## Requirements

- Apple Silicon Mac (M1 / M2 / M3 / M4)
- macOS 12+
- Go 1.21+
- Xcode Command Line Tools (`xcode-select --install`)
- `sudo` to run (IOKit HID sensor requires root)

## Install

```bash
git clone https://github.com/your-username/RageQuit
cd RageQuit
go build -o RageQuit .
```

On first run the display binary (~Swift) is compiled and cached to `~/.cache/ragequit/display` — this takes ~10 seconds once.

## Setup

Put one or more images in `~/RageQuitImgs/`:

```bash
mkdir -p ~/RageQuit
cp ~/Downloads/yourimage.jpg ~/RageQuitImgs/
```

Supported formats: `.jpg` `.jpeg` `.png` `.gif` `.bmp` `.tiff` `.webp`

## Usage

```bash
sudo ./RageQuit
```

```
RageQuit: found 3 image(s) in ~/RageQuitImgs/
RageQuit: sensitivity 0.60g, cooldown 750ms
RageQuit: display binary ready.
```

Spank your Mac → random image appears fullscreen on all displays → click or press any key to dismiss → repeat.

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `--image-dir`, `-i` | `~/RageQuitImgs/` | Folder containing images |
| `--min-amplitude`, `-a` | `0.6` | Impact threshold in g-force (lower = more sensitive) |
| `--cooldown`, `-c` | `750` | Milliseconds between triggers |

```bash
# More sensitive (light tap triggers it)
sudo ./RageQuit --min-amplitude 0.3

# Less sensitive (requires a hard hit)
sudo ./RageQuit --min-amplitude 0.9

# Custom image folder
sudo ./RageQuit --image-dir ~/Pictures/reactions/
```

Press `Ctrl+C` to quit.

## Architecture

```
RageQuit (Go, no CGo)
├── sensor goroutine   — IOKit HID via purego → reads accelerometer
├── detection goroutine — threshold + cooldown → triggers display
└── display subprocess  — compiled Swift binary → fullscreen NSWindow on all NSScreens
```

The sensor and display are deliberately in **separate processes**: the accelerometer uses [purego](https://github.com/ebitengine/purego) for CGo-free IOKit access, which conflicts with AppKit's CGo bindings if they share a process. The Swift display binary is compiled once and cached.

## Troubleshooting

**No impact detected** — lower `--min-amplitude` to `0.15` and try again. Verify you're running with `sudo`.

**Image doesn't appear** — check `~/RageQuitImgs/` has at least one image. Delete `~/.cache/ragequit/display` to force a recompile of the display binary.

**Stale display binary** — delete and let it recompile:
```bash
rm ~/.cache/ragequit/display
```

**Shared memory conflict** — if a previous run crashed without cleanup:
```bash
sudo ipcs -M | grep RageQuit
sudo ipcrm -M /ragequit-accel
```

## Credits

- Accelerometer reading: [taigrr/apple-silicon-accelerometer](https://github.com/taigrr/apple-silicon-accelerometer)
- Inspired by: [taigrr/spank](https://github.com/taigrr/spank)

## License

MIT
