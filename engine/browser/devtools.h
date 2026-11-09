// DevTools' remote debugging endpoint, as Chrome opens it: off unless the command line
// asks with --remote-debugging-port or --remote-debugging-pipe. Content runs the
// protocol; this file answers content's questions about our targets. See
// docs/devtools.md.

#ifndef PROJECT_B_ENGINE_BROWSER_DEVTOOLS_H_
#define PROJECT_B_ENGINE_BROWSER_DEVTOOLS_H_

#include <memory>
#include <optional>
#include <string>

#include "base/containers/flat_map.h"
#include "content/public/browser/devtools_manager_delegate.h"

namespace project_b {

// Call once the engine exists, and stop before it goes.
void StartRemoteDebugging();
void StopRemoteDebugging();

// Closes the app on a later task, for a client that asked: Browser.close, or the pipe
// closing.
void CloseBrowserSoon();

class DevToolsSession;

// Every tab is a tab target over a page target. The shell is neither.
class DevToolsManagerDelegate : public content::DevToolsManagerDelegate {
 public:
  DevToolsManagerDelegate();
  DevToolsManagerDelegate(const DevToolsManagerDelegate&) = delete;
  DevToolsManagerDelegate& operator=(const DevToolsManagerDelegate&) = delete;
  ~DevToolsManagerDelegate() override;

 private:
  content::BrowserContext* GetDefaultBrowserContext() override;
  // Target.createTarget and /json/new. One window, so a new window is a new tab.
  scoped_refptr<content::DevToolsAgentHost> CreateNewTarget(
      const GURL& url,
      TargetType target_type,
      bool new_window) override;
  void Activate(content::DevToolsAgentHost* agent_host) override;
  std::string GetTargetType(content::WebContents* web_contents) override;
  std::optional<bool> ShouldReportAsTabTarget(
      content::WebContents* web_contents) override;

  // Each client gets a session for the commands content leaves to us.
  void ClientAttached(content::DevToolsAgentHostClientChannel* channel) override;
  void ClientDetached(content::DevToolsAgentHostClientChannel* channel) override;
  void HandleCommand(content::DevToolsAgentHostClientChannel* channel,
                     base::span<const uint8_t> message,
                     NotHandledCallback callback) override;

  base::flat_map<content::DevToolsAgentHostClientChannel*,
                 std::unique_ptr<DevToolsSession>>
      sessions_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_DEVTOOLS_H_
