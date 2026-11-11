#include "project_b/engine/extensions/browser/extensions.h"

#include <stdlib.h>
#include <string.h>

#include <utility>

#include "base/barrier_closure.h"
#include "base/base64.h"
#include "base/check.h"
#include "base/command_line.h"
#include "base/containers/fixed_flat_map.h"
#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/functional/callback_helpers.h"
#include "base/json/json_reader.h"
#include "base/json/json_writer.h"
#include "base/logging.h"
#include "base/memory/ptr_util.h"
#include "base/no_destructor.h"
#include "base/strings/strcat.h"
#include "base/strings/utf_string_conversions.h"
#include "base/task/thread_pool.h"
#include "base/version_info/version_info.h"
#include "components/crx_file/crx_verifier.h"
#include "components/custom_handlers/protocol_handler_registry.h"
#include "components/enterprise/browser/promotion/promotion_prefs.h"
#include "components/keyed_service/content/browser_context_dependency_manager.h"
#include "components/pref_registry/pref_registry_syncable.h"
#include "components/prefs/json_pref_store.h"
#include "components/prefs/pref_service.h"
#include "components/prefs/pref_service_factory.h"
#include "components/update_client/update_query_params.h"
#include "components/update_client/update_query_params_delegate.h"
#include "components/user_prefs/user_prefs.h"
#include "content/public/browser/browser_context.h"
#include "content/public/browser/storage_partition.h"
#include "extensions/browser/api/runtime/runtime_api.h"
#include "extensions/browser/api/storage/storage_frontend.h"
#include "extensions/browser/blocklist.h"
#include "extensions/browser/browser_context_keyed_service_factories.h"
#include "extensions/browser/crx_file_info.h"
#include "extensions/browser/crx_installer.h"
#include "extensions/browser/disable_reason.h"
#include "extensions/browser/event_router.h"
#include "extensions/browser/extension_file_task_runner.h"
#include "extensions/browser/extension_prefs.h"
#include "extensions/browser/extension_registry.h"
#include "extensions/browser/extension_system.h"
#include "extensions/browser/extension_util.h"
#include "extensions/browser/forced_extensions/install_stage_tracker.h"
#include "extensions/browser/install/crx_install_error.h"
#include "extensions/browser/install_tracker.h"
#include "extensions/browser/install_verifier.h"
#include "extensions/browser/permissions/permissions_updater.h"
#include "extensions/browser/permissions_manager.h"
#include "extensions/browser/shared_module_service.h"
#include "extensions/browser/uninstall_reason.h"
#include "extensions/browser/unpacked_installer.h"
#include "extensions/common/api/commands/commands_handler.h"
#include "extensions/common/api/extension_action/action_info.h"
#include "extensions/common/extension.h"
#include "extensions/common/extension_resource.h"
#include "extensions/common/file_util.h"
#include "extensions/common/manifest.h"
#include "extensions/common/manifest_handlers/description_info.h"
#include "extensions/common/manifest_handlers/icons_handler.h"
#include "extensions/common/manifest_handlers/options_page_info.h"
#include "extensions/common/permissions/permissions_data.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/extensions/browser/choose_path.h"
#include "project_b/engine/extensions/browser/extension_system.h"
#include "project_b/engine/extensions/browser/extensions_browser_client.h"
#include "project_b/engine/extensions/browser/popup.h"
#include "third_party/skia/include/core/SkBitmap.h"
#include "ui/gfx/codec/png_codec.h"

namespace extensions {

// //extensions lets only a class of this name make its shared module service, and
// leaves the class to the embedder. Chrome's is a keyed service factory; ours makes one.
class SharedModuleServiceFactory {
 public:
  static std::unique_ptr<SharedModuleService> Create(
      content::BrowserContext* browser_context) {
    return base::WrapUnique(new SharedModuleService(browser_context));
  }
};

}  // namespace extensions

namespace project_b {

namespace {

Extensions* g_extensions = nullptr;

// Whoever installs asks its client first; ours passes the question to Go. The
// installer of a package asks with ConfirmInstall, once it has unpacked it. A store's
// page asks with ShowInstallDialog, before anything is downloaded, with the manifest
// and the icon the page gave.
class PromptClient : public extensions::ExtensionInstallPromptClient {
  void OnInstallSuccess(scoped_refptr<const extensions::Extension> extension,
                        SkBitmap* icon) override {}
  void OnInstallFailure(const extensions::CrxInstallError& error) override {}
  void SetUseAppInstalledBubble(bool use_bubble) override {}
  void SetSkipPostInstallUI(bool skip_ui) override {}
  void ConfirmInstall(DoneCallback install_callback,
                      const extensions::Extension* extension) override {
    Extensions::Get()->AskToInstall(extension, std::string(),
                                    std::move(install_callback));
  }
  // The installer asks these of a client it was told is updating from Chrome's own
  // extensions page or its web store, which ours never is.
  void ConfirmReEnable(DoneCallback install_callback,
                       const extensions::Extension* extension,
                       content::BrowserContext* browser_context) override {
    NOTREACHED();
  }
  void ConfirmPermissions(
      DoneCallback done_callback,
      const extensions::Extension* extension,
      std::unique_ptr<const extensions::PermissionSet> custom_permissions)
      override {
    NOTREACHED();
  }
  void ShowInstallDialog(DoneCallback install_callback,
                         const extensions::Extension* extension,
                         const SkBitmap* icon) override {
    std::string data_url;
    if (icon && !icon->drawsNothing()) {
      std::optional<std::vector<uint8_t>> png =
          gfx::PNGCodec::EncodeBGRASkBitmap(*icon,
                                            /*discard_transparency=*/false);
      data_url = base::StrCat({"data:image/png;base64,", base::Base64Encode(*png)});
    }
    Extensions::Get()->AskToInstall(extension, data_url,
                                    std::move(install_callback));
  }
};

// The Web Store's download address wants to know which browser asks: without the
// version it has no package to give. This says what Chrome says, for the Chromium we
// carry.
class StoreQueryParams : public update_client::UpdateQueryParamsDelegate {
  std::string GetExtraParams() override {
    return base::StrCat({"&prodchannel=stable&prodversion=",
                         version_info::GetVersionNumber(), "&lang=en-US"});
  }
};

// A scheme an extension handles is handled inside the browser only: nothing is told
// to macOS, so the browser never becomes the system's handler for it.
class ProtocolHandlerDelegate
    : public custom_handlers::ProtocolHandlerRegistry::Delegate {
  void RegisterExternalHandler(const std::string& protocol) override {}
  bool IsExternalHandlerRegistered(const std::string& protocol) override {
    return false;
  }
  void RegisterWithOSAsDefaultClient(
      const std::string& protocol,
      DefaultClientCallback callback) override {
    std::move(callback).Run(false);
  }
  void CheckDefaultClientWithOS(
      const std::string& protocol,
      DefaultClientCallback callback) override {
    std::move(callback).Run(false);
  }
  bool ShouldRemoveHandlersNotInOS() override { return false; }
};

// Reads a picture in an extension's folder into a data: URL. Finding the file touches
// the disk too, so all of it runs off the main thread.
std::string ReadAsDataURL(const extensions::ExtensionResource& resource) {
  std::string bytes;
  if (resource.empty() ||
      !base::ReadFileToString(resource.GetFilePath(), &bytes)) {
    return std::string();
  }
  return base::StrCat({"data:image/png;base64,", base::Base64Encode(bytes)});
}

// The icon nearest a size, bigger rather than smaller.
extensions::ExtensionResource Icon(const extensions::Extension* extension,
                                   const ExtensionIconSet& icons,
                                   int size) {
  std::string path = icons.Get(size, ExtensionIconSet::Match::kBigger);
  if (path.empty()) {
    path = icons.Get(size, ExtensionIconSet::Match::kSmaller);
  }
  return path.empty() ? extensions::ExtensionResource()
                      : extension->GetResource(path);
}

// The names a manifest lists under a key, as it wrote them.
void AppendStrings(const extensions::Extension* extension,
                   const char* key,
                   base::ListValue* out) {
  const base::ListValue* list =
      extension->manifest()->available_values().FindList(key);
  if (!list) {
    return;
  }
  for (const base::Value& value : *list) {
    if (value.is_string()) {
      out->Append(value.GetString());
    }
  }
}

scoped_refptr<extensions::Extension> LoadFromFolder(
    const base::FilePath& path,
    extensions::mojom::ManifestLocation location,
    int flags) {
  std::u16string error;
  scoped_refptr<extensions::Extension> extension =
      extensions::file_util::LoadExtension(path, location, flags, &error);
  if (!extension) {
    LOG(ERROR) << "The extension in " << path << " did not load: " << error;
  }
  return extension;
}

char* CopyForGo(const std::string& s) {
  return strdup(s.c_str());
}

}  // namespace

void OpenTab(const GURL& url) {
  Engine* engine = Engine::Get();
  const int64_t tab = engine->CreateTab();
  Engine::TabOpened(0, tab, url, true);
  engine->LoadURL(tab, url.spec().c_str());
}

Extensions::Extensions(content::BrowserContext* browser_context)
    : browser_context_(browser_context) {
  DCHECK(!g_extensions);
  g_extensions = this;

  static base::NoDestructor<StoreQueryParams> store_query_params;
  update_client::UpdateQueryParams::SetDelegate(store_query_params.get());

  client_ = std::make_unique<ExtensionsBrowserClient>(browser_context);
  extensions::ExtensionsBrowserClient::Set(client_.get());
  extensions::EnsureBrowserContextKeyedServiceFactoriesBuilt();
  ExtensionSystemFactory::GetInstance();

  // //extensions keeps what is installed, and what each extension was granted, in
  // prefs of its own: one file in the profile, read here before anything asks.
  auto registry = base::MakeRefCounted<user_prefs::PrefRegistrySyncable>();
  extensions::ExtensionPrefs::RegisterProfilePrefs(registry.get());
  extensions::PermissionsManager::RegisterProfilePrefs(registry.get());
  extensions::RuntimeAPI::RegisterPrefs(registry.get());
  custom_handlers::ProtocolHandlerRegistry::RegisterProfilePrefs(
      registry.get());
  // The store's page asks its API about an enterprise banner, which keeps its answer here.
  enterprise_promotion::RegisterProfilePrefs(registry.get());
  PrefServiceFactory factory;
  factory.set_user_prefs(base::MakeRefCounted<JsonPrefStore>(
      browser_context->GetPath().Append("extension-prefs.json")));
  prefs_ = factory.Create(registry);
  user_prefs::UserPrefs::Set(browser_context, prefs_.get());
  protocol_handlers_ = custom_handlers::ProtocolHandlerRegistry::Create(
      prefs_.get(), std::make_unique<ProtocolHandlerDelegate>());

  BrowserContextDependencyManager::GetInstance()->CreateBrowserContextServices(
      browser_context);
  blocklist_ = std::make_unique<extensions::Blocklist>(browser_context);
  install_tracker_ = std::make_unique<extensions::InstallTracker>(
      browser_context, extensions::ExtensionPrefs::Get(browser_context));
  install_stage_tracker_ =
      std::make_unique<extensions::InstallStageTracker>(browser_context);
  install_verifier_ = std::make_unique<extensions::InstallVerifier>(
      extensions::ExtensionPrefs::Get(browser_context), browser_context);
  shared_modules_ =
      extensions::SharedModuleServiceFactory::Create(browser_context);
  extensions::ExtensionSystem* system =
      extensions::ExtensionSystem::Get(browser_context);
  system->InitForRegularProfile(true);
  extensions::ExtensionRegistrar::Get(browser_context)
      ->Init(this, true, base::CommandLine::ForCurrentProcess(),
             browser_context->GetPath().Append("Extensions"),
             browser_context->GetPath().Append("UnpackedExtensions"));
  extensions::ExtensionRegistry::Get(browser_context)->AddObserver(this);
  LoadInstalled();
  static_cast<ExtensionSystem*>(system)->SetReady();
}

Extensions::~Extensions() {
  popup_.reset();
  extensions::ExtensionRegistry::Get(browser_context_)->RemoveObserver(this);
  protocol_handlers_->Shutdown();
  shared_modules_.reset();
  install_verifier_.reset();
  install_stage_tracker_.reset();
  install_tracker_.reset();
  blocklist_.reset();
  BrowserContextDependencyManager::GetInstance()->DestroyBrowserContextServices(
      browser_context_);
  extensions::ExtensionsBrowserClient::Set(nullptr);
  g_extensions = nullptr;
}

Extensions* Extensions::Get() {
  return g_extensions;
}

// A packaged extension comes back from the manifest //extensions saved when it was
// installed. An unpacked one is read from its folder again, since its files may have
// changed.
void Extensions::LoadInstalled() {
  auto* prefs = extensions::ExtensionPrefs::Get(browser_context_);
  auto* registrar = extensions::ExtensionRegistrar::Get(browser_context_);
  for (const auto& record : prefs->GetInstalledExtensionsInfo()) {
    const int flags = prefs->GetCreationFlags(record.extension_id);
    if (extensions::Manifest::IsUnpackedLocation(record.extension_location)) {
      extensions::GetExtensionFileTaskRunner()->PostTaskAndReplyWithResult(
          FROM_HERE,
          base::BindOnce(&LoadFromFolder, record.extension_path,
                         record.extension_location, flags),
          base::BindOnce(
              [](base::WeakPtr<Extensions> self,
                 scoped_refptr<extensions::Extension> extension) {
                if (self && extension) {
                  extensions::ExtensionRegistrar::Get(self->browser_context_)
                      ->AddExtension(extension);
                }
              },
              weak_factory_.GetWeakPtr()));
      continue;
    }
    std::u16string error;
    scoped_refptr<extensions::Extension> extension =
        extensions::Extension::Create(
            record.extension_path, record.extension_location,
            *record.extension_manifest,
            flags | extensions::Extension::REQUIRE_KEY, &error);
    if (!extension) {
      LOG(ERROR) << "The extension " << record.extension_id
                 << " did not load: " << error;
      continue;
    }
    registrar->AddExtension(extension);
  }
}

// The package is checked as a plain CRX3: its own signature, and nothing about who
// published it. docs/extensions.md lists what that leaves unchecked.
void Extensions::Install(const base::FilePath& path) {
  scoped_refptr<extensions::CrxInstaller> installer =
      extensions::CrxInstaller::Create(browser_context_,
                                       std::make_unique<PromptClient>());
  installer->set_off_store_install_allow_reason(
      extensions::CrxInstaller::OffStoreInstallAllowedBecausePref);
  installer->set_install_immediately(true);
  installer->AddInstallerCallback(
      base::BindOnce(&Extensions::OnPackageInstalled,
                     weak_factory_.GetWeakPtr(), path, installer));
  installer->InstallCrxFile(
      extensions::CRXFileInfo(path, crx_file::VerifierFormat::CRX3));
}

void Extensions::OnPackageInstalled(
    const base::FilePath& path,
    scoped_refptr<extensions::CrxInstaller> installer,
    const std::optional<extensions::CrxInstallError>& error) {
  if (!error) {
    InstallDone(path, installer->extension()->id(), std::string(), false);
    return;
  }
  const bool cancelled =
      error->detail() == extensions::CrxInstallErrorDetail::USER_CANCELED;
  InstallDone(path, std::string(),
              cancelled ? std::string() : base::UTF16ToUTF8(error->message()),
              cancelled);
}

void Extensions::LoadUnpacked(const base::FilePath& dir) {
  scoped_refptr<extensions::UnpackedInstaller> installer =
      extensions::UnpackedInstaller::Create(browser_context_);
  installer->set_be_noisy_on_failure(false);
  installer->set_completion_callback(base::BindOnce(
      &Extensions::OnUnpackedLoaded, weak_factory_.GetWeakPtr(), dir));
  installer->Load(dir);
}

void Extensions::OnUnpackedLoaded(const base::FilePath& dir,
                                  const extensions::Extension* extension,
                                  const base::FilePath& path,
                                  const std::u16string& error) {
  InstallDone(dir, extension ? extension->id() : std::string(),
              base::UTF16ToUTF8(error), false);
}

void Extensions::InstallDone(const base::FilePath& path,
                             const std::string& id,
                             const std::string& error,
                             bool cancelled) {
  hooks::Args args;
  args.Str(path.value());
  args.Str(id);
  args.Str(error);
  args.Bool(cancelled);
  hooks::Fire(PB_EVENT_EXTENSION_INSTALL_DONE, args);
}

std::unique_ptr<extensions::ExtensionInstallPromptClient>
Extensions::CreateInstallPrompt() {
  return std::make_unique<PromptClient>();
}

// The prompt carries what the manifest asks for, as it wrote it, and the icon: the
// one the asker has, or else the one in the unpacked package.
void Extensions::AskToInstall(
    const extensions::Extension* extension,
    const std::string& icon,
    extensions::ExtensionInstallPromptClient::DoneCallback done) {
  const int64_t prompt = next_prompt_++;
  prompts_[prompt] = base::BindOnce(
      [](extensions::ExtensionInstallPromptClient::DoneCallback done,
         bool accept) {
        using Client = extensions::ExtensionInstallPromptClient;
        std::move(done).Run(Client::DoneCallbackPayload(
            accept ? Client::Result::ACCEPTED
                   : Client::Result::USER_CANCELED));
      },
      std::move(done));

  base::DictValue details;
  details.Set("id", extension->id());
  details.Set("name", extension->name());
  details.Set("version", extension->VersionString());
  base::ListValue permissions;
  AppendStrings(extension, "permissions", &permissions);
  AppendStrings(extension, "host_permissions", &permissions);
  details.Set("permissions", std::move(permissions));

  if (!icon.empty()) {
    OnPromptIconRead(prompt, std::move(details), Icons{icon, {}});
    return;
  }
  base::ThreadPool::PostTaskAndReplyWithResult(
      FROM_HERE, {base::MayBlock()},
      base::BindOnce(
          [](extensions::ExtensionResource icon) {
            return Icons{ReadAsDataURL(icon), {}};
          },
          Icon(extension, extensions::IconsInfo::GetIcons(extension), 64)),
      base::BindOnce(&Extensions::OnPromptIconRead, weak_factory_.GetWeakPtr(),
                     prompt, std::move(details)));
}

void Extensions::OnPromptIconRead(int64_t prompt,
                                  base::DictValue details,
                                  Icons icons) {
  details.Set("icon", std::move(icons.icon));
  hooks::Args args;
  args.Int(prompt);
  args.Str(base::WriteJson(details).value_or("{}"));
  hooks::Fire(PB_EVENT_EXTENSION_INSTALL_PROMPT, args);
}

void Extensions::AnswerInstallPrompt(int64_t prompt, bool accept) {
  auto it = prompts_.find(prompt);
  if (it == prompts_.end()) {
    return;
  }
  auto done = std::move(it->second);
  prompts_.erase(it);
  std::move(done).Run(accept);
}

// The same prompt, marked as a removal, with the icon the toolbar already shows.
void Extensions::AskToRemove(const extensions::Extension* extension,
                             base::OnceCallback<void(bool)> done) {
  const int64_t prompt = next_prompt_++;
  prompts_[prompt] = std::move(done);

  base::DictValue details;
  details.Set("id", extension->id());
  details.Set("name", extension->name());
  details.Set("version", extension->VersionString());
  details.Set("remove", true);
  OnPromptIconRead(prompt, std::move(details), Icons{icons_[extension->id()].icon, {}});
}

void Extensions::Uninstall(const std::string& id) {
  std::u16string error;
  extensions::ExtensionRegistrar::Get(browser_context_)
      ->UninstallExtension(id, extensions::UNINSTALL_REASON_USER_INITIATED,
                           &error);
}

void Extensions::SetEnabled(const std::string& id, bool on) {
  auto* registrar = extensions::ExtensionRegistrar::Get(browser_context_);
  if (on) {
    registrar->EnableExtension(id);
  } else {
    registrar->DisableExtension(
        id, {extensions::disable_reason::DISABLE_USER_ACTION});
  }
}

// Each extension as Go's engine.Extension.
std::string Extensions::List() {
  auto* registry = extensions::ExtensionRegistry::Get(browser_context_);
  base::ListValue list;
  for (const auto& extension : registry->GenerateInstalledExtensionsSet()) {
    const Icons& icons = icons_[extension->id()];
    base::DictValue e;
    e.Set("id", extension->id());
    e.Set("name", extension->name());
    e.Set("version", extension->VersionString());
    e.Set("description",
          extensions::DescriptionInfo::GetDescription(*extension));
    e.Set("enabled", registry->enabled_extensions().Contains(extension->id()));
    e.Set("unpacked",
          extensions::Manifest::IsUnpackedLocation(extension->location()));
    e.Set("path", extension->path().value());
    e.Set("icon", icons.icon);
    base::ListValue permissions;
    AppendStrings(extension.get(), "permissions", &permissions);
    e.Set("permissions", std::move(permissions));
    base::ListValue hosts;
    AppendStrings(extension.get(), "host_permissions", &hosts);
    e.Set("hosts", std::move(hosts));
    if (const extensions::ActionInfo* action =
            extensions::ActionInfo::GetExtensionActionInfo(extension.get())) {
      e.Set("hasAction", true);
      e.Set("actionTitle", action->default_title);
      e.Set("actionPopup", action->default_popup_url.spec());
      e.Set("actionIcon", icons.action_icon);
    }
    e.Set("optionsPage",
          extensions::OptionsPageInfo::GetOptionsPage(extension.get()).spec());
    base::ListValue commands;
    if (const ui::CommandMap* named =
            extensions::CommandsInfo::GetNamedCommands(extension.get())) {
      for (const auto& [name, command] : *named) {
        base::DictValue c;
        c.Set("name", name);
        c.Set("description", base::UTF16ToUTF8(command.description()));
        commands.Append(std::move(c));
      }
    }
    e.Set("commands", std::move(commands));
    list.Append(std::move(e));
  }
  return base::WriteJson(list).value_or("[]");
}

// //extensions counts every event it sends under a number of its own, and refuses one
// without. These are the events Go fires.
void Extensions::DispatchEvent(const std::string& id,
                               const std::string& event,
                               const std::string& args_json) {
  static constexpr auto kNumbers = base::MakeFixedFlatMap<
      std::string_view, extensions::events::HistogramValue>({
      {"action.onClicked", extensions::events::ACTION_ON_CLICKED},
      {"commands.onCommand", extensions::events::COMMANDS_ON_COMMAND},
      {"tabs.onActivated", extensions::events::TABS_ON_ACTIVATED},
      {"tabs.onCreated", extensions::events::TABS_ON_CREATED},
      {"tabs.onRemoved", extensions::events::TABS_ON_REMOVED},
      {"tabs.onUpdated", extensions::events::TABS_ON_UPDATED},
  });
  std::optional<base::ListValue> args =
      base::JSONReader::ReadList(args_json, base::JSON_PARSE_RFC);
  extensions::EventRouter::Get(browser_context_)
      ->DispatchEventToExtension(
          id, std::make_unique<extensions::Event>(kNumbers.at(event), event,
                                                  std::move(*args)));
}

void Extensions::OpenPopup(const std::string& id,
                           const GURL& url,
                           const gfx::RectF& anchor) {
  const extensions::Extension* extension =
      extensions::ExtensionRegistry::Get(browser_context_)
          ->enabled_extensions()
          .GetByID(id);
  if (!extension) {
    return;
  }
  popup_ = std::make_unique<ExtensionPopup>(
      extension, browser_context_, Engine::Get()->window(), url, anchor,
      base::BindOnce(&Extensions::ClosePopup, weak_factory_.GetWeakPtr()));
}

void Extensions::ClosePopup() {
  if (!popup_) {
    return;
  }
  popup_.reset();
  hooks::Fire(PB_EVENT_EXTENSION_POPUP_CLOSED, hooks::Args());
}

void Extensions::ChooseForDriver(const base::FilePath& path) {
  driver_choice_ = path;
}

content::WebContents* Extensions::popup_contents() const {
  return popup_ ? popup_->host_contents() : nullptr;
}

void Extensions::ChoosePath(int kind) {
  if (!driver_choice_.empty()) {
    PathChosen(std::exchange(driver_choice_, base::FilePath()));
    return;
  }
  ShowPathChooser(kind, base::BindOnce(&Extensions::PathChosen,
                                       weak_factory_.GetWeakPtr()));
}

void Extensions::PathChosen(const base::FilePath& path) {
  hooks::Args args;
  args.Str(path.value());
  hooks::Fire(PB_EVENT_PATH_CHOSEN, args);
}

// Reads the extension's icons off the main thread, then tells Go the list changed.
void Extensions::ReadIcons(const extensions::Extension* extension) {
  extensions::ExtensionResource icon =
      Icon(extension, extensions::IconsInfo::GetIcons(extension), 64);
  extensions::ExtensionResource action_icon;
  if (const extensions::ActionInfo* action =
          extensions::ActionInfo::GetExtensionActionInfo(extension)) {
    action_icon = Icon(extension, action->default_icon, 32);
  }
  base::ThreadPool::PostTaskAndReplyWithResult(
      FROM_HERE, {base::MayBlock()},
      base::BindOnce(
          [](extensions::ExtensionResource icon,
             extensions::ExtensionResource action_icon) {
            return Icons{ReadAsDataURL(icon), ReadAsDataURL(action_icon)};
          },
          std::move(icon), std::move(action_icon)),
      base::BindOnce(&Extensions::OnIconsRead, weak_factory_.GetWeakPtr(),
                     extension->id()));
}

void Extensions::OnIconsRead(const std::string& id, Icons icons) {
  icons_[id] = std::move(icons);
  Changed();
}

void Extensions::Changed() {
  hooks::Fire(PB_EVENT_EXTENSIONS_CHANGED, hooks::Args());
}

void Extensions::OnExtensionLoaded(content::BrowserContext* browser_context,
                                   const extensions::Extension* extension) {
  ReadIcons(extension);
}

void Extensions::OnExtensionUnloaded(
    content::BrowserContext* browser_context,
    const extensions::Extension* extension,
    extensions::UnloadedExtensionReason reason) {
  Changed();
}

void Extensions::OnExtensionUninstalled(
    content::BrowserContext* browser_context,
    const extensions::Extension* extension,
    extensions::UninstallReason reason) {
  icons_.erase(extension->id());
  Changed();
}

void Extensions::PreAddExtension(const extensions::Extension* extension,
                                 const extensions::Extension* old_extension) {}

void Extensions::OnAddNewOrUpdatedExtension(
    const extensions::Extension* extension) {}

void Extensions::PostActivateExtension(
    scoped_refptr<const extensions::Extension> extension) {}

void Extensions::PostDeactivateExtension(
    scoped_refptr<const extensions::Extension> extension) {}

void Extensions::PreUninstallExtension(
    scoped_refptr<const extensions::Extension> extension) {}

// What an extension stored goes with it: what its pages and service worker kept under
// its origin, and its chrome.storage. A packaged extension's files are ours to remove
// too, the folder of all its versions; an unpacked one's folder is the user's.
void Extensions::PostUninstallExtension(
    scoped_refptr<const extensions::Extension> extension,
    base::OnceClosure done_callback) {
  const bool packaged =
      !extensions::Manifest::IsUnpackedLocation(extension->location());
  base::RepeatingClosure done = base::BarrierClosure(
      packaged ? 3 : 2,
      done_callback ? std::move(done_callback) : base::DoNothing());
  extensions::util::GetStoragePartitionForExtensionId(extension->id(),
                                                      browser_context_)
      ->ClearDataForOrigin(
          ~content::StoragePartition::REMOVE_DATA_MASK_SHADER_CACHE,
          extension->url(), done);
  extensions::StorageFrontend::Get(browser_context_)
      ->DeleteStorageSoon(extension->id(), done);
  if (packaged) {
    extensions::GetExtensionFileTaskRunner()->PostTaskAndReply(
        FROM_HERE,
        base::GetDeletePathRecursivelyCallback(extension->path().DirName()),
        done);
  }
}

void Extensions::LoadExtensionForReload(
    const extensions::ExtensionId& extension_id,
    const base::FilePath& path) {
  scoped_refptr<extensions::UnpackedInstaller> installer =
      extensions::UnpackedInstaller::Create(browser_context_);
  installer->set_be_noisy_on_failure(false);
  installer->Load(path);
}

void Extensions::LoadExtensionForReloadWithQuietFailure(
    const extensions::ExtensionId& extension_id,
    const base::FilePath& path) {
  LoadExtensionForReload(extension_id, path);
}

void Extensions::ShowExtensionDisabledError(
    const extensions::Extension* extension,
    bool is_remote_install) {}

bool Extensions::CanEnableExtension(const extensions::Extension* extension) {
  return true;
}

bool Extensions::CanDisableExtension(const extensions::Extension* extension) {
  return true;
}

// What the manifest asks for is granted when the user accepts the install.
void Extensions::GrantActivePermissions(
    const extensions::Extension* extension) {
  extensions::PermissionsUpdater(browser_context_)
      .GrantActivePermissions(extension);
}

void Extensions::UpdateExternalExtensionAlert() {}

void Extensions::OnExtensionInstalled(const extensions::Extension* extension,
                                      const syncer::StringOrdinal& page_ordinal,
                                      int install_flags,
                                      base::DictValue ruleset_install_prefs) {
  extensions::ExtensionRegistrar::Get(browser_context_)
      ->AddNewOrUpdatedExtension(extension, install_flags, page_ordinal,
                                 std::string(),
                                 std::move(ruleset_install_prefs));
}

const PBExtensionCalls* Extensions::Calls() {
  static const PBExtensionCalls calls = {
      .install =
          [](const char* path) { Get()->Install(base::FilePath(path)); },
      .answer_install_prompt =
          [](int64_t prompt, int accept) {
            Get()->AnswerInstallPrompt(prompt, accept != 0);
          },
      .load_unpacked =
          [](const char* dir) { Get()->LoadUnpacked(base::FilePath(dir)); },
      .uninstall = [](const char* id) { Get()->Uninstall(id); },
      .set_enabled =
          [](const char* id, int on) { Get()->SetEnabled(id, on != 0); },
      .list = []() { return CopyForGo(Get()->List()); },
      .dispatch_event =
          [](const char* id, const char* event, const char* args_json) {
            Get()->DispatchEvent(id, event, args_json);
          },
      .open_popup =
          [](const char* id, const char* url, double x, double y, double w,
             double h) {
            Get()->OpenPopup(id, GURL(url), gfx::RectF(x, y, w, h));
          },
      .close_popup = []() { Get()->ClosePopup(); },
      .choose_path = [](int32_t kind) { Get()->ChoosePath(kind); },
  };
  return &calls;
}

}  // namespace project_b
