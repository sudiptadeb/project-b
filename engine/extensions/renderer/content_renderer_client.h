#ifndef PROJECT_B_ENGINE_EXTENSIONS_RENDERER_CONTENT_RENDERER_CLIENT_H_
#define PROJECT_B_ENGINE_EXTENSIONS_RENDERER_CONTENT_RENDERER_CLIENT_H_

#include <memory>

#include "content/public/renderer/content_renderer_client.h"

namespace extensions {
class ExtensionsRendererClient;
}

namespace project_b {

// Content's hooks in a renderer, handed to //extensions: content scripts, the chrome.*
// bindings, and extension service workers. Nothing else of ours runs in a renderer.
class ContentRendererClient : public content::ContentRendererClient {
 public:
  ContentRendererClient();
  ContentRendererClient(const ContentRendererClient&) = delete;
  ContentRendererClient& operator=(const ContentRendererClient&) = delete;
  ~ContentRendererClient() override;

 private:
  void RenderThreadStarted() override;
  void RenderFrameCreated(content::RenderFrame* render_frame) override;
  void WebViewCreated(blink::WebView* web_view,
                      bool was_created_by_renderer,
                      const url::Origin* outermost_origin) override;
  void RunScriptsAtDocumentStart(content::RenderFrame* render_frame) override;
  void RunScriptsAtDocumentEnd(content::RenderFrame* render_frame) override;
  void RunScriptsAtDocumentIdle(content::RenderFrame* render_frame) override;
  void WillSendRequest(blink::WebLocalFrame* frame,
                       ui::PageTransition transition_type,
                       const blink::WebURL& upstream_url,
                       const blink::WebURL& target_url,
                       const net::SiteForCookies& site_for_cookies,
                       const url::Origin* initiator_origin,
                       GURL* new_url) override;
  bool AllowPopup() override;
  bool ShouldNotifyServiceWorkerOnWebSocketActivity(
      v8::Local<v8::Context> context) override;
  void DidInitializeServiceWorkerContextOnWorkerThread(
      blink::WebServiceWorkerContextProxy* context_proxy,
      const GURL& service_worker_scope,
      const GURL& script_url) override;
  void WillEvaluateServiceWorkerOnWorkerThread(
      blink::WebServiceWorkerContextProxy* context_proxy,
      v8::Local<v8::Context> v8_context,
      int64_t service_worker_version_id,
      const GURL& service_worker_scope,
      const GURL& script_url,
      const blink::ServiceWorkerToken& service_worker_token) override;
  void DidStartServiceWorkerContextOnWorkerThread(
      int64_t service_worker_version_id,
      const GURL& service_worker_scope,
      const GURL& script_url,
      const blink::ServiceWorkerToken& service_worker_token) override;
  void WillDestroyServiceWorkerContextOnWorkerThread(
      v8::Local<v8::Context> context,
      int64_t service_worker_version_id,
      const GURL& service_worker_scope,
      const GURL& script_url,
      const blink::ServiceWorkerToken& service_worker_token) override;

  std::unique_ptr<extensions::ExtensionsRendererClient> extensions_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_RENDERER_CONTENT_RENDERER_CLIENT_H_
