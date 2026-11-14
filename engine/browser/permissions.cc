#include "project_b/engine/browser/permissions.h"

#include <utility>

#include "base/functional/callback.h"
#include "content/public/browser/permission_request_description.h"
#include "content/public/browser/permission_result.h"
#include "third_party/blink/public/common/permissions/permission_utils.h"
#include "url/gurl.h"
#include "url/origin.h"

namespace project_b {

namespace {

blink::mojom::PermissionStatus StatusOf(
    const blink::mojom::PermissionDescriptorPtr& descriptor) {
  return blink::PermissionDescriptorToPermissionType(descriptor) ==
                 blink::PermissionType::CLIPBOARD_SANITIZED_WRITE
             ? blink::mojom::PermissionStatus::GRANTED
             : blink::mojom::PermissionStatus::DENIED;
}

}  // namespace

Permissions::Permissions() = default;
Permissions::~Permissions() = default;

void Permissions::RequestPermissionsFromCurrentDocument(
    content::RenderFrameHost* render_frame_host,
    const content::PermissionRequestDescription& request_description,
    base::OnceCallback<void(const std::vector<content::PermissionResult>&)>
        callback) {
  std::vector<content::PermissionResult> results;
  for (const auto& permission : request_description.permissions) {
    results.emplace_back(StatusOf(permission));
  }
  std::move(callback).Run(results);
}

blink::mojom::PermissionStatus Permissions::GetPermissionStatus(
    const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
    const GURL& requesting_origin,
    const GURL& embedding_origin) {
  return StatusOf(permission_descriptor);
}

content::PermissionResult Permissions::GetPermissionResultForOriginWithoutContext(
    const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
    const url::Origin& requesting_origin,
    const url::Origin& embedding_origin) {
  return content::PermissionResult(StatusOf(permission_descriptor));
}

content::PermissionResult Permissions::GetPermissionResultForCurrentDocument(
    const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
    content::RenderFrameHost* render_frame_host,
    bool should_include_device_status) {
  return content::PermissionResult(StatusOf(permission_descriptor));
}

content::PermissionResult Permissions::GetPermissionResultForWorker(
    const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
    content::RenderProcessHost* render_process_host,
    const GURL& worker_origin) {
  return content::PermissionResult(StatusOf(permission_descriptor));
}

content::PermissionResult Permissions::GetPermissionResultForEmbeddedRequester(
    const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
    content::RenderFrameHost* render_frame_host,
    const url::Origin& overridden_origin) {
  return content::PermissionResult(StatusOf(permission_descriptor));
}

void Permissions::ResetPermission(blink::PermissionType permission,
                                  const GURL& requesting_origin,
                                  const GURL& embedding_origin) {}

}  // namespace project_b
