#ifndef PROJECT_B_ENGINE_BROWSER_DEVTOOLS_SESSION_H_
#define PROJECT_B_ENGINE_BROWSER_DEVTOOLS_SESSION_H_

#include <memory>
#include <optional>
#include <string>

#include "base/containers/span.h"
#include "base/memory/raw_ptr.h"
#include "content/public/browser/devtools_manager_delegate.h"
#include "project_b/engine/browser/protocol/browser.h"
#include "project_b/engine/browser/protocol/protocol.h"

namespace content {
class DevToolsAgentHostClientChannel;
}

namespace project_b {

// One DevTools client's commands that content leaves to the embedder: Browser's window
// commands, answered as Chrome answers them (chrome/browser/devtools/protocol/
// browser_handler.cc) for our one window, whose id is 1. The rest fall through to
// content. Generated from Blink's protocol as content_shell's are; see
// browser/protocol_config.json.
class DevToolsSession : public protocol::FrontendChannel,
                        public protocol::Browser::Backend {
 public:
  explicit DevToolsSession(content::DevToolsAgentHostClientChannel* channel);
  DevToolsSession(const DevToolsSession&) = delete;
  DevToolsSession& operator=(const DevToolsSession&) = delete;
  ~DevToolsSession() override;

  void HandleCommand(
      base::span<const uint8_t> message,
      content::DevToolsManagerDelegate::NotHandledCallback callback);

 private:
  // protocol::FrontendChannel:
  void SendProtocolResponse(
      int call_id,
      std::unique_ptr<protocol::Serializable> message) override;
  void SendProtocolNotification(
      std::unique_ptr<protocol::Serializable> message) override;
  void FlushProtocolNotifications() override;

  // protocol::Browser::Backend:
  protocol::Response GetWindowForTarget(
      std::optional<std::string> target_id,
      int* out_window_id,
      std::unique_ptr<protocol::Browser::Bounds>* out_bounds) override;
  protocol::Response GetWindowBounds(
      int window_id,
      std::unique_ptr<protocol::Browser::Bounds>* out_bounds) override;
  protocol::Response SetWindowBounds(
      int window_id,
      std::unique_ptr<protocol::Browser::Bounds> window_bounds) override;
  protocol::Response Close() override;

  const raw_ptr<content::DevToolsAgentHostClientChannel> channel_;
  // The target this client attached to, for a getWindowForTarget with no id.
  const std::string target_id_;
  protocol::UberDispatcher dispatcher_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_DEVTOOLS_SESSION_H_
