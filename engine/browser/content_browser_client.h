#ifndef PROJECT_B_ENGINE_BROWSER_CONTENT_BROWSER_CLIENT_H_
#define PROJECT_B_ENGINE_BROWSER_CONTENT_BROWSER_CLIENT_H_

#include <memory>
#include <string>
#include <vector>

#include "build/build_config.h"
#include "project_b/engine/browser/hooks_gen.h"
#include "project_b/engine/buildflags/buildflags.h"

namespace project_b {

// The general counter: content asks this object questions. The generated base carries
// them to Go; what is answered here needs content's objects or runs before Go starts.
class ContentBrowserClient : public ContentBrowserClientHooks {
 public:
  ContentBrowserClient();
  ContentBrowserClient(const ContentBrowserClient&) = delete;
  ContentBrowserClient& operator=(const ContentBrowserClient&) = delete;
  ~ContentBrowserClient() override;

 private:
  std::unique_ptr<content::BrowserMainParts> CreateBrowserMainParts(
      bool is_integration_test) override;
#if BUILDFLAG(IS_ANDROID)
  void GetAdditionalMappedFilesForChildProcess(
      const base::CommandLine& command_line,
      int child_process_id,
      content::PosixFileDescriptorInfo* mappings) override;
#else
  base::FilePath GetChildProcessPath(int flags) override;
  std::unique_ptr<content::DevToolsManagerDelegate>
  CreateDevToolsManagerDelegate() override;
#endif
  void CreateThrottlesForNavigation(
      content::NavigationThrottleRegistry& registry) override;
  std::string GetProduct() override;
  std::string GetUserAgent() override;
  std::vector<base::FilePath> GetNetworkContextsParentDirectory() override;

};

#if BUILDFLAG(PB_EXTENSIONS)
// The counter when the Extensions flag is on (ExtensionsOn()): it adds what //extensions
// needs from content's embedder: an extension's pages in a process of their own, the
// chrome-extension: scheme, its Mojo interfaces, and its place in front of the network
// for the request APIs. All in engine/extensions/browser/content_hooks.cc. With the flag
// off the plain counter is used and none of it runs.
class ExtensionsContentBrowserClient : public ContentBrowserClient {
 private:
  void SiteInstanceGotProcessAndSite(
      content::SiteInstance* site_instance) override;
  bool DoesSiteRequireDedicatedProcess(content::BrowserContext* browser_context,
                                       const GURL& effective_site_url) override;
  bool ShouldUseProcessPerSite(
      content::BrowserContext* browser_context,
      const content::SecurityPrincipal& security_principal) override;
  bool IsSuitableHost(
      content::RenderProcessHost* process_host,
      const content::SecurityPrincipal& security_principal) override;
  bool CanCommitURL(content::RenderProcessHost* process_host,
                    const GURL& url) override;
  bool IsHandledURL(const GURL& url) override;
  bool ShouldUseSpareRenderProcessHost(
      content::BrowserContext* browser_context,
      const GURL& site_url,
      std::optional<SpareProcessRefusedByEmbedderReason>& refused_reason)
      override;
  void AppendExtraCommandLineSwitches(base::CommandLine* command_line,
                                      int child_process_id) override;
  void ExposeInterfacesToRenderer(
      service_manager::BinderRegistry* registry,
      blink::AssociatedInterfaceRegistry* associated_registry,
      content::RenderProcessHost* render_process_host) override;
  void RegisterAssociatedInterfaceBindersForRenderFrameHost(
      content::RenderFrameHost& render_frame_host,
      blink::AssociatedInterfaceRegistry& associated_registry) override;
  void RegisterAssociatedInterfaceBindersForServiceWorker(
      const content::ServiceWorkerVersionBaseInfo& service_worker_version_info,
      blink::AssociatedInterfaceRegistry& associated_registry) override;
  void RegisterBrowserInterfaceBindersForFrame(
      content::RenderFrameHost* render_frame_host,
      mojo::BinderMapWithContext<content::RenderFrameHost*>* map) override;
  mojo::PendingRemote<network::mojom::URLLoaderFactory>
  CreateNonNetworkNavigationURLLoaderFactory(
      const std::string& scheme,
      content::FrameTreeNodeId frame_tree_node_id) override;
  void RegisterNonNetworkWorkerMainResourceURLLoaderFactories(
      content::BrowserContext* browser_context,
      const std::optional<url::Origin>& request_initiator,
      network::mojom::RequestDestination request_destination,
      NonNetworkURLLoaderFactoryMap* factories) override;
  void RegisterNonNetworkServiceWorkerUpdateURLLoaderFactories(
      content::BrowserContext* browser_context,
      NonNetworkURLLoaderFactoryMap* factories) override;
  void RegisterNonNetworkSubresourceURLLoaderFactories(
      int render_process_id,
      int render_frame_id,
      const std::optional<url::Origin>& request_initiator_origin,
      NonNetworkURLLoaderFactoryMap* factories) override;
  void WillCreateURLLoaderFactory(
      content::BrowserContext* browser_context,
      content::RenderFrameHost* frame,
      int render_process_id,
      URLLoaderFactoryType type,
      const url::Origin& request_initiator,
      const net::IsolationInfo& isolation_info,
      std::optional<int64_t> navigation_id,
      ukm::SourceIdObj ukm_source_id,
      network::URLLoaderFactoryBuilder& factory_builder,
      mojo::PendingRemote<network::mojom::TrustedURLLoaderHeaderClient>*
          header_client,
      bool* bypass_redirect_checks,
      bool* disable_secure_dns,
      network::mojom::URLLoaderFactoryOverridePtr* factory_override,
      scoped_refptr<base::SequencedTaskRunner> navigation_response_task_runner,
      bool is_for_network_service) override;
  void OverrideURLLoaderFactoryParams(
      content::BrowserContext* browser_context,
      const url::Origin& origin,
      bool is_for_isolated_world,
      bool is_for_service_worker,
      network::mojom::URLLoaderFactoryParams* factory_params) override;
};
#endif


}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_CONTENT_BROWSER_CLIENT_H_
