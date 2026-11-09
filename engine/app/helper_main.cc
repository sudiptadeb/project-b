// The helper binary: the same content, no Go. Content launches it for every child
// process, with --type saying which.

#include "base/command_line.h"
#include "base/files/file_path.h"
#include "content/public/app/content_main.h"
#include "project_b/engine/common/main_delegate.h"
#include "sandbox/mac/seatbelt_exec.h"

int main(int argc, const char** argv) {
  base::CommandLine::Init(argc, argv);
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
