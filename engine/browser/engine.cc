#include "project_b/engine/browser/engine.h"

#include <algorithm>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

#include "base/barrier_closure.h"
#include "base/check.h"
#include "base/functional/bind.h"
#include "base/location.h"
#include "base/scoped_observation.h"
#include "base/strings/utf_string_conversions.h"
#include "base/task/single_thread_task_runner.h"
#include "base/time/time.h"
#include "build/build_config.h"
#include "content/public/browser/browser_context.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/browsing_data_filter_builder.h"
#include "content/public/browser/browsing_data_remover.h"
#include "content/public/browser/host_zoom_map.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/reload_type.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_widget_host_view.h"
#include "content/public/browser/site_instance.h"
#include "content/public/browser/storage_partition.h"
#include "content/public/browser/storage_partition_config.h"
#include "content/public/browser/web_contents.h"
#include "content/public/common/stop_find_action.h"
#include "third_party/blink/public/mojom/frame/find_in_page.mojom.h"
#include "project_b/engine/browser/choose_path.h"
#include "project_b/engine/browser/dialogs.h"
#include "project_b/engine/browser/downloads.h"
#include "project_b/engine/browser/exit_reason.h"
#include "project_b/engine/browser/go.h"
#include "base/strings/string_split.h"
#include "project_b/engine/abi/pb_content_abi.h"
#include "project_b/engine/browser/printing.h"
#include "project_b/engine/browser/site_data.h"
#include "project_b/engine/browser/site_features.h"
#include "project_b/engine/common/block_rules.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/permissions.h"
#include "project_b/engine/buildflags/buildflags.h"
#if BUILDFLAG(IS_ANDROID)
#include "project_b/engine/android/passkeys_android.h"
#else
#include "project_b/engine/browser/notifications.h"
#include "project_b/engine/browser/passkeys.h"
#endif
#include "project_b/engine/browser/shell_page.h"
#if BUILDFLAG(IS_MAC)
#include "project_b/engine/browser/mac_app.h"
#endif
#if BUILDFLAG(IS_LINUX)
#include "project_b/engine/browser/driver_platform.h"
#endif
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/window.h"
#include "third_party/blink/public/common/page/page_zoom.h"
#include "ui/base/clipboard/scoped_clipboard_writer.h"
#include "ui/gfx/geometry/rect_conversions.h"
#include "ui/gfx/geometry/rect_f.h"
#include "ui/gfx/geometry/size_f.h"
#include "ui/native_theme/native_theme.h"
#include "url/gurl.h"

#if BUILDFLAG(PB_EXTENSIONS)
#include "extensions/browser/extensions_browser_client.h"
#include "project_b/engine/extensions/browser/extensions.h"
#endif

namespace project_b {

namespace {

#if BUILDFLAG(IS_ANDROID)
// A native shell's page, "native/<id>": no page, but the platform's own views for the shell
// id, which the window makes (docs/native-shell.md).
constexpr std::string_view kNativeShell = "native/";

bool IsNativeShell(const std::string& page) {
  return page.starts_with(kNativeShell);
}

std::string NativeShellId(const std::string& page) {
  return page.substr(kNativeShell.size());
}
#endif

Engine* g_engine = nullptr;

// Every workspace's partition is in this domain of the profile's context, named by the
// workspace's id and kept on disk, in chromium/Storage/ext/workspace/ in the profile's
// folder.
constexpr char kWorkspaceDomain[] = "workspace";

// What erasing a workspace takes: everything a site keeps in a partition, and the cache.
// The two left out change how the rest is removed, not what.
constexpr uint64_t kErased =
    content::BrowsingDataRemover::DATA_TYPE_ON_STORAGE_PARTITION &
    ~(content::BrowsingDataRemover::DATA_TYPE_AVOID_CLOSING_CONNECTIONS |
      content::BrowsingDataRemover::DATA_TYPE_LOGICAL_CLEAR);

// PB_ERASE_CACHE: the HTTP cache and the pages kept ready ahead. PB_ERASE_SITE_DATA is the
// rest of kErased.
constexpr uint64_t kErasedCache =
    content::BrowsingDataRemover::DATA_TYPE_CACHE |
    content::BrowsingDataRemover::DATA_TYPE_PREFETCH_CACHE |
    content::BrowsingDataRemover::DATA_TYPE_PRERENDER_CACHE;

uint64_t ErasedTypes(int32_t kinds) {
  uint64_t types = 0;
  if (kinds & PB_ERASE_SITE_DATA) {
    types |= kErased & ~kErasedCache;
  }
  if (kinds & PB_ERASE_CACHE) {
    types |= kErasedCache;
  }
  return types;
}

// Web origins, whatever their storage policy. An extension's chrome-extension:// origin
// isn't one, so an extension keeps what it stored in the profile's own storage.
constexpr uint64_t kErasedOrigins =
    content::BrowsingDataRemover::ORIGIN_TYPE_UNPROTECTED_WEB |
    content::BrowsingDataRemover::ORIGIN_TYPE_PROTECTED_WEB;

// Waits for one erase, marks it on the timeline, says it is done, and goes.
class Eraser : public content::BrowsingDataRemover::Observer {
 public:
  Eraser(content::BrowsingDataRemover* remover,
         const std::string& workspace,
         base::OnceClosure done)
      : workspace_(workspace), done_(std::move(done)) {
    observation_.Observe(remover);
  }

 private:
  void OnBrowsingDataRemoverDone(uint64_t failed_data_types) override {
    timeline::Mark("workspace.erased", "workspace=" + workspace_);
    if (done_) {
      std::move(done_).Run();
    }
    delete this;
  }

  const std::string workspace_;
  base::OnceClosure done_;
  base::ScopedObservation<content::BrowsingDataRemover,
                          content::BrowsingDataRemover::Observer>
      observation_{this};
};

content::StoragePartitionConfig PartitionOf(content::BrowserContext* context,
                                            const std::string& workspace) {
  return content::StoragePartitionConfig::Create(
      context, kWorkspaceDomain, workspace, /*in_memory=*/false);
}

// Every call from Go except post_wakeup must already be on the main thread.
void CheckMainThread() {
  CHECK(content::BrowserThread::CurrentlyOn(content::BrowserThread::UI));
}

int64_t CallCreateTab(const char* workspace, int empty) {
  CheckMainThread();
  return g_engine->CreateTab(workspace, empty)->id();
}

int64_t CallCreateInternalTab() {
  CheckMainThread();
  return g_engine->CreateInternalTab()->id();
}

void CallSetFirstWorkspace(const char* workspace) {
  CheckMainThread();
  g_engine->SetFirstWorkspace(workspace);
}

Engine::Erasure ErasureOf(const PBErasure* e) {
  auto sites = [](const char* list) {
    return base::SplitString(list, ",", base::TRIM_WHITESPACE,
                             base::SPLIT_WANT_NONEMPTY);
  };
  return {.since = e->since ? base::Time::FromTimeT(e->since) : base::Time(),
          .kinds = e->kinds,
          .sites = sites(e->sites),
          .keep = sites(e->keep)};
}

void CallEraseWorkspace(const char* workspace, const PBErasure* erasure) {
  CheckMainThread();
  g_engine->EraseWorkspace(workspace, ErasureOf(erasure));
}

void CallListSiteData(const char* workspace) {
  CheckMainThread();
  g_engine->ListSiteData(workspace);
}

void CallSetQuitErasure(const char* workspaces, const PBErasure* erasure) {
  CheckMainThread();
  // "" is a workspace too, the profile's own storage.
  g_engine->SetQuitErasure(base::SplitString(workspaces, ",", base::TRIM_WHITESPACE,
                                             base::SPLIT_WANT_ALL),
                           ErasureOf(erasure));
}

void CallCloseTab(int64_t tab) {
  CheckMainThread();
  g_engine->CloseTab(tab);
}

void CallLoadURL(int64_t tab, const char* url) {
  CheckMainThread();
  g_engine->LoadURL(tab, url);
}

void CallGoBack(int64_t tab) {
  CheckMainThread();
  g_engine->GoBack(tab);
}

void CallGoForward(int64_t tab) {
  CheckMainThread();
  g_engine->GoForward(tab);
}

void CallReload(int64_t tab) {
  CheckMainThread();
  g_engine->Reload(tab);
}

void CallReloadBypassingCache(int64_t tab) {
  CheckMainThread();
  g_engine->ReloadBypassingCache(tab);
}

void CallStop(int64_t tab) {
  CheckMainThread();
  g_engine->Stop(tab);
}

void CallSetZoom(int64_t tab, double factor) {
  CheckMainThread();
  g_engine->SetZoom(tab, factor);
}

double CallGetZoom(int64_t tab) {
  CheckMainThread();
  return g_engine->GetZoom(tab);
}

int64_t CallDuplicateTab(int64_t tab) {
  CheckMainThread();
  return g_engine->DuplicateTab(tab);
}

void CallEdit(int32_t command) {
  CheckMainThread();
  g_engine->Edit(command);
}

void CallWriteClipboard(const char* text, size_t length) {
  CheckMainThread();
  g_engine->WriteClipboard(std::string(text, length));
}

void CallDownloadURL(int64_t tab, const char* url, int save_as) {
  CheckMainThread();
  // Tab 0 is the browser's own download, as Retry starts one again.
  PageTab* page = tab ? g_engine->tab(tab) : nullptr;
  if (tab && !page) {
    return;
  }
  // Asking for the manager makes the profile's downloads, if nothing has yet.
  (page ? page->contents()->GetBrowserContext() : g_engine->browser_context())
      ->GetDownloadManager();
  if (Downloads* downloads = Downloads::Get()) {
    downloads->DownloadURL(page ? page->contents() : nullptr, GURL(url), save_as);
  }
}

void CallDownloadCommand(uint32_t download, int32_t command) {
  CheckMainThread();
  if (Downloads* downloads = Downloads::Get()) {
    downloads->Command(download, command);
  }
}

void CallPublishFile(const char* path) {
  CheckMainThread();
  // No type: the phone tells it from the file's extension.
  PublishFile(base::FilePath::FromUTF8Unsafe(path), std::string());
}

void CallAnswerDialog(int64_t dialog, int ok, const char* text) {
  CheckMainThread();
  Dialogs::Get()->Answer(dialog, ok, text);
}

void CallFind(int64_t tab, int32_t request, const char* text, int forward, int find_next) {
  CheckMainThread();
  PageTab* page = g_engine->tab(tab);
  if (!page) {
    return;
  }
  auto options = blink::mojom::FindOptions::New();
  options->forward = forward;
  options->new_session = !find_next;
  page->contents()->Find(request, base::UTF8ToUTF16(text), std::move(options),
                         /*skip_delay=*/false);
}

void CallPrint(int64_t tab) {
  CheckMainThread();
  if (PageTab* page = g_engine->tab(tab)) {
    printing::Print(page->contents());
  }
}

void CallCaptureThumbnail(int64_t tab, int32_t max_width) {
  CheckMainThread();
  if (PageTab* page = g_engine->tab(tab)) {
    page->CaptureThumbnail(max_width);
  }
}

void CallStopFinding(int64_t tab) {
  CheckMainThread();
  if (PageTab* page = g_engine->tab(tab)) {
    page->contents()->StopFinding(content::STOP_FIND_ACTION_KEEP_SELECTION);
  }
}

void CallAnswerSignIn(int64_t dialog, int ok, const char* username, const char* password) {
  CheckMainThread();
  Dialogs::Get()->AnswerSignIn(dialog, ok, username, password);
}

void CallAnswerPasskey(int64_t dialog, const char* action, const char* text) {
  CheckMainThread();
#if BUILDFLAG(IS_ANDROID)
  AnswerPasskey(dialog, action, text);
#else
  PasskeyRequest::Answer(dialog, action, text);
#endif
}

void CallExitPageFullScreen(int64_t tab) {
  CheckMainThread();
  if (PageTab* page = g_engine->tab(tab)) {
    page->contents()->ExitFullscreen(/*will_cause_resize=*/true);
  }
}

void CallClickNotification(const char* id) {
  CheckMainThread();
#if !BUILDFLAG(IS_ANDROID)  // A phone shows no page's notification yet.
  Notifications::Click(id);
#endif
}

void CallAnswerPermission(uint32_t request, const int32_t* statuses, size_t count) {
  CheckMainThread();
  Permissions::Answer(request, statuses, count);
}

void CallSendToPage(int64_t tab, const char* json, size_t length) {
  CheckMainThread();
  g_engine->SendToPage(tab, json, length);
}

void CallSetMenu(const char* json, size_t length) {
  CheckMainThread();
#if BUILDFLAG(IS_MAC)  // A phone has no menu bar; Linux's is the shell's strip (step 3).
  SetMainMenu(std::string(json, length));
#elif BUILDFLAG(IS_LINUX)
  driver::KeepMenu(std::string(json, length));
#endif
}

void CallAppCommand(int32_t command) {
  CheckMainThread();
  if (command == PB_APP_QUIT) {
    g_engine->Quit();
    return;
  }
#if BUILDFLAG(IS_MAC)
  RunAppCommand(command);
#endif
}

#if !BUILDFLAG(IS_ANDROID)
void CallChooseFolder(int64_t window, const char* start) {
  CheckMainThread();
  Engine::AppWindow* w = g_engine->window(window);
  auto chosen = [](int64_t window, const base::FilePath& path) {
    hooks::Args args;
    args.Int(window);
    args.Str(path.value());
    hooks::Fire(PB_EVENT_FOLDER_CHOSEN, args);
  };
  if (!w) {
    chosen(window, base::FilePath());
    return;
  }
  ShowPathChooser(w->window->native_window(), PathKind::kFolder,
                  base::FilePath::FromUTF8Unsafe(start), base::BindOnce(chosen, window));
}
#endif


// Go only names windows it opened and the engine hasn't closed.
Window* NativeWindow(int64_t window) {
  Engine::AppWindow* w = g_engine->window(window);
  CHECK(w) << "no window " << window;
  return w->window.get();
}

void CallRequestWindow(int64_t window, int32_t request) {
  CheckMainThread();
  switch (request) {
    case PB_WINDOW_CLOSE:
      g_engine->CloseWindow(window);
      break;
    case PB_WINDOW_MINIMISE:
      NativeWindow(window)->Minimize();
      break;
    case PB_WINDOW_ZOOM:
      NativeWindow(window)->Zoom();
      break;
    case PB_WINDOW_FULL_SCREEN:
      NativeWindow(window)->ToggleFullScreen();
      break;
    case PB_WINDOW_FRONT:
      NativeWindow(window)->BringToFront();
      break;
  }
}

// A layout from Go as the window and the engine take it.
Window::Layout LayoutOf(const PBWindowLayout* l,
                        const PBRegion* regions,
                        size_t region_count) {
  Window::Layout layout = {
      .viewport = gfx::SizeF(l->view_w, l->view_h),
      .frame_rgb = static_cast<uint32_t>(l->frame_rgb),
      .frame_rgb_dark = static_cast<uint32_t>(l->frame_rgb_dark),
      .frame_width = l->frame_width,
      .page_rgb = static_cast<uint32_t>(l->page_rgb),
      .page_rgb_dark = static_cast<uint32_t>(l->page_rgb_dark),
      .theme = l->theme,
  };
  layout.regions.reserve(region_count);
  for (size_t i = 0; i < region_count; i++) {
    const PBRegion& r = UNSAFE_BUFFERS(regions[i]);
    layout.regions.push_back({gfx::RectF(r.x, r.y, r.w, r.h),
                              static_cast<Window::Area>(r.area)});
  }
  return layout;
}

std::vector<Engine::VisibleTab> TabsOf(const PBVisibleTab* tabs,
                                       size_t tab_count) {
  std::vector<Engine::VisibleTab> visible;
  visible.reserve(tab_count);
  for (size_t i = 0; i < tab_count; i++) {
    const PBVisibleTab& t = UNSAFE_BUFFERS(tabs[i]);
    visible.push_back({t.tab, gfx::RectF(t.x, t.y, t.w, t.h)});
  }
  return visible;
}

void CallOpenProfile(const char* dir) {
  CheckMainThread();
  g_engine->OpenProfile(dir);
}

void CallCreateWindow(int64_t window,
                      int64_t near,
                      const char* page,
                      const PBWindowLayout* l,
                      const PBRegion* regions,
                      size_t region_count,
                      const PBVisibleTab* tabs,
                      size_t tab_count) {
  CheckMainThread();
  g_engine->CreateWindow(window, near, page, LayoutOf(l, regions, region_count),
                         TabsOf(tabs, tab_count));
}

void CallSetWindowPage(int64_t window, const char* page) {
  CheckMainThread();
  g_engine->SetWindowPage(window, page);
}

void CallSetWindowLayout(int64_t window,
                         const PBWindowLayout* l,
                         const PBRegion* regions,
                         size_t region_count,
                         const PBVisibleTab* tabs,
                         size_t tab_count) {
  CheckMainThread();
  g_engine->SetTheme(l->theme);
  if (NativeWindow(window)->SetLayout(LayoutOf(l, regions, region_count))) {
    g_engine->ShowTabs(g_engine->window(window), TabsOf(tabs, tab_count));
  }
}

void CallSetWindowBounds(int64_t window, double x, double y, double w, double h) {
  CheckMainThread();
#if !BUILDFLAG(IS_ANDROID)  // A phone's window fills the screen.
  NativeWindow(window)->SetBounds(gfx::ToRoundedRect(gfx::RectF(x, y, w, h)));
#endif
}

void CallSetWindowMode(int64_t window, int32_t mode) {
  CheckMainThread();
#if !BUILDFLAG(IS_ANDROID)  // A phone's window is always as it is.
  NativeWindow(window)->SetMode(mode);
#endif
}

void CallFocusTab(int64_t tab) {
  CheckMainThread();
  g_engine->FocusTab(tab);
}

void CallFocusShell(int64_t window) {
  CheckMainThread();
  g_engine->FocusShell(window);
}

void CallSetShellBack(int64_t window, int on) {
  CheckMainThread();
  if (auto* w = g_engine->window(window)) {
    w->shell_back = on != 0;
  }
}

void CallShareFile(int64_t window,
                   const char* path,
                   double x,
                   double y,
                   double w,
                   double h) {
  CheckMainThread();
  if (auto* app_window = g_engine->window(window)) {
    app_window->window->ShareFile(path, gfx::RectF(x, y, w, h));
  }
}

void CallShareText(int64_t window, const char* text) {
  CheckMainThread();
  if (auto* app_window = g_engine->window(window)) {
    app_window->window->ShareText(text);
  }
}

// A save for a tab already gone is still told, as Go waits on every save it asked for.
void CallSaveImage(int64_t tab, const char* url, const char* path, int32_t max_size) {
  CheckMainThread();
  if (PageTab* page = g_engine->tab(tab)) {
    page->SaveImage(GURL(url), path, max_size);
    return;
  }
  hooks::Args args;
  args.Str(path);
  args.Int(0);
  hooks::Fire(PB_EVENT_IMAGE_SAVED, args);
}

void CallCopyImage(const char* path) {
  CheckMainThread();
  CopyImageFile(path);
}

void CallSendToShell(int64_t window, const char* json, size_t length) {
  CheckMainThread();
  g_engine->SendToShell(window, json, length);
}

void CallSetHookSubscribed(int32_t hook, int on) {
  CheckMainThread();
  hooks::SetSubscribed(hook, on != 0);
}

void CallSetBlockRules(const char* hosts) {
  CheckMainThread();
  block_rules::Set(base::SplitString(hosts, "\n", base::TRIM_WHITESPACE,
                                     base::SPLIT_WANT_NONEMPTY));
}

void CallSetAdRules(const char* hosts, const char* off) {
  CheckMainThread();
  site_features::SetAdRules(
      base::SplitString(hosts, "\n", base::TRIM_WHITESPACE,
                        base::SPLIT_WANT_NONEMPTY),
      base::SplitString(off, "\n", base::TRIM_WHITESPACE,
                        base::SPLIT_WANT_NONEMPTY));
}

const PBContentHooks* CallContentHooks() {
  static const PBContentHooks kHooks = {
      .table_hash = hooks::kTableHash,
      .set_subscribed = CallSetHookSubscribed,
  };
  return &kHooks;
}

#if BUILDFLAG(IS_ANDROID)
void CallInstallUpdate(const char* path) {
  CheckMainThread();
  // Android has one window, and the installer belongs to the app, not to it.
  g_engine->windows().begin()->second->window->InstallPackage(path);
}

int CallDeleteDownload(const char* path) {
  CheckMainThread();
  return DeleteDownload(base::FilePath::FromUTF8Unsafe(path)) ? 1 : 0;
}

void CallShowDownload(int64_t id,
                      const char* name,
                      const char* path,
                      int32_t state,
                      int64_t received,
                      int64_t total,
                      int paused) {
  CheckMainThread();
  ShowDownload(id, name, base::FilePath::FromUTF8Unsafe(path), state, received, total,
               paused != 0);
}
#endif

// The one call that any thread may make.
void CallPostWakeup() {
  content::GetUIThreadTaskRunner({})->PostTask(FROM_HERE,
                                               base::BindOnce(Go().run_posted));
}

}  // namespace

Engine::AppWindow::AppWindow(int64_t id) : id(id) {}
Engine::AppWindow::~AppWindow() = default;

Engine::Engine(content::BrowserContext* pages_context,
               OpenProfileCallback open_profile,
               base::RepeatingClosure quit)
    : pages_context_(pages_context),
      open_profile_(std::move(open_profile)),
      quit_(std::move(quit)) {
  CHECK(!g_engine);
  g_engine = this;
}

Engine::~Engine() {
  g_engine = nullptr;
}

// static
Engine* Engine::Get() {
  return g_engine;
}

// static
void Engine::TabOpened(int64_t opener,
                       int64_t tab,
                       const GURL& url,
                       bool foreground,
                       bool new_window) {
  hooks::Args args;
  args.Int(opener);
  args.Int(tab);
  args.Str(url.spec());
  args.Str(Get()->tab(tab)->workspace());
  args.Bool(foreground);
  args.Bool(new_window);
  hooks::Fire(PB_EVENT_TAB_OPENED, args);
}

// static
const PBEngineCalls* Engine::Calls() {
  static const PBEngineCalls calls = {
      .open_profile = CallOpenProfile,
      .create_tab = CallCreateTab,
      .create_internal_tab = CallCreateInternalTab,
      .set_first_workspace = CallSetFirstWorkspace,
      .erase_workspace = CallEraseWorkspace,
      .list_site_data = CallListSiteData,
      .set_quit_erasure = CallSetQuitErasure,
      .close_tab = CallCloseTab,
      .load_url = CallLoadURL,
      .go_back = CallGoBack,
      .go_forward = CallGoForward,
      .reload = CallReload,
      .reload_bypassing_cache = CallReloadBypassingCache,
      .stop = CallStop,
      .set_zoom = CallSetZoom,
      .get_zoom = CallGetZoom,
      .duplicate_tab = CallDuplicateTab,
      .edit = CallEdit,
      .write_clipboard = CallWriteClipboard,
      .download_url = CallDownloadURL,
      .download_command = CallDownloadCommand,
      .answer_dialog = CallAnswerDialog,
      .answer_sign_in = CallAnswerSignIn,
      .answer_passkey = CallAnswerPasskey,
      .find = CallFind,
      .stop_finding = CallStopFinding,
      .print = CallPrint,
      .capture_thumbnail = CallCaptureThumbnail,
      .exit_page_full_screen = CallExitPageFullScreen,
      .click_notification = CallClickNotification,
      .answer_permission = CallAnswerPermission,
      .create_window = CallCreateWindow,
      .set_window_page = CallSetWindowPage,
      .request_window = CallRequestWindow,
      .set_window_bounds = CallSetWindowBounds,
      .set_window_mode = CallSetWindowMode,
      .set_window_layout = CallSetWindowLayout,
      .focus_tab = CallFocusTab,
      .focus_shell = CallFocusShell,
      .send_to_shell = CallSendToShell,
      .set_shell_back = CallSetShellBack,
      .share_file = CallShareFile,
      .share_text = CallShareText,
      .save_image = CallSaveImage,
      .copy_image = CallCopyImage,
      .publish_file = CallPublishFile,
      .send_to_page = CallSendToPage,
      .set_menu = CallSetMenu,
      .app_command = CallAppCommand,
      .set_block_rules = CallSetBlockRules,
      .set_ad_rules = CallSetAdRules,
      .content_hooks = CallContentHooks,
      .post_wakeup = CallPostWakeup,
#if BUILDFLAG(IS_ANDROID)
      .install_update = CallInstallUpdate,
#endif
#if BUILDFLAG(PB_EXTENSIONS)
      .extensions = Extensions::Calls,
#endif
#if BUILDFLAG(IS_ANDROID)
      .show_download = CallShowDownload,
#else
      .choose_folder = CallChooseFolder,
#endif
#if BUILDFLAG(IS_ANDROID)
      .exit_reason = ExitReason,
      .delete_download = CallDeleteDownload,
#endif
  };
  return &calls;
}

PageTab* Engine::tab(int64_t id) const {
  auto it = tabs_.find(id);
  return it == tabs_.end() ? nullptr : it->second.get();
}

void Engine::SetTheme(int theme) {
  if (theme == theme_) {
    return;
  }
  theme_ = theme;
  // Every page hears it as an update of the web's native theme, and asks again
  // (ContentBrowserClient::WebPreferencesNeedUpdateForColorRelatedStateChanges).
  ui::NativeTheme::GetInstanceForWeb()->NotifyOnNativeThemeUpdated();
}

Engine::AppWindow* Engine::window(int64_t id) const {
  auto it = windows_.find(id);
  return it == windows_.end() ? nullptr : it->second.get();
}

Engine::AppWindow* Engine::WindowOf(int64_t tab) const {
  PageTab* t = this->tab(tab);
  return t ? window(t->window()) : nullptr;
}

PageTab* Engine::FocusedTab(const AppWindow* window) const {
  return tab(window->focused_tab);
}

namespace {

bool Lists(const std::vector<Engine::VisibleTab>& tabs, int64_t tab) {
  return std::ranges::any_of(
      tabs, [tab](const Engine::VisibleTab& v) { return v.tab == tab; });
}

}  // namespace

// A tab that closed after Go sent the layout is left out. A tab another window shows now
// has moved there, and is shown there.
void Engine::ShowTabs(AppWindow* window, std::vector<VisibleTab> tabs) {
  std::erase_if(tabs, [this](const VisibleTab& v) { return !tab(v.tab); });
  std::vector<Window::Page> pages;
  for (const VisibleTab& v : tabs) {
    pages.push_back({tab(v.tab)->contents(), v.rect, v.tab});
  }
  window->window->SetPages(pages);
  for (const VisibleTab& v : window->visible) {
    PageTab* t = tab(v.tab);
    if (t && !Lists(tabs, v.tab) && t->window() == window->id) {
      t->contents()->WasHidden();
    }
  }
  for (const VisibleTab& v : tabs) {
    if (!Lists(window->visible, v.tab)) {
      tab(v.tab)->set_window(window->id);
      tab(v.tab)->contents()->WasShown();
    }
  }
  window->visible = std::move(tabs);
  if (!Lists(window->visible, window->focused_tab)) {
    window->focused_tab =
        window->visible.empty() ? 0 : window->visible.front().tab;
  }
  UpdateTitle(window);
#if !BUILDFLAG(IS_ANDROID)
  if (!window->visible.empty()) {
    ShowPagesOnceDrawn(window);
  }
#endif
}

#if !BUILDFLAG(IS_ANDROID)
// The shell sends a layout only once it has drawn what it measured, so its next frame
// holds that layout, whatever it drew before, such as a splash. Content runs the callback
// once the shell has submitted that frame; the frame or so it then takes to reach the
// screen, the pages take too, and their fade in covers the difference.
void Engine::ShowPagesOnceDrawn(AppWindow* window) {
  if (window->pages_shown || window->awaiting_frame) {
    return;
  }
  window->awaiting_frame = true;
  window->shell->contents()->GetPrimaryMainFrame()->InsertVisualStateCallback(
      base::BindOnce(&Engine::ShellDrew, weak_factory_.GetWeakPtr(), window->id,
                     window->shell_document));
}

// drawn is false when the shell's renderer went first; the next shell's layout shows the
// pages then. The window may have closed meanwhile.
void Engine::ShellDrew(int64_t id, int document, bool drawn) {
  AppWindow* w = window(id);
  if (!w || w->shell_document != document) {
    return;
  }
  w->awaiting_frame = false;
  if (drawn) {
    w->pages_shown = true;
    w->window->ShowPages();
  }
}

// A window closing may hear its shell go after it has gone.
void Engine::ShellGone(int64_t id) {
  AppWindow* w = window(id);
  if (!w) {
    return;
  }
  w->shell_document++;
  w->awaiting_frame = false;
  w->pages_shown = false;
  w->window->HidePages();
}
#endif

void Engine::TabFocused(PageTab* tab) {
  if (AppWindow* w = WindowOf(tab->id())) {
    w->focused_tab = tab->id();
    UpdateTitle(w);
  }
}

void Engine::TabTitleChanged(PageTab* tab) {
  if (AppWindow* w = WindowOf(tab->id()); w && w->focused_tab == tab->id()) {
    UpdateTitle(w);
  }
}

// A window is titled after its focused tab, and with no tab shown, as with no tabs, with
// the app's own title.
void Engine::UpdateTitle(AppWindow* window) {
  PageTab* t = FocusedTab(window);
  window->window->SetTitle(t ? t->contents()->GetTitle()
                             : std::u16string(Window::kTitle));
}

void Engine::OpenProfile(const char* dir) {
  CHECK(!browser_context_);
  browser_context_ = open_profile_.Run(base::FilePath(dir));
}

void Engine::CreateWindow(int64_t id,
                          int64_t near,
                          const std::string& page,
                          Window::Layout layout,
                          std::vector<VisibleTab> tabs) {
  auto w = std::make_unique<AppWindow>(id);
  w->window = std::make_unique<Window>(
      base::BindOnce(&Engine::WindowClosed, base::Unretained(this), id),
      base::BindRepeating(
          [](int64_t id) {
            hooks::Args args;
            args.Int(id);
            hooks::Fire(PB_EVENT_WINDOW_FOCUSED, args);
          },
          id),
      base::BindRepeating(&Engine::WindowMinimised, base::Unretained(this),
                          id));
  timeline::Mark("window.created");
  if (AppWindow* other = window(near)) {
    w->window->PlaceNear(other->window.get());
  }
#if BUILDFLAG(IS_ANDROID)
  if (IsNativeShell(page)) {
    w->native_shell = true;
    w->window->SetNativeShell(NativeShellId(page));
  } else {
    w->shell = std::make_unique<ShellPage>(pages_context_, id, page);
    w->window->SetShellContents(w->shell->contents());
  }
#else
  w->shell = std::make_unique<ShellPage>(pages_context_, id, page);
  w->window->SetShellView(w->shell->contents()->GetNativeView());
#endif
  timeline::Mark("shell.created");
  AppWindow* raw = w.get();
  windows_[id] = std::move(w);
  if (raw->shell) {
    raw->shell->Load();
  }
  // The frame is in the first layout, so even the window's first frame on screen is the
  // frame colour. It still shows before any renderer has drawn.
  if (raw->window->SetLayout(std::move(layout))) {
    ShowTabs(raw, std::move(tabs));
  }
  raw->window->Show();
  timeline::Mark("window.shown");
  // Go hears about the window once the call that made it has returned.
  content::GetUIThreadTaskRunner({})->PostTask(
      FROM_HERE,
      base::BindOnce(&Engine::ListenToWindow, base::Unretained(this), id));
}

void Engine::ListenToWindow(int64_t id) {
  AppWindow* w = window(id);
  if (!w) {
    return;
  }
  w->window->ListenForState(base::BindRepeating(
      [](int64_t id, const Window::State& s) {
        hooks::Args args;
        args.Int(id);
        args.Double(s.size.width());
        args.Double(s.size.height());
        args.Double(s.controls.x());
        args.Double(s.controls.y());
        args.Double(s.controls.width());
        args.Double(s.controls.height());
        args.Double(s.corner_radius);
        args.Bool(s.full_screen);
        args.Double(s.bounds.x());
        args.Double(s.bounds.y());
        args.Double(s.bounds.width());
        args.Double(s.bounds.height());
        args.Bool(s.minimised);
        args.Bool(s.zoomed);
        hooks::Fire(PB_EVENT_WINDOW_CHANGED, args);
      },
      id));
}

// The native window is in the middle of closing, so the window and its pages die on a
// later task. It is posted before Go hears, so it runs before anything Go asks for in turn:
// a quit, when this was the last window, would otherwise end the app with the window's
// pages still alive. Go closes the window's tabs as it hears.
void Engine::WindowClosed(int64_t id) {
  auto it = windows_.find(id);
  content::GetUIThreadTaskRunner({})->DeleteSoon(FROM_HERE,
                                                 std::move(it->second));
  windows_.erase(it);
  hooks::Args args;
  args.Int(id);
  hooks::Fire(PB_EVENT_WINDOW_CLOSED, args);
}

void Engine::WindowMinimised(int64_t id, bool minimised) {
  for (const VisibleTab& v : window(id)->visible) {
    if (minimised) {
      tab(v.tab)->contents()->WasHidden();
    } else {
      tab(v.tab)->contents()->WasShown();
    }
  }
}

void Engine::Quit() {
#if BUILDFLAG(IS_ANDROID)
  // A phone's app ends with its activity: closing its one window finishes it, and the
  // activity's onDestroy stops Go and ends the process.
  if (!windows_.empty()) {
    windows_.begin()->second->window->Close();
  }
#else
  // What Go asked to be erased as the app quits goes first: the run loop ends once every
  // workspace's erase is done, or after kQuitEraseWait whatever is left. A second quit
  // meanwhile waits with the first.
  if (quitting_) {
    return;
  }
  quitting_ = true;
  if (!quit_erasure_.kinds || quit_workspaces_.empty()) {
    QuitNow();
    return;
  }
  base::RepeatingClosure erased = base::BarrierClosure(
      quit_workspaces_.size(),
      base::BindOnce(&Engine::QuitNow, base::Unretained(this)));
  for (const std::string& workspace : quit_workspaces_) {
    Erase(workspace, quit_erasure_, erased);
  }
  content::GetUIThreadTaskRunner({})->PostDelayedTask(
      FROM_HERE, base::BindOnce(&Engine::QuitNow, base::Unretained(this)),
      kQuitEraseWait);
#endif
}

void Engine::QuitNow() {
  if (quit_done_) {
    return;
  }
  quit_done_ = true;
  timeline::Mark("app.quit", "");
  quit_.Run();
}

void Engine::SetWindowPage(int64_t window, const std::string& page) {
  AppWindow* w = this->window(window);
#if BUILDFLAG(IS_ANDROID)
  // A native shell and a page are different views: the one there leaves the window, then
  // the other comes.
  const bool to_native = IsNativeShell(page);
  if (to_native || w->native_shell) {
    w->window->RemoveShell();
    w->shell.reset();
    w->native_shell = to_native;
    if (to_native) {
      w->window->SetNativeShell(NativeShellId(page));
    } else {
      w->shell = std::make_unique<ShellPage>(pages_context_, window, page);
      w->window->SetShellContents(w->shell->contents());
      w->shell->Load();
    }
    return;
  }
#endif
  w->shell->SetPage(page);
}

#if BUILDFLAG(IS_ANDROID)
void Engine::RenewShell(int64_t window) {
  AppWindow* w = this->window(window);
  if (!w || !w->shell) {
    return;
  }
  const std::string page = w->shell->page();
  w->window->RemoveShell();
  w->shell = std::make_unique<ShellPage>(pages_context_, window, page);
  w->window->SetShellContents(w->shell->contents());
  w->shell->Load();
}
#endif

void Engine::FocusShellForTraversal(int64_t from_tab, bool reverse) {
#if BUILDFLAG(IS_ANDROID)
  if (WindowOf(from_tab)->native_shell) {
    WindowOf(from_tab)->window->FocusNativeShell();
    return;
  }
#endif
  content::WebContents* shell = WindowOf(from_tab)->shell->contents();
  shell->Focus();
  shell->FocusThroughTabTraversal(reverse);
}

void Engine::FocusPageForTraversal(int64_t window, bool reverse) {
  if (PageTab* tab = FocusedTab(this->window(window))) {
    tab->contents()->Focus();
    tab->contents()->FocusThroughTabTraversal(reverse);
  }
}

// The profile's own storage is the context's default partition, which content refuses as
// a fixed one, so its tabs get no SiteInstance and take the default as any page does. An
// empty tab, a new tab or one restored for later, starts no renderer until it navigates:
// by default content hands a tab the spare one, which an empty tab would hold idle. A tab
// about to load keeps that default, so its first page doesn't wait for a process to start.
PageTab* Engine::CreateTab(const std::string& workspace, bool empty) {
  content::WebContents::CreateParams params(browser_context_);
  if (empty) {
    params.desired_renderer_state =
        content::WebContents::CreateParams::kNoRendererProcess;
  }
  if (!workspace.empty()) {
    params.site_instance = content::SiteInstance::CreateForFixedStoragePartition(
        browser_context_, GURL(), PartitionOf(browser_context_, workspace));
  }
  params.initially_hidden = true;
  return Add(content::WebContents::Create(params), workspace);
}

PageTab* Engine::CreateInternalTab() {
  content::WebContents::CreateParams params(pages_context_);
  params.initially_hidden = true;
  return Add(content::WebContents::Create(params), std::string(),
             /*internal=*/true);
}

PageTab* Engine::AdoptTab(std::unique_ptr<content::WebContents> contents) {
  const std::string workspace = WorkspaceOf(contents.get());
  return Add(std::move(contents), workspace);
}

std::string Engine::WorkspaceOf(content::WebContents* contents) const {
  const content::StoragePartitionConfig& partition =
      browser_context_->GetStoragePartition(contents->GetSiteInstance())
          ->GetConfig();
  return partition.partition_domain() == kWorkspaceDomain
             ? partition.partition_name()
             : std::string();
}

void Engine::SetFirstWorkspace(const std::string& workspace) {
  first_workspace_ = workspace;
}

// Each closed tab dies on a later task (CloseTab), so the erase waits for a task after
// theirs.
void Engine::EraseWorkspace(const std::string& workspace, Erasure erasure) {
  content::GetUIThreadTaskRunner({})->PostTask(
      FROM_HERE, base::BindOnce(&Engine::Erase, base::Unretained(this), workspace,
                                std::move(erasure), base::OnceClosure()));
}

void Engine::SetQuitErasure(std::vector<std::string> workspaces, Erasure erasure) {
  quit_workspaces_ = std::move(workspaces);
  quit_erasure_ = std::move(erasure);
}

void Engine::ListSiteData(const std::string& workspace) {
  site_data::List(
      workspace.empty() ? browser_context_->GetDefaultStoragePartition()
                        : browser_context_->GetStoragePartition(
                              PartitionOf(browser_context_, workspace)),
      workspace);
}

// The profile's own storage is the context's default partition, which the remover
// erases when it is given none.
void Engine::Erase(const std::string& workspace,
                   const Erasure& erasure,
                   base::OnceClosure done) {
  // With sites named, it deletes theirs; else it preserves the sites kept, or nothing.
  auto filter = content::BrowsingDataFilterBuilder::Create(
      erasure.sites.empty() ? content::BrowsingDataFilterBuilder::Mode::kPreserve
                            : content::BrowsingDataFilterBuilder::Mode::kDelete);
  for (const std::string& site :
       erasure.sites.empty() ? erasure.keep : erasure.sites) {
    filter->AddRegisterableDomain(site);
  }
  if (!workspace.empty()) {
    filter->SetStoragePartitionConfig(PartitionOf(browser_context_, workspace));
  }
  content::BrowsingDataRemover* remover =
      browser_context_->GetBrowsingDataRemover();
  remover->RemoveWithFilterAndReply(
      erasure.since, base::Time::Max(), ErasedTypes(erasure.kinds), kErasedOrigins,
      std::move(filter), new Eraser(remover, workspace, std::move(done)));
}

// A new tab starts hidden, whoever made it. A window's layout is the only way to the
// screen.
PageTab* Engine::Add(std::unique_ptr<content::WebContents> contents,
                     const std::string& workspace,
                     bool internal) {
  const int64_t id = next_tab_++;
  auto tab = std::make_unique<PageTab>(id, std::move(contents), workspace, internal);
  PageTab* raw = tab.get();
  tab->contents()->WasHidden();
#if BUILDFLAG(PB_EXTENSIONS)
  // Extensions run in every web tab: content scripts, messages, chrome.scripting. They are
  // the profile's, and none of ours is in its context.
  if (Extensions::Get() && !raw->internal()) {
    extensions::ExtensionsBrowserClient::Get()
        ->CreateExtensionWebContentsObserver(tab->contents());
  }
#endif
  if (!raw->internal()) {
    site_features::Follow(raw->contents());
  }
  printing::Follow(raw->contents());
#if BUILDFLAG(IS_ANDROID)
  // Android has one window, and every tab's view is in it from the start.
  windows_.begin()->second->window->AddPageContents(raw->contents());
#endif
  tabs_[id] = std::move(tab);
  return raw;
}

// Go may ask for this from inside one of the tab's own events (Cmd+W arrives through
// the tab's keyboard handling), so the tab leaves the map now and dies on a later task.
void Engine::CloseTab(int64_t tab) {
  auto it = tabs_.find(tab);
  if (it == tabs_.end()) {
    return;
  }
#if BUILDFLAG(IS_ANDROID)
  windows_.begin()->second->window->RemovePageContents(it->second->contents());
#endif
  if (AppWindow* w = WindowOf(tab)) {
#if !BUILDFLAG(IS_ANDROID)
    w->window->RemovePageView(it->second->contents()->GetNativeView());
#endif
    std::erase_if(w->visible, [tab](const VisibleTab& v) { return v.tab == tab; });
    if (w->focused_tab == tab) {
      w->focused_tab = w->visible.empty() ? 0 : w->visible.front().tab;
    }
  }
  content::GetUIThreadTaskRunner({})->DeleteSoon(FROM_HERE,
                                                 std::move(it->second));
  tabs_.erase(it);
}

void Engine::LoadURL(int64_t tab, const char* url) {
  auto it = tabs_.find(tab);
  if (it == tabs_.end()) {
    return;
  }
  content::NavigationController::LoadURLParams params((GURL(url)));
  params.transition_type = ui::PageTransitionFromInt(
      ui::PAGE_TRANSITION_TYPED | ui::PAGE_TRANSITION_FROM_ADDRESS_BAR);
  // In a workspace the blank page stays in the tab's own SiteInstance. Content would give
  // it one of its own, and the blank page leaves a SiteInstance with no site: for the tab's
  // next navigation content then reads the partition from that empty site, the default,
  // and stops on the mismatch with the fixed one (BrowsingInstance::ComputeSiteInfoForURL).
  // The tab's own SiteInstance has a site in the workspace's partition already.
  if (!it->second->workspace().empty() && params.url.IsAboutBlank()) {
    params.source_site_instance = it->second->contents()->GetSiteInstance();
  }
  it->second->contents()->GetController().LoadURLWithParams(params);
}

void Engine::GoBack(int64_t tab) {
  auto it = tabs_.find(tab);
  if (it != tabs_.end() && it->second->contents()->GetController().CanGoBack()) {
    it->second->contents()->GetController().GoBack();
  }
}

void Engine::GoForward(int64_t tab) {
  auto it = tabs_.find(tab);
  if (it != tabs_.end() &&
      it->second->contents()->GetController().CanGoForward()) {
    it->second->contents()->GetController().GoForward();
  }
}

void Engine::Reload(int64_t tab) {
  auto it = tabs_.find(tab);
  if (it != tabs_.end()) {
    it->second->contents()->GetController().Reload(content::ReloadType::NORMAL,
                                                   /*check_for_repost=*/false);
  }
}

content::WebContents* Engine::Contents(int64_t tab) const {
  auto it = tabs_.find(tab);
  return it == tabs_.end() ? nullptr : it->second->contents();
}

content::WebContents* Engine::WebTabContents(int64_t tab) const {
  PageTab* t = this->tab(tab);
  return t && !t->internal() ? t->contents() : nullptr;
}

void Engine::ReloadBypassingCache(int64_t tab) {
  if (content::WebContents* contents = Contents(tab)) {
    contents->GetController().Reload(content::ReloadType::BYPASSING_CACHE,
                                     /*check_for_repost=*/false);
  }
}

void Engine::Stop(int64_t tab) {
  if (content::WebContents* contents = Contents(tab)) {
    contents->Stop();
  }
}

void Engine::SetZoom(int64_t tab, double factor) {
  if (content::WebContents* contents = Contents(tab)) {
    content::HostZoomMap::SetZoomLevel(contents,
                                       blink::ZoomFactorToZoomLevel(factor));
  }
}

double Engine::GetZoom(int64_t tab) {
  content::WebContents* contents = Contents(tab);
  return contents ? blink::ZoomLevelToZoomFactor(
                        content::HostZoomMap::GetZoomLevel(contents))
                  : 1;
}

// A clone has its history but no page until it is told to load. It loads once this has
// returned, as a new tab loads only once Go asks: Go hears of the copy only then, and a
// navigation in a tab Go doesn't know yet is blocked, which showed the copy failed,
// ERR_BLOCKED_BY_CLIENT (docs/fragility.md, "A duplicated tab that failed to load").
int64_t Engine::DuplicateTab(int64_t tab) {
  content::WebContents* contents = Contents(tab);
  if (!contents) {
    return 0;
  }
  // The copy has the tab's SiteInstance, so it is in the same workspace.
  PageTab* copy = Add(contents->Clone(), this->tab(tab)->workspace(),
                      this->tab(tab)->internal());
  content::GetUIThreadTaskRunner({})->PostTask(
      FROM_HERE, base::BindOnce(
                     [](base::WeakPtr<Engine> engine, int64_t id) {
                       if (PageTab* copy = engine ? engine->tab(id) : nullptr) {
                         copy->contents()->GetController().LoadIfNecessary();
                       }
                     },
                     weak_factory_.GetWeakPtr(), copy->id()));
  return copy->id();
}

content::WebContents* Engine::FocusedContents() const {
  for (const auto& [id, w] : windows_) {
    std::vector<content::WebContents*> pages;
    if (w->shell) {
      pages.push_back(w->shell->contents());
    }
    for (const VisibleTab& v : w->visible) {
      pages.push_back(tab(v.tab)->contents());
    }
    for (content::WebContents* contents : pages) {
      if (contents->GetRenderWidgetHostView() &&
          contents->GetRenderWidgetHostView()->HasFocus()) {
        return contents;
      }
    }
  }
  return nullptr;
}

void Engine::WriteClipboard(const std::string& text) {
  ui::ScopedClipboardWriter(ui::ClipboardBuffer::kCopyPaste).WriteText(base::UTF8ToUTF16(text));
}

void Engine::Edit(int32_t command) {
  content::WebContents* contents = FocusedContents();
  if (!contents) {
    return;
  }
  switch (command) {
    case PB_EDIT_UNDO:
      contents->Undo();
      break;
    case PB_EDIT_REDO:
      contents->Redo();
      break;
    case PB_EDIT_CUT:
      contents->Cut();
      break;
    case PB_EDIT_COPY:
      contents->Copy();
      break;
    case PB_EDIT_PASTE:
      contents->Paste();
      break;
    case PB_EDIT_PASTE_AND_MATCH_STYLE:
      contents->PasteAndMatchStyle();
      break;
    case PB_EDIT_DELETE:
      contents->Delete();
      break;
    case PB_EDIT_SELECT_ALL:
      contents->SelectAll();
      break;
  }
}

void Engine::SendToPage(int64_t tab, const char* json, size_t length) {
  if (PageTab* t = this->tab(tab); t && t->internal()) {
    t->SendToPage(std::string(json, length));
  }
}

// Same reason as CloseTab: the request can arrive inside a view's own event. The window
// may have closed by then, by its own button or an earlier request.
void Engine::CloseWindow(int64_t window) {
  content::GetUIThreadTaskRunner({})->PostTask(
      FROM_HERE, base::BindOnce(
                     [](Engine* engine, int64_t window) {
                       if (AppWindow* w = engine->window(window)) {
                         w->window->Close();
                       }
                     },
                     base::Unretained(this), window));
}

void Engine::FocusTab(int64_t tab) {
  PageTab* t = this->tab(tab);
  if (t && WindowOf(tab)) {
    t->contents()->Focus();
  }
}

void Engine::FocusShell(int64_t window) {
  AppWindow* w = this->window(window);
#if BUILDFLAG(IS_ANDROID)
  if (w->native_shell) {
    w->window->FocusNativeShell();
    return;
  }
#endif
  w->shell->contents()->Focus();
}

void Engine::SendToShell(int64_t window, const char* json, size_t length) {
  AppWindow* w = this->window(window);
#if BUILDFLAG(IS_ANDROID)
  if (w->native_shell) {
    w->window->SendToNativeShell(std::string(json, length));
    return;
  }
#endif
  w->shell->Send(std::string(json, length));
}

}  // namespace project_b
