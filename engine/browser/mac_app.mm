#include "project_b/engine/browser/mac_app.h"

#import <Cocoa/Cocoa.h>

#include <optional>
#include <string>

#include "base/check_op.h"
#include "base/json/json_reader.h"
#include "base/logging.h"
#include "base/no_destructor.h"
#include "base/json/json_writer.h"
#include "base/observer_list.h"
#include "base/strings/sys_string_conversions.h"
#include "base/values.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "ui/base/accelerators/accelerator.h"
#include "ui/events/event_constants.h"
#include "ui/events/keycodes/keyboard_code_conversion_mac.h"

#import "base/mac/scoped_sending_event.h"
#import "base/message_loop/message_pump_apple.h"
#import "ui/base/accelerators/platform_accelerator_cocoa.h"
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

// Every menu item Go made runs here: it tells Go which action was chosen.
@interface PBMenuTarget : NSObject
- (void)choose:(NSMenuItem*)item;
@end

@implementation PBMenuTarget

- (void)choose:(NSMenuItem*)item {
  project_b::hooks::Args args;
  args.Str(base::SysNSStringToUTF8(item.representedObject));
  project_b::hooks::Fire(PB_EVENT_MENU_CHOSEN, args);
}

@end

// Watches the keys macOS App Shortcuts gives our menu items, and tells Go.
@interface PBKeyBindingsWatcher : NSObject
@end

namespace project_b {

namespace {

constexpr char kKeyEquivalents[] = "NSUserKeyEquivalents";

PBMenuTarget* Target() {
  static PBMenuTarget* target = [[PBMenuTarget alloc] init];
  return target;
}

int FlagsOf(int mods) {
  int flags = 0;
  if (mods & PB_MOD_CTRL) {
    flags |= ui::EF_CONTROL_DOWN;
  }
  if (mods & PB_MOD_ALT) {
    flags |= ui::EF_ALT_DOWN;
  }
  if (mods & PB_MOD_SHIFT) {
    flags |= ui::EF_SHIFT_DOWN;
  }
  if (mods & PB_MOD_CMD) {
    flags |= ui::EF_COMMAND_DOWN;
  }
  return flags;
}

NSMenu* BuildMenu(NSString* title, const base::ListValue& items);

NSMenuItem* BuildItem(const base::DictValue& item) {
  if (item.FindBool("separator").value_or(false)) {
    return [NSMenuItem separatorItem];
  }
  const std::string* title = item.FindString("title");
  NSString* ns_title = base::SysUTF8ToNSString(title ? *title : std::string());
  const base::ListValue* items = item.FindList("items");
  const std::string* role = item.FindString("role");
  // A submenu has items, or a role, for one the OS fills (Services).
  if (items || role) {
    NSMenuItem* parent = [[NSMenuItem alloc] initWithTitle:ns_title
                                                    action:nil
                                             keyEquivalent:@""];
    static const base::NoDestructor<base::ListValue> kNone;
    parent.submenu = BuildMenu(ns_title, items ? *items : *kNone);
    if (role && *role == "services") {
      NSApp.servicesMenu = parent.submenu;
    }
    return parent;
  }
  NSMenuItem* menu_item = [[NSMenuItem alloc] initWithTitle:ns_title
                                                     action:@selector(choose:)
                                              keyEquivalent:@""];
  menu_item.target = Target();
  const std::string* action = item.FindString("action");
  menu_item.representedObject =
      base::SysUTF8ToNSString(action ? *action : std::string());
  menu_item.enabled = item.FindBool("enabled").value_or(false);
  menu_item.state = item.FindBool("checked").value_or(false)
                        ? NSControlStateValueOn
                        : NSControlStateValueOff;
  if (const base::DictValue* key = item.FindDict("key")) {
    ui::Accelerator accelerator(
        static_cast<ui::KeyboardCode>(key->FindInt("code").value_or(0)),
        FlagsOf(key->FindInt("mods").value_or(0)));
    KeyEquivalentAndModifierMask* equivalent =
        ui::GetKeyEquivalentAndModifierMaskFromAccelerator(accelerator);
    menu_item.keyEquivalent = equivalent.keyEquivalent;
    menu_item.keyEquivalentModifierMask = equivalent.modifierMask;
  }
  return menu_item;
}

NSMenu* BuildMenu(NSString* title, const base::ListValue& items) {
  NSMenu* menu = [[NSMenu alloc] initWithTitle:title];
  // Go says which items are enabled; AppKit mustn't decide again.
  menu.autoenablesItems = NO;
  for (const base::Value& item : items) {
    if (item.is_dict()) {
      [menu addItem:BuildItem(item.GetDict())];
    }
  }
  return menu;
}

// "@~n" is Cmd+Option+N: macOS writes the modifiers as @ Cmd, ~ Option, ^ Ctrl and
// $ Shift, then the key's character.
bool ParseKeyEquivalent(NSString* text, int* code, int* mods) {
  *mods = 0;
  NSUInteger i = 0;
  for (; i + 1 < text.length; i++) {
    switch ([text characterAtIndex:i]) {
      case '@':
        *mods |= PB_MOD_CMD;
        continue;
      case '~':
        *mods |= PB_MOD_ALT;
        continue;
      case '^':
        *mods |= PB_MOD_CTRL;
        continue;
      case '$':
        *mods |= PB_MOD_SHIFT;
        continue;
    }
    break;
  }
  if (i + 1 != text.length) {
    return false;
  }
  unichar c = [text characterAtIndex:i];
  if (c >= 'A' && c <= 'Z') {
    *mods |= PB_MOD_SHIFT;
  }
  *code = ui::KeyboardCodeFromCharCode(c);
  return *code != 0;
}

}  // namespace

void InitMacApp() {
  CHECK_EQ(NSApp, nil);
  [PBApplication sharedApplication];
  [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
  // Go's menu has its own Full Screen item, so AppKit mustn't add another.
  [NSUserDefaults.standardUserDefaults
      registerDefaults:@{@"NSFullScreenMenuItemEverywhere" : @NO}];
}

void SetMainMenu(const std::string& json) {
  std::optional<base::Value> menu = base::JSONReader::Read(json, 0);
  if (!menu || !menu->is_list()) {
    LOG(WARNING) << "menu from Go isn't a list";
    return;
  }
  NSMenu* bar = [[NSMenu alloc] initWithTitle:@""];
  for (const base::Value& item : menu->GetList()) {
    if (!item.is_dict()) {
      continue;
    }
    NSMenuItem* top = BuildItem(item.GetDict());
    const std::string* role = item.GetDict().FindString("role");
    if (role && *role == "app") {
      // The app menu's title is the app's name, which AppKit fills in.
      top.submenu.title = @"";
    }
    if (role && *role == "window") {
      NSApp.windowsMenu = top.submenu;
    }
    [bar addItem:top];
  }
  NSApp.mainMenu = bar;
}

void RunAppCommand(int32_t command) {
  switch (command) {
    case PB_APP_ABOUT:
      [NSApp orderFrontStandardAboutPanel:nil];
      break;
    case PB_APP_HIDE:
      [NSApp hide:nil];
      break;
    case PB_APP_HIDE_OTHERS:
      [NSApp hideOtherApplications:nil];
      break;
    case PB_APP_SHOW_ALL:
      [NSApp unhideAllApplications:nil];
      break;
    case PB_APP_BRING_ALL_TO_FRONT:
      [NSApp arrangeInFront:nil];
      break;
  }
}

void ReportOSKeyBindings() {
  // All Applications first, then ours over them, as macOS applies them.
  NSMutableDictionary* all = [NSMutableDictionary dictionary];
  NSDictionary* global = [NSUserDefaults.standardUserDefaults
      persistentDomainForName:NSGlobalDomain][@(kKeyEquivalents)];
  NSDictionary* ours =
      [NSUserDefaults.standardUserDefaults dictionaryForKey:@(kKeyEquivalents)];
  for (NSDictionary* d in @[ global ?: @{}, ours ?: @{} ]) {
    [all addEntriesFromDictionary:d];
  }
  base::DictValue out;
  for (NSString* title in all) {
    id value = all[title];
    int code = 0;
    int mods = 0;
    if (![value isKindOfClass:[NSString class]] ||
        !ParseKeyEquivalent(value, &code, &mods)) {
      continue;
    }
    out.Set(base::SysNSStringToUTF8(title),
            base::DictValue().Set("code", code).Set("mods", mods));
  }
  hooks::Args args;
  args.Str(base::WriteJson(out).value_or("{}"));
  hooks::Fire(PB_EVENT_OS_BINDINGS, args);
}

void WatchOSKeyBindings() {
  static PBKeyBindingsWatcher* watcher = [[PBKeyBindingsWatcher alloc] init];
  [NSUserDefaults.standardUserDefaults addObserver:watcher
                                        forKeyPath:@(kKeyEquivalents)
                                           options:0
                                           context:nullptr];
  ReportOSKeyBindings();
}

}  // namespace project_b

@implementation PBKeyBindingsWatcher

// Another process (System Settings) changed them; KVO tells us on our main thread.
- (void)observeValueForKeyPath:(NSString*)keyPath
                      ofObject:(id)object
                        change:(NSDictionary*)change
                       context:(void*)context {
  project_b::ReportOSKeyBindings();
}

@end
