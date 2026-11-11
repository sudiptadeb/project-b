// The content hooks //extensions needs its embedder to answer. Each does what Chrome's
// does (chrome/browser/extensions/chrome_content_browser_client_extensions_part.cc),
// for one profile with no guests, apps or policy.

#include "base/command_line.h"
#include "base/functional/bind.h"
#include "content/public/browser/browser_context.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_process_host.h"
#include "content/public/browser/security_principal.h"
#include "content/public/browser/service_worker_version_base_info.h"
#include "content/public/browser/site_instance.h"
#include "content/public/browser/web_contents.h"
#include "extensions/browser/api/web_request/extension_web_request_event_router.h"
#include "extensions/browser/api/web_request/web_request_api.h"
#include "extensions/browser/event_router.h"
#include "extensions/browser/extension_protocols.h"
#include "extensions/browser/extension_registry.h"
#include "extensions/browser/extension_web_contents_observer.h"
#include "extensions/browser/extensions_browser_client.h"
#include "extensions/browser/process_map.h"
#include "extensions/browser/renderer_startup_helper.h"
#include "extensions/browser/service_worker/service_worker_host.h"
#include "extensions/browser/url_loader_factory_manager.h"
#include "extensions/common/constants.h"
#include "extensions/common/manifest_handlers/sandboxed_page_info.h"
#include "extensions/common/mojom/event_router.mojom.h"
#include "extensions/common/mojom/frame.mojom.h"
#include "extensions/common/mojom/renderer_host.mojom.h"
#include "extensions/common/mojom/service_worker_host.mojom.h"
#include "extensions/common/mojom/web_request_host.mojom.h"
#include "extensions/common/switches.h"
#include "project_b/engine/browser/content_browser_client.h"
#include "project_b/engine/common/extensions_on.h"
#include "third_party/blink/public/common/associated_interfaces/associated_interface_registry.h"
#include "url/gurl.h"

namespace project_b {

namespace {

// The enabled extension a site belongs to, or null.
const extensions::Extension* ExtensionFor(
    content::BrowserContext* browser_context,
    const content::SecurityPrincipal& security_principal) {
  if (!security_principal.SchemeIs(extensions::kExtensionScheme)) {
    return nullptr;
  }
  return extensions::ExtensionRegistry::Get(browser_context)
      ->enabled_extensions()
      .GetByID(std::string(security_principal.GetHost()));
}

}  // namespace

// A process that will run an extension's pages is recorded as that extension's, which
// is what lets it use the extension's APIs.
void ExtensionsContentBrowserClient::SiteInstanceGotProcessAndSite(
    content::SiteInstance* site_instance) {
  ContentBrowserClientHooks::SiteInstanceGotProcessAndSite(site_instance);
  content::BrowserContext* context =
      site_instance->GetProcess()->GetBrowserContext();
  const extensions::Extension* extension =
      ExtensionFor(context, site_instance->GetSecurityPrincipal());
  if (!extension || site_instance->GetSecurityPrincipal().IsSandboxed()) {
    return;
  }
  extensions::ProcessMap::Get(context)->Insert(
      extension->id(), site_instance->GetProcess()->GetID());
}

bool ExtensionsContentBrowserClient::DoesSiteRequireDedicatedProcess(
    content::BrowserContext* browser_context,
    const GURL& effective_site_url) {
  return extensions::ExtensionRegistry::Get(browser_context)
             ->enabled_extensions()
             .GetExtensionOrAppByURL(effective_site_url) != nullptr ||
         ContentBrowserClientHooks::DoesSiteRequireDedicatedProcess(
             browser_context, effective_site_url);
}

// All of an extension's pages share one process.
bool ExtensionsContentBrowserClient::ShouldUseProcessPerSite(
    content::BrowserContext* browser_context,
    const content::SecurityPrincipal& security_principal) {
  return ExtensionFor(browser_context, security_principal) != nullptr;
}

// An extension's pages go only in that extension's process, and nothing else does.
bool ExtensionsContentBrowserClient::IsSuitableHost(
    content::RenderProcessHost* process_host,
    const content::SecurityPrincipal& security_principal) {
  content::BrowserContext* context = process_host->GetBrowserContext();
  extensions::ProcessMap* process_map = extensions::ProcessMap::Get(context);
  const extensions::Extension* extension =
      ExtensionFor(context, security_principal);
  if (extension) {
    return process_map->Contains(extension->id(), process_host->GetID());
  }
  return !process_map->Contains(process_host->GetID());
}

bool ExtensionsContentBrowserClient::CanCommitURL(content::RenderProcessHost* process_host,
                                        const GURL& url) {
  content::BrowserContext* context = process_host->GetBrowserContext();
  const extensions::Extension* extension =
      extensions::ExtensionRegistry::Get(context)
          ->enabled_extensions()
          .GetExtensionOrAppByURL(url);
  if (!extension) {
    return ContentBrowserClientHooks::CanCommitURL(process_host, url);
  }
  return extensions::ProcessMap::Get(context)->Contains(
             extension->id(), process_host->GetID()) ||
         extensions::SandboxedPageInfo::IsSandboxedPage(extension,
                                                        url.GetPath());
}

bool ExtensionsContentBrowserClient::IsHandledURL(const GURL& url) {
  return url.SchemeIs(extensions::kExtensionScheme) ||
         ContentBrowserClientHooks::IsHandledURL(url);
}

// An extension's process is told what it is on its command line, so one content
// started ahead of need, for any site, can't become one.
bool ExtensionsContentBrowserClient::ShouldUseSpareRenderProcessHost(
    content::BrowserContext* browser_context,
    const GURL& site_url,
    std::optional<SpareProcessRefusedByEmbedderReason>& refused_reason) {
  if (site_url.SchemeIs(extensions::kExtensionScheme)) {
    refused_reason = SpareProcessRefusedByEmbedderReason::ExtensionProcess;
    return false;
  }
  return ContentBrowserClientHooks::ShouldUseSpareRenderProcessHost(
      browser_context, site_url, refused_reason);
}

// A child process learns at its start that extensions are on, whether it is an
// extension's process, and where the store is.
void ExtensionsContentBrowserClient::AppendExtraCommandLineSwitches(
    base::CommandLine* command_line,
    int child_process_id) {
  // A renderer has to know which origin is the store to give its page the store's API.
  static const char* const kStoreSwitches[] = {
      extensions::switches::kAppsGalleryURL};
  command_line->AppendSwitch(kExtensionsSwitch);
  command_line->CopySwitchesFrom(*base::CommandLine::ForCurrentProcess(),
                                 kStoreSwitches);
  content::RenderProcessHost* process =
      content::RenderProcessHost::FromID(child_process_id);
  if (process &&
      extensions::ProcessMap::Get(process->GetBrowserContext())
          ->GetEnabledExtensionByProcessID(process->GetID())) {
    command_line->AppendSwitch(extensions::switches::kExtensionProcess);
  }
}

void ExtensionsContentBrowserClient::ExposeInterfacesToRenderer(
    service_manager::BinderRegistry* registry,
    blink::AssociatedInterfaceRegistry* associated_registry,
    content::RenderProcessHost* render_process_host) {
  associated_registry->AddInterface<extensions::mojom::RendererHost>(
      base::BindRepeating(&extensions::RendererStartupHelper::BindForRenderer,
                          render_process_host->GetID()));
  associated_registry->AddInterface<extensions::mojom::WebRequestHost>(
      base::BindRepeating(&extensions::WebRequestEventRouter::BindForRenderer,
                          render_process_host->GetID()));
}

void ExtensionsContentBrowserClient::RegisterAssociatedInterfaceBindersForRenderFrameHost(
    content::RenderFrameHost& render_frame_host,
    blink::AssociatedInterfaceRegistry& associated_registry) {
  ContentBrowserClientHooks::
      RegisterAssociatedInterfaceBindersForRenderFrameHost(render_frame_host,
                                                           associated_registry);
  const content::ChildProcessId process =
      render_frame_host.GetProcess()->GetID();
  associated_registry.AddInterface<extensions::mojom::RendererHost>(
      base::BindRepeating(&extensions::RendererStartupHelper::BindForRenderer,
                          process));
  associated_registry.AddInterface<extensions::mojom::EventRouter>(
      base::BindRepeating(&extensions::EventRouter::BindForRenderer, process));
  associated_registry.AddInterface<extensions::mojom::LocalFrameHost>(
      base::BindRepeating(
          [](content::RenderFrameHost* render_frame_host,
             mojo::PendingAssociatedReceiver<extensions::mojom::LocalFrameHost>
                 receiver) {
            extensions::ExtensionWebContentsObserver::BindLocalFrameHost(
                std::move(receiver), render_frame_host);
          },
          &render_frame_host));
}

void ExtensionsContentBrowserClient::RegisterAssociatedInterfaceBindersForServiceWorker(
    const content::ServiceWorkerVersionBaseInfo& service_worker_version_info,
    blink::AssociatedInterfaceRegistry& associated_registry) {
  const content::ChildProcessId process =
      service_worker_version_info.process_id;
  associated_registry.AddInterface<extensions::mojom::RendererHost>(
      base::BindRepeating(&extensions::RendererStartupHelper::BindForRenderer,
                          process));
  associated_registry.AddInterface<extensions::mojom::ServiceWorkerHost>(
      base::BindRepeating(&extensions::ServiceWorkerHost::BindReceiver,
                          process));
  associated_registry.AddInterface<extensions::mojom::WebRequestHost>(
      base::BindRepeating(&extensions::WebRequestEventRouter::BindForRenderer,
                          process));
  associated_registry.AddInterface<extensions::mojom::EventRouter>(
      base::BindRepeating(&extensions::EventRouter::BindForRenderer, process));
}

// The interfaces an extension's own frames get, beyond what a web page has.
void ExtensionsContentBrowserClient::RegisterBrowserInterfaceBindersForFrame(
    content::RenderFrameHost* render_frame_host,
    mojo::BinderMapWithContext<content::RenderFrameHost*>* map) {
  const extensions::Extension* extension = ExtensionFor(
      render_frame_host->GetBrowserContext(),
      render_frame_host->GetSiteInstance()->GetSecurityPrincipal());
  if (extension) {
    extensions::ExtensionsBrowserClient::Get()
        ->RegisterBrowserInterfaceBindersForFrame(map, render_frame_host,
                                                  extension);
  }
}

// chrome-extension: URLs are served from the extensions' folders, by //extensions: a
// factory each for a navigation, a worker's script, a service worker's update and a
// page's own requests.
mojo::PendingRemote<network::mojom::URLLoaderFactory>
ExtensionsContentBrowserClient::CreateNonNetworkNavigationURLLoaderFactory(
    const std::string& scheme,
    content::FrameTreeNodeId frame_tree_node_id) {
  if (scheme != extensions::kExtensionScheme) {
    return {};
  }
  return extensions::CreateExtensionNavigationURLLoaderFactory(
      content::WebContents::FromFrameTreeNodeId(frame_tree_node_id)
          ->GetBrowserContext(),
      false);
}

void ExtensionsContentBrowserClient::RegisterNonNetworkWorkerMainResourceURLLoaderFactories(
    content::BrowserContext* browser_context,
    const std::optional<url::Origin>& request_initiator,
    network::mojom::RequestDestination request_destination,
    NonNetworkURLLoaderFactoryMap* factories) {
  factories->emplace(
      extensions::kExtensionScheme,
      extensions::CreateExtensionWorkerMainResourceURLLoaderFactory(
          browser_context, request_initiator));
}

void ExtensionsContentBrowserClient::
    RegisterNonNetworkServiceWorkerUpdateURLLoaderFactories(
        content::BrowserContext* browser_context,
        NonNetworkURLLoaderFactoryMap* factories) {
  factories->emplace(
      extensions::kExtensionScheme,
      extensions::CreateExtensionServiceWorkerScriptURLLoaderFactory(
          browser_context));
}

void ExtensionsContentBrowserClient::RegisterNonNetworkSubresourceURLLoaderFactories(
    int render_process_id,
    int render_frame_id,
    const std::optional<url::Origin>& request_initiator_origin,
    NonNetworkURLLoaderFactoryMap* factories) {
  factories->emplace(
      extensions::kExtensionScheme,
      extensions::CreateExtensionURLLoaderFactory(
          content::ChildProcessId::FromUnsafeValue(render_process_id),
          render_frame_id));
}

// webRequest and declarativeNetRequest sit in front of the network here: each new
// loader factory is wrapped when an extension has rules or listeners for it.
void ExtensionsContentBrowserClient::WillCreateURLLoaderFactory(
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
    bool is_for_network_service) {
  const bool proxied =
      extensions::BrowserContextKeyedAPIFactory<extensions::WebRequestAPI>::Get(
          browser_context)
          ->MaybeProxyURLLoaderFactory(
              browser_context, frame, render_process_id, type,
              std::move(navigation_id), ukm_source_id, factory_builder,
              header_client, navigation_response_task_runner,
              request_initiator);
  if (bypass_redirect_checks) {
    *bypass_redirect_checks = proxied;
  }
}

// What an extension's own requests, and its content scripts', may reach across origins.
void ExtensionsContentBrowserClient::OverrideURLLoaderFactoryParams(
    content::BrowserContext* browser_context,
    const url::Origin& origin,
    bool is_for_isolated_world,
    bool is_for_service_worker,
    network::mojom::URLLoaderFactoryParams* factory_params) {
  extensions::URLLoaderFactoryManager::OverrideURLLoaderFactoryParams(
      browser_context, origin, is_for_isolated_world, is_for_service_worker,
      factory_params);
}

}  // namespace project_b
