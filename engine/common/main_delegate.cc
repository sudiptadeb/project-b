#include "project_b/engine/common/main_delegate.h"

#include <string>

#include "base/check.h"
#include "base/command_line.h"
#include "base/files/file_path.h"
#include "base/logging.h"
#include "base/logging/logging_settings.h"
#include "base/path_service.h"
#include "build/build_config.h"
#include "content/public/common/content_switches.h"
#include "project_b/engine/common/content_client.h"
#include "project_b/engine/common/extensions_on.h"
#include "ui/base/resource/resource_bundle.h"

#if BUILDFLAG(PB_EXTENSIONS)
#include "content/public/renderer/content_renderer_client.h"
#include "content/public/utility/content_utility_client.h"
#include "project_b/engine/extensions/common/extensions_client.h"
#include "project_b/engine/extensions/renderer/content_renderer_client.h"
#include "project_b/engine/extensions/utility/content_utility_client.h"
#endif

#if BUILDFLAG(IS_MAC)
#include "base/apple/bundle_locations.h"
#include "base/apple/foundation_util.h"
#endif

#if BUILDFLAG(IS_ANDROID)
#include "base/android/apk_assets.h"
#include "base/files/file.h"
#include "base/posix/global_descriptors.h"
#include "content/public/browser/android/compositor.h"
#include "content/public/browser/browser_main_runner.h"
#include "project_b/engine/android/descriptors.h"
#endif

namespace project_b {

namespace {

// Web APIs whose browser side is Chrome's, not content's: a page that calls one asks for
// an interface no one binds, and content kills its renderer for it, as DuckDuckGo's
// search did with getInstalledRelatedApps on Android. Turned off, they are simply not
// there, and a page that checks for them carries on without.
constexpr char kChromeOnlyBlinkFeatures[] =
    "InstalledApp,WebShare,DigitalGoods,PaymentRequest,SubApps";

// The browser process turns them off; content passes the switch to every renderer.
void DisableChromeOnlyBlinkFeatures() {
  base::CommandLine* command_line = base::CommandLine::ForCurrentProcess();
  if (command_line->HasSwitch(switches::kProcessType)) {
    return;
  }
  std::string features = kChromeOnlyBlinkFeatures;
  std::string given =
      command_line->GetSwitchValueASCII(switches::kDisableBlinkFeatures);
  if (!given.empty()) {
    features = given + "," + features;
  }
  command_line->AppendSwitchASCII(switches::kDisableBlinkFeatures, features);
}

}  // namespace

MainDelegate::MainDelegate() = default;
MainDelegate::~MainDelegate() = default;

std::optional<int> MainDelegate::BasicStartupComplete() {
  DisableChromeOnlyBlinkFeatures();
#if BUILDFLAG(IS_MAC)
  // Inside an app, content looks for its files under the "framework bundle". We have
  // no framework, so that's the app's Contents folder.
  if (base::apple::AmIBundled()) {
    base::apple::SetOverrideFrameworkBundlePath(
        base::apple::MainBundlePath().Append(FILE_PATH_LITERAL("Contents")));
  }
#endif
#if BUILDFLAG(IS_ANDROID)
  content::Compositor::Initialize();
#endif

  logging::LoggingSettings settings;
  settings.logging_dest =
      logging::LOG_TO_SYSTEM_DEBUG_LOG | logging::LOG_TO_STDERR;
  CHECK(logging::InitLogging(settings));

  content_client_ = std::make_unique<ContentClient>();
  content::SetContentClient(content_client_.get());
  return std::nullopt;
}

void MainDelegate::PreSandboxStartup() {
#if BUILDFLAG(PB_EXTENSIONS)
  // Here, not earlier: the client holds the store's URLs, and a URL can't be made
  // before content has named its schemes.
  if (ExtensionsOn()) {
    extensions_client_ = std::make_unique<ExtensionsClient>();
    extensions::ExtensionsClient::Set(extensions_client_.get());
  }
#endif
#if BUILDFLAG(IS_ANDROID)
  // A renderer has a different user and can't open the APK, so it was handed an open file.
  auto* descriptors = base::GlobalDescriptors::GetInstance();
  int pak_fd = descriptors->MaybeGet(kPakDescriptor);
  base::MemoryMappedFile::Region region;
  if (pak_fd >= 0) {
    region = descriptors->GetRegion(kPakDescriptor);
  } else {
    pak_fd = base::android::OpenApkAsset("assets/project_b.pak", &region);
    CHECK_GE(pak_fd, 0);
    descriptors->Set(kPakDescriptor, pak_fd, region);
  }
  base::File pak_file(pak_fd);
  ui::ResourceBundle::InitSharedInstanceWithPakFileRegion(pak_file.Duplicate(),
                                                          region);
  ui::ResourceBundle::GetSharedInstance().AddDataPackFromFileRegion(
      std::move(pak_file), region, ui::k100Percent);
#else
  base::FilePath pak;
  CHECK(base::PathService::Get(base::DIR_ASSETS, &pak));
  ui::ResourceBundle::InitSharedInstanceWithPakPath(
      pak.Append(FILE_PATH_LITERAL("project_b.pak")));
#endif
}

#if BUILDFLAG(PB_EXTENSIONS)
content::ContentRendererClient* MainDelegate::CreateContentRendererClient() {
  if (!ExtensionsOn()) {
    return content::ContentMainDelegate::CreateContentRendererClient();
  }
  renderer_client_ = std::make_unique<ContentRendererClient>();
  return renderer_client_.get();
}

content::ContentUtilityClient* MainDelegate::CreateContentUtilityClient() {
  if (!ExtensionsOn()) {
    return content::ContentMainDelegate::CreateContentUtilityClient();
  }
  utility_client_ = std::make_unique<ContentUtilityClient>();
  return utility_client_.get();
}
#endif

#if BUILDFLAG(IS_ANDROID)
// Android's looper is the message loop, so this only starts the browser and returns. Content never
// shuts down on Android, so the runner is left alive on purpose.
std::variant<int, content::MainFunctionParams> MainDelegate::RunProcess(
    const std::string& process_type,
    content::MainFunctionParams main_function_params) {
  if (!process_type.empty()) {
    return std::move(main_function_params);
  }
  std::unique_ptr<content::BrowserMainRunner> runner =
      content::BrowserMainRunner::Create();
  int code = runner->Initialize(std::move(main_function_params));
  CHECK_LT(code, 0);
  std::ignore = runner.release();
  return 0;
}
#endif

}  // namespace project_b
