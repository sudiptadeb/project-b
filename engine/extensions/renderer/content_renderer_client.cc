#include "project_b/engine/extensions/renderer/content_renderer_client.h"

#include "content/public/common/isolated_world_ids.h"
#include "content/public/renderer/render_frame.h"
#include "content/public/renderer/render_frame_observer.h"
#include "extensions/common/constants.h"
#include "extensions/renderer/api/core_extensions_renderer_api_provider.h"
#include "extensions/renderer/dispatcher.h"
#include "extensions/renderer/extensions_renderer_client.h"
#include "services/service_manager/public/cpp/binder_registry.h"
#include "third_party/blink/public/platform/web_string.h"
#include "third_party/blink/public/web/web_security_policy.h"

namespace project_b {

namespace {

class RendererClient : public extensions::ExtensionsRendererClient {
 public:
  RendererClient() { extensions::ExtensionsRendererClient::Set(this); }
  ~RendererClient() override {
    extensions::ExtensionsRendererClient::Set(nullptr);
  }

 private:
  bool IsIncognitoProcess() const override { return false; }
  // Content scripts get the isolated worlds from the first one content leaves free.
  int GetLowestIsolatedWorldId() const override {
    return content::ISOLATED_WORLD_ID_CONTENT_END + 1;
  }
};

// The Mojo interfaces //extensions offers the browser from a frame. It adds them to
// this registry when the frame is made, and content asks here when the browser binds
// one. It lives and dies with its frame.
class FrameInterfaces : public content::RenderFrameObserver {
 public:
  explicit FrameInterfaces(content::RenderFrame* render_frame)
      : content::RenderFrameObserver(render_frame) {}

  service_manager::BinderRegistry* registry() { return &registry_; }

 private:
  void OnInterfaceRequestForFrame(
      const std::string& interface_name,
      mojo::ScopedMessagePipeHandle* interface_pipe) override {
    registry_.TryBindInterface(interface_name, interface_pipe);
  }
  void OnDestruct() override { delete this; }

  service_manager::BinderRegistry registry_;
};

}  // namespace

ContentRendererClient::ContentRendererClient() = default;
ContentRendererClient::~ContentRendererClient() = default;

void ContentRendererClient::RenderThreadStarted() {
  extensions_ = std::make_unique<RendererClient>();
  extensions_->AddAPIProvider(
      std::make_unique<extensions::CoreExtensionsRendererAPIProvider>());
  extensions_->RenderThreadStarted();
  blink::WebSecurityPolicy::RegisterURLSchemeAsExtension(
      blink::WebString::FromAscii(extensions::kExtensionScheme));
  blink::WebSecurityPolicy::RegisterURLSchemeAsCodeCacheWithHashing(
      blink::WebString::FromAscii(extensions::kExtensionScheme));
}

void ContentRendererClient::RenderFrameCreated(
    content::RenderFrame* render_frame) {
  extensions_->RenderFrameCreated(
      render_frame, (new FrameInterfaces(render_frame))->registry());
}

void ContentRendererClient::WebViewCreated(blink::WebView* web_view,
                                           bool was_created_by_renderer,
                                           const url::Origin* outermost_origin) {
  extensions_->WebViewCreated(web_view, outermost_origin);
}

void ContentRendererClient::RunScriptsAtDocumentStart(
    content::RenderFrame* render_frame) {
  extensions_->RunScriptsAtDocumentStart(render_frame);
}

void ContentRendererClient::RunScriptsAtDocumentEnd(
    content::RenderFrame* render_frame) {
  extensions_->RunScriptsAtDocumentEnd(render_frame);
}

void ContentRendererClient::RunScriptsAtDocumentIdle(
    content::RenderFrame* render_frame) {
  extensions_->RunScriptsAtDocumentIdle(render_frame);
}

void ContentRendererClient::WillSendRequest(
    blink::WebLocalFrame* frame,
    ui::PageTransition transition_type,
    const blink::WebURL& upstream_url,
    const blink::WebURL& target_url,
    const net::SiteForCookies& site_for_cookies,
    const url::Origin* initiator_origin,
    GURL* new_url) {
  extensions_->WillSendRequest(frame, transition_type, upstream_url, target_url,
                               site_for_cookies, initiator_origin, new_url);
}

bool ContentRendererClient::AllowPopup() {
  return extensions_->AllowPopup();
}

bool ContentRendererClient::ShouldNotifyServiceWorkerOnWebSocketActivity(
    v8::Local<v8::Context> context) {
  return extensions::Dispatcher::ShouldNotifyServiceWorkerOnWebSocketActivity(
      context);
}

void ContentRendererClient::DidInitializeServiceWorkerContextOnWorkerThread(
    blink::WebServiceWorkerContextProxy* context_proxy,
    const GURL& service_worker_scope,
    const GURL& script_url) {
  extensions_->dispatcher()->DidInitializeServiceWorkerContextOnWorkerThread(
      context_proxy, service_worker_scope, script_url);
}

void ContentRendererClient::WillEvaluateServiceWorkerOnWorkerThread(
    blink::WebServiceWorkerContextProxy* context_proxy,
    v8::Local<v8::Context> v8_context,
    int64_t service_worker_version_id,
    const GURL& service_worker_scope,
    const GURL& script_url,
    const blink::ServiceWorkerToken& service_worker_token) {
  extensions_->dispatcher()->WillEvaluateServiceWorkerOnWorkerThread(
      context_proxy, v8_context, service_worker_version_id,
      service_worker_scope, script_url, service_worker_token);
}

void ContentRendererClient::DidStartServiceWorkerContextOnWorkerThread(
    int64_t service_worker_version_id,
    const GURL& service_worker_scope,
    const GURL& script_url,
    const blink::ServiceWorkerToken& service_worker_token) {
  extensions_->dispatcher()->DidStartServiceWorkerContextOnWorkerThread(
      service_worker_version_id, service_worker_scope, script_url,
      service_worker_token);
}

void ContentRendererClient::WillDestroyServiceWorkerContextOnWorkerThread(
    v8::Local<v8::Context> context,
    int64_t service_worker_version_id,
    const GURL& service_worker_scope,
    const GURL& script_url,
    const blink::ServiceWorkerToken& service_worker_token) {
  extensions_->dispatcher()->WillDestroyServiceWorkerContextOnWorkerThread(
      context, service_worker_version_id, service_worker_scope, script_url,
      service_worker_token);
}

}  // namespace project_b
