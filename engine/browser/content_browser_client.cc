#include "project_b/engine/browser/content_browser_client.h"

#include "base/check.h"
#include "base/files/file_path.h"
#include "base/path_service.h"
#include "components/embedder_support/user_agent_utils.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/navigation_throttle_registry.h"
#include "project_b/engine/browser/browser_main_parts.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/navigation_gate.h"
#include "project_b/engine/browser/page_tab.h"

namespace project_b {

ContentBrowserClient::ContentBrowserClient() = default;
ContentBrowserClient::~ContentBrowserClient() = default;

std::unique_ptr<content::BrowserMainParts>
ContentBrowserClient::CreateBrowserMainParts(bool is_integration_test) {
  return std::make_unique<BrowserMainParts>();
}

// Every child process runs the helper, which has no Go in it.
base::FilePath ContentBrowserClient::GetChildProcessPath(int flags) {
  base::FilePath dir;
  CHECK(base::PathService::Get(base::DIR_EXE, &dir));
  return dir.Append(FILE_PATH_LITERAL("project_b_helper"));
}

void ContentBrowserClient::CreateThrottlesForNavigation(
    content::NavigationThrottleRegistry& registry) {
  content::NavigationHandle& handle = registry.GetNavigationHandle();
  if (!handle.IsInPrimaryMainFrame()) {
    return;
  }
  // The shell's own navigation isn't a tab, so it has no gate.
  if (PageTab* tab = Engine::Get()->TabFor(handle.GetWebContents())) {
    registry.AddThrottle(std::make_unique<NavigationGate>(registry, tab->id()));
  }
}

std::string ContentBrowserClient::GetUserAgent() {
  return embedder_support::GetUserAgent();
}

// The sandboxed network process may only write under these folders.
std::vector<base::FilePath>
ContentBrowserClient::GetNetworkContextsParentDirectory() {
  return {ChromiumDataDir()};
}

}  // namespace project_b
