#include "project_b/engine/browser/browser_context.h"

#include "base/functional/bind.h"
#include "build/build_config.h"
#include "components/client_hints/browser/in_memory_client_hints_controller_delegate.h"
#include "components/embedder_support/user_agent_utils.h"
#include "content/public/browser/network_service_instance.h"
#include "services/network/public/cpp/network_quality_tracker.h"
#include "url/gurl.h"
#include "project_b/engine/browser/downloads.h"
#if !BUILDFLAG(IS_ANDROID)
#include "project_b/engine/browser/notifications.h"
#endif

namespace project_b {

BrowserContext::BrowserContext() = default;

BrowserContext::BrowserContext(const base::FilePath& path) : path_(path) {}

BrowserContext::~BrowserContext() {
  NotifyWillBeDestroyed();
  ShutdownStoragePartitions();
}

std::unique_ptr<content::ZoomLevelDelegate>
BrowserContext::CreateZoomLevelDelegate(const base::FilePath& partition_path) {
  return nullptr;
}

base::FilePath BrowserContext::GetPath() const {
  return path_;
}

// Only our pages' context has no folder.
bool BrowserContext::IsOffTheRecord() {
  return path_.empty();
}

// A profile's downloads; our pages download nothing.
content::DownloadManagerDelegate* BrowserContext::GetDownloadManagerDelegate() {
  if (!downloads_ && !path_.empty()) {
    downloads_ = std::make_unique<Downloads>();
    downloads_->SetDownloadManager(GetDownloadManager());
  }
  return downloads_.get();
}

content::BrowserPluginGuestManager* BrowserContext::GetGuestManager() {
  return nullptr;
}

storage::SpecialStoragePolicy* BrowserContext::GetSpecialStoragePolicy() {
  return nullptr;
}

// A profile's pages' notifications; our own pages show none. A phone's come later
// (docs/daily-driver.md).
content::PlatformNotificationService*
BrowserContext::GetPlatformNotificationService() {
#if !BUILDFLAG(IS_ANDROID)
  if (!notifications_ && !path_.empty()) {
    notifications_ = std::make_unique<Notifications>(this);
  }
  return notifications_.get();
#else
  return nullptr;
#endif
}

content::PushMessagingService* BrowserContext::GetPushMessagingService() {
  return nullptr;
}

content::StorageNotificationService*
BrowserContext::GetStorageNotificationService() {
  return nullptr;
}

content::SSLHostStateDelegate* BrowserContext::GetSSLHostStateDelegate() {
  return nullptr;
}

content::PermissionControllerDelegate*
BrowserContext::GetPermissionControllerDelegate() {
  return &permissions_;
}

content::ReduceAcceptLanguageControllerDelegate*
BrowserContext::GetReduceAcceptLanguageControllerDelegate() {
  return nullptr;
}

// Chrome's client hints on every request, navigations' too, from Chrome's own user-agent
// metadata (content_browser_client.cc); a page's JavaScript is always allowed, as the app has
// no per-site switch for it that hints should follow.
content::ClientHintsControllerDelegate*
BrowserContext::GetClientHintsControllerDelegate() {
  if (!client_hints_) {
    network_quality_tracker_ = std::make_unique<network::NetworkQualityTracker>(
        base::BindRepeating(&content::GetNetworkService));
    client_hints_ =
        std::make_unique<client_hints::InMemoryClientHintsControllerDelegate>(
            network_quality_tracker_.get(),
            base::BindRepeating([](const GURL&) { return true; }),
            embedder_support::GetUserAgentMetadata());
  }
  return client_hints_.get();
}

content::BackgroundFetchDelegate* BrowserContext::GetBackgroundFetchDelegate() {
  return nullptr;
}

content::BackgroundSyncController*
BrowserContext::GetBackgroundSyncController() {
  return nullptr;
}

content::BrowsingDataRemoverDelegate*
BrowserContext::GetBrowsingDataRemoverDelegate() {
  return nullptr;
}

}  // namespace project_b
