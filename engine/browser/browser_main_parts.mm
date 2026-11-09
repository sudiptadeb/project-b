#include "project_b/engine/browser/browser_main_parts.h"

#include "base/check.h"
#include "base/command_line.h"
#include "base/files/file_path.h"
#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/location.h"
#include "base/run_loop.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/browser_context.h"
#include "project_b/engine/browser/driver.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/hooks_gen.h"
#include "project_b/engine/browser/mac_app.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/shell_page.h"
#include "project_b/engine/browser/window.h"

namespace project_b {

base::FilePath ProfileDir() {
  base::FilePath profile =
      base::CommandLine::ForCurrentProcess()->GetSwitchValuePath("profile");
  return profile.empty() ? base::GetHomeDir().Append(".project-b/default")
                         : profile;
}

base::FilePath ChromiumDataDir() {
  return ProfileDir().Append("chromium");
}

BrowserMainParts::BrowserMainParts() = default;
BrowserMainParts::~BrowserMainParts() = default;

int BrowserMainParts::PreMainMessageLoopRun() {
  const base::CommandLine* command_line =
      base::CommandLine::ForCurrentProcess();

  screen_ = std::make_unique<display::ScopedNativeScreen>();
  CHECK(base::CreateDirectory(ChromiumDataDir()));
  browser_context_ = std::make_unique<BrowserContext>(ChromiumDataDir());
  RegisterShellWebUI(browser_context_.get());
  InstallMainMenu();

  window_ = std::make_unique<Window>(base::BindOnce(
      [](base::OnceClosure* quit) {
        content::GetUIThreadTaskRunner({})->PostTask(FROM_HERE,
                                                     std::move(*quit));
      },
      &quit_));

  shell_ = std::make_unique<ShellPage>(browser_context_.get());
  window_->SetShellView(shell_->contents()->GetNativeView());
  engine_ = std::make_unique<Engine>(browser_context_.get(), window_.get(),
                                     shell_.get());
  shell_->Load();

  Go().start(Engine::Calls(), ProfileDir().value().c_str(),
             command_line->GetSwitchValueASCII("block").c_str(),
             hooks::kTableHash);
  // Go has given the window its frame, so its first frame on screen is already the
  // frame colour. It still shows before any renderer has drawn.
  window_->Show();
  window_->ListenForControls(
      base::BindRepeating([](const gfx::RectF& r, double corner_radius) {
        hooks::Args args;
        args.Double(r.x());
        args.Double(r.y());
        args.Double(r.width());
        args.Double(r.height());
        args.Double(corner_radius);
        hooks::Fire(PB_EVENT_WINDOW_CONTROLS_CHANGED, args);
      }));

  if (command_line->HasSwitch("pb-driver")) {
    StartDriver(command_line->GetSwitchValuePath("pb-driver"));
  }
  return 0;
}

void BrowserMainParts::WillRunMainMessageLoop(
    std::unique_ptr<base::RunLoop>& run_loop) {
  quit_ = run_loop->QuitClosure();
}

void BrowserMainParts::PostMainMessageLoopRun() {
  Go().stop();
  engine_.reset();
  shell_.reset();
  window_.reset();
  browser_context_.reset();
  screen_.reset();
}

}  // namespace project_b
