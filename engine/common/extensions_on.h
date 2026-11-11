#ifndef PROJECT_B_ENGINE_COMMON_EXTENSIONS_ON_H_
#define PROJECT_B_ENGINE_COMMON_EXTENSIONS_ON_H_

#include "base/command_line.h"
#include "project_b/engine/buildflags/buildflags.h"

namespace project_b {

// The switch a process runs extensions with. Go adds it to the browser's command line
// when the Extensions flag was on at launch (docs/flags.md), and the browser passes it
// to every child process it starts.
inline constexpr char kExtensionsSwitch[] = "pb-extensions";

// Whether this process runs extensions: the build has them and the flag is on. Off, the
// code is in the build and none of it starts.
inline bool ExtensionsOn() {
#if BUILDFLAG(PB_EXTENSIONS)
  return base::CommandLine::ForCurrentProcess()->HasSwitch(kExtensionsSwitch);
#else
  return false;
#endif
}

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_COMMON_EXTENSIONS_ON_H_
