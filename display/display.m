#import <Cocoa/Cocoa.h>

static NSMutableArray *imageWindows;
static id globalMouseMonitor = nil;

// Delegate that keeps the app alive after all windows close.
@interface SpankImgDelegate : NSObject <NSApplicationDelegate>
@end

@implementation SpankImgDelegate
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
    return NO;
}
@end

// Forward declaration so DismissView methods can call it
void dismissImageOnMainThread(void);

// DismissView handles local clicks and keypresses when our window is key.
@interface DismissView : NSView
@end

@implementation DismissView
- (BOOL)acceptsFirstResponder { return YES; }
- (void)mouseDown:(NSEvent *)event { dismissImageOnMainThread(); }
- (void)keyDown:(NSEvent *)event   { dismissImageOnMainThread(); }
@end

void initDisplay(void) {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    [NSApp setDelegate:[[SpankImgDelegate alloc] init]];
    imageWindows = [[NSMutableArray alloc] init];
}

// Must be called from the main thread.
void dismissImageOnMainThread(void) {
    // Remove global monitor first so it doesn't fire during teardown.
    if (globalMouseMonitor) {
        [NSEvent removeMonitor:globalMouseMonitor];
        globalMouseMonitor = nil;
    }
    for (NSWindow *w in imageWindows) {
        [w close];
    }
    [imageWindows removeAllObjects];
}

void dismissImage(void) {
    if ([NSThread isMainThread]) {
        dismissImageOnMainThread();
    } else {
        dispatch_async(dispatch_get_main_queue(), ^{
            dismissImageOnMainThread();
        });
    }
}

void showImageOnAllScreens(const char *imagePath) {
    NSString *path = [NSString stringWithUTF8String:imagePath];
    dispatch_async(dispatch_get_main_queue(), ^{
        // Tear down any previous display.
        dismissImageOnMainThread();

        NSImage *image = [[NSImage alloc] initWithContentsOfFile:path];
        if (!image) {
            NSLog(@"spankimg: failed to load image at %@", path);
            return;
        }

        for (NSScreen *screen in [NSScreen screens]) {
            NSRect frame = [screen frame];

            NSWindow *window = [[NSWindow alloc]
                initWithContentRect:frame
                styleMask:NSWindowStyleMaskBorderless
                backing:NSBackingStoreBuffered
                defer:NO
                screen:screen];

            [window setLevel:NSScreenSaverWindowLevel];
            [window setBackgroundColor:[NSColor blackColor]];
            [window setOpaque:YES];
            [window setHidesOnDeactivate:NO];
            [window setCollectionBehavior:
                NSWindowCollectionBehaviorCanJoinAllSpaces |
                NSWindowCollectionBehaviorFullScreenAuxiliary];

            DismissView *contentView = [[DismissView alloc]
                initWithFrame:NSMakeRect(0, 0, frame.size.width, frame.size.height)];

            NSImageView *imageView = [[NSImageView alloc]
                initWithFrame:[contentView bounds]];
            [imageView setImage:image];
            [imageView setImageScaling:NSImageScaleProportionallyUpOrDown];
            [imageView setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
            [contentView addSubview:imageView];

            [window setContentView:contentView];
            [window makeKeyAndOrderFront:nil];
            [window makeFirstResponder:contentView];
            [imageWindows addObject:window];
        }

        // Global mouse monitor catches clicks even when our app is not the
        // active app (avoids calling activateIgnoringOtherApps which conflicts
        // with the IOKit HID sensor on another thread).
        // Mouse monitors do not require Accessibility permissions.
        globalMouseMonitor = [NSEvent
            addGlobalMonitorForEventsMatchingMask:
                NSEventMaskLeftMouseDown | NSEventMaskRightMouseDown
            handler:^(NSEvent *event) {
                dispatch_async(dispatch_get_main_queue(), ^{
                    dismissImageOnMainThread();
                });
            }];
    });
}

void runMainLoop(void) {
    [NSApp run];
}
