#ifndef PROJECT_B_ENGINE_BROWSER_BROWSER_CONTEXT_H_
#define PROJECT_B_ENGINE_BROWSER_BROWSER_CONTEXT_H_

#include <memory>

#include "base/files/file_path.h"
#include "build/build_config.h"
#include "content/public/browser/browser_context.h"
#include "project_b/engine/browser/permissions.h"

namespace client_hints {
class InMemoryClientHintsControllerDelegate;
}
namespace network {
class NetworkQualityTracker;
}

namespace project_b {

class Downloads;
class Notifications;

// A context content asks for: a profile's, kept in its folder, or the one our own pages
// load in, which has no folder and is off the record, so content keeps its storage in
// memory. The first app says no to everything optional.
class BrowserContext : public content::BrowserContext {
 public:
  // Our pages' context.
  BrowserContext();
  // A profile's, with Chromium's files in path.
  explicit BrowserContext(const base::FilePath& path);
  BrowserContext(const BrowserContext&) = delete;
  BrowserContext& operator=(const BrowserContext&) = delete;
  ~BrowserContext() override;

 private:
  std::unique_ptr<content::ZoomLevelDelegate> CreateZoomLevelDelegate(
      const base::FilePath& partition_path) override;
  base::FilePath GetPath() const override;
  bool IsOffTheRecord() override;
  content::DownloadManagerDelegate* GetDownloadManagerDelegate() override;
  content::BrowserPluginGuestManager* GetGuestManager() override;
  storage::SpecialStoragePolicy* GetSpecialStoragePolicy() override;
  content::PlatformNotificationService* GetPlatformNotificationService()
      override;
  content::PushMessagingService* GetPushMessagingService() override;
  content::StorageNotificationService* GetStorageNotificationService() override;
  content::SSLHostStateDelegate* GetSSLHostStateDelegate() override;
  content::PermissionControllerDelegate* GetPermissionControllerDelegate()
      override;
  content::ReduceAcceptLanguageControllerDelegate*
  GetReduceAcceptLanguageControllerDelegate() override;
  content::ClientHintsControllerDelegate* GetClientHintsControllerDelegate()
      override;
  content::BackgroundFetchDelegate* GetBackgroundFetchDelegate() override;
  content::BackgroundSyncController* GetBackgroundSyncController() override;
  content::BrowsingDataRemoverDelegate* GetBrowsingDataRemoverDelegate()
      override;

  const base::FilePath path_;
  Permissions permissions_;
  // A profile's downloads, made when content first asks.
  std::unique_ptr<Downloads> downloads_;
  // The client hints a navigation's request carries, Sec-CH-UA among them, made when content
  // first asks: without a delegate a page's own request had none, and Google's sign-in took
  // the app for a browser that may not be secure.
  std::unique_ptr<network::NetworkQualityTracker> network_quality_tracker_;
  std::unique_ptr<client_hints::InMemoryClientHintsControllerDelegate> client_hints_;
#if !BUILDFLAG(IS_ANDROID)
  // A profile's notifications, made when content first asks.
  std::unique_ptr<Notifications> notifications_;
#endif
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_BROWSER_CONTEXT_H_
