#include "project_b/engine/abi/pb_main.h"

#include <utility>

#include "base/check.h"
#include "base/command_line.h"
#include "base/files/file_path.h"
#include "content/public/app/content_main.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser_main_delegate.h"
#include "project_b/engine/common/main_delegate.h"
#include "sandbox/mac/seatbelt_exec.h"

int ProjectBMain(int argc, const char** argv, const PBGoCalls* go) {
  base::CommandLine::Init(argc, argv);

  if (go) {
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
