#include "project_b/engine/extensions/browser/extensions_browser_client.h"

#include <utility>

#include "base/containers/span.h"
#include "base/functional/bind.h"
#include "base/notreached.h"
#include "build/build_config.h"
#include "components/image_fetcher/core/image_decoder.h"
#include "components/version_info/channel.h"
#include "content/public/browser/browser_context.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/web_contents.h"
#include "content/public/browser/web_contents_delegate.h"
#include "extensions/browser/api/core_extensions_browser_api_provider.h"
#include "extensions/browser/api/extensions_api_client.h"
#include "extensions/browser/api/runtime/runtime_api_delegate.h"
#include "extensions/browser/crx_installer.h"
#include "extensions/browser/event_router.h"
#include "extensions/browser/extension_host_delegate.h"
#include "extensions/browser/extension_install_prompt_client.h"
#include "extensions/browser/extension_management_client.h"
#include "extensions/browser/extension_registrar.h"
#include "extensions/browser/extensions_browser_interface_binders.h"
#include "extensions/browser/install_prompt_data.h"
#include "extensions/browser/kiosk/kiosk_delegate.h"
#include "extensions/browser/managed_installation_mode.h"
#include "extensions/browser/process_manager_delegate.h"
#include "extensions/browser/safe_browsing_delegate.h"
#include "extensions/browser/scoped_extension_keep_alive.h"
#include "extensions/browser/updater/null_extension_cache.h"
#include "extensions/browser/url_request_util.h"
#include "extensions/browser/webstore_installer.h"
#include "extensions/common/api/runtime.h"
#include "extensions/common/features/feature_channel.h"
#include "extensions/common/manifest_handlers/options_page_info.h"
#include "extensions/common/url_pattern_set.h"
#include "net/http/http_response_headers.h"
#include "project_b/engine/browser/browser_main_parts.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/extensions/browser/api_client.h"
#include "project_b/engine/extensions/browser/extension_system.h"
#include "project_b/engine/extensions/browser/extensions.h"
#include "project_b/engine/extensions/browser/go_function.h"
#include "project_b/engine/extensions/browser/web_contents_observer.h"
#include "services/data_decoder/public/cpp/decode_image.h"
#include "services/network/public/mojom/url_loader.mojom.h"
#include "services/network/public/mojom/url_loader_factory.mojom.h"
#include "third_party/blink/public/mojom/mediastream/media_stream.mojom.h"
#include "ui/gfx/image/image.h"

namespace project_b {

namespace {

// Background pages and service workers start whenever an extension asks.
class ProcessManagerDelegate : public extensions::ProcessManagerDelegate {
  bool AreBackgroundPagesAllowedForContext(
      content::BrowserContext* context) const override {
    return true;
  }
  bool IsExtensionBackgroundPageAllowed(
      content::BrowserContext* context,
      const extensions::Extension& extension) const override {
    return true;
  }
  bool DeferCreatingStartupBackgroundHosts(
      content::BrowserContext* context) const override {
    return false;
  }
};

// What a page of an extension's own, its popup or its background page, asks its owner.
class ExtensionHostDelegate : public extensions::ExtensionHostDelegate {
  void OnExtensionHostCreated(content::WebContents* web_contents) override {
    WebContentsObserver::CreateForWebContents(web_contents);
  }
  // The page opened a window: it becomes a tab, as it does for any page.
  void CreateTab(std::unique_ptr<content::WebContents> web_contents,
                 const GURL& target_url,
                 const extensions::ExtensionId& extension_id,
                 WindowOpenDisposition disposition,
                 const blink::mojom::WindowFeatures& window_features,
                 bool user_gesture) override {
    PageTab* tab = Engine::Get()->AdoptTab(std::move(web_contents));
    Engine::TabOpened(0, tab->id(), target_url, true, false);
  }
  // There is no camera or microphone prompt yet, so no page of an extension gets either.
  void ProcessMediaAccessRequest(content::WebContents* web_contents,
                                 const content::MediaStreamRequest& request,
                                 content::MediaResponseCallback callback,
                                 const extensions::Extension* extension) override {
    std::move(callback).Run(
        blink::mojom::StreamDevicesSet(),
        blink::mojom::MediaStreamRequestResult::NOT_SUPPORTED, nullptr);
  }
  bool CheckMediaAccessPermission(content::RenderFrameHost* render_frame_host,
                                  const url::Origin& security_origin,
                                  blink::mojom::MediaStreamType type,
                                  const extensions::Extension* extension) override {
    return false;
  }
  content::PictureInPictureResult EnterPictureInPicture(
      content::WebContents* web_contents) override {
    return content::PictureInPictureResult::kNotSupported;
  }
  void ExitPictureInPicture() override {}
};

// What chrome.runtime needs from the browser.
class RuntimeAPIDelegate : public extensions::RuntimeAPIDelegate {
 public:
  explicit RuntimeAPIDelegate(content::BrowserContext* browser_context)
      : browser_context_(browser_context) {}

 private:
  // Updates come from the store Go names, when Go installs a newer package.
  void AddUpdateObserver(extensions::UpdateObserver* observer) override {}
  void RemoveUpdateObserver(extensions::UpdateObserver* observer) override {}
  void ReloadExtension(const extensions::ExtensionId& extension_id) override {
    extensions::ExtensionRegistrar::Get(browser_context_)
        ->ReloadExtension(extension_id);
  }
  bool CheckForUpdates(const extensions::ExtensionId& extension_id,
                       UpdateCheckCallback callback) override {
    return false;
  }
  void OpenURL(const GURL& uninstall_url) override { OpenTab(uninstall_url); }
  bool GetPlatformInfo(extensions::api::runtime::PlatformInfo* info) override {
    info->os = extensions::api::runtime::PlatformOs::kMac;
    info->arch = extensions::api::runtime::PlatformArch::kArm64;
    info->nacl_arch = extensions::api::runtime::PlatformNaclArch::kArm;
    return true;
  }
  bool RestartDevice(std::string* error_message) override {
    *error_message = "Restart is not supported.";
    return false;
  }
  void OpenOptionsPage(const extensions::Extension* extension,
                       content::BrowserContext* browser_context,
                       base::OnceCallback<void(bool)> callback) override {
    OpenTab(extensions::OptionsPageInfo::GetOptionsPage(extension));
    std::move(callback).Run(true);
  }
  int GetDeveloperToolsWindowId(
      content::WebContents* developer_tools_web_contents) override {
    return -1;
  }

  const raw_ptr<content::BrowserContext> browser_context_;
};

// The one profile lives as long as the app does, so a hold on it holds nothing.
class ProfileKeepAlive : public extensions::ScopedBrowserContextKeepAlive {
 public:
  ~ProfileKeepAlive() override = default;
};

// There is no policy: nothing forbids or forces an extension, a permission or a host.
// docs/extensions.md lists this among the checks to discuss.
class NoPolicy : public extensions::ExtensionManagementClient {
  bool UpdatesFromWebstore(const extensions::Extension& extension) override {
    return false;
  }
  bool IsInstallationExplicitlyAllowed(
      const extensions::ExtensionId& id) override {
    return false;
  }
  bool IsForceInstalledInLowTrustEnvironment(
      const extensions::Extension& extension) override {
    return false;
  }
  const extensions::URLPatternSet& GetPolicyBlockedHosts(
      const extensions::Extension* extension) override {
    return no_hosts_;
  }
  const extensions::URLPatternSet& GetPolicyAllowedHosts(
      const extensions::Extension* extension) override {
    return no_hosts_;
  }
  bool UsesDefaultPolicyHostRestrictions(
      const extensions::Extension* extension) override {
    return true;
  }
  bool BlocklistedByDefault() const override { return false; }
  // An extension is updated from the store Go names, not from where its manifest says.
  GURL GetEffectiveUpdateURL(const extensions::Extension& extension) override {
    return GURL();
  }
  bool IsAllowedManifestType(extensions::Manifest::Type manifest_type,
                             const std::string& extension_id) const override {
    return true;
  }
  extensions::ManagedInstallationMode GetInstallationMode(
      const extensions::Extension* extension) override {
    return extensions::ManagedInstallationMode::kAllowed;
  }
  extensions::ManagedInstallationMode GetInstallationMode(
      const extensions::ExtensionId& extension_id,
      const std::string& update_url) override {
    return extensions::ManagedInstallationMode::kAllowed;
  }
  bool IsInstallationExplicitlyBlocked(
      const extensions::ExtensionId& id) override {
    return false;
  }
  const std::string BlockedInstallMessage(
      const extensions::ExtensionId& id) override {
    return std::string();
  }
  bool IsPermissionSetAllowed(const extensions::Extension* extension,
                              const extensions::PermissionSet& perms) override {
    return true;
  }
  bool IsPermissionSetAllowed(const extensions::ExtensionId& extension_id,
                              const std::string& update_url,
                              const extensions::PermissionSet& perms) override {
    return true;
  }

  const extensions::URLPatternSet no_hosts_;
};

// Decodes a picture a store's page named, in the sandboxed decoder process.
class ImageDecoder : public image_fetcher::ImageDecoder {
  void DecodeImage(const std::string& image_data,
                   const gfx::Size& desired_image_frame_size,
                   data_decoder::DataDecoder* data_decoder,
                   image_fetcher::ImageDecodedCallback callback) override {
    data_decoder::DecodeImageCallback decoded = base::BindOnce(
        [](image_fetcher::ImageDecodedCallback callback,
           const SkBitmap& bitmap) {
          std::move(callback).Run(gfx::Image::CreateFrom1xBitmap(bitmap));
        },
        std::move(callback));
    // The caller may have a decoder process to share; without one this picture gets
    // its own.
    if (data_decoder) {
      data_decoder::DecodeImage(
          data_decoder, base::as_byte_span(image_data),
          data_decoder::mojom::ImageCodec::kDefault, /*shrink_to_fit=*/false,
          data_decoder::kDefaultMaxSizeInBytes, desired_image_frame_size,
          std::move(decoded));
    } else {
      data_decoder::DecodeImageIsolated(
          base::as_byte_span(image_data),
          data_decoder::mojom::ImageCodec::kDefault, /*shrink_to_fit=*/false,
          data_decoder::kDefaultMaxSizeInBytes, desired_image_frame_size,
          std::move(decoded));
    }
  }
};

class KioskDelegate : public extensions::KioskDelegate {
  bool IsAutoLaunchedKioskApp(
      const extensions::ExtensionId& id) const override {
    return false;
  }
};

}  // namespace

ExtensionsBrowserClient::ExtensionsBrowserClient(
    content::BrowserContext* browser_context)
    : browser_context_(browser_context),
      api_client_(std::make_unique<ExtensionsAPIClient>()),
      process_manager_delegate_(std::make_unique<ProcessManagerDelegate>()),
      extension_cache_(std::make_unique<extensions::NullExtensionCache>()),
      kiosk_delegate_(std::make_unique<KioskDelegate>()),
      safe_browsing_delegate_(
          std::make_unique<extensions::SafeBrowsingDelegate>()),
      management_client_(std::make_unique<NoPolicy>()) {
  AddAPIProvider(
      std::make_unique<extensions::CoreExtensionsBrowserAPIProvider>());
  AddAPIProvider(std::make_unique<GoAPIProvider>());
  // Which APIs exist depends on the channel; ours is the released one.
  extensions::SetCurrentChannel(version_info::Channel::STABLE);
}

ExtensionsBrowserClient::~ExtensionsBrowserClient() = default;

void ExtensionsBrowserClient::Init() {}

bool ExtensionsBrowserClient::IsShuttingDown() {
  return false;
}

bool ExtensionsBrowserClient::AreExtensionsDisabled(
    const base::CommandLine& command_line,
    content::BrowserContext* context) {
  return false;
}

bool ExtensionsBrowserClient::IsValidContext(void* context) {
  return context == browser_context_;
}

bool ExtensionsBrowserClient::IsSameContext(content::BrowserContext* first,
                                            content::BrowserContext* second) {
  return first == second;
}

bool ExtensionsBrowserClient::HasOffTheRecordContext(
    content::BrowserContext* context) {
  return false;
}

content::BrowserContext* ExtensionsBrowserClient::GetOffTheRecordContext(
    content::BrowserContext* context) {
  return nullptr;
}

content::BrowserContext* ExtensionsBrowserClient::GetOriginalContext(
    content::BrowserContext* context) {
  return context;
}

content::BrowserContext* ExtensionsBrowserClient::GetContextRedirectedToOriginal(
    content::BrowserContext* context) {
  return context;
}

content::BrowserContext*
ExtensionsBrowserClient::GetContextRedirectedToOriginalWithoutAshInternals(
    content::BrowserContext* context) {
  return context;
}

content::BrowserContext* ExtensionsBrowserClient::GetContextOwnInstance(
    content::BrowserContext* context) {
  return context;
}

content::BrowserContext* ExtensionsBrowserClient::GetContextForOriginalOnly(
    content::BrowserContext* context) {
  return context;
}

bool ExtensionsBrowserClient::AreExtensionsDisabledForContext(
    content::BrowserContext* context) {
  return false;
}

bool ExtensionsBrowserClient::IsGuestSession(
    content::BrowserContext* context) const {
  return false;
}

bool ExtensionsBrowserClient::IsExtensionIncognitoEnabled(
    const extensions::ExtensionId& extension_id,
    content::BrowserContext* context) const {
  return false;
}

bool ExtensionsBrowserClient::IsExtensionIncognitoEnabled(
    const extensions::Extension* extension,
    content::BrowserContext* context) const {
  return false;
}

bool ExtensionsBrowserClient::CanExtensionCrossIncognito(
    const extensions::Extension* extension,
    content::BrowserContext* context) const {
  return false;
}

// No extension's files come from our resource pack: each has its folder on disk.
base::FilePath ExtensionsBrowserClient::GetBundleResourcePath(
    const network::ResourceRequest& request,
    const base::FilePath& extension_resources_path,
    int* resource_id) const {
  *resource_id = 0;
  return base::FilePath();
}

void ExtensionsBrowserClient::LoadResourceFromResourceBundle(
    const network::ResourceRequest& request,
    mojo::PendingReceiver<network::mojom::URLLoader> loader,
    const base::FilePath& resource_relative_path,
    int resource_id,
    scoped_refptr<net::HttpResponseHeaders> headers,
    mojo::PendingRemote<network::mojom::URLLoaderClient> client,
    content::BrowserContext* browser_context) {
  NOTREACHED();
}

bool ExtensionsBrowserClient::AllowCrossRendererResourceLoad(
    const network::ResourceRequest& request,
    network::mojom::RequestDestination destination,
    ui::PageTransition page_transition,
    content::ChildProcessId child_id,
    bool is_incognito,
    const extensions::Extension* extension,
    const extensions::ExtensionSet& extensions,
    const extensions::ProcessMap& process_map,
    const GURL& upstream_url) {
  bool allowed = false;
  // //extensions' own rule: an extension's process, or a resource the manifest lets
  // web pages use.
  extensions::url_request_util::AllowCrossRendererResourceLoad(
      request, destination, page_transition, child_id, is_incognito, extension,
      extensions, process_map, upstream_url, &allowed);
  return allowed;
}

void ExtensionsBrowserClient::GetEarlyExtensionPrefsObservers(
    content::BrowserContext* context,
    std::vector<extensions::EarlyExtensionPrefsObserver*>* observers) const {}

extensions::ProcessManagerDelegate*
ExtensionsBrowserClient::GetProcessManagerDelegate() const {
  return process_manager_delegate_.get();
}

mojo::PendingRemote<network::mojom::URLLoaderFactory>
ExtensionsBrowserClient::GetControlledFrameEmbedderURLLoader(
    const url::Origin& app_origin,
    content::FrameTreeNodeId frame_tree_node_id,
    content::BrowserContext* browser_context) {
  return mojo::PendingRemote<network::mojom::URLLoaderFactory>();
}

std::unique_ptr<extensions::ExtensionHostDelegate>
ExtensionsBrowserClient::CreateExtensionHostDelegate() {
  return std::make_unique<ExtensionHostDelegate>();
}

bool ExtensionsBrowserClient::DidVersionUpdate(
    content::BrowserContext* context) {
  return false;
}

void ExtensionsBrowserClient::PermitExternalProtocolHandler() {}

bool ExtensionsBrowserClient::IsInDemoMode() {
  return false;
}

bool ExtensionsBrowserClient::IsScreensaverInDemoMode(
    const std::string& app_id) {
  return false;
}

bool ExtensionsBrowserClient::IsRunningInForcedAppMode() {
  return false;
}

bool ExtensionsBrowserClient::IsAppModeForcedForApp(
    const extensions::ExtensionId& id) {
  return false;
}

bool ExtensionsBrowserClient::IsLoggedInAsPublicAccount() {
  return false;
}

extensions::ExtensionSystemProvider*
ExtensionsBrowserClient::GetExtensionSystemFactory() {
  return ExtensionSystemFactory::GetInstance();
}

void ExtensionsBrowserClient::RegisterBrowserInterfaceBindersForFrame(
    mojo::BinderMapWithContext<content::RenderFrameHost*>* binder_map,
    content::RenderFrameHost* render_frame_host,
    const extensions::Extension* extension) const {
  extensions::PopulateExtensionFrameBinders(binder_map, render_frame_host,
                                            extension);
}

std::unique_ptr<extensions::RuntimeAPIDelegate>
ExtensionsBrowserClient::CreateRuntimeAPIDelegate(
    content::BrowserContext* context) const {
  return std::make_unique<RuntimeAPIDelegate>(context);
}

const extensions::ComponentExtensionResourceManager*
ExtensionsBrowserClient::GetComponentExtensionResourceManager() {
  return nullptr;
}

void ExtensionsBrowserClient::BroadcastEventToRenderers(
    extensions::events::HistogramValue histogram_value,
    const std::string& event_name,
    base::ListValue args,
    bool dispatch_to_off_the_record_profiles) {
  extensions::EventRouter::Get(browser_context_)
      ->BroadcastEvent(std::make_unique<extensions::Event>(
          histogram_value, event_name, std::move(args)));
}

extensions::ExtensionCache* ExtensionsBrowserClient::GetExtensionCache() {
  return extension_cache_.get();
}

bool ExtensionsBrowserClient::IsBackgroundUpdateAllowed() {
  return true;
}

// An extension's minimum_chrome_version is compared with the Chromium we carry.
bool ExtensionsBrowserClient::IsMinBrowserVersionSupported(
    const std::string& min_version) {
  return true;
}

void ExtensionsBrowserClient::CreateExtensionWebContentsObserver(
    content::WebContents* web_contents) {
  WebContentsObserver::CreateForWebContents(web_contents);
}

extensions::ExtensionWebContentsObserver*
ExtensionsBrowserClient::GetExtensionWebContentsObserver(
    content::WebContents* web_contents) {
  return WebContentsObserver::FromWebContents(web_contents);
}

extensions::KioskDelegate* ExtensionsBrowserClient::GetKioskDelegate() {
  return kiosk_delegate_.get();
}

extensions::SafeBrowsingDelegate*
ExtensionsBrowserClient::GetSafeBrowsingDelegate() {
  return safe_browsing_delegate_.get();
}

std::string ExtensionsBrowserClient::GetApplicationLocale() {
  return "en-US";
}

void ExtensionsBrowserClient::GetTabAndWindowIdForWebContents(
    content::WebContents* web_contents,
    int* tab_id,
    int* window_id) {
  const int64_t tab = hooks::TabOf(web_contents);
  *tab_id = tab > 0 ? static_cast<int>(tab) : -1;
  *window_id = tab > 0 ? kWindowId : -1;
}

bool ExtensionsBrowserClient::IsValidTabId(
    content::BrowserContext* browser_context,
    int tab_id,
    bool include_incognito,
    content::WebContents** web_contents) const {
  content::WebContents* contents = Engine::Get()->Contents(tab_id);
  if (web_contents) {
    *web_contents = contents;
  }
  return contents != nullptr;
}

extensions::ScriptExecutor* ExtensionsBrowserClient::GetScriptExecutorForTab(
    content::WebContents& web_contents) {
  WebContentsObserver* observer =
      WebContentsObserver::FromWebContents(&web_contents);
  return observer ? observer->script_executor() : nullptr;
}

base::FilePath ExtensionsBrowserClient::GetUserDataDir() {
  return ProfileDir();
}

std::unique_ptr<extensions::ScopedBrowserContextKeepAlive>
ExtensionsBrowserClient::CreateUpdaterKeepAlive(
    content::BrowserContext* context) {
  return std::make_unique<ProfileKeepAlive>();
}

extensions::Blocklist* ExtensionsBrowserClient::GetBlocklist(
    content::BrowserContext* context) {
  return Extensions::Get()->blocklist();
}

extensions::InstallTracker* ExtensionsBrowserClient::GetInstallTracker(
    content::BrowserContext* context) {
  return Extensions::Get()->install_tracker();
}

extensions::InstallStageTracker*
ExtensionsBrowserClient::GetInstallStageTracker(
    content::BrowserContext* context) {
  return Extensions::Get()->install_stage_tracker();
}

extensions::InstallVerifier* ExtensionsBrowserClient::GetInstallVerifier(
    content::BrowserContext* context) {
  return Extensions::Get()->install_verifier();
}

extensions::SharedModuleService*
ExtensionsBrowserClient::GetSharedModuleService(
    content::BrowserContext* context) {
  return Extensions::Get()->shared_modules();
}

extensions::ExtensionManagementClient*
ExtensionsBrowserClient::GetExtensionManagementClient(
    content::BrowserContext* context) {
  return management_client_.get();
}

std::unique_ptr<extensions::ExtensionInstallPromptClient>
ExtensionsBrowserClient::CreateInstallPrompt(
    content::WebContents* web_contents,
    std::unique_ptr<extensions::InstallPromptData> prompt) {
  return Extensions::Get()->CreateInstallPrompt();
}

std::unique_ptr<image_fetcher::ImageDecoder>
ExtensionsBrowserClient::CreateImageDecoder() {
  return std::make_unique<ImageDecoder>();
}

// The user said yes on the store's page, which the approval records, so this installer
// asks nothing. The package is checked as any other is: a plain CRX3.
scoped_refptr<extensions::CrxInstaller>
ExtensionsBrowserClient::CreateCrxInstallerFromDownloadItem(
    content::BrowserContext* context,
    const download::DownloadItem& download) {
  scoped_refptr<extensions::CrxInstaller> installer =
      extensions::CrxInstaller::Create(
          context, nullptr,
          extensions::WebstoreInstaller::GetAssociatedApproval(download));
  installer->set_off_store_install_allow_reason(
      extensions::CrxInstaller::OffStoreInstallAllowedBecausePref);
  installer->set_install_immediately(true);
  installer->set_delete_source(true);
  return installer;
}

custom_handlers::ProtocolHandlerRegistry*
ExtensionsBrowserClient::GetProtocolHandlerRegistry(
    content::BrowserContext* context) {
  return Extensions::Get()->protocol_handlers();
}

}  // namespace project_b
