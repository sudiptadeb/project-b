#include "project_b/engine/browser/browser_context.h"

#include "project_b/engine/browser/downloads.h"

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

// Our pages download nothing. A phone's downloads come later (docs/daily-driver.md).
content::DownloadManagerDelegate* BrowserContext::GetDownloadManagerDelegate() {
#if BUILDFLAG(IS_MAC)
  if (!downloads_ && !path_.empty()) {
    downloads_ = std::make_unique<Downloads>();
    downloads_->SetDownloadManager(GetDownloadManager());
  }
  return downloads_.get();
#else
  return nullptr;
#endif
}

content::BrowserPluginGuestManager* BrowserContext::GetGuestManager() {
  return nullptr;
}

storage::SpecialStoragePolicy* BrowserContext::GetSpecialStoragePolicy() {
  return nullptr;
}

content::PlatformNotificationService*
BrowserContext::GetPlatformNotificationService() {
  return nullptr;
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
  return nullptr;
}

content::ReduceAcceptLanguageControllerDelegate*
BrowserContext::GetReduceAcceptLanguageControllerDelegate() {
  return nullptr;
}

content::ClientHintsControllerDelegate*
BrowserContext::GetClientHintsControllerDelegate() {
  return nullptr;
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
