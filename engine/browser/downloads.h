#ifndef PROJECT_B_ENGINE_BROWSER_DOWNLOADS_H_
#define PROJECT_B_ENGINE_BROWSER_DOWNLOADS_H_

#include <cstdint>
#include <string>

#include "base/files/file_path.h"
#include "base/functional/callback.h"
#include "base/memory/raw_ptr.h"
#include "base/memory/weak_ptr.h"
#include "components/download/public/common/download_item.h"
#include "content/public/browser/download_manager.h"
#include "content/public/browser/download_manager_delegate.h"
#include "url/gurl.h"

namespace content {
class WebContents;
}  // namespace content

namespace project_b {

// A profile's downloads (docs/daily-driver.md). Go says where each goes, or refuses it,
// and hears every change to each; the Save panel is the system's own. The engine keeps
// no list: Go does.
class Downloads : public content::DownloadManagerDelegate,
                  public content::DownloadManager::Observer,
                  public download::DownloadItem::Observer {
 public:
  Downloads();
  Downloads(const Downloads&) = delete;
  Downloads& operator=(const Downloads&) = delete;
  ~Downloads() override;

  // The profile's, or null before its context has made its download manager.
  static Downloads* Get();

  void SetDownloadManager(content::DownloadManager* manager);

  // Downloads url as tab's page would, showing the Save panel first when save_as.
  void DownloadURL(content::WebContents* contents, const GURL& url, bool save_as);
  // Runs a PBDownloadCommand on download id.
  void Command(uint32_t id, int32_t command);

  // content::DownloadManagerDelegate:
  void Shutdown() override;
  bool DetermineDownloadTarget(download::DownloadItem* item,
                               download::DownloadTargetCallback* callback) override;
  void GetNextId(content::DownloadIdCallback callback) override;

  // content::DownloadManager::Observer:
  void OnDownloadCreated(content::DownloadManager* manager,
                         download::DownloadItem* item) override;
  void ManagerGoingDown(content::DownloadManager* manager) override;

  // download::DownloadItem::Observer:
  void OnDownloadUpdated(download::DownloadItem* item) override;
  void OnDownloadDestroyed(download::DownloadItem* item) override;

 private:
  void Report(download::DownloadItem* item);
  void Targeted(uint32_t id,
                download::DownloadTargetCallback callback,
                const base::FilePath& path);

  raw_ptr<content::DownloadManager> manager_;
  uint32_t next_id_ = download::DownloadItem::kInvalidId + 1;
  base::WeakPtrFactory<Downloads> weak_factory_{this};
};

// What the platform does: the Save panel over the page's window, which answers with the
// path chosen or an empty one; and opening a file, or showing it in the file manager.
void ChooseSavePath(content::WebContents* contents,
                    const base::FilePath& suggested,
                    base::OnceCallback<void(const base::FilePath&)> chosen);
void OpenFile(const base::FilePath& path);
void RevealFile(const base::FilePath& path);

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_DOWNLOADS_H_
