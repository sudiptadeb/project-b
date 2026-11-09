// The main binary: content plus Go. It only ever runs as the main process, because
// content launches project_b_helper for every child.

#include "base/command_line.h"
#include "content/public/app/content_main.h"
#include "project_b/engine/browser_main_delegate.h"

int main(int argc, const char** argv) {
  base::CommandLine::Init(argc, argv);

  project_b::BrowserMainDelegate delegate;
  content::ContentMainParams params(&delegate);
  return content::ContentMain(std::move(params));
}
