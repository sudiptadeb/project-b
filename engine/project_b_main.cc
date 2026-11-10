#include "project_b/engine/abi/pb_main.h"

#include <utility>

#include "base/check.h"
#include "base/command_line.h"
#include "base/files/file_path.h"
#include "content/public/app/content_main.h"
#include "content/public/common/content_switches.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser_main_delegate.h"
#include "project_b/engine/common/main_delegate.h"
#include "sandbox/mac/seatbelt_exec.h"

int ProjectBMain(int argc, const char** argv, const PBGoCalls* go) {
  base::CommandLine::Init(argc, argv);

  if (go) {
    // Go's runtime started when the binary loaded and handles SIGSEGV, SIGBUS and
    // SIGFPE, passing on what isn't Go's. Content would reset them and, outside official
    // builds, put its stack dumper in their place, so a nil dereference in Go would never
    // reach Go. This switch is content's own for a runtime that owns these signals, as
    // a sanitizer does.
    base::CommandLine::ForCurrentProcess()->AppendSwitch(
        switches::kDisableInProcessStackTraces);
    project_b::SetGo(go);
    project_b::BrowserMainDelegate delegate;
    content::ContentMainParams params(&delegate);
    return content::ContentMain(std::move(params));
  }

  base::CommandLine* command_line = base::CommandLine::ForCurrentProcess();
  sandbox::SeatbeltExecServer::CreateFromArgumentsResult seatbelt =
      sandbox::SeatbeltExecServer::CreateFromArguments(
          command_line->GetProgram().value().c_str(), argc,
          const_cast<char**>(argv));
  if (seatbelt.sandbox_required) {
    CHECK(seatbelt.server->InitializeSandbox());
  }

  project_b::MainDelegate delegate;
  content::ContentMainParams params(&delegate);
  return content::ContentMain(std::move(params));
}
