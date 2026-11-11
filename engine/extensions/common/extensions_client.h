#ifndef PROJECT_B_ENGINE_EXTENSIONS_COMMON_EXTENSIONS_CLIENT_H_
#define PROJECT_B_ENGINE_EXTENSIONS_COMMON_EXTENSIONS_CLIENT_H_

#include <string>

#include "extensions/common/extensions_client.h"
#include "extensions/common/permissions/permission_message_provider.h"
#include "url/gurl.h"

namespace project_b {

// What //extensions asks its embedder in every process: which APIs exist, and a few
// facts about the product. Ours adds the browser's own APIs to the core ones.
class ExtensionsClient : public extensions::ExtensionsClient {
 public:
  ExtensionsClient();
  ExtensionsClient(const ExtensionsClient&) = delete;
  ExtensionsClient& operator=(const ExtensionsClient&) = delete;
  ~ExtensionsClient() override;

 private:
  // No warning sentences: the install prompt shows the manifest's permission names.
  class NoMessages : public extensions::PermissionMessageProvider {
    extensions::PermissionMessages GetPermissionMessages(
        const extensions::PermissionIDSet& permissions) const override;
    bool IsPrivilegeIncrease(
        const extensions::PermissionSet& granted_permissions,
        const extensions::PermissionSet& requested_permissions,
        extensions::Manifest::Type extension_type) const override;
    extensions::PermissionIDSet GetAllPermissionIDs(
        const extensions::PermissionSet& permissions,
        extensions::Manifest::Type extension_type) const override;
  };

  void Initialize() override;
  void InitializeWebStoreUrls(base::CommandLine* command_line) override;
  const extensions::PermissionMessageProvider& GetPermissionMessageProvider()
      const override;
  const std::string GetProductName() override;
  void FilterHostPermissions(
      const extensions::URLPatternSet& hosts,
      extensions::URLPatternSet* new_hosts,
      extensions::PermissionIDSet* permissions) const override;
  void SetScriptingAllowlist(const ScriptingAllowlist& allowlist) override;
  const ScriptingAllowlist& GetScriptingAllowlist() const override;
  extensions::URLPatternSet GetPermittedChromeSchemeHosts(
      const extensions::Extension* extension,
      const extensions::APIPermissionSet& api_permissions) const override;
  bool IsScriptableURL(const GURL& url, std::string* error) const override;
  const GURL& GetWebstoreBaseURL() const override;
  const GURL& GetNewWebstoreBaseURL() const override;
  const GURL& GetWebstoreUpdateURL() const override;
  bool IsBlocklistUpdateURL(const GURL& url) const override;

  const NoMessages no_messages_;
  ScriptingAllowlist scripting_allowlist_;
  // There is no web store: installs come from the store Go names. These stay empty.
  const GURL no_url_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_COMMON_EXTENSIONS_CLIENT_H_
