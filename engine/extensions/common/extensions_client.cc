#include "project_b/engine/extensions/common/extensions_client.h"

#include <memory>
#include <string_view>

#include "base/command_line.h"
#include "extensions/common/alias.h"
#include "extensions/common/core_extensions_api_provider.h"
#include "extensions/common/extension_urls.h"
#include "extensions/common/extensions_api_provider.h"
#include "extensions/common/mojom/api_permission_id.mojom-shared.h"
#include "extensions/common/permissions/api_permission.h"
#include "extensions/common/permissions/permissions_info.h"
#include "extensions/common/switches.h"
#include "extensions/common/url_pattern_set.h"
#include "extensions/common/webstore_override.h"
#include "project_b/engine/extension_api_features.h"
#include "project_b/engine/extension_permission_features.h"
#include "project_b/engine/generated_schemas.h"

namespace project_b {

namespace {

// The browser's own APIs: tabs, windows, action and commands, which Go answers, and the
// permissions Chrome declares outside //extensions.
class APIProvider : public extensions::ExtensionsAPIProvider {
 public:
  void AddAPIFeatures(extensions::FeatureProvider* provider) override {
    AddProjectBAPIFeatures(provider);
  }
  void AddManifestFeatures(extensions::FeatureProvider* provider) override {}
  void AddPermissionFeatures(extensions::FeatureProvider* provider) override {
    AddProjectBPermissionFeatures(provider);
  }
  void AddBehaviorFeatures(extensions::FeatureProvider* provider) override {}
  // Only Chromium's own tests read the features as JSON.
  void AddAPIJSONSources(
      extensions::JSONFeatureProviderSource* json_source) override {}
  bool IsAPISchemaGenerated(const std::string& name) override {
    return api::ProjectBGeneratedSchemas::IsGenerated(name);
  }
  std::string_view GetAPISchema(const std::string& name) override {
    return api::ProjectBGeneratedSchemas::Get(name);
  }
  void RegisterPermissions(
      extensions::PermissionsInfo* permissions_info) override {
    using extensions::APIPermissionInfo;
    using extensions::mojom::APIPermissionID;
    static constexpr APIPermissionInfo::InitInfo permissions[] = {
        {APIPermissionID::kScripting, "scripting",
         APIPermissionInfo::kFlagRequiresManagementUIWarning},
        {APIPermissionID::kTab, "tabs",
         APIPermissionInfo::kFlagRequiresManagementUIWarning},
    };
    // "windows" has always meant "tabs".
    static constexpr extensions::Alias aliases[] = {
        extensions::Alias("windows", "tabs")};
    permissions_info->RegisterPermissions(permissions, aliases);
  }
  void RegisterManifestHandlers(
      extensions::ManifestHandlerRegistry* registry) override {}
};

}  // namespace

ExtensionsClient::ExtensionsClient() {
  AddAPIProvider(std::make_unique<extensions::CoreExtensionsAPIProvider>());
  AddAPIProvider(std::make_unique<APIProvider>());
  // chrome.webstorePrivate, which a store's page installs with, belongs to the Chrome
  // Web Store's own origin. This is Chromium's rule for giving it to one more: the
  // origin --apps-gallery-url names, which is how our mock store gets it.
  SetFeatureDelegatedAvailabilityCheckMap(
      extensions::webstore_override::CreateAvailabilityCheckMap());
}

ExtensionsClient::~ExtensionsClient() = default;

extensions::PermissionMessages ExtensionsClient::NoMessages::GetPermissionMessages(
    const extensions::PermissionIDSet& permissions) const {
  return {};
}

// An update that asks for more is installed like any other. docs/extensions.md lists
// this among the checks to discuss.
bool ExtensionsClient::NoMessages::IsPrivilegeIncrease(
    const extensions::PermissionSet& granted_permissions,
    const extensions::PermissionSet& requested_permissions,
    extensions::Manifest::Type extension_type) const {
  return false;
}

extensions::PermissionIDSet ExtensionsClient::NoMessages::GetAllPermissionIDs(
    const extensions::PermissionSet& permissions,
    extensions::Manifest::Type extension_type) const {
  return {};
}

void ExtensionsClient::Initialize() {
  InitializeWebStoreUrls(base::CommandLine::ForCurrentProcess());
}

void ExtensionsClient::InitializeWebStoreUrls(base::CommandLine* command_line) {
  webstore_update_url_ = extension_urls::GetDefaultWebstoreUpdateUrl();
  if (command_line->HasSwitch(extensions::switches::kAppsGalleryURL)) {
    webstore_base_url_ = new_webstore_base_url_ = GURL(
        command_line->GetSwitchValueASCII(extensions::switches::kAppsGalleryURL));
  } else {
    webstore_base_url_ = GURL(extension_urls::kChromeWebstoreBaseURL);
    new_webstore_base_url_ = GURL(extension_urls::kNewChromeWebstoreBaseURL);
  }
}

const extensions::PermissionMessageProvider&
ExtensionsClient::GetPermissionMessageProvider() const {
  return no_messages_;
}

const std::string ExtensionsClient::GetProductName() {
  return "Project B";
}

void ExtensionsClient::FilterHostPermissions(
    const extensions::URLPatternSet& hosts,
    extensions::URLPatternSet* new_hosts,
    extensions::PermissionIDSet* permissions) const {
  for (const URLPattern& pattern : hosts) {
    new_hosts->AddPattern(pattern);
  }
}

void ExtensionsClient::SetScriptingAllowlist(
    const ScriptingAllowlist& allowlist) {
  scripting_allowlist_ = allowlist;
}

const ExtensionsClient::ScriptingAllowlist&
ExtensionsClient::GetScriptingAllowlist() const {
  return scripting_allowlist_;
}

extensions::URLPatternSet ExtensionsClient::GetPermittedChromeSchemeHosts(
    const extensions::Extension* extension,
    const extensions::APIPermissionSet& api_permissions) const {
  return {};
}

bool ExtensionsClient::IsScriptableURL(const GURL& url,
                                       std::string* error) const {
  return true;
}

const GURL& ExtensionsClient::GetWebstoreBaseURL() const {
  return webstore_base_url_;
}

const GURL& ExtensionsClient::GetNewWebstoreBaseURL() const {
  return new_webstore_base_url_;
}

const GURL& ExtensionsClient::GetWebstoreUpdateURL() const {
  return webstore_update_url_;
}

bool ExtensionsClient::IsBlocklistUpdateURL(const GURL& url) const {
  return false;
}

}  // namespace project_b
