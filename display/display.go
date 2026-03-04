package display

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// swiftSource is the Swift program that shows a fullscreen image and exits
// when the user clicks or presses a key. It runs as a separate subprocess
// to avoid conflicts between CGo/AppKit and the purego/IOKit sensor goroutine.
const swiftSource = `import Cocoa
import ImageIO

guard CommandLine.arguments.count > 1 else {
    fputs("Usage: ragequit-display <image-path>\n", stderr)
    exit(1)
}

let imagePath = CommandLine.arguments[1]

// MARK: - GIF support

struct GIFData {
    let frames: [NSImage]
    let delays: [TimeInterval]
}

func loadGIF(path: String) -> GIFData? {
    let url = URL(fileURLWithPath: path)
    guard let source = CGImageSourceCreateWithURL(url as CFURL, nil) else { return nil }
    let count = CGImageSourceGetCount(source)
    guard count > 0 else { return nil }

    var frames: [NSImage] = []
    var delays: [TimeInterval] = []

    for i in 0..<count {
        guard let cgImage = CGImageSourceCreateImageAtIndex(source, i, nil) else { continue }
        let size = NSSize(width: cgImage.width, height: cgImage.height)
        frames.append(NSImage(cgImage: cgImage, size: size))

        var delay: TimeInterval = 0.1
        if let props = CGImageSourceCopyPropertiesAtIndex(source, i, nil) as? [String: Any],
           let gifProps = props[kCGImagePropertyGIFDictionary as String] as? [String: Any],
           let d = gifProps[kCGImagePropertyGIFDelayTime as String] as? Double {
            delay = max(d, 0.1)
        }
        delays.append(delay)
    }

    guard !frames.isEmpty else { return nil }
    return GIFData(frames: frames, delays: delays)
}

// MARK: - App setup

var activeTimers: [Timer] = []

// FullscreenWindow overrides canBecomeKey so that borderless windows receive
// keyboard events. NSWindow.canBecomeKey returns false for .borderless windows
// by default, which means keyDown is never called on the first responder.
class FullscreenWindow: NSWindow {
    override var canBecomeKey: Bool { true }
}

class DismissView: NSView {
    override var acceptsFirstResponder: Bool { true }
    override func mouseDown(with event: NSEvent) {
        for t in activeTimers { t.invalidate() }
        NSApp.terminate(nil)
    }
    override func keyDown(with event: NSEvent) {
        // Only ESC (keyCode 53) dismisses; other keys are silently ignored.
        guard event.keyCode == 53 else { return }
        for t in activeTimers { t.invalidate() }
        NSApp.terminate(nil)
    }
}

class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationShouldTerminateAfterLastWindowClosed(_ app: NSApplication) -> Bool {
        return true
    }
}

// MARK: - Load content

let isGIF = imagePath.lowercased().hasSuffix(".gif")
let gifData: GIFData? = isGIF ? loadGIF(path: imagePath) : nil
let staticImage: NSImage? = isGIF ? nil : NSImage(contentsOfFile: imagePath)

if isGIF && gifData == nil {
    fputs("ragequit: cannot load GIF: \(imagePath)\n", stderr)
    exit(1)
}
if !isGIF && staticImage == nil {
    fputs("ragequit: cannot load image: \(imagePath)\n", stderr)
    exit(1)
}

// MARK: - Create windows

let app = NSApplication.shared
app.setActivationPolicy(.accessory)
let delegate = AppDelegate()
app.delegate = delegate

for screen in NSScreen.screens {
    let globalFrame = screen.frame
    let localFrame = NSRect(origin: .zero, size: globalFrame.size)
    let window = FullscreenWindow(
        contentRect: localFrame,
        styleMask: .borderless,
        backing: .buffered,
        defer: false,
        screen: screen
    )
    window.setFrame(globalFrame, display: false)
    window.level = .screenSaver
    window.backgroundColor = .black
    window.isOpaque = true
    window.hidesOnDeactivate = false
    window.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]

    let contentView = DismissView(frame: NSRect(origin: .zero, size: globalFrame.size))
    let imageView = NSImageView(frame: contentView.bounds)
    imageView.imageScaling = .scaleProportionallyUpOrDown
    imageView.autoresizingMask = [.width, .height]
    contentView.addSubview(imageView)
    window.contentView = contentView
    window.makeKeyAndOrderFront(nil)
    window.makeFirstResponder(contentView)

    if let gif = gifData {
        imageView.image = gif.frames[0]
        if gif.frames.count > 1 {
            var frameIndex = 0
            func scheduleNext() {
                let t = Timer.scheduledTimer(withTimeInterval: gif.delays[frameIndex], repeats: false) { _ in
                    frameIndex = (frameIndex + 1) % gif.frames.count
                    imageView.image = gif.frames[frameIndex]
                    scheduleNext()
                }
                activeTimers.append(t)
            }
            scheduleNext()
        }
    } else {
        imageView.image = staticImage
    }
}

app.activate(ignoringOtherApps: true)
app.run()
`

var (
	mu         sync.Mutex
	currentCmd *exec.Cmd
)

// BinaryPath returns the path where the compiled display binary is cached.
func BinaryPath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".cache", "RageQuit")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "display")
}

// sourceHash returns a hex SHA-256 of the embedded Swift source.
// Used to detect when the source has changed and the cached binary is stale.
func sourceHash() string {
	h := sha256.Sum256([]byte(swiftSource))
	return fmt.Sprintf("%x", h)
}

// CompileIfNeeded compiles the embedded Swift display binary if it does not
// already exist or if the Swift source has changed. This is a one-time operation (~10 seconds).
func CompileIfNeeded() error {
	binaryPath := BinaryPath()
	versionPath := binaryPath + ".version"
	currentHash := sourceHash()

	// Skip recompilation only if binary exists AND hash matches.
	if _, err := os.Stat(binaryPath); err == nil {
		if data, err := os.ReadFile(versionPath); err == nil && string(data) == currentHash {
			return nil
		}
	}

	// Binary is missing or stale — remove both and recompile.
	os.Remove(binaryPath)
	os.Remove(versionPath)

	fmt.Println("ragequit: compiling display binary (first run or source changed)...")

	srcPath := filepath.Join(os.TempDir(), "ragequit-display.swift")
	if err := os.WriteFile(srcPath, []byte(swiftSource), 0644); err != nil {
		return fmt.Errorf("writing swift source: %w", err)
	}
	defer os.Remove(srcPath)

	cmd := exec.Command("swiftc", "-O", "-o", binaryPath, srcPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(binaryPath)
		return fmt.Errorf("compiling display binary: %w", err)
	}

	if err := os.WriteFile(versionPath, []byte(currentHash), 0644); err != nil {
		return fmt.Errorf("writing version file: %w", err)
	}

	fmt.Println("ragequit: display binary ready.")
	return nil
}

// Show displays the image fullscreen on all connected displays.
// Kills any previous display subprocess before showing the new one.
// Safe to call from any goroutine.
func Show(imagePath string) {
	mu.Lock()
	prev := currentCmd
	currentCmd = nil
	mu.Unlock()

	if prev != nil && prev.Process != nil {
		prev.Process.Kill()
		prev.Wait()
	}

	cmd := exec.Command(BinaryPath(), imagePath)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ragequit: failed to start display: %v\n", err)
		return
	}

	mu.Lock()
	currentCmd = cmd
	mu.Unlock()
}
