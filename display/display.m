#import <Cocoa/Cocoa.h>

static NSMutableArray *imageWindows;

// Forward declaration so DismissView methods can call it
void dismissImageOnMainThread(void);

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
    imageWindows = [[NSMutableArray alloc] init];
}

// Must be called from the main thread.
void dismissImageOnMainThread(void) {
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
        // Close any currently shown windows
        for (NSWindow *w in imageWindows) {
            [w close];
        }
        [imageWindows removeAllObjects];

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

            // DismissView handles click/keypress to close
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

        [NSApp activateIgnoringOtherApps:YES];
    });
}

void runMainLoop(void) {
    [NSApp run];
}
