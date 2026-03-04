package display

import (
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

guard CommandLine.arguments.count > 1 else {
    fputs("Usage: ragequit-display <image-path>\n", stderr)
    exit(1)
}

let imagePath = CommandLine.arguments[1]
guard let image = NSImage(contentsOfFile: imagePath) else {
    fputs("ragequit: cannot load image: \(imagePath)\n", stderr)
    exit(1)
}

class DismissView: NSView {
    override var acceptsFirstResponder: Bool { true }
    override func mouseDown(with event: NSEvent) { NSApp.terminate(nil) }
    override func keyDown(with event: NSEvent) { NSApp.terminate(nil) }
}

class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationShouldTerminateAfterLastWindowClosed(_ app: NSApplication) -> Bool {
        return true
    }
}

let app = NSApplication.shared
app.setActivationPolicy(.accessory)
let delegate = AppDelegate()
app.delegate = delegate

for screen in NSScreen.screens {
    let frame = screen.frame
    let window = NSWindow(
        contentRect: frame,
        styleMask: .borderless,
        backing: .buffered,
        defer: false,
        screen: screen
    )
    window.level = .screenSaver
    window.backgroundColor = .black
    window.isOpaque = true
    window.hidesOnDeactivate = false
    window.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]

    let contentView = DismissView(frame: NSRect(origin: .zero, size: frame.size))
    let imageView = NSImageView(frame: contentView.bounds)
    imageView.image = image
    imageView.imageScaling = .scaleProportionallyUpOrDown
    imageView.autoresizingMask = [.width, .height]
    contentView.addSubview(imageView)

    window.contentView = contentView
    window.makeKeyAndOrderFront(nil)
    window.makeFirstResponder(contentView)
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

// CompileIfNeeded compiles the embedded Swift display binary if it does not
// already exist. This is a one-time operation (~10 seconds).
func CompileIfNeeded() error {
	binaryPath := BinaryPath()
	if _, err := os.Stat(binaryPath); err == nil {
		return nil
	}

	fmt.Println("ragequit: compiling display binary (first run only, ~10s)...")

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
