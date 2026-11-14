#include "project_b/engine/common/content_client.h"

#include "base/memory/ref_counted_memory.h"
#include "ui/base/resource/resource_bundle.h"

#if BUILDFLAG(PB_EXTENSIONS)
#include "extensions/common/constants.h"
#endif

namespace project_b {

ContentClient::ContentClient() = default;
ContentClient::~ContentClient() = default;

#if BUILDFLAG(PB_EXTENSIONS)
// chrome-extension: is a scheme like https: a real origin, secure, with service workers,
// and not held back by a page's own content security policy.
void ContentClient::AddAdditionalSchemes(Schemes* schemes) {
  schemes->standard_schemes.push_back(extensions::kExtensionScheme);
  schemes->extension_schemes.push_back(extensions::kExtensionScheme);
  schemes->savable_schemes.push_back(extensions::kExtensionScheme);
  schemes->secure_schemes.push_back(extensions::kExtensionScheme);
  schemes->service_worker_schemes.push_back(extensions::kExtensionScheme);
  schemes->cors_enabled_schemes.push_back(extensions::kExtensionScheme);
  schemes->csp_bypassing_schemes.push_back(extensions::kExtensionScheme);
}
#endif

std::string_view ContentClient::GetDataResource(
    int resource_id,
    ui::ResourceScaleFactor scale_factor) {
  return ui::ResourceBundle::GetSharedInstance().GetRawDataResourceForScale(
      resource_id, scale_factor);
}

scoped_refptr<base::RefCountedMemory> ContentClient::GetDataResourceBytes(
    int resource_id) {
  return ui::ResourceBundle::GetSharedInstance().LoadDataResourceBytes(
      resource_id);
}

// Content's own images, such as the default favicon a drag from a page shows when it has
// no picture of its own. Content's default is an empty image, and a drag that showed it
// aborted the app.
gfx::Image& ContentClient::GetNativeImageNamed(int resource_id) {
  return ui::ResourceBundle::GetSharedInstance().GetNativeImageNamed(resource_id);
}

}  // namespace project_b
