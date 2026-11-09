#include "project_b/engine/common/main_delegate.h"

#include "base/apple/bundle_locations.h"
#include "base/apple/foundation_util.h"
#include "base/check.h"
#include "base/files/file_path.h"
#include "base/logging.h"
#include "base/logging/logging_settings.h"
#include "base/path_service.h"
#include "project_b/engine/common/content_client.h"
#include "ui/base/resource/resource_bundle.h"

namespace project_b {

MainDelegate::MainDelegate() = default;
MainDelegate::~MainDelegate() = default;

std::optional<int> MainDelegate::BasicStartupComplete() {
  // Inside an app, content looks for its files under the "framework bundle". We have
  // no framework, so that's the app's Contents folder.
  if (base::apple::AmIBundled()) {
    base::apple::SetOverrideFrameworkBundlePath(
        base::apple::MainBundlePath().Append(FILE_PATH_LITERAL("Contents")));
  }

  logging::LoggingSettings settings;
  settings.logging_dest =
      logging::LOG_TO_SYSTEM_DEBUG_LOG | logging::LOG_TO_STDERR;
  CHECK(logging::InitLogging(settings));

  content_client_ = std::make_unique<ContentClient>();
  content::SetContentClient(content_client_.get());
  return std::nullopt;
}

void MainDelegate::PreSandboxStartup() {
  base::FilePath pak;
  CHECK(base::PathService::Get(base::DIR_ASSETS, &pak));
  ui::ResourceBundle::InitSharedInstanceWithPakPath(
      pak.Append(FILE_PATH_LITERAL("project_b.pak")));
}

}  // namespace project_b
