#include "project_b/engine/extensions/browser/extension_system.h"

#include "base/no_destructor.h"
#include "base/notreached.h"
#include "components/keyed_service/content/browser_context_dependency_manager.h"
#include "components/value_store/value_store_factory_impl.h"
#include "content/public/browser/browser_context.h"
#include "extensions/browser/extension_prefs_factory.h"
#include "extensions/browser/extension_registry_factory.h"
#include "extensions/browser/management_policy.h"
#include "extensions/browser/null_app_sorting.h"
#include "extensions/browser/quota_service.h"
#include "extensions/browser/service_worker_manager.h"
#include "extensions/browser/state_store.h"
#include "extensions/browser/user_script_manager.h"

namespace project_b {

// The stores exist from the start: the profile's other services take them as they are
// made, before InitForRegularProfile.
ExtensionSystem::ExtensionSystem(content::BrowserContext* browser_context)
    : browser_context_(browser_context) {
  store_factory_ = base::MakeRefCounted<value_store::ValueStoreFactoryImpl>(
      browser_context_->GetPath());
  state_store_ = std::make_unique<extensions::StateStore>(
      browser_context_, store_factory_,
      extensions::StateStore::BackendType::STATE, true);
  rules_store_ = std::make_unique<extensions::StateStore>(
      browser_context_, store_factory_,
      extensions::StateStore::BackendType::RULES, false);
  dynamic_user_scripts_store_ = std::make_unique<extensions::StateStore>(
      browser_context_, store_factory_,
      extensions::StateStore::BackendType::SCRIPTS, false);
}

ExtensionSystem::~ExtensionSystem() = default;

void ExtensionSystem::SetReady() {
  ready_.Signal();
}

void ExtensionSystem::Shutdown() {}

void ExtensionSystem::InitForRegularProfile(bool extensions_enabled) {
  management_policy_ = std::make_unique<extensions::ManagementPolicy>();
  service_worker_manager_ =
      std::make_unique<extensions::ServiceWorkerManager>(browser_context_);
  user_script_manager_ =
      std::make_unique<extensions::UserScriptManager>(browser_context_);
  quota_service_ = std::make_unique<extensions::QuotaService>();
  app_sorting_ = std::make_unique<extensions::NullAppSorting>();
}

// Chrome's own installer service, which we don't have: engine/extensions/browser/
// extensions.cc installs through //extensions' registrar instead.
extensions::ExtensionService* ExtensionSystem::extension_service() {
  return nullptr;
}

extensions::ManagementPolicy* ExtensionSystem::management_policy() {
  return management_policy_.get();
}

extensions::ServiceWorkerManager* ExtensionSystem::service_worker_manager() {
  return service_worker_manager_.get();
}

extensions::UserScriptManager* ExtensionSystem::user_script_manager() {
  return user_script_manager_.get();
}

extensions::StateStore* ExtensionSystem::state_store() {
  return state_store_.get();
}

extensions::StateStore* ExtensionSystem::rules_store() {
  return rules_store_.get();
}

extensions::StateStore* ExtensionSystem::dynamic_user_scripts_store() {
  return dynamic_user_scripts_store_.get();
}

scoped_refptr<value_store::ValueStoreFactory> ExtensionSystem::store_factory() {
  return store_factory_;
}

extensions::QuotaService* ExtensionSystem::quota_service() {
  return quota_service_.get();
}

extensions::AppSorting* ExtensionSystem::app_sorting() {
  return app_sorting_.get();
}

const base::OneShotEvent& ExtensionSystem::ready() const {
  return ready_;
}

bool ExtensionSystem::is_ready() const {
  return ready_.is_signaled();
}

// Nothing checks an installed extension's files against its package while it runs.
// docs/extensions.md lists this among the checks to discuss.
extensions::ContentVerifier* ExtensionSystem::content_verifier() {
  return nullptr;
}

// Chromium's own updater calls these two, and it isn't running: Go updates from the
// store by installing the newer package.
void ExtensionSystem::InstallUpdate(
    const extensions::ExtensionId& extension_id,
    const std::string& public_key,
    const base::FilePath& unpacked_dir,
    bool install_immediately,
    InstallUpdateCallback install_update_callback) {
  NOTREACHED();
}

void ExtensionSystem::PerformActionBasedOnOmahaAttributes(
    const extensions::ExtensionId& extension_id,
    const base::DictValue& attributes) {
  NOTREACHED();
}

ExtensionSystemFactory* ExtensionSystemFactory::GetInstance() {
  static base::NoDestructor<ExtensionSystemFactory> instance;
  return instance.get();
}

extensions::ExtensionSystem* ExtensionSystemFactory::GetForBrowserContext(
    content::BrowserContext* context) {
  return static_cast<ExtensionSystem*>(
      GetServiceForBrowserContext(context, true));
}

ExtensionSystemFactory::ExtensionSystemFactory()
    : ExtensionSystemProvider(
          "ExtensionSystem",
          BrowserContextDependencyManager::GetInstance()) {
  DependsOn(extensions::ExtensionPrefsFactory::GetInstance());
  DependsOn(extensions::ExtensionRegistryFactory::GetInstance());
}

ExtensionSystemFactory::~ExtensionSystemFactory() = default;

std::unique_ptr<KeyedService>
ExtensionSystemFactory::BuildServiceInstanceForBrowserContext(
    content::BrowserContext* context) const {
  return std::make_unique<ExtensionSystem>(context);
}

bool ExtensionSystemFactory::ServiceIsCreatedWithBrowserContext() const {
  return true;
}

}  // namespace project_b
