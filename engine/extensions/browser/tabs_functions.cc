#include <optional>
#include <string>
#include <utility>

#include "base/functional/bind.h"
#include "base/task/bind_post_task.h"
#include "base/task/thread_pool.h"
#include "base/types/optional_util.h"
#include "content/public/browser/web_contents.h"
#include "extensions/common/api/extension_types.h"
#include "extensions/common/extension.h"
#include "extensions/common/permissions/permissions_data.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/extensions/browser/engine_functions.h"
#include "third_party/skia/include/core/SkBitmap.h"

namespace project_b {

// captureVisibleTab(windowId, options). Go answers {"tab": id}, the tab to take.
ExtensionFunction::ResponseAction TabsCaptureVisibleTabFunction::Run() {
  GoAnswer answer = AskGo(args());
  if (!answer.error.empty()) {
    return RespondNow(Error(std::move(answer.error)));
  }
  std::optional<int> tab;
  if (answer.result && answer.result->is_dict()) {
    tab = answer.result->GetDict().FindInt("tab");
  }
  content::WebContents* contents = tab ? Engine::Get()->Contents(*tab) : nullptr;
  if (!contents) {
    return RespondNow(Error("No active web contents to capture"));
  }
  std::string error;
  if (!extension()->permissions_data()->CanCaptureVisiblePage(
          contents->GetLastCommittedURL(), *tab, &error,
          extensions::CaptureRequirement::kActiveTabOrAllUrls)) {
    return RespondNow(Error(std::move(error)));
  }
  std::optional<extensions::api::extension_types::ImageDetails> details;
  if (args().size() > 1) {
    details = extensions::api::extension_types::ImageDetails::FromValue(args()[1]);
  }
  // The picture may come back on another thread.
  const CaptureResult result = CaptureAsync(
      contents, base::OptionalToPtr(details),
      base::BindPostTaskToCurrentDefault(base::BindOnce(
          &TabsCaptureVisibleTabFunction::CopyFromSurfaceComplete, this)));
  if (result != OK) {
    OnCaptureFailure(result);
  }
  return did_respond() ? AlreadyResponded() : RespondLater();
}

base::expected<void, extensions::ScreenshotAccessError>
TabsCaptureVisibleTabFunction::GetScreenshotAccess(
    content::WebContents* web_contents) const {
  return base::ok();
}

bool TabsCaptureVisibleTabFunction::ClientAllowsTransparency() {
  return false;
}

void TabsCaptureVisibleTabFunction::OnCaptureSuccess(const SkBitmap& bitmap) {
  base::ThreadPool::PostTaskAndReplyWithResult(
      FROM_HERE, {base::TaskPriority::USER_VISIBLE},
      base::BindOnce(&TabsCaptureVisibleTabFunction::Encode, this, bitmap),
      base::BindOnce(&TabsCaptureVisibleTabFunction::OnEncoded, this));
}

void TabsCaptureVisibleTabFunction::OnEncoded(
    std::optional<std::string> data_url) {
  if (!data_url) {
    OnCaptureFailure(FAILURE_REASON_ENCODING_FAILED);
    return;
  }
  Respond(WithArguments(std::move(*data_url)));
}

void TabsCaptureVisibleTabFunction::OnCaptureFailure(CaptureResult result) {
  const char* reason = "internal error";
  switch (result) {
    case FAILURE_REASON_READBACK_FAILED:
      reason = "image readback failed";
      break;
    case FAILURE_REASON_ENCODING_FAILED:
      reason = "encoding failed";
      break;
    case FAILURE_REASON_VIEW_INVISIBLE:
      reason = "view is invisible";
      break;
    default:
      break;
  }
  Respond(Error(std::string("Failed to capture tab: ") + reason));
}

}  // namespace project_b
