#ifndef PROJECT_B_ENGINE_EXTENSIONS_BROWSER_MANAGEMENT_DELEGATE_H_
#define PROJECT_B_ENGINE_EXTENSIONS_BROWSER_MANAGEMENT_DELEGATE_H_

#include <memory>
#include <string>

#include "extensions/browser/api/management/management_api_delegate.h"

namespace project_b {

// What chrome.management asks the browser. A store's page uses that API beside
// chrome.webstorePrivate: to hear that an install finished, to see what is installed,
// and for its Remove button. There are no apps, so everything about launching one and
// its shortcuts answers no.
class ManagementDelegate : public extensions::ManagementAPIDelegate {
 public:
  ManagementDelegate();
  ManagementDelegate(const ManagementDelegate&) = delete;
  ManagementDelegate& operator=(const ManagementDelegate&) = delete;
  ~ManagementDelegate() override;

 private:
  bool LaunchAppFunctionDelegate(
      const extensions::Extension* extension,
      content::BrowserContext* context) const override;
  GURL GetFullLaunchURL(const extensions::Extension* extension) const override;
  extensions::LaunchType GetLaunchType(
      const extensions::ExtensionPrefs* prefs,
      const extensions::Extension* extension) const override;
  std::unique_ptr<extensions::InstallPromptDelegate> SetEnabledFunctionDelegate(
      content::WebContents* web_contents,
      content::BrowserContext* browser_context,
      const extensions::Extension* extension,
      base::OnceCallback<void(bool)> callback) const override;
  void EnableExtension(content::BrowserContext* context,
                       const extensions::ExtensionId& extension_id) const override;
  void DisableExtension(
      content::BrowserContext* context,
      const extensions::Extension* source_extension,
      const extensions::ExtensionId& extension_id,
      extensions::disable_reason::DisableReason disable_reason) const override;
  std::unique_ptr<extensions::UninstallDialogDelegate> UninstallFunctionDelegate(
      extensions::ManagementUninstallFunctionBase* function,
      const extensions::Extension* target_extension,
      bool show_programmatic_uninstall_ui) const override;
  bool UninstallExtension(content::BrowserContext* context,
                          const std::string& transient_extension_id,
                          extensions::UninstallReason reason,
                          std::u16string* error) const override;
  bool CreateAppShortcutFunctionDelegate(
      extensions::ManagementCreateAppShortcutFunction* function,
      const extensions::Extension* extension,
      std::string* error) const override;
  void SetLaunchType(content::BrowserContext* context,
                     const extensions::ExtensionId& extension_id,
                     extensions::LaunchType launch_type) const override;
  std::unique_ptr<extensions::AppForLinkDelegate>
  GenerateAppForLinkFunctionDelegate(
      extensions::ManagementGenerateAppForLinkFunction* function,
      content::BrowserContext* context,
      const std::string& title,
      const GURL& launch_url) const override;
  bool CanContextInstallWebApps(content::BrowserContext* context) const override;
  void InstallOrLaunchReplacementWebApp(
      content::BrowserContext* context,
      const GURL& web_app_url,
      InstallOrLaunchWebAppCallback callback) const override;
  GURL GetIconURL(const extensions::Extension* extension,
                  int icon_size,
                  ExtensionIconSet::Match match,
                  bool grayscale) const override;
  GURL GetEffectiveUpdateURL(const extensions::Extension& extension,
                             content::BrowserContext* context) const override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_BROWSER_MANAGEMENT_DELEGATE_H_
