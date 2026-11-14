#ifndef PROJECT_B_ENGINE_BROWSER_PERMISSIONS_H_
#define PROJECT_B_ENGINE_BROWSER_PERMISSIONS_H_

#include <vector>

#include "content/public/browser/permission_controller_delegate.h"

namespace project_b {

// What a page may do without asking: write to the clipboard, as every browser lets a page
// do (Blink holds it to a focused page). Everything else is denied with no prompt, as it
// was before there was this delegate, until a prompt and a Go manager decide it.
class Permissions : public content::PermissionControllerDelegate {
 public:
  Permissions();
  Permissions(const Permissions&) = delete;
  Permissions& operator=(const Permissions&) = delete;
  ~Permissions() override;

  void RequestPermissionsFromCurrentDocument(
      content::RenderFrameHost* render_frame_host,
      const content::PermissionRequestDescription& request_description,
      base::OnceCallback<void(const std::vector<content::PermissionResult>&)>
          callback) override;
  blink::mojom::PermissionStatus GetPermissionStatus(
      const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
      const GURL& requesting_origin,
      const GURL& embedding_origin) override;
  content::PermissionResult GetPermissionResultForOriginWithoutContext(
      const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
      const url::Origin& requesting_origin,
      const url::Origin& embedding_origin) override;
  content::PermissionResult GetPermissionResultForCurrentDocument(
      const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
      content::RenderFrameHost* render_frame_host,
      bool should_include_device_status) override;
  content::PermissionResult GetPermissionResultForWorker(
      const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
      content::RenderProcessHost* render_process_host,
      const GURL& worker_origin) override;
  content::PermissionResult GetPermissionResultForEmbeddedRequester(
      const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
      content::RenderFrameHost* render_frame_host,
      const url::Origin& overridden_origin) override;
  void ResetPermission(blink::PermissionType permission,
                       const GURL& requesting_origin,
                       const GURL& embedding_origin) override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_PERMISSIONS_H_
