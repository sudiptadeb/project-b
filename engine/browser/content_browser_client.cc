#include "project_b/engine/browser/content_browser_client.h"

#include "base/check.h"
#include "base/files/file_path.h"
#include "base/path_service.h"
#include "components/embedder_support/user_agent_utils.h"
#include "components/version_info/version_info_with_user_agent.h"
#include "content/public/browser/browser_context.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/navigation_throttle_registry.h"
#include "project_b/engine/browser/browser_main_parts.h"
#include "services/network/public/mojom/network_context.mojom.h"

#if BUILDFLAG(IS_ANDROID)
#include "base/posix/global_descriptors.h"
#include "content/public/browser/posix_file_descriptor_info.h"
#include "project_b/engine/android/descriptors.h"
#else
#include "project_b/engine/browser/devtools.h"
#endif

namespace project_b {

ContentBrowserClient::ContentBrowserClient() = default;
ContentBrowserClient::~ContentBrowserClient() = default;

std::unique_ptr<content::BrowserMainParts>
ContentBrowserClient::CreateBrowserMainParts(bool is_integration_test) {
  return std::make_unique<BrowserMainParts>();
}

#if BUILDFLAG(IS_ANDROID)
// A renderer can't open the APK, so the pak goes along as an open file.
void ContentBrowserClient::GetAdditionalMappedFilesForChildProcess(
    const base::CommandLine& command_line,
    int child_process_id,
    content::PosixFileDescriptorInfo* mappings) {
  auto* descriptors = base::GlobalDescriptors::GetInstance();
  mappings->ShareWithRegion(kPakDescriptor, descriptors->Get(kPakDescriptor),
                            descriptors->GetRegion(kPakDescriptor));
}
#else
// Every child process runs the helper, which has no Go in it.
base::FilePath ContentBrowserClient::GetChildProcessPath(int flags) {
  base::FilePath dir;
  CHECK(base::PathService::Get(base::DIR_EXE, &dir));
  return dir.Append(FILE_PATH_LITERAL("project_b_helper"));
}

std::unique_ptr<content::DevToolsManagerDelegate>
ContentBrowserClient::CreateDevToolsManagerDelegate() {
  return std::make_unique<DevToolsManagerDelegate>();
}
#endif

// Every navigation gets the generated throttle, whose hooks cross to Go only when Go
// subscribed to them (core/content/wire.go decides which navigations it gates).
void ContentBrowserClient::CreateThrottlesForNavigation(
    content::NavigationThrottleRegistry& registry) {
  registry.AddThrottle(std::make_unique<NavigationThrottleHooks>(registry));
}

// What Chrome reports, "Chrome/155.0.8059.41": DevTools' /json/version and
// Browser.getVersion show it, and ChromeDriver reads the browser's version from it. The
// user agent pages see comes from GetUserAgent, not from this.
std::string ContentBrowserClient::GetProduct() {
  return std::string(version_info::GetProductNameAndVersionForUserAgent());
}

std::string ContentBrowserClient::GetUserAgent() {
  return embedder_support::GetUserAgent();
}

// Cookies and the HTTP cache are saved in the partition's folder, as Chrome lays a
// profile out: chromium/Network/Cookies and chromium/Cache. Cookies are stored as
// Chromium writes them, unencrypted, with no OS keychain. Session cookies are not kept
// across a quit, Chromium's default. An in-memory partition gets no files. DevTools
// lays its own proxy over these params after this.
void ContentBrowserClient::ConfigureNetworkContextParams(
    content::BrowserContext* context,
    bool in_memory,
    const base::FilePath& relative_partition_path,
    network::mojom::NetworkContextParams* network_context_params,
    cert_verifier::mojom::CertVerifierCreationParams*
        cert_verifier_creation_params) {
  ContentBrowserClientHooks::ConfigureNetworkContextParams(
      context, in_memory, relative_partition_path, network_context_params,
      cert_verifier_creation_params);
  network_context_params->enable_encrypted_cookies = false;
  if (in_memory) {
    return;
  }
  base::FilePath path = context->GetPath().Append(relative_partition_path);
  auto& file_paths = network_context_params->file_paths;
  file_paths = network::mojom::NetworkContextFilePaths::New();
  file_paths->data_directory = path.Append(FILE_PATH_LITERAL("Network"));
  file_paths->cookie_database_name =
      base::FilePath(FILE_PATH_LITERAL("Cookies"));
  file_paths->http_server_properties_file_name =
      base::FilePath(FILE_PATH_LITERAL("Network Persistent State"));
  file_paths->http_cache_directory = path.Append(FILE_PATH_LITERAL("Cache"));
}

// The sandboxed network process may only write under these folders. It starts before a
// copy that picks its profile has one, so it may write under the copy's folder, which
// holds every profile it can run on.
std::vector<base::FilePath>
ContentBrowserClient::GetNetworkContextsParentDirectory() {
  return {CopyDir()};
}

}  // namespace project_b
