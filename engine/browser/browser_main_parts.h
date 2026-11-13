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

namespace content {
class BrowserContext;
}

namespace project_b {

class BrowserContext;
class ChildProcesses;
class Engine;
class Extensions;

// The profile folder, and the folder inside it that belongs to Chromium. Empty in a copy
// that hasn't chosen its profile yet.
base::FilePath ProfileDir();
base::FilePath ChromiumDataDir();
// The folder this copy keeps its own files in, its timeline and DevToolsActivePort: the
// profile's, or the root for a copy that picks its profile after it starts. Every profile
// it can run on is inside it.
base::FilePath CopyDir();
#if !BUILDFLAG(IS_ANDROID)
// Go says it, before content starts.
void SetCopyDir(const base::FilePath& dir);
#endif

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
  // Makes the profile's context, and the extension runtime with it, when Go asks.
  content::BrowserContext* OpenProfile(const base::FilePath& dir);

  int PreMainMessageLoopRun() override;
  void WillRunMainMessageLoop(
      std::unique_ptr<base::RunLoop>& run_loop) override;
  void PostMainMessageLoopRun() override;

#if BUILDFLAG(IS_MAC)
  std::unique_ptr<display::ScopedNativeScreen> screen_;
#endif
#if !BUILDFLAG(IS_ANDROID)
  // Our pages' context, made first, and the profile's.
  std::unique_ptr<BrowserContext> pages_context_;
#endif
  std::unique_ptr<BrowserContext> browser_context_;
#if BUILDFLAG(PB_EXTENSIONS)
  std::unique_ptr<Extensions> extensions_;
#endif
  std::unique_ptr<Engine> engine_;
  std::unique_ptr<ChildProcesses> child_processes_;
  base::RepeatingClosure quit_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_BROWSER_MAIN_PARTS_H_
