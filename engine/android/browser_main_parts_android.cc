#include "project_b/engine/browser/browser_main_parts.h"

#include "base/android/path_utils.h"
#include "base/check.h"
#include "base/files/file_path.h"
#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/functional/callback_helpers.h"
#include "project_b/engine/browser/browser_context.h"
#include "project_b/engine/browser/child_processes.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/hooks_gen.h"
#include "project_b/engine/browser/shell_page.h"
#include "project_b/engine/browser/window.h"

namespace project_b {

namespace {
BrowserMainParts* g_parts = nullptr;
}  // namespace

base::FilePath ProfileDir() {
  base::FilePath data;
  CHECK(base::android::GetDataDirectory(&data));
  return data.Append("profile");
}

base::FilePath ChromiumDataDir() {
  return ProfileDir().Append("chromium");
}

BrowserMainParts::BrowserMainParts() {
  g_parts = this;
}

BrowserMainParts::~BrowserMainParts() = default;

// static
BrowserMainParts* BrowserMainParts::Get() {
  return g_parts;
}

// Android owns the message loop and the window, so the window can't come first here as it does on
// the Mac. Java asks for StartApp once content says it's ready.
int BrowserMainParts::PreMainMessageLoopRun() {
  CHECK(base::CreateDirectory(ChromiumDataDir()));
  browser_context_ = std::make_unique<BrowserContext>(ChromiumDataDir());
  RegisterShellWebUI(browser_context_.get());
  return 0;
}

void BrowserMainParts::StartApp(const std::string& blocked_hosts) {
  window_ = std::make_unique<Window>(base::DoNothing());
  shell_ = std::make_unique<ShellPage>(browser_context_.get());
  window_->SetShellContents(shell_->contents());
  engine_ = std::make_unique<Engine>(browser_context_.get(), window_.get(),
                                     shell_.get());
  shell_->Load();
  Go().start(Engine::Calls(), ProfileDir().value().c_str(),
             blocked_hosts.c_str(), hooks::kTableHash);
  child_processes_ = std::make_unique<ChildProcesses>();
  window_->Show();
  window_->ListenForState(base::BindRepeating([](const Window::State& s) {
    hooks::Args args;
    args.Double(s.size.width());
    args.Double(s.size.height());
    args.Double(s.controls.x());
    args.Double(s.controls.y());
    args.Double(s.controls.width());
    args.Double(s.controls.height());
    args.Double(s.corner_radius);
    args.Bool(s.full_screen);
    hooks::Fire(PB_EVENT_WINDOW_CHANGED, args);
  }));
}

void BrowserMainParts::WillRunMainMessageLoop(
    std::unique_ptr<base::RunLoop>& run_loop) {}

void BrowserMainParts::PostMainMessageLoopRun() {}

}  // namespace project_b
