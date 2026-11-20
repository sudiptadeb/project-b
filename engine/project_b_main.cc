#include "project_b/engine/abi/pb_main.h"

#include <utility>

#include "base/check.h"
#include "base/command_line.h"
#include "base/compiler_specific.h"
#include "base/files/file_path.h"
#include "base/process/process.h"
#include "base/time/time.h"
#include "build/build_config.h"
#include "content/public/app/content_main.h"
#include "project_b/engine/browser/browser_main_parts.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser_main_delegate.h"
#include "project_b/engine/common/main_delegate.h"

#if BUILDFLAG(IS_MAC)
#include "sandbox/mac/seatbelt_exec.h"
#endif

int ProjectBMain(const char** argv, const PBGoCalls* go) {
  // NULL-terminated, as C gives main its argv.
  int argc = 0;
  while (UNSAFE_BUFFERS(argv[argc])) {
    ++argc;
  }
  base::CommandLine::Init(argc, argv);

  // The main binary's: the Go core has chosen and claimed the profile, and finished the
  // command line (PBInit, docs/layers.md).
  if (go) {
    project_b::SetGo(go);
    char* copy_dir = go->copy_dir();
    project_b::SetCopyDir(base::FilePath(copy_dir));
    go->free(copy_dir);
    project_b::timeline::Start(
        project_b::CopyDir(),
        base::Time::Now() - base::Process::Current().CreationTime());
    project_b::timeline::Mark("main.entered");
    project_b::BrowserMainDelegate delegate;
    content::ContentMainParams params(&delegate);
    return content::ContentMain(std::move(params));
  }

#if BUILDFLAG(IS_MAC)
  base::CommandLine* command_line = base::CommandLine::ForCurrentProcess();
  sandbox::SeatbeltExecServer::CreateFromArgumentsResult seatbelt =
      sandbox::SeatbeltExecServer::CreateFromArguments(
          command_line->GetProgram().value().c_str(), argc,
          const_cast<char**>(argv));
  if (seatbelt.sandbox_required) {
    CHECK(seatbelt.server->InitializeSandbox());
  }
#endif

  project_b::MainDelegate delegate;
  content::ContentMainParams params(&delegate);
  return content::ContentMain(std::move(params));
}
