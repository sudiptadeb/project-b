#ifndef PROJECT_B_ENGINE_BROWSER_BROWSER_CONTEXT_H_
#define PROJECT_B_ENGINE_BROWSER_BROWSER_CONTEXT_H_

#include <memory>

#include "base/files/file_path.h"
#include "build/build_config.h"
#include "content/public/browser/browser_context.h"

namespace project_b {

class Downloads;

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
#if BUILDFLAG(IS_MAC)
  // A profile's downloads, made when content first asks.
  std::unique_ptr<Downloads> downloads_;
#endif
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_BROWSER_CONTEXT_H_
