#ifndef PROJECT_B_ENGINE_BROWSER_BROWSER_MAIN_PARTS_H_
#define PROJECT_B_ENGINE_BROWSER_BROWSER_MAIN_PARTS_H_

#include <memory>

#include "base/files/file_path.h"
#include "base/functional/callback.h"
#include "content/public/browser/browser_main_parts.h"
#include "ui/display/screen.h"

namespace project_b {

class BrowserContext;
class ChildProcesses;
class Engine;
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

 private:
  int PreMainMessageLoopRun() override;
  void WillRunMainMessageLoop(
      std::unique_ptr<base::RunLoop>& run_loop) override;
  void PostMainMessageLoopRun() override;

  std::unique_ptr<display::ScopedNativeScreen> screen_;
  std::unique_ptr<BrowserContext> browser_context_;
  std::unique_ptr<Window> window_;
  std::unique_ptr<ShellPage> shell_;
  std::unique_ptr<Engine> engine_;
  std::unique_ptr<ChildProcesses> child_processes_;
  base::OnceClosure quit_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_BROWSER_MAIN_PARTS_H_
