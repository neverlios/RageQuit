package display

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void initDisplay(void);
void showImageOnAllScreens(const char *imagePath);
void dismissImage(void);
void runMainLoop(void);
*/
import "C"
import "unsafe"

// Init initializes NSApplication. Call once before Show or RunLoop.
func Init() {
	C.initDisplay()
}

// Show displays the image at imagePath fullscreen on all connected displays.
// Safe to call from any goroutine.
func Show(imagePath string) {
	cs := C.CString(imagePath)
	defer C.free(unsafe.Pointer(cs))
	C.showImageOnAllScreens(cs)
}

// Dismiss closes all currently displayed image windows.
// Safe to call from any goroutine.
func Dismiss() {
	C.dismissImage()
}

// RunLoop starts the NSApp main run loop. This blocks forever.
// Must be called from the main OS thread (i.e., from main() after
// runtime.LockOSThread()).
func RunLoop() {
	C.runMainLoop()
}
