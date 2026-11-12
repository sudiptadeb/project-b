#include "project_b/engine/browser/devtools.h"

#include <memory>
#include <string_view>
#include <utility>

#include "base/check.h"
#include "base/command_line.h"
#include "base/files/file_path.h"
#include "base/functional/bind.h"
#include "base/location.h"
#include "base/strings/string_number_conversions.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/devtools_agent_host.h"
#include "content/public/browser/devtools_socket_factory.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/web_contents.h"
#include "content/public/browser/web_contents_delegate.h"
#include "content/public/common/content_switches.h"
#include "net/base/net_errors.h"
#include "net/log/net_log_source.h"
#include "net/socket/tcp_server_socket.h"
#include "project_b/engine/browser/browser_main_parts.h"
#include "project_b/engine/browser/devtools_session.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/shell_page.h"
#include "ui/base/page_transition_types.h"
#include "url/gurl.h"

namespace project_b {

namespace {

constexpr int kBackLog = 10;

// Chrome's (chrome/browser/devtools/remote_debugging_server.cc): this machine only,
// IPv4 first. No tethering.
class TCPServerSocketFactory : public content::DevToolsSocketFactory {
 public:
  explicit TCPServerSocketFactory(uint16_t port) : port_(port) {}
  TCPServerSocketFactory(const TCPServerSocketFactory&) = delete;
  TCPServerSocketFactory& operator=(const TCPServerSocketFactory&) = delete;

 private:
  std::unique_ptr<net::ServerSocket> CreateForHttpServer() override {
    auto socket =
        std::make_unique<net::TCPServerSocket>(nullptr, net::NetLogSource());
    if (socket->ListenWithAddressAndPort("127.0.0.1", port_, kBackLog) ==
        net::OK) {
      return socket;
    }
    if (socket->ListenWithAddressAndPort("::1", port_, kBackLog) == net::OK) {
      return socket;
    }
    return nullptr;
  }

  std::unique_ptr<net::ServerSocket> CreateForTethering(
      std::string* name) override {
    return nullptr;
  }

  const uint16_t port_;
};

// A port is 0 to 65535; 0 lets the system pick.
std::optional<uint16_t> ParsePort(std::string_view text) {
  int port;
  if (base::StringToInt(text, &port) && port >= 0 && port <= 65535) {
    return static_cast<uint16_t>(port);
  }
  return std::nullopt;
}

bool IsShell(content::WebContents* web_contents) {
  Engine* engine = Engine::Get();
  if (!engine || !web_contents) {
    return false;
  }
  for (const auto& [id, w] : engine->windows()) {
    if (web_contents == w->shell->contents()) {
      return true;
    }
  }
  return false;
}

}  // namespace

void StartRemoteDebugging() {
  const base::CommandLine& command_line =
      *base::CommandLine::ForCurrentProcess();
  const bool pipe = command_line.HasSwitch(switches::kRemoteDebuggingPipe);
  const std::optional<uint16_t> port = ParsePort(
      command_line.GetSwitchValueASCII(switches::kRemoteDebuggingPort));
  if (!pipe && !port) {
    return;
  }

  if (pipe) {
    content::DevToolsAgentHost::StartRemoteDebuggingPipeHandler(
        base::BindOnce(&CloseBrowserSoon));
  }
  if (port) {
    // Content prints "DevTools listening on ws://..." to stderr. For a port the
    // system picked, it also writes the port to DevToolsActivePort in the profile,
    // which is where ChromeDriver looks.
    content::DevToolsAgentHost::StartRemoteDebuggingServer(
        std::make_unique<TCPServerSocketFactory>(*port),
        *port == 0 ? ProfileDir() : base::FilePath(),
        /*debug_frontend_dir=*/base::FilePath());
  }
}

// Chrome's ChromeDevToolsManagerDelegate::CloseBrowserSoon, with our one window.
void CloseBrowserSoon() {
  content::GetUIThreadTaskRunner({})->PostTask(
      FROM_HERE, base::BindOnce([] {
        if (Engine* engine = Engine::Get()) {
          engine->CloseWindow(1);
        }
      }));
}

void StopRemoteDebugging() {
  content::DevToolsAgentHost::StopRemoteDebuggingServer();
  content::DevToolsAgentHost::StopRemoteDebuggingPipeHandler();
}

DevToolsManagerDelegate::DevToolsManagerDelegate() = default;
DevToolsManagerDelegate::~DevToolsManagerDelegate() = default;

content::BrowserContext* DevToolsManagerDelegate::GetDefaultBrowserContext() {
  Engine* engine = Engine::Get();
  return engine ? engine->browser_context() : nullptr;
}

// The same way a link with target=_blank opens a tab: the adapter makes it, Go hears
// of it before its first navigation and shows it, then it loads.
scoped_refptr<content::DevToolsAgentHost>
DevToolsManagerDelegate::CreateNewTarget(const GURL& url,
                                         TargetType target_type,
                                         bool new_window) {
  Engine* engine = Engine::Get();
  if (!engine) {
    return nullptr;
  }
  PageTab* tab = engine->AdoptTab(content::WebContents::Create(
      content::WebContents::CreateParams(engine->browser_context())));
  Engine::TabOpened(/*opener=*/0, tab->id(), url, /*foreground=*/true);
  content::NavigationController::LoadURLParams params(url);
  params.transition_type = ui::PAGE_TRANSITION_AUTO_TOPLEVEL;
  tab->contents()->GetController().LoadURLWithParams(params);
  return target_type == kTab
             ? content::DevToolsAgentHost::GetOrCreateForTab(tab->contents())
             : content::DevToolsAgentHost::GetOrCreateFor(tab->contents());
}

// Chrome's: the tab's delegate is asked to show it, which for us goes to Go.
void DevToolsManagerDelegate::Activate(content::DevToolsAgentHost* agent_host) {
  content::WebContents* web_contents = agent_host->GetWebContents();
  if (web_contents && web_contents->GetDelegate()) {
    web_contents->GetDelegate()->ActivateContents(web_contents);
  }
}

// The shell is no tab and no page, so ChromeDriver, which drives pages, never picks
// it. Content calls everything else a page.
std::string DevToolsManagerDelegate::GetTargetType(
    content::WebContents* web_contents) {
  return IsShell(web_contents) ? content::DevToolsAgentHost::kTypeOther
                               : std::string();
}

std::optional<bool> DevToolsManagerDelegate::ShouldReportAsTabTarget(
    content::WebContents* web_contents) {
  return IsShell(web_contents) ? std::optional<bool>(false) : std::nullopt;
}

void DevToolsManagerDelegate::ClientAttached(
    content::DevToolsAgentHostClientChannel* channel) {
  CHECK(!sessions_.contains(channel));
  sessions_.emplace(channel, std::make_unique<DevToolsSession>(channel));
}

void DevToolsManagerDelegate::ClientDetached(
    content::DevToolsAgentHostClientChannel* channel) {
  sessions_.erase(channel);
}

void DevToolsManagerDelegate::HandleCommand(
    content::DevToolsAgentHostClientChannel* channel,
    base::span<const uint8_t> message,
    NotHandledCallback callback) {
  sessions_.at(channel)->HandleCommand(message, std::move(callback));
}

}  // namespace project_b
