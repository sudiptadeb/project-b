// The adapter's way onto the timeline, the app's one record of what happened and when
// (core/timeline, docs/logging.md). Go keeps it; these hand it entries.
//
// A plain line of log needs nothing from here: LOG(INFO), LOG(WARNING) and LOG(ERROR)
// in engine/ land on the timeline as info, warn and error, in the main process.
//
// Any thread may record once Start has run. Before it, only the main thread.

#ifndef PROJECT_B_ENGINE_BROWSER_TIMELINE_H_
#define PROJECT_B_ENGINE_BROWSER_TIMELINE_H_

#include <string_view>

#include "base/time/time.h"

namespace base {
class FilePath;
}

namespace content {
class NavigationHandle;
}

namespace project_b {
namespace timeline {

// Starts the timeline, in the main process once Go can be reached: its file goes in the
// profile, and time zero is process_age ago. What was recorded before is handed over
// with the times it had.
void Start(const base::FilePath& profile_dir, base::TimeDelta process_age);

// A moment, by name: "window.created". text is a detail such as "tab=3"; never a URL.
void Mark(std::string_view name, std::string_view text = {});
// A moment content measured for us, such as a frame's time on screen.
void MarkAt(std::string_view name, base::TimeTicks at);

// The two ends of a span, such as "settings.open". Marks between them are its stages.
void Begin(std::string_view name);
void End(std::string_view name);

// One click or key press, as a span: it begins when the OS made the event, by the
// event's own time, has the stage "<name>.window" now, as our window gets it, and is
// ended by whoever the event was for once it has it: the page's script for a click
// ("input.click"), Go for a key ("input.key"). The first gap is the OS's, the second ours.
void Input(std::string_view name, base::TimeTicks made);

// Call from DidFinishNavigation of one of our pages, "shell" or "settings". When the
// page's document committed, its JavaScript's clock started at the navigation's start;
// this tells the timeline when that was, and marks "<page>.committed".
void PageCommitted(std::string_view page, content::NavigationHandle* handle);

}  // namespace timeline
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_TIMELINE_H_
