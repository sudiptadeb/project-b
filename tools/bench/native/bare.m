// The floor for Project B's start-up and memory: a bare Mac app with one window, timed
// like the app's timeline, in ms since the kernel started the process
// (docs/benchmarks.md). `bare` draws a plain view; `bare web` puts a WKWebView in the
// window with a page like a shell's top row. It prints each mark and quits once the first
// frame or the page's first paint is out; with KEEP set it stays, for tools/bench/native.py
// to read its memory. Built and run by tools/bench/native.py.
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <QuartzCore/QuartzCore.h>
#include <sys/sysctl.h>
static double start_ms;
static double now_ms(void) { struct timeval t; gettimeofday(&t, NULL); return t.tv_sec * 1e3 + t.tv_usec / 1e3; }
static void mark(const char* n) { printf("%8.1f %s\n", now_ms() - start_ms, n); fflush(stdout); }
static BOOL web;
@interface V : NSView @end
@implementation V { BOOL drawn; }
- (void)drawRect:(NSRect)r {
  [[NSColor colorWithWhite:0.95 alpha:1] setFill]; NSRectFill(r);
  if (!drawn) { drawn = YES; mark("view.first_draw");
    [CATransaction setCompletionBlock:^{ mark("first_frame.committed"); }]; }
}
@end
@interface D : NSObject <NSApplicationDelegate, WKScriptMessageHandler> @property NSWindow* w; @end
@implementation D
- (void)applicationDidFinishLaunching:(NSNotification*)n { mark("app.did_finish_launching"); }
- (void)userContentController:(WKUserContentController*)c didReceiveScriptMessage:(WKScriptMessage*)m {
  mark([m.body UTF8String]);
  if ([m.body isEqual:@"page.first_paint"] && !getenv("KEEP")) dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 300*NSEC_PER_MSEC), dispatch_get_main_queue(), ^{ exit(0); });
}
- (void)build {
  self.w = [[NSWindow alloc] initWithContentRect:NSMakeRect(200, 200, 1200, 800)
      styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskResizable|NSWindowStyleMaskMiniaturizable|NSWindowStyleMaskFullSizeContentView
      backing:NSBackingStoreBuffered defer:NO];
  self.w.titlebarAppearsTransparent = YES; self.w.titleVisibility = NSWindowTitleHidden;
  mark("window.created");
  if (web) {
    WKWebViewConfiguration* c = [WKWebViewConfiguration new];
    [c.userContentController addScriptMessageHandler:self name:@"t"];
    WKWebView* v = [[WKWebView alloc] initWithFrame:self.w.contentView.bounds configuration:c];
    v.autoresizingMask = NSViewWidthSizable|NSViewHeightSizable;
    self.w.contentView = v;
    NSString* html = @"<!doctype html><style>body{margin:0;font:13px system-ui}.bar{height:34px;background:#eee;display:flex;gap:6px;align-items:center;padding-left:80px}.tab{padding:4px 10px;background:#fff;border-radius:4px}</style>"
      "<div class=bar><div class=tab>New Tab</div><button>+</button></div>"
      "<script>webkit.messageHandlers.t.postMessage('page.script');requestAnimationFrame(()=>requestAnimationFrame(()=>webkit.messageHandlers.t.postMessage('page.first_paint')))</script>";
    mark("webview.created");
    [v loadHTMLString:html baseURL:nil];
  } else {
    self.w.contentView = [[V alloc] initWithFrame:self.w.contentView.bounds];
    if (!getenv("KEEP")) dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 500*NSEC_PER_MSEC), dispatch_get_main_queue(), ^{ exit(0); });
  }
  [self.w makeKeyAndOrderFront:nil];
  mark("window.shown");
}
@end
int main(int argc, char** argv) {
  struct kinfo_proc kp; size_t len = sizeof kp; int mib[4] = {CTL_KERN, KERN_PROC, KERN_PROC_PID, getpid()};
  sysctl(mib, 4, &kp, &len, NULL, 0);
  start_ms = kp.kp_proc.p_starttime.tv_sec * 1e3 + kp.kp_proc.p_starttime.tv_usec / 1e3;
  web = argc > 1 && !strcmp(argv[1], "web");
  mark("main.entered");
  @autoreleasepool {
    NSApplication* app = [NSApplication sharedApplication];
    [app setActivationPolicy:NSApplicationActivationPolicyRegular];
    mark("app.created");
    D* d = [D new]; app.delegate = d;
    [d build];
    [app activateIgnoringOtherApps:YES];
    [app run];
  }
}
