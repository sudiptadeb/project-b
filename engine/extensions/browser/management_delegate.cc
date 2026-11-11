#include "project_b/engine/extensions/browser/management_delegate.h"

#include <utility>

#include "base/functional/bind.h"
#include "content/public/browser/browser_context.h"
#include "extensions/browser/api/management/management_api.h"
#include "extensions/browser/disable_reason.h"
#include "extensions/browser/extension_registrar.h"
#include "extensions/browser/uninstall_reason.h"
#include "extensions/common/constants.h"
#include "extensions/common/extension.h"
#include "project_b/engine/extensions/browser/extensions.h"
#include "url/gurl.h"

namespace project_b {

ManagementDelegate::ManagementDelegate() = default;
ManagementDelegate::~ManagementDelegate() = default;

bool ManagementDelegate::LaunchAppFunctionDelegate(
    const extensions::Extension* extension,
    content::BrowserContext* context) const {
  return false;
}

GURL ManagementDelegate::GetFullLaunchURL(
    const extensions::Extension* extension) const {
  return GURL();
}

extensions::LaunchType ManagementDelegate::GetLaunchType(
    const extensions::ExtensionPrefs* prefs,
    const extensions::Extension* extension) const {
  return extensions::LaunchType::kRegular;
}

// Chrome asks the user here before it turns on an extension that wants more than it
// was granted. Nothing here withholds a grant, so the answer is yes.
std::unique_ptr<extensions::InstallPromptDelegate>
ManagementDelegate::SetEnabledFunctionDelegate(
    content::WebContents* web_contents,
    content::BrowserContext* browser_context,
    const extensions::Extension* extension,
    base::OnceCallback<void(bool)> callback) const {
  std::move(callback).Run(true);
  return nullptr;
}

void ManagementDelegate::EnableExtension(
    content::BrowserContext* context,
    const extensions::ExtensionId& extension_id) const {
  extensions::ExtensionRegistrar::Get(context)->EnableExtension(extension_id);
}

void ManagementDelegate::DisableExtension(
    content::BrowserContext* context,
    const extensions::Extension* source_extension,
    const extensions::ExtensionId& extension_id,
    extensions::disable_reason::DisableReason disable_reason) const {
  extensions::ExtensionRegistrar::Get(context)->DisableExtension(
      extension_id, {disable_reason});
}

// A store page's Remove button. Chrome asks "Remove?" in a dialog of its own first; we
// ask through Go, as for an install, and remove only on a yes.
std::unique_ptr<extensions::UninstallDialogDelegate>
ManagementDelegate::UninstallFunctionDelegate(
    extensions::ManagementUninstallFunctionBase* function,
    const extensions::Extension* target_extension,
    bool show_programmatic_uninstall_ui) const {
  Extensions::Get()->AskToRemove(
      target_extension,
      base::BindOnce(
          [](scoped_refptr<extensions::ManagementUninstallFunctionBase> function,
             extensions::ExtensionId id, bool accept) {
            std::u16string error;
            if (accept) {
              extensions::ExtensionRegistrar::Get(function->browser_context())
                  ->UninstallExtension(
                      id, extensions::UNINSTALL_REASON_MANAGEMENT_API, &error);
            }
            function->OnExtensionUninstallDialogClosed(accept, error);
          },
          base::WrapRefCounted(function), target_extension->id()));
  return nullptr;
}

bool ManagementDelegate::UninstallExtension(
    content::BrowserContext* context,
    const std::string& transient_extension_id,
    extensions::UninstallReason reason,
    std::u16string* error) const {
  return extensions::ExtensionRegistrar::Get(context)->UninstallExtension(
      transient_extension_id, reason, error);
}

bool ManagementDelegate::CreateAppShortcutFunctionDelegate(
    extensions::ManagementCreateAppShortcutFunction* function,
    const extensions::Extension* extension,
    std::string* error) const {
  *error = "There are no apps in Project B.";
  return false;
}

void ManagementDelegate::SetLaunchType(
    content::BrowserContext* context,
    const extensions::ExtensionId& extension_id,
    extensions::LaunchType launch_type) const {}

std::unique_ptr<extensions::AppForLinkDelegate>
ManagementDelegate::GenerateAppForLinkFunctionDelegate(
    extensions::ManagementGenerateAppForLinkFunction* function,
    content::BrowserContext* context,
    const std::string& title,
    const GURL& launch_url) const {
  return nullptr;
}

bool ManagementDelegate::CanContextInstallWebApps(
    content::BrowserContext* context) const {
  return false;
}

void ManagementDelegate::InstallOrLaunchReplacementWebApp(
    content::BrowserContext* context,
    const GURL& web_app_url,
    InstallOrLaunchWebAppCallback callback) const {
  std::move(callback).Run(InstallOrLaunchWebAppResult::kUnknownError);
}

// Chrome serves an extension's icon to pages at an address of its own. We have none.
GURL ManagementDelegate::GetIconURL(const extensions::Extension* extension,
                                    int icon_size,
                                    ExtensionIconSet::Match match,
                                    bool grayscale) const {
  return GURL();
}

GURL ManagementDelegate::GetEffectiveUpdateURL(
    const extensions::Extension& extension,
    content::BrowserContext* context) const {
  return GURL();
}

}  // namespace project_b
