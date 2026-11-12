#ifndef PROJECT_B_ENGINE_EXTENSIONS_BROWSER_ENGINE_FUNCTIONS_H_
#define PROJECT_B_ENGINE_EXTENSIONS_BROWSER_ENGINE_FUNCTIONS_H_

#include <optional>
#include <string>

#include "base/task/task_runner.h"
#include "extensions/browser/api/web_contents_capture_client.h"
#include "project_b/engine/extensions/browser/go_function.h"

namespace project_b {

// The functions of Go's APIs that need the engine for a part only C++ can do. Each is
// listed in go_function.cc's kEngineFunctions.

// action.setIcon: an icon named by a file inside the extension is read here, and Go
// gets the picture as a data: URL under "icon". One the extension drew comes from the
// renderer as that already.
class ActionSetIconFunction : public GoFunction {
 private:
  ~ActionSetIconFunction() override = default;
  ResponseAction Run() override;
  void OnIconRead(std::string data_url);
};

// tabs.captureVisibleTab: Go says which tab is visible, and the engine takes its
// picture, if the extension may see that page. As Chrome's, in
// chrome/browser/extensions/api/tabs/tabs_api.cc.
class TabsCaptureVisibleTabFunction
    : public GoFunction,
      public extensions::WebContentsCaptureClient {
 private:
  ~TabsCaptureVisibleTabFunction() override = default;
  ResponseAction Run() override;

  base::expected<void, extensions::ScreenshotAccessError> GetScreenshotAccess(
      content::WebContents* web_contents) const override;
  bool ClientAllowsTransparency() override;
  void OnCaptureSuccess(const SkBitmap& bitmap) override;
  void OnCaptureFailure(CaptureResult result) override;
  std::optional<std::string> Encode(const SkBitmap& bitmap) {
    return EncodeBitmap(bitmap);
  }
  void OnEncoded(std::optional<std::string> data_url);
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_BROWSER_ENGINE_FUNCTIONS_H_
