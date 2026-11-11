#ifndef PROJECT_B_ENGINE_BROWSER_BROWSER_MAIN_PARTS_H_
#define PROJECT_B_ENGINE_BROWSER_BROWSER_MAIN_PARTS_H_

#include <memory>
#include <string>

#include "base/files/file_path.h"
#include "base/functional/callback.h"
#include "build/build_config.h"
#include "content/public/browser/browser_main_parts.h"
#include "project_b/engine/buildflags/buildflags.h"
#include "ui/display/screen.h"

namespace project_b {

class BrowserContext;
class ChildProcesses;
class Engine;
class Extensions;
class ShellPage;
class Window;

// The profile folder, and the folder inside it that belongs to Chromium.
base::FilePath ProfileDir();
base::FilePath ChromiumDataDir();

// Content calls this at fixed moments. PreMainMessageLoopRun is our startup.
class BrowserMainParts : public content::BrowserMainParts {
 public:
  BrowserMainParts();
  BrowserMainParts(const BrowserMainParts&) = delete;
  BrowserMainParts& operator=(const BrowserMainParts&) = delete;
  ~BrowserMainParts() override;

#if BUILDFLAG(IS_ANDROID)
  static BrowserMainParts* Get();
  // Java calls this once content is up and the window exists.
  void StartApp(const std::string& blocked_hosts);
#endif

 private:
  int PreMainMessageLoopRun() override;
  void WillRunMainMessageLoop(
      std::unique_ptr<base::RunLoop>& run_loop) override;
  void PostMainMessageLoopRun() override;

#if BUILDFLAG(IS_MAC)
  std::unique_ptr<display::ScopedNativeScreen> screen_;
#endif
  std::unique_ptr<BrowserContext> browser_context_;
#if BUILDFLAG(PB_EXTENSIONS)
  std::unique_ptr<Extensions> extensions_;
#endif
  std::unique_ptr<Window> window_;
  std::unique_ptr<ShellPage> shell_;
  std::unique_ptr<Engine> engine_;
  std::unique_ptr<ChildProcesses> child_processes_;
  base::OnceClosure quit_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_BROWSER_MAIN_PARTS_H_
