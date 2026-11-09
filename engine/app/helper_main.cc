// The helper binary: no Go. Content launches it for every child process, with --type
// saying which, and all of that lives in libproject_b_content.dylib.

#include "project_b/engine/abi/pb_main.h"

int main(int argc, const char** argv) {
  return ProjectBMain(argc, argv, nullptr);
}
