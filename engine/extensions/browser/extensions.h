#ifndef PROJECT_B_ENGINE_EXTENSIONS_BROWSER_EXTENSIONS_H_
#define PROJECT_B_ENGINE_EXTENSIONS_BROWSER_EXTENSIONS_H_

#include <cstdint>
#include <map>
#include <memory>
#include <string>

#include "base/files/file_path.h"
#include "base/memory/raw_ptr.h"
#include "base/memory/weak_ptr.h"
#include "extensions/browser/extension_install_prompt_client.h"
#include "extensions/browser/extension_registrar.h"
#include "extensions/browser/extension_registry_observer.h"
#include "project_b/engine/abi/pb_extensions_abi.h"
#include "ui/gfx/geometry/rect_f.h"
#include "url/gurl.h"

class PrefService;

namespace custom_handlers {
class ProtocolHandlerRegistry;
}

namespace extensions {
class Blocklist;
class CrxInstallError;
class CrxInstaller;
class InstallStageTracker;
class InstallTracker;
}  // namespace extensions

namespace project_b {

class ExtensionPopup;
class ExtensionsBrowserClient;

// Every tab is in the one window, which Chrome's API knows by this id.
inline constexpr int kWindowId = 1;

// Opens a URL in a new tab, as a page asking for one does: the engine makes the tab and
// tells Go.
void OpenTab(const GURL& url);

// The extension runtime, as Go sees it: Chromium's //extensions for one profile. It is
// what PBExtensionCalls reaches, and it is where //extensions' registrar asks about
// installs. What is installed is kept in the profile by //extensions itself
// (extension-prefs.json and the Extensions folder), and loaded again here at start.
class Extensions : public extensions::ExtensionRegistryObserver,
                   public extensions::ExtensionRegistrar::Delegate {
 public:
  explicit Extensions(content::BrowserContext* browser_context);
  Extensions(const Extensions&) = delete;
  Extensions& operator=(const Extensions&) = delete;
  ~Extensions() override;

  static Extensions* Get();
  // The table handed to Go. Lives as long as the process.
  static const PBExtensionCalls* Calls();

  // What Go asks, as engine.Extensions in core/engine/extensions.go describes.
  void Install(const base::FilePath& path);
  void AnswerInstallPrompt(int64_t prompt, bool accept);
  void LoadUnpacked(const base::FilePath& dir);
  void Uninstall(const std::string& id);
  void SetEnabled(const std::string& id, bool on);
  std::string List();
  void DispatchEvent(const std::string& id,
                     const std::string& event,
                     const std::string& args_json);
  void OpenPopup(const std::string& id, const GURL& url, const gfx::RectF& anchor);
  void ClosePopup();
  void ChoosePath(int kind);

  // Development only, for the driver, which can't press the buttons of the system's
  // panel: the next ChoosePath answers with this path instead of asking.
  void ChooseForDriver(const base::FilePath& path);
  // The open popup's page, or null.
  content::WebContents* popup_contents() const;

  // What //extensions' installer asks the browser for, beside the registry of
  // schemes extensions handle. Each is Chromium's own class, with nothing behind it
  // that could say no: the blocklist has no list.
  custom_handlers::ProtocolHandlerRegistry* protocol_handlers() const {
    return protocol_handlers_.get();
  }
  extensions::Blocklist* blocklist() const { return blocklist_.get(); }
  extensions::InstallTracker* install_tracker() const {
    return install_tracker_.get();
  }
  extensions::InstallStageTracker* install_stage_tracker() const {
    return install_stage_tracker_.get();
  }

  // The installer's question, which goes to Go. done runs with the answer.
  void AskToInstall(
      const extensions::Extension* extension,
      extensions::ExtensionInstallPromptClient::DoneCallback done);

 private:
  // What an extension shows in our UI that takes reading files: its icons, as data: URLs.
  struct Icons {
    std::string icon;
    std::string action_icon;
  };

  void LoadInstalled();
  void OnPackageInstalled(
      const base::FilePath& path,
      scoped_refptr<extensions::CrxInstaller> installer,
      const std::optional<extensions::CrxInstallError>& error);
  void OnUnpackedLoaded(const base::FilePath& dir,
                        const extensions::Extension* extension,
                        const base::FilePath& path,
                        const std::u16string& error);
  void InstallDone(const base::FilePath& path,
                   const std::string& id,
                   const std::string& error,
                   bool cancelled);
  void ReadIcons(const extensions::Extension* extension);
  void OnIconsRead(const std::string& id, Icons icons);
  void OnPromptIconRead(int64_t prompt, base::DictValue details, Icons icons);
  void PathChosen(const base::FilePath& path);
  void Changed();

  // extensions::ExtensionRegistryObserver:
  void OnExtensionLoaded(content::BrowserContext* browser_context,
                         const extensions::Extension* extension) override;
  void OnExtensionUnloaded(content::BrowserContext* browser_context,
                           const extensions::Extension* extension,
                           extensions::UnloadedExtensionReason reason) override;
  void OnExtensionUninstalled(content::BrowserContext* browser_context,
                              const extensions::Extension* extension,
                              extensions::UninstallReason reason) override;
  // The observer has a notice of the same name as the registrar's question below.
  using extensions::ExtensionRegistryObserver::OnExtensionInstalled;

  // extensions::ExtensionRegistrar::Delegate:
  void PreAddExtension(const extensions::Extension* extension,
                       const extensions::Extension* old_extension) override;
  void OnAddNewOrUpdatedExtension(
      const extensions::Extension* extension) override;
  void PostActivateExtension(
      scoped_refptr<const extensions::Extension> extension) override;
  void PostDeactivateExtension(
      scoped_refptr<const extensions::Extension> extension) override;
  void PreUninstallExtension(
      scoped_refptr<const extensions::Extension> extension) override;
  void PostUninstallExtension(
      scoped_refptr<const extensions::Extension> extension,
      base::OnceClosure done_callback) override;
  void LoadExtensionForReload(const extensions::ExtensionId& extension_id,
                              const base::FilePath& path) override;
  void LoadExtensionForReloadWithQuietFailure(
      const extensions::ExtensionId& extension_id,
      const base::FilePath& path) override;
  void ShowExtensionDisabledError(const extensions::Extension* extension,
                                  bool is_remote_install) override;
  bool CanEnableExtension(const extensions::Extension* extension) override;
  bool CanDisableExtension(const extensions::Extension* extension) override;
  void GrantActivePermissions(const extensions::Extension* extension) override;
  void UpdateExternalExtensionAlert() override;
  void OnExtensionInstalled(const extensions::Extension* extension,
                            const syncer::StringOrdinal& page_ordinal,
                            int install_flags,
                            base::DictValue ruleset_install_prefs) override;

  const raw_ptr<content::BrowserContext> browser_context_;
  std::unique_ptr<ExtensionsBrowserClient> client_;
  std::unique_ptr<PrefService> prefs_;
  std::unique_ptr<custom_handlers::ProtocolHandlerRegistry> protocol_handlers_;
  std::unique_ptr<extensions::Blocklist> blocklist_;
  std::unique_ptr<extensions::InstallTracker> install_tracker_;
  std::unique_ptr<extensions::InstallStageTracker> install_stage_tracker_;
  std::map<std::string, Icons> icons_;
  // The install prompts Go hasn't answered yet.
  std::map<int64_t, extensions::ExtensionInstallPromptClient::DoneCallback>
      prompts_;
  int64_t next_prompt_ = 1;
  std::unique_ptr<ExtensionPopup> popup_;
  base::FilePath driver_choice_;
  base::WeakPtrFactory<Extensions> weak_factory_{this};
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_BROWSER_EXTENSIONS_H_
