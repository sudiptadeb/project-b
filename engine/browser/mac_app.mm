#include "project_b/engine/browser/mac_app.h"

#import <Cocoa/Cocoa.h>

#include "base/check_op.h"
#include "base/observer_list.h"

#import "base/mac/scoped_sending_event.h"
#import "base/message_loop/message_pump_apple.h"
#import "content/public/browser/native_event_processor_mac.h"
#import "content/public/browser/native_event_processor_observer_mac.h"

@interface PBApplication
    : NSApplication <CrAppProtocol, CrAppControlProtocol, NativeEventProcessor>
@end

@implementation PBApplication {
  base::ObserverList<content::NativeEventProcessorObserver>::Unchecked
      _observers;
  BOOL _handlingSendEvent;
}

- (BOOL)isHandlingSendEvent {
  return _handlingSendEvent;
}

- (void)setHandlingSendEvent:(BOOL)handlingSendEvent {
  _handlingSendEvent = handlingSendEvent;
}

- (void)sendEvent:(NSEvent*)event {
  base::mac::ScopedSendingEvent sendingEventScoper;
  content::ScopedNotifyNativeEventProcessorObserver scopedObserverNotifier(
      &self->_observers, event);
  [super sendEvent:event];
}

- (void)addNativeEventProcessorObserver:
    (content::NativeEventProcessorObserver*)observer {
  _observers.AddObserver(observer);
}

- (void)removeNativeEventProcessorObserver:
    (content::NativeEventProcessorObserver*)observer {
  _observers.RemoveObserver(observer);
}

@end

namespace project_b {

namespace {

NSMenuItem* Item(NSString* title, SEL action, NSString* key) {
  return [[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:key];
}

}  // namespace

void InitMacApp() {
  CHECK_EQ(NSApp, nil);
  [PBApplication sharedApplication];
  [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
}

void InstallMainMenu() {
  NSMenu* bar = [[NSMenu alloc] initWithTitle:@""];

  NSMenu* app = [[NSMenu alloc] initWithTitle:@""];
  // Closing the window is what quits, so shutdown runs and history flushes.
  [app addItem:Item(@"Quit", @selector(performClose:), @"q")];
  NSMenuItem* appItem = [[NSMenuItem alloc] init];
  appItem.submenu = app;
  [bar addItem:appItem];

  NSMenu* edit = [[NSMenu alloc] initWithTitle:@"Edit"];
  [edit addItem:Item(@"Undo", @selector(undo:), @"z")];
  [edit addItem:Item(@"Cut", @selector(cut:), @"x")];
  [edit addItem:Item(@"Copy", @selector(copy:), @"c")];
  [edit addItem:Item(@"Paste", @selector(paste:), @"v")];
  [edit addItem:Item(@"Select All", @selector(selectAll:), @"a")];
  NSMenuItem* editItem = [[NSMenuItem alloc] init];
  editItem.submenu = edit;
  [bar addItem:editItem];

  NSApp.mainMenu = bar;
}

}  // namespace project_b
