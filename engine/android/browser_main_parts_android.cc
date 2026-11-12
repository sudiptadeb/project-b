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
#include "project_b/engine/browser/timeline.h"
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
  timeline::Mark("content.ready");
  CHECK(base::CreateDirectory(ChromiumDataDir()));
  browser_context_ = std::make_unique<BrowserContext>(ChromiumDataDir());
  RegisterShellWebUI(browser_context_.get());
  return 0;
}

void BrowserMainParts::StartApp(const std::string& blocked_hosts) {
  // Android ends the app itself, with the activity.
  engine_ = std::make_unique<Engine>(browser_context_.get(), base::DoNothing());
  Go().start(Engine::Calls(), ProfileDir().value().c_str(),
             blocked_hosts.c_str(), hooks::kTableHash);
  child_processes_ = std::make_unique<ChildProcesses>();
}

void BrowserMainParts::WillRunMainMessageLoop(
    std::unique_ptr<base::RunLoop>& run_loop) {}

void BrowserMainParts::PostMainMessageLoopRun() {}

}  // namespace project_b
