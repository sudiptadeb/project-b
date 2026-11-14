#include "project_b/engine/browser/native_shell_mac.h"

#import <AppKit/AppKit.h>

#include "base/functional/bind.h"
#include "base/location.h"
#include "base/strings/sys_string_conversions.h"
#include "base/task/sequenced_task_runner.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/ProjectBNative-Swift.h"

namespace project_b {

struct NativeShell::Impl {
  PBNativeShellView* __strong view;
};

bool NativeShell::Is(const std::string& page) {
  return page == "shell/native";
}

void NativeShell::Detach(gfx::NativeView view) {
  [view.GetNativeNSView() removeFromSuperview];
}

NativeShell::NativeShell(int64_t window) : impl_(std::make_unique<Impl>()) {
  // Each message crosses in a task of its own, as chrome.send's do, so a message the
  // view sends while Go is calling it (shown, in answer to the snapshot) never re-enters Go.
  impl_->view = [[PBNativeShellView alloc] initWithSend:^(NSString* json) {
    base::SequencedTaskRunner::GetCurrentDefault()->PostTask(
        FROM_HERE, base::BindOnce(
                       [](int64_t window, std::string json) {
                         // One sent just before its window closed arrives with the
                         // window gone.
                         if (!Engine::Get()->window(window)) {
                           return;
                         }
                         hooks::Args args;
                         args.Int(window);
                         args.Str(std::move(json));
                         hooks::Fire(PB_EVENT_SHELL_MESSAGE, args);
                       },
                       window, base::SysNSStringToUTF8(json)));
  }];
}

NativeShell::~NativeShell() {
  [impl_->view removeFromSuperview];
}

gfx::NativeView NativeShell::view() const {
  return gfx::NativeView(impl_->view);
}

void NativeShell::Send(const std::string& json) {
  [impl_->view receive:base::SysUTF8ToNSString(json)];
}

void NativeShell::Focus() {
  [impl_->view focusShell];
}

}  // namespace project_b
