#include "project_b/engine/browser/devtools_session.h"

#include <string>
#include <string_view>
#include <utility>
#include <vector>

#include "base/check.h"
#include "base/containers/span.h"
#include "base/strings/strcat.h"
#include "base/strings/string_number_conversions.h"
#include "base/strings/string_util.h"
#include "base/strings/string_view_util.h"
#include "content/public/browser/devtools_agent_host.h"
#include "content/public/browser/devtools_agent_host_client_channel.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/devtools.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/window.h"
#include "third_party/inspector_protocol/crdtp/dispatch.h"
#include "third_party/inspector_protocol/crdtp/json.h"
#include "ui/gfx/geometry/rect.h"

namespace project_b {

namespace {

// There is one window.
constexpr int kWindowId = 1;

Window* GetWindow(int window_id) {
  Engine* engine = Engine::Get();
  return engine && window_id == kWindowId ? engine->window() : nullptr;
}

std::unique_ptr<protocol::Browser::Bounds> BoundsOf(Window* window) {
  std::string window_state = "normal";
  if (window->IsMinimized()) {
    window_state = "minimized";
  }
  if (window->IsMaximized()) {
    window_state = "maximized";
  }
  if (window->IsFullscreen()) {
    window_state = "fullscreen";
  }
  // A minimised window keeps the frame it will come back to.
  const gfx::Rect bounds = window->GetBounds();
  return protocol::Browser::Bounds::Create()
      .SetLeft(bounds.x())
      .SetTop(bounds.y())
      .SetWidth(bounds.width())
      .SetHeight(bounds.height())
      .SetWindowState(window_state)
      .Build();
}

}  // namespace

DevToolsSession::DevToolsSession(
    content::DevToolsAgentHostClientChannel* channel)
    : channel_(channel),
      target_id_(channel->GetAgentHost()->GetId()),
      dispatcher_(this) {
  protocol::Browser::Dispatcher::wire(&dispatcher_, this);
}

DevToolsSession::~DevToolsSession() = default;

// content_shell's (content/shell/browser/protocol/shell_devtools_session.cc): what no
// handler here takes goes back to content.
void DevToolsSession::HandleCommand(
    base::span<const uint8_t> message,
    content::DevToolsManagerDelegate::NotHandledCallback callback) {
  crdtp::Dispatchable dispatchable(
      crdtp::SpanFrom(message), std::string_view(),
      [cb = std::move(callback)](int call_id, crdtp::span<uint8_t> method,
                                 crdtp::span<uint8_t> message,
                                 std::string_view fallthrough_data) {
        cb.Run(message);
      });
  // content::DevToolsSession has already parsed this message.
  CHECK(dispatchable.ok());
  if (HandleOurs(dispatchable)) {
    return;
  }
  dispatcher_.Dispatch(dispatchable);
}

// Our own commands, ProjectB.*, are Go's: the method and its params as JSON cross, and
// Go's answer is the reply's body. One Go doesn't answer falls through to content, which
// replies that there is no such method.
bool DevToolsSession::HandleOurs(const crdtp::Dispatchable& dispatchable) {
  const std::string_view method =
      base::as_string_view(base::span(dispatchable.Method()));
  if (!base::StartsWith(method, "ProjectB.")) {
    return false;
  }
  std::string params = "{}";
  if (!dispatchable.Params().empty()) {
    params.clear();
    CHECK(crdtp::json::ConvertCBORToJSON(dispatchable.Params(), &params).ok());
  }
  hooks::Args args;
  args.Str(std::string(method));
  args.Str(params);
  PBValue result{};
  if (!hooks::Ask(PB_EVENT_DEVTOOLS_COMMAND, args, &result)) {
    return false;
  }
  const std::string body = hooks::TakeString(result);
  // The body is a JSON object; the reply is the same object with the command's id first.
  CHECK(body.size() > 2 && body.front() == '{');
  const std::string reply = base::StrCat(
      {"{\"id\":", base::NumberToString(dispatchable.CallId()), ",",
       std::string_view(body).substr(1)});
  std::vector<uint8_t> cbor;
  CHECK(crdtp::json::ConvertJSONToCBOR(crdtp::SpanFrom(reply), &cbor).ok());
  channel_->DispatchProtocolMessageToClient(std::move(cbor));
  return true;
}

void DevToolsSession::SendProtocolResponse(
    int call_id,
    std::unique_ptr<protocol::Serializable> message) {
  channel_->DispatchProtocolMessageToClient(message->Serialize());
}

void DevToolsSession::SendProtocolNotification(
    std::unique_ptr<protocol::Serializable> message) {
  channel_->DispatchProtocolMessageToClient(message->Serialize());
}

void DevToolsSession::FlushProtocolNotifications() {}

protocol::Response DevToolsSession::GetWindowForTarget(
    std::optional<std::string> target_id,
    int* out_window_id,
    std::unique_ptr<protocol::Browser::Bounds>* out_bounds) {
  scoped_refptr<content::DevToolsAgentHost> host =
      content::DevToolsAgentHost::GetForId(target_id.value_or(target_id_));
  if (!host) {
    return protocol::Response::ServerError("No target with given id");
  }
  content::WebContents* web_contents = host->GetWebContents();
  if (!web_contents) {
    return protocol::Response::ServerError("No web contents in the target");
  }
  Window* window = GetWindow(kWindowId);
  if (!window || hooks::TabOf(web_contents) <= 0) {
    return protocol::Response::ServerError("Browser window not found");
  }
  *out_window_id = kWindowId;
  *out_bounds = BoundsOf(window);
  return protocol::Response::Success();
}

protocol::Response DevToolsSession::GetWindowBounds(
    int window_id,
    std::unique_ptr<protocol::Browser::Bounds>* out_bounds) {
  Window* window = GetWindow(window_id);
  if (!window) {
    return protocol::Response::ServerError("Browser window not found");
  }
  *out_bounds = BoundsOf(window);
  return protocol::Response::Success();
}

protocol::Response DevToolsSession::SetWindowBounds(
    int window_id,
    std::unique_ptr<protocol::Browser::Bounds> window_bounds) {
  Window* window = GetWindow(window_id);
  if (!window) {
    return protocol::Response::ServerError("Browser window not found");
  }
  gfx::Rect bounds = window->GetBounds();
  const bool set_bounds = window_bounds->HasLeft() || window_bounds->HasTop() ||
                          window_bounds->HasWidth() ||
                          window_bounds->HasHeight();
  if (set_bounds) {
    bounds.set_x(window_bounds->GetLeft(bounds.x()));
    bounds.set_y(window_bounds->GetTop(bounds.y()));
    bounds.set_width(window_bounds->GetWidth(bounds.width()));
    bounds.set_height(window_bounds->GetHeight(bounds.height()));
  }

  const std::string window_state = window_bounds->GetWindowState("normal");
  if (set_bounds && window_state != "normal") {
    return protocol::Response::InvalidParams(
        "The 'minimized', 'maximized' and 'fullscreen' states cannot be "
        "combined with 'left', 'top', 'width' or 'height'");
  }

  if (window_state == "fullscreen") {
    if (window->IsMinimized()) {
      return protocol::Response::ServerError(
          "To make minimized window fullscreen, "
          "restore it to normal state first.");
    }
    window->SetFullscreen(true);
  } else if (window_state == "maximized") {
    if (window->IsMinimized() || window->IsFullscreen()) {
      return protocol::Response::ServerError(
          "To maximize a minimized or fullscreen "
          "window, restore it to normal state first.");
    }
    window->Maximize();
  } else if (window_state == "minimized") {
    if (window->IsFullscreen()) {
      return protocol::Response::ServerError(
          "To minimize a fullscreen window, restore it to normal "
          "state first.");
    }
    window->Minimize();
  } else if (window_state == "normal") {
    if (window->IsFullscreen()) {
      window->SetFullscreen(false);
    } else if (window->IsMinimized() || window->IsMaximized()) {
      window->Restore();
    } else if (set_bounds) {
      window->SetBounds(bounds);
    }
  } else {
    return protocol::Response::InvalidParams("Invalid window state: " +
                                             window_state);
  }
  return protocol::Response::Success();
}

protocol::Response DevToolsSession::Close() {
  CloseBrowserSoon();
  return protocol::Response::Success();
}

}  // namespace project_b
