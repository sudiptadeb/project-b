// Linux's one binary, the wrapper with the engine in it (docs/research/linux.md, 3). A child
// process, which content starts with --type, goes straight into content. Only the browser
// process loads the Go core, libcore.so beside the binary, and starts it, then content: so no
// child ever maps Go, as on the Mac, where children run a helper with no Go in it.

#include <dlfcn.h>
#include <limits.h>
#include <unistd.h>

#include <array>
#include <iostream>
#include <string>
#include <string_view>

#include "base/compiler_specific.h"

#include "project_b/engine/abi/pb_exports.h"
#include "project_b/engine/abi/pb_main.h"

namespace {

// Whether this is a child process: content gives every one --type=<kind>.
bool IsChild(int argc, const char** argv) {
  for (int i = 1; i < argc; ++i) {
    if (std::string_view(UNSAFE_BUFFERS(argv[i])).starts_with("--type=")) {
      return true;
    }
  }
  return false;
}

// The folder this binary is in, from the kernel, so a symlink to it still finds the library.
std::string ExeDir() {
  std::array<char, PATH_MAX> path;
  const ssize_t n = readlink("/proc/self/exe", path.data(), path.size());
  if (n <= 0) {
    return ".";
  }
  const std::string_view exe(path.data(), static_cast<size_t>(n));
  return std::string(exe.substr(0, exe.rfind('/')));
}

}  // namespace

int main(int argc, const char** argv) {
  if (IsChild(argc, argv)) {
    return ProjectBMain(argv, nullptr);
  }

  const std::string library = ExeDir() + "/libcore.so";
  void* core = dlopen(library.c_str(), RTLD_NOW | RTLD_LOCAL);
  if (!core) {
    std::cerr << "Project B can't start: its core, " << library << ", didn't load: " << dlerror()
              << "\n";
    return 1;
  }
  bool missing = false;
  auto find = [&](const char* name) {
    void* symbol = dlsym(core, name);
    if (!symbol) {
      std::cerr << "Project B can't start: its core has no " << name << "\n";
      missing = true;
    }
    return symbol;
  };
#define PB_FIND(name) reinterpret_cast<decltype(&name)>(find(#name))
  auto init = PB_FIND(PBInit);
  static const PBGoCalls go = {
      .copy_dir = PB_FIND(PBCopyDir),
      .start = PB_FIND(PBStart),
      .stop = PB_FIND(PBStop),
      .run_posted = PB_FIND(PBRunPosted),
      .hook = PB_FIND(PBHook),
      .force_gc = PB_FIND(PBForceGC),
      .recover_nil_deref = PB_FIND(PBRecoverNilDeref),
      .crash = PB_FIND(PBCrash),
      .go_stats = PB_FIND(PBGoStats),
      .shell_file = PB_FIND(PBShellFile),
      .timeline_start = PB_FIND(PBTimelineStart),
      .timeline_now = PB_FIND(PBTimelineNow),
      .timeline_record = PB_FIND(PBTimelineRecord),
      .timeline_page_origin = PB_FIND(PBTimelinePageOrigin),
      .free = PB_FIND(PBFree),
  };
#undef PB_FIND
  if (missing) {
    return 1;
  }
  const char** args = init(argc, argv);  // the Go core starts
  if (!args) {
    return 0;  // it printed the version, or handed over to the copy that owns the profile
  }
  return ProjectBMain(args, &go);  // content starts
}
