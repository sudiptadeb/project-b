#include "project_b/engine/browser/browser_main_parts.h"

#include "base/check.h"
#include "base/command_line.h"
#include "base/files/file_path.h"
#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/location.h"
#include "base/no_destructor.h"
#include "base/run_loop.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/browser_context.h"
#include "project_b/engine/browser/child_processes.h"
#include "project_b/engine/browser/devtools.h"
#include "project_b/engine/browser/driver.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/hooks_gen.h"
#include "project_b/engine/browser/mac_app.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/shell_page.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/window.h"
#include "project_b/engine/common/extensions_on.h"

#if BUILDFLAG(PB_EXTENSIONS)
#include "project_b/engine/extensions/browser/extensions.h"
#endif

namespace project_b {

namespace {

base::FilePath& Profile() {
  static base::NoDestructor<base::FilePath> profile;
  return *profile;
}

base::FilePath& Copy() {
  static base::NoDestructor<base::FilePath> copy;
  return *copy;
}

}  // namespace

base::FilePath ProfileDir() {
  return Profile();
}

base::FilePath CopyDir() {
  return Copy();
}

void SetCopyDir(const base::FilePath& dir) {
  Copy() = dir;
}

base::FilePath ChromiumDataDir() {
  return ProfileDir().Append("chromium");
}

BrowserMainParts::BrowserMainParts() = default;
BrowserMainParts::~BrowserMainParts() = default;

int BrowserMainParts::PreMainMessageLoopRun() {
  const base::CommandLine* command_line =
      base::CommandLine::ForCurrentProcess();
  timeline::Mark("content.ready");

  screen_ = std::make_unique<display::ScopedNativeScreen>();
  // Our pages' context comes first, and only it can load them. The profile's comes when
  // Go opens the profile: at once, or once the picker has chosen it.
  pages_context_ = std::make_unique<BrowserContext>();
  RegisterShellWebUI(pages_context_.get());

  // Quitting ends the run loop on a later task, so whatever asked for it returns first.
  engine_ = std::make_unique<Engine>(
      pages_context_.get(),
      base::BindRepeating(&BrowserMainParts::OpenProfile, base::Unretained(this)),
      base::BindRepeating(
          [](base::RepeatingClosure* quit) {
            content::GetUIThreadTaskRunner({})->PostTask(FROM_HERE, *quit);
          },
          &quit_));

  Go().start(Engine::Calls(),
             command_line->GetSwitchValueASCII("block").c_str(),
             hooks::kTableHash);
  WatchOSKeyBindings();
  child_processes_ = std::make_unique<ChildProcesses>();
  StartRemoteDebugging();

  if (command_line->HasSwitch("pb-driver")) {
    StartDriver(command_line->GetSwitchValuePath("pb-driver"));
  }
  return 0;
}

content::BrowserContext* BrowserMainParts::OpenProfile(
    const base::FilePath& dir) {
  Profile() = dir;
  CHECK(base::CreateDirectory(ChromiumDataDir()));
  browser_context_ = std::make_unique<BrowserContext>(ChromiumDataDir());
#if BUILDFLAG(PB_EXTENSIONS)
  // The extension runtime comes up with the profile and loads what is installed, so Go
  // finds it ready when the profile's managers start.
  if (ExtensionsOn()) {
    extensions_ = std::make_unique<Extensions>(browser_context_.get());
  }
#endif
  return browser_context_.get();
}

void BrowserMainParts::WillRunMainMessageLoop(
    std::unique_ptr<base::RunLoop>& run_loop) {
  quit_ = run_loop->QuitClosure();
}

void BrowserMainParts::PostMainMessageLoopRun() {
  StopRemoteDebugging();
  child_processes_.reset();
  Go().stop();
  engine_.reset();
#if BUILDFLAG(PB_EXTENSIONS)
  extensions_.reset();
#endif
  browser_context_.reset();
  pages_context_.reset();
  screen_.reset();
}

}  // namespace project_b
