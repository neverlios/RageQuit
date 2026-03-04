# spankimg Design

**Date**: 2026-03-03
**Status**: Approved

## Overview

A macOS CLI tool that detects physical impacts on Apple Silicon MacBooks via the built-in accelerometer and responds by displaying a fullscreen image on all connected displays. Inspired by `github.com/taigrr/spank`.

## Requirements

- **Platform**: Apple Silicon Mac (M1+), requires root for hardware sensor access
- **Trigger**: Physical spank/impact detected via accelerometer
- **Action**: Show one image fullscreen on all connected displays simultaneously
- **Dismiss**: Any mouse click or keypress closes the image
- **Image source**: Single image in a local folder (`~/spankimg/` by default)
- **Language**: Go with CGo for AppKit display layer

## Architecture

```
spankimg/
├── go.mod
├── main.go          # flags, orchestration, sensor + detection loop
└── display/
    ├── display.go   # CGo bindings exported to Go
    └── display.m    # Objective-C: NSWindow fullscreen image on all screens
```

## Components

### main.go
- Parses CLI flags
- Loads image path from `--image-dir` (finds first image file)
- Initializes display (calls `display.Init()`)
- Starts accelerometer goroutine
- Runs detection loop with threshold + cooldown
- On impact: calls `display.Show(imagePath)`
- Calls `display.RunLoop()` on main thread (blocks, NSApp run loop)

### display/display.m (Objective-C via CGo)
- `initDisplay()` — initializes `NSApplication`, sets activation policy to Accessory (no Dock icon)
- `showImageOnAllScreens(const char *path)` — dispatches to main queue, creates one borderless `NSWindow` per `NSScreen` at `NSScreenSaverWindowLevel`, shows `NSImageView` fitted to window
- `dismissImage()` — dispatches to main queue, closes/hides all image windows
- Event handling — `NSEvent` monitor for mouse and key events triggers dismiss
- `runMainLoop()` — calls `[NSApp run]`, never returns (called from Go main goroutine)

### display/display.go
- CGo import block linking `display.m`
- Exported Go functions: `Init()`, `Show(path string)`, `RunLoop()`

## Data Flow

```
[accelerometer goroutine]
  reads samples → checks magnitude > threshold → fires impact event

[detection goroutine]
  receives impact → checks cooldown → calls display.Show(imagePath)

[main thread / NSApp run loop]
  dispatch_async ← show windows on all NSScreens
  NSEvent monitor → on click/keypress → dismiss windows
```

## Configuration Flags

| Flag              | Default        | Description                        |
|-------------------|----------------|------------------------------------|
| `--image-dir`     | `~/spankimg/`  | Directory containing the image     |
| `--min-amplitude` | `0.3`          | Impact threshold in g-force        |
| `--cooldown`      | `750`          | Milliseconds between triggers      |

## Key Technical Notes

- AppKit **must** run on the main OS thread; Go goroutines must use `dispatch_async(dispatch_get_main_queue(), ...)` for all UI calls
- `[NSApp run]` is called at the end of `main()` and never returns; all Go logic runs in goroutines started before this call
- Image windows use `NSScreenSaverWindowLevel + 1` to appear above most other windows
- Requires `sudo` to access IOKit accelerometer hardware
- The apple-silicon-accelerometer package handles the IOKit HID interaction
