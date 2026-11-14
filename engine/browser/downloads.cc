#include "project_b/engine/browser/downloads.h"

#include <memory>
#include <optional>
#include <utility>

#include "base/check.h"
#include "base/functional/bind.h"
#include "base/json/json_reader.h"
#include "base/task/thread_pool.h"
#include "base/values.h"
#include "components/download/public/common/download_target_info.h"
#include "components/download/public/common/download_url_parameters.h"
#include "content/public/browser/download_item_utils.h"
#include "content/public/browser/download_request_utils.h"
#include "content/public/browser/web_contents.h"
#include "net/base/filename_util.h"
#include "net/traffic_annotation/network_traffic_annotation.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"

namespace project_b {

namespace {

Downloads* g_downloads = nullptr;

// The file name content would give the download, from its headers and address.
std::string NameOf(const GURL& url,
                   const std::string& content_disposition,
                   const std::string& suggested,
                   const std::string& mime_type) {
  return net::GenerateFileName(url, content_disposition, std::string(), suggested,
                               mime_type, "download")
      .AsUTF8Unsafe();
}

int32_t StateOf(const download::DownloadItem* item) {
  switch (item->GetState()) {
    case download::DownloadItem::COMPLETE:
      return PB_DOWNLOAD_DONE;
    case download::DownloadItem::CANCELLED:
      return PB_DOWNLOAD_CANCELLED;
    case download::DownloadItem::INTERRUPTED:
      return PB_DOWNLOAD_FAILED;
    default:
      return PB_DOWNLOAD_PROGRESS;
  }
}

}  // namespace

Downloads::Downloads() {
  g_downloads = this;
}

void Downloads::SetDownloadManager(content::DownloadManager* manager) {
  manager_ = manager;
  manager_->AddObserver(this);
}

Downloads::~Downloads() {
  if (manager_) {
    manager_->RemoveObserver(this);
  }
  if (g_downloads == this) {
    g_downloads = nullptr;
  }
}

// static
Downloads* Downloads::Get() {
  return g_downloads;
}

void Downloads::Shutdown() {
  weak_factory_.InvalidateWeakPtrs();
}

void Downloads::OnDownloadDestroyed(download::DownloadItem* item) {
  item->RemoveObserver(this);
}

void Downloads::ManagerGoingDown(content::DownloadManager* manager) {
  manager_->RemoveObserver(this);
  manager_ = nullptr;
}

void Downloads::GetNextId(content::DownloadIdCallback callback) {
  std::move(callback).Run(next_id_++);
}

void Downloads::OnDownloadCreated(content::DownloadManager* manager,
                                  download::DownloadItem* item) {
  item->AddObserver(this);
}

void Downloads::OnDownloadUpdated(download::DownloadItem* item) {
  // Nothing until Go has placed it: its path is what the list shows.
  if (!item->GetTargetFilePath().empty()) {
    Report(item);
  }
  if (item->IsDone()) {
    item->RemoveObserver(this);
  }
}

void Downloads::Report(download::DownloadItem* item) {
  hooks::Args args;
  args.Int(item->GetId());
  args.Str(item->GetTargetFilePath().AsUTF8Unsafe());
  args.Str(item->GetURL().spec());
  args.Str(item->GetMimeType());
  args.Int(StateOf(item));
  args.Int(item->GetReceivedBytes());
  args.Int(item->GetTotalBytes());
  args.Bool(item->IsPaused());
  hooks::Fire(PB_EVENT_DOWNLOAD_UPDATED, args);
}

bool Downloads::DetermineDownloadTarget(download::DownloadItem* item,
                                        download::DownloadTargetCallback* callback) {
  if (!item->GetForcedFilePath().empty()) {
    download::DownloadTargetInfo target;
    target.target_path = item->GetForcedFilePath();
    target.intermediate_path = item->GetForcedFilePath();
    std::move(*callback).Run(std::move(target));
    return true;
  }
  content::WebContents* contents = content::DownloadItemUtils::GetWebContents(item);
  hooks::Args args;
  args.Int(item->GetId());
  args.Int(contents ? hooks::TabOf(contents) : 0);
  args.Str(item->GetURL().spec());
  args.Str(NameOf(item->GetURL(), item->GetContentDisposition(),
                  item->GetSuggestedFilename(), item->GetMimeType()));
  args.Str(item->GetMimeType());
  args.Bool(item->GetTargetDisposition() ==
            download::DownloadItem::TARGET_DISPOSITION_PROMPT);
  // Go's answer is {"path", "prompt"}; none, or no path, refuses the download.
  PBValue result;
  base::FilePath path;
  bool prompt = false;
  if (hooks::Ask(PB_EVENT_DOWNLOAD_TARGET, args, &result)) {
    std::optional<base::Value> answer =
        base::JSONReader::Read(hooks::TakeString(result), 0);
    if (answer && answer->is_dict()) {
      if (const std::string* p = answer->GetDict().FindString("path")) {
        path = base::FilePath::FromUTF8Unsafe(*p);
      }
      prompt = answer->GetDict().FindBool("prompt").value_or(false);
    }
  }
  auto targeted = base::BindOnce(&Downloads::Targeted, weak_factory_.GetWeakPtr(),
                                 item->GetId(), std::move(*callback));
  if (prompt && !path.empty() && contents) {
    project_b::ChooseSavePath(contents, path, std::move(targeted));
  } else {
    std::move(targeted).Run(path);
  }
  return true;
}

void Downloads::Targeted(uint32_t id,
                         download::DownloadTargetCallback callback,
                         const base::FilePath& path) {
  download::DownloadTargetInfo target;
  if (path.empty()) {
    // An empty target cancels it.
    target.interrupt_reason = download::DOWNLOAD_INTERRUPT_REASON_USER_CANCELED;
  } else {
    target.target_path = path;
    target.intermediate_path = path.AddExtension(FILE_PATH_LITERAL("crdownload"));
    target.target_disposition = download::DownloadItem::TARGET_DISPOSITION_OVERWRITE;
  }
  std::move(callback).Run(std::move(target));
}

void Downloads::DownloadURL(content::WebContents* contents, const GURL& url, bool save_as) {
  if (!manager_ || !contents) {
    return;
  }
  std::unique_ptr<download::DownloadUrlParameters> params =
      content::DownloadRequestUtils::CreateDownloadForWebContentsMainFrame(
          contents, url, NO_TRAFFIC_ANNOTATION_YET);
  params->set_prompt(save_as);
  manager_->DownloadUrl(std::move(params));
}

void Downloads::Command(uint32_t id, int32_t command) {
  download::DownloadItem* item = manager_ ? manager_->GetDownload(id) : nullptr;
  if (!item) {
    return;
  }
  switch (command) {
    case PB_DOWNLOAD_CANCEL:
      item->Cancel(/*user_cancel=*/true);
      break;
    case PB_DOWNLOAD_PAUSE:
      item->Pause();
      break;
    case PB_DOWNLOAD_RESUME:
      item->Resume(/*user_resume=*/true);
      break;
    case PB_DOWNLOAD_REMOVE:
      item->RemoveObserver(this);
      item->Remove();
      break;
  }
}

}  // namespace project_b
