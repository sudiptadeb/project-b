#include "project_b/engine/browser/content_browser_client.h"

#include "base/check.h"
#include "base/files/file_path.h"
#include "base/path_service.h"
#include "components/embedder_support/user_agent_utils.h"
#include "components/version_info/version_info_with_user_agent.h"
#include "content/public/browser/browser_context.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/navigation_throttle_registry.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/service_worker_version_base_info.h"
#include "content/public/browser/certificate_request_result_type.h"
#include "content/public/browser/web_contents.h"
#include "mojo/public/cpp/bindings/binder_map.h"
#include "net/base/net_errors.h"
#include "net/http/http_request_headers.h"
#include "services/network/public/cpp/resource_request.h"
#include "services/network/public/mojom/websocket.mojom.h"
#include "third_party/blink/public/common/loader/url_loader_throttle.h"
#include "project_b/engine/browser/printing.h"
#include "project_b/engine/browser/site_features.h"
#include "project_b/engine/common/block_rules.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/browser_main_parts.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/events.h"
#include "services/network/public/mojom/network_context.mojom.h"
#include "third_party/blink/public/mojom/badging/badging.mojom.h"
#include "third_party/blink/public/mojom/persistent_renderer_prefs.mojom.h"
#include "third_party/blink/public/common/web_preferences/web_preferences.h"
#include "ui/native_theme/native_theme.h"

#if BUILDFLAG(IS_ANDROID)
#include "base/posix/global_descriptors.h"
#include "content/public/browser/posix_file_descriptor_info.h"
#include "project_b/engine/android/descriptors.h"
#else
#include "base/no_destructor.h"
#include "project_b/engine/browser/devtools.h"
#include "project_b/engine/browser/passkeys.h"
#endif
#include "content/public/browser/login_delegate.h"
#include "net/base/auth.h"
#include "net/http/http_response_headers.h"
#include "project_b/engine/browser/dialogs.h"

#if BUILDFLAG(IS_MAC)
#include "base/apple/bundle_locations.h"
#include "base/apple/foundation_util.h"
#endif

namespace project_b {

ContentBrowserClient::ContentBrowserClient() = default;
ContentBrowserClient::~ContentBrowserClient() = default;

std::unique_ptr<content::BrowserMainParts>
ContentBrowserClient::CreateBrowserMainParts(bool is_integration_test) {
  return std::make_unique<BrowserMainParts>();
}

#if BUILDFLAG(IS_ANDROID)
// A renderer can't open the APK, so the pak goes along as an open file.
void ContentBrowserClient::GetAdditionalMappedFilesForChildProcess(
    const base::CommandLine& command_line,
    int child_process_id,
    content::PosixFileDescriptorInfo* mappings) {
  auto* descriptors = base::GlobalDescriptors::GetInstance();
  mappings->ShareWithRegion(kPakDescriptor, descriptors->Get(kPakDescriptor),
                            descriptors->GetRegion(kPakDescriptor));
}
#else
// Every child process runs the helper, which has no Go in it.
// The helper beside the main binary in a build, and in an app of its own in the packed app:
// one whose Info.plist says LSUIElement, so no child process ever shows in the Dock, as a
// capture service that touches the camera would otherwise.
base::FilePath ContentBrowserClient::GetChildProcessPath(int flags) {
#if BUILDFLAG(IS_LINUX)
  // Linux has one binary: a child is this one with --type, and never loads Go (app/main_linux.cc).
  base::FilePath exe;
  CHECK(base::PathService::Get(base::FILE_EXE, &exe));
  return exe;
#else
#if BUILDFLAG(IS_MAC)
  if (base::apple::AmIBundled()) {
    return base::apple::OuterBundlePath()
        .Append(FILE_PATH_LITERAL("Contents/Frameworks/Project B Helper.app"))
        .Append(FILE_PATH_LITERAL("Contents/MacOS/project_b_helper"));
  }
#endif
  base::FilePath dir;
  CHECK(base::PathService::Get(base::DIR_EXE, &dir));
  return dir.Append(FILE_PATH_LITERAL("project_b_helper"));
#endif
}

std::unique_ptr<content::DevToolsManagerDelegate>
ContentBrowserClient::CreateDevToolsManagerDelegate() {
  return std::make_unique<DevToolsManagerDelegate>();
}
#endif

void ContentBrowserClient::AppendExtraCommandLineSwitches(
    base::CommandLine* command_line,
    int child_process_id) {
  block_rules::AppendTo(command_line);
}

void ContentBrowserClient::ExposeInterfacesToRenderer(
    service_manager::BinderRegistry* registry,
    blink::AssociatedInterfaceRegistry* associated_registry,
    content::RenderProcessHost* render_process_host) {
  site_features::ExposeTo(associated_registry, render_process_host);
}

void ContentBrowserClient::RegisterAssociatedInterfaceBindersForRenderFrameHost(
    content::RenderFrameHost& render_frame_host,
    blink::AssociatedInterfaceRegistry& associated_registry) {
  ContentBrowserClientHooks::RegisterAssociatedInterfaceBindersForRenderFrameHost(
      render_frame_host, associated_registry);
  printing::BindFrame(render_frame_host, associated_registry);
}

void ContentBrowserClient::RenderProcessWillLaunch(
    content::RenderProcessHost* host) {
  site_features::RendererLaunching(host);
  ContentBrowserClientHooks::RenderProcessWillLaunch(host);
}

bool ContentBrowserClient::WillInterceptWebSocket(
    content::RenderFrameHost* frame) {
  return block_rules::Any();
}

// One to a blocked host is dropped, which the page sees as a failed connection; any
// other opens as content would have opened it.
void ContentBrowserClient::CreateWebSocket(
    content::RenderFrameHost* frame,
    WebSocketFactory factory,
    const GURL& url,
    const net::SiteForCookies& site_for_cookies,
    const std::optional<std::string>& user_agent,
    mojo::PendingRemote<network::mojom::WebSocketHandshakeClient>
        handshake_client,
    WebSocketOptions options) {
  if (block_rules::Blocks(url)) {
    return;
  }
  std::vector<network::mojom::HttpHeaderPtr> headers;
  if (user_agent) {
    headers.push_back(network::mojom::HttpHeader::New(
        net::HttpRequestHeaders::kUserAgent, *user_agent));
  }
  std::move(factory).Run(url, std::move(headers), std::move(handshake_client),
                         mojo::NullRemote(), std::move(options.header_client));
}

// Every navigation gets the gate, which asks Go about a tab's top-level ones, and the
// generated throttle, whose hooks cross to Go only when something subscribed to them.
std::vector<std::unique_ptr<blink::URLLoaderThrottle>>
ContentBrowserClient::CreateURLLoaderThrottles(
    const network::ResourceRequest& request,
    content::BrowserContext* browser_context,
    const base::RepeatingCallback<content::WebContents*()>& wc_getter,
    content::NavigationUIData* navigation_ui_data,
    content::FrameTreeNodeId frame_tree_node_id,
    std::optional<int64_t> navigation_id) {
  std::vector<std::unique_ptr<blink::URLLoaderThrottle>> throttles;
  if (navigation_id) {
    if (auto easing = site_features::EasePoliciesFor(request, wc_getter)) {
      throttles.push_back(std::move(easing));
    }
  }
  return throttles;
}

void ContentBrowserClient::CreateThrottlesForNavigation(
    content::NavigationThrottleRegistry& registry) {
  registry.AddThrottle(std::make_unique<events::Gate>(registry));
  registry.AddThrottle(std::make_unique<NavigationThrottleHooks>(registry));
}

// Content's certificate errors as Go names them; the rest are "invalid".
void ContentBrowserClient::AllowCertificateError(
    content::WebContents* web_contents,
    int cert_error,
    const net::SSLInfo& ssl_info,
    const GURL& request_url,
    bool is_primary_main_frame_request,
    bool strict_enforcement,
    base::OnceCallback<void(content::CertificateRequestResultType)> callback) {
  std::string reason = "invalid";
  switch (cert_error) {
    case net::ERR_CERT_DATE_INVALID:
      reason = "expired";
      break;
    case net::ERR_CERT_AUTHORITY_INVALID:
      reason = "untrusted";
      break;
    case net::ERR_CERT_COMMON_NAME_INVALID:
      reason = "wrong-host";
      break;
    case net::ERR_CERT_REVOKED:
      reason = "revoked";
      break;
  }
  hooks::Args args;
  args.Int(web_contents ? hooks::TabOf(web_contents) : 0);
  args.Str(request_url.spec());
  args.Str(reason);
  args.Bool(is_primary_main_frame_request);
  // A revoked certificate, and a site that demands a valid one, can't be passed.
  args.Bool(!strict_enforcement && cert_error != net::ERR_CERT_REVOKED);
  PBValue allow;
  bool go = hooks::Ask(PB_EVENT_CERTIFICATE_ERROR, args, &allow) && allow.i;
  std::move(callback).Run(
      go ? content::CERTIFICATE_REQUEST_RESULT_TYPE_CONTINUE
         : content::CERTIFICATE_REQUEST_RESULT_TYPE_DENY);
}

// What Chrome reports, "Chrome/155.0.8059.41": DevTools' /json/version and
// Browser.getVersion show it, and ChromeDriver reads the browser's version from it. The
// user agent pages see comes from GetUserAgent, not from this.
std::unique_ptr<content::LoginDelegate> ContentBrowserClient::CreateLoginDelegate(
    const net::AuthChallengeInfo& auth_info,
    content::WebContents* web_contents,
    content::BrowserContext* browser_context,
    const content::GlobalRequestID& request_id,
    bool is_request_for_primary_main_frame_navigation,
    bool is_request_for_navigation,
    const GURL& url,
    scoped_refptr<net::HttpResponseHeaders> response_headers,
    bool first_auth_attempt,
    content::GuestPageHolder* guest_page_holder,
    content::LoginDelegate::LoginAuthRequiredCallback auth_required_callback) {
  return Dialogs::Get()->SignIn(web_contents, auth_info, std::move(auth_required_callback));
}

#if !BUILDFLAG(IS_ANDROID)
content::WebAuthenticationDelegate*
ContentBrowserClient::GetWebAuthenticationDelegate() {
  static base::NoDestructor<PasskeyPolicy> policy;
  return policy.get();
}

std::unique_ptr<content::AuthenticatorRequestClientDelegate>
ContentBrowserClient::GetWebAuthenticationRequestDelegate(
    content::RenderFrameHost* render_frame_host) {
  return std::make_unique<PasskeyRequest>(render_frame_host);
}
#endif

std::string ContentBrowserClient::GetProduct() {
  return std::string(version_info::GetProductNameAndVersionForUserAgent());
}

std::string ContentBrowserClient::GetUserAgent() {
  return embedder_support::GetUserAgent();
}

// The client hints a page reads, navigator.userAgentData and the Sec-CH-UA headers: Chrome's,
// with its brands, platform and version. Without this, content's default is empty, and sites
// that check them, Google's sign-in among them, took the app for an unsafe browser.
blink::UserAgentMetadata ContentBrowserClient::GetUserAgentMetadata() {
  return embedder_support::GetUserAgentMetadata();
}

// Cookies and the HTTP cache are saved in the partition's folder, as Chrome lays a
// profile out: chromium/Network/Cookies and chromium/Cache. Cookies are stored as
// Chromium writes them, unencrypted, with no OS keychain. Session cookies are not kept
// across a quit, Chromium's default. An in-memory partition gets no files. DevTools
// lays its own proxy over these params after this.
void ContentBrowserClient::ConfigureNetworkContextParams(
    content::BrowserContext* context,
    bool in_memory,
    const base::FilePath& relative_partition_path,
    network::mojom::NetworkContextParams* network_context_params,
    cert_verifier::mojom::CertVerifierCreationParams*
        cert_verifier_creation_params) {
  ContentBrowserClientHooks::ConfigureNetworkContextParams(
      context, in_memory, relative_partition_path, network_context_params,
      cert_verifier_creation_params);
  network_context_params->enable_encrypted_cookies = false;
  if (in_memory) {
    return;
  }
  base::FilePath path = context->GetPath().Append(relative_partition_path);
  auto& file_paths = network_context_params->file_paths;
  file_paths = network::mojom::NetworkContextFilePaths::New();
  file_paths->data_directory = path.Append(FILE_PATH_LITERAL("Network"));
  file_paths->cookie_database_name =
      base::FilePath(FILE_PATH_LITERAL("Cookies"));
  file_paths->http_server_properties_file_name =
      base::FilePath(FILE_PATH_LITERAL("Network Persistent State"));
  file_paths->http_cache_directory = path.Append(FILE_PATH_LITERAL("Cache"));
}

// The sandboxed network process may only write under these folders. It starts before a
// copy that picks its profile has one, so it may write under the copy's folder, which
// holds every profile it can run on.
std::vector<base::FilePath>
ContentBrowserClient::GetNetworkContextsParentDirectory() {
  return {CopyDir()};
}

// A binder that takes the receiver and drops it: the call goes nowhere, and the page that
// made it keeps its renderer, where an interface with no binder at all gets it killed.
template <typename Context, typename Interface>
void DropReceiver(Context, mojo::PendingReceiver<Interface>) {}

void ContentBrowserClient::RegisterBrowserInterfaceBindersForFrame(
    content::RenderFrameHost* render_frame_host,
    mojo::BinderMapWithContext<content::RenderFrameHost*>* map) {
  // navigator.setAppBadge, which chat and mail pages call. It has no feature to turn off.
  map->Add<blink::mojom::BadgeService>(
      &DropReceiver<content::RenderFrameHost*, blink::mojom::BadgeService>);
  // The line wrap box on a view-source: page.
  map->Add<blink::mojom::PersistentRendererPrefsService>(
      &DropReceiver<content::RenderFrameHost*,
                    blink::mojom::PersistentRendererPrefsService>);
}

void ContentBrowserClient::RegisterBrowserInterfaceBindersForServiceWorker(
    content::BrowserContext* browser_context,
    const content::ServiceWorkerVersionBaseInfo& service_worker_version_info,
    mojo::BinderMapWithContext<const content::ServiceWorkerVersionBaseInfo&>* map) {
  // navigator.setAppBadge from a push handler.
  map->Add<blink::mojom::BadgeService>(
      &DropReceiver<const content::ServiceWorkerVersionBaseInfo&,
                    blink::mojom::BadgeService>);
}

namespace {

// The colour scheme pages prefer: the app's theme when it is light or dark, and as the
// system is otherwise, which the web's native theme follows: on the Mac its appearance,
// on Android the phone's dark mode (BrowserWindow.applyTheme).
blink::mojom::PreferredColorScheme PagesColorScheme() {
  int theme = Engine::Get() ? Engine::Get()->theme() : 0;
  bool dark = theme == 2 ||
              (theme == 0 && ui::NativeTheme::GetInstanceForWeb()->preferred_color_scheme() ==
                                 ui::NativeTheme::PreferredColorScheme::kDark);
  return dark ? blink::mojom::PreferredColorScheme::kDark
              : blink::mojom::PreferredColorScheme::kLight;
}

}  // namespace

void ContentBrowserClient::OverrideWebPreferences(
    content::WebContents* web_contents,
    content::SiteInstance& main_frame_site,
    blink::web_pref::WebPreferences* prefs) {
  prefs->preferred_color_scheme = PagesColorScheme();
  prefs->preferred_root_scrollbar_color_scheme = prefs->preferred_color_scheme;
}

bool ContentBrowserClient::WebPreferencesNeedUpdateForColorRelatedStateChanges(
    content::WebContents& web_contents,
    const content::SiteInstance& main_frame_site) const {
  return web_contents.GetOrCreateWebPreferences().preferred_color_scheme !=
         PagesColorScheme();
}

}  // namespace project_b
