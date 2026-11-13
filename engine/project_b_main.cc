#include "project_b/engine/abi/pb_main.h"

#include <stdlib.h>
#include <unistd.h>

#include <string>
#include <string_view>
#include <utility>

#include "base/check.h"
#include "base/command_line.h"
#include "base/files/file_path.h"
#include "base/files/file_util.h"
#include "base/process/process.h"
#include "base/strings/strcat.h"
#include "base/strings/string_split.h"
#include "base/strings/string_util.h"
#include "base/time/time.h"
#include "base/version_info/version_info.h"
#include "content/public/app/content_main.h"
#include "content/public/common/content_switches.h"
#include "project_b/engine/browser/browser_main_parts.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser_main_delegate.h"
#include "project_b/engine/common/main_delegate.h"
#include "sandbox/mac/seatbelt_exec.h"

int ProjectBMain(int argc, const char** argv, const PBGoCalls* go) {
  base::CommandLine::Init(argc, argv);

  if (go && base::CommandLine::ForCurrentProcess()->HasSwitch("version")) {
    char* ours = go->version();
    std::string line = base::StrCat(
        {ours, ", Chromium ", version_info::GetVersionNumber(), "\n"});
    free(ours);
    base::WriteFileDescriptor(STDOUT_FILENO, line);
    return 0;
  }

  if (go) {
    project_b::SetGo(go);
    // Go chooses the profile. One copy of the app per profile folder: a copy started on
    // a folder another copy owns hands it the URLs on its command line and quits, having
    // written nothing there. --user-data-dir names the profile's folder as it does for
    // Chrome, so tools that drive Chrome (ChromeDriver gives every session a fresh one)
    // drive us the same way.
    const base::CommandLine* switches = base::CommandLine::ForCurrentProcess();
    std::string urls = base::JoinString(switches->GetArgs(), "\n");
    char* copy_dir = go->claim_profile(
        switches->GetSwitchValuePath("root").value().c_str(),
        switches->GetSwitchValueASCII("profile").c_str(),
        switches->GetSwitchValuePath("user-data-dir").value().c_str(),
        switches->HasSwitch("picker"), urls.c_str());
    if (!copy_dir) {
      return 0;
    }
    project_b::SetCopyDir(base::FilePath(copy_dir));
    free(copy_dir);
    project_b::timeline::Start(
        project_b::CopyDir(),
        base::Time::Now() - base::Process::Current().CreationTime());
    project_b::timeline::Mark("main.entered");
    // Go's runtime started when the binary loaded and handles SIGSEGV, SIGBUS and
    // SIGFPE, passing on what isn't Go's. Content would reset them and, outside official
    // builds, put its stack dumper in their place, so a nil dereference in Go would never
    // reach Go. This switch is content's own for a runtime that owns these signals, as
    // a sanitizer does.
    base::CommandLine::ForCurrentProcess()->AppendSwitch(
        switches::kDisableInProcessStackTraces);
    // The flags saved in the profile, as switches. One given on the command line wins.
    base::CommandLine* command_line = base::CommandLine::ForCurrentProcess();
    char* flags = go->launch_switches(project_b::CopyDir().value().c_str());
    for (std::string_view line : base::SplitStringPiece(
             flags, "\n", base::TRIM_WHITESPACE, base::SPLIT_WANT_NONEMPTY)) {
      line.remove_prefix(2);  // "--"
      const size_t equals = line.find('=');
      const std::string_view name = line.substr(0, equals);
      if (!command_line->HasSwitch(name)) {
        command_line->AppendSwitchASCII(
            name, equals == std::string_view::npos ? "" : line.substr(equals + 1));
      }
    }
    free(flags);
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
