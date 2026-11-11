#ifndef PROJECT_B_ENGINE_EXTENSIONS_BROWSER_EXTENSION_SYSTEM_H_
#define PROJECT_B_ENGINE_EXTENSIONS_BROWSER_EXTENSION_SYSTEM_H_

#include <memory>
#include <string>

#include "base/memory/raw_ptr.h"
#include "base/memory/scoped_refptr.h"
#include "base/one_shot_event.h"
#include "extensions/browser/extension_system.h"
#include "extensions/browser/extension_system_provider.h"

namespace base {
template <typename T>
class NoDestructor;
}

namespace project_b {

// The services //extensions expects one object to own for a profile: its stores, its
// service worker and user script managers, its quotas.
class ExtensionSystem : public extensions::ExtensionSystem {
 public:
  explicit ExtensionSystem(content::BrowserContext* browser_context);
  ExtensionSystem(const ExtensionSystem&) = delete;
  ExtensionSystem& operator=(const ExtensionSystem&) = delete;
  ~ExtensionSystem() override;

  // Says the installed extensions are loaded, which lets everything waiting on
  // ready() go on.
  void SetReady();

 private:
  // KeyedService:
  void Shutdown() override;

  // extensions::ExtensionSystem:
  void InitForRegularProfile(bool extensions_enabled) override;
  extensions::ExtensionService* extension_service() override;
  extensions::ManagementPolicy* management_policy() override;
  extensions::ServiceWorkerManager* service_worker_manager() override;
  extensions::UserScriptManager* user_script_manager() override;
  extensions::StateStore* state_store() override;
  extensions::StateStore* rules_store() override;
  extensions::StateStore* dynamic_user_scripts_store() override;
  scoped_refptr<value_store::ValueStoreFactory> store_factory() override;
  extensions::QuotaService* quota_service() override;
  extensions::AppSorting* app_sorting() override;
  const base::OneShotEvent& ready() const override;
  bool is_ready() const override;
  extensions::ContentVerifier* content_verifier() override;
  void InstallUpdate(const extensions::ExtensionId& extension_id,
                     const std::string& public_key,
                     const base::FilePath& unpacked_dir,
                     bool install_immediately,
                     InstallUpdateCallback install_update_callback) override;
  void PerformActionBasedOnOmahaAttributes(
      const extensions::ExtensionId& extension_id,
      const base::DictValue& attributes) override;

  const raw_ptr<content::BrowserContext> browser_context_;
  scoped_refptr<value_store::ValueStoreFactory> store_factory_;
  std::unique_ptr<extensions::StateStore> state_store_;
  std::unique_ptr<extensions::StateStore> rules_store_;
  std::unique_ptr<extensions::StateStore> dynamic_user_scripts_store_;
  std::unique_ptr<extensions::ManagementPolicy> management_policy_;
  std::unique_ptr<extensions::ServiceWorkerManager> service_worker_manager_;
  std::unique_ptr<extensions::UserScriptManager> user_script_manager_;
  std::unique_ptr<extensions::QuotaService> quota_service_;
  std::unique_ptr<extensions::AppSorting> app_sorting_;
  base::OneShotEvent ready_;
};

// Makes the one ExtensionSystem of a profile, as a keyed service.
class ExtensionSystemFactory : public extensions::ExtensionSystemProvider {
 public:
  static ExtensionSystemFactory* GetInstance();

  // extensions::ExtensionSystemProvider:
  extensions::ExtensionSystem* GetForBrowserContext(
      content::BrowserContext* context) override;

 private:
  friend class base::NoDestructor<ExtensionSystemFactory>;
  ExtensionSystemFactory();
  ~ExtensionSystemFactory() override;

  // BrowserContextKeyedServiceFactory:
  std::unique_ptr<KeyedService> BuildServiceInstanceForBrowserContext(
      content::BrowserContext* context) const override;
  bool ServiceIsCreatedWithBrowserContext() const override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_BROWSER_EXTENSION_SYSTEM_H_
