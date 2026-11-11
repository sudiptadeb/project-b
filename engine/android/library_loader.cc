// What Android runs when any process loads the content library.

#include "base/android/jni_android.h"
#include "base/android/library_loader/library_loader_hooks.h"
#include "content/public/app/content_jni_onload.h"
#include "content/public/app/content_main.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser_main_delegate.h"

namespace {

bool InitializeNative(
    base::android::LibraryProcessType library_process_type) {
  if (!content::android::OnJNIOnLoadInit()) {
    return false;
  }
  // Android has no helper binary, so every process gets this delegate. Only code that runs in the
  // main process ever reaches Go.
  content::SetContentMainDelegate(new project_b::BrowserMainDelegate());
  return true;
}

}  // namespace

JNI_EXPORT jint JNI_OnLoad(JavaVM* vm, void* reserved) {
  base::android::InitVM(vm);
  // Kept until the main process starts the timeline; a child's is never read.
  project_b::timeline::Mark("main.entered");
  base::android::SetNativeInitializationHook(InitializeNative);
  return JNI_VERSION_1_4;
}
