#include "project_b/engine/browser/timeline.h"

#include <string>
#include <vector>

#include "base/files/file_path.h"
#include "base/logging.h"
#include "base/no_destructor.h"
#include "base/strings/strcat.h"
#include "base/strings/string_number_conversions.h"
#include "base/strings/string_util.h"
#include "content/public/browser/navigation_handle.h"
#include "project_b/engine/browser/go.h"

namespace project_b {
namespace timeline {

namespace {

// Process start as base::TimeTicks reads it. base::TimeTicks and Go's clock count the
// same system counter (mach_absolute_time on a Mac, CLOCK_MONOTONIC on Android), so one
// subtraction puts any TimeTicks on the timeline. Null until Start.
base::TimeTicks g_origin;

struct Early {
  PBTimelineKind kind;
  base::TimeTicks at;
  std::string name;
};

std::vector<Early>& EarlyEntries() {
  static base::NoDestructor<std::vector<Early>> entries;
  return *entries;
}

// A null time means now, which Go reads as it records.
void Record(PBTimelineKind kind,
            base::TimeTicks at,
            std::string_view name,
            std::string_view text) {
  if (g_origin.is_null()) {
    EarlyEntries().push_back({kind, base::TimeTicks::Now(), std::string(name)});
    return;
  }
  Go().timeline_record(kind, at.is_null() ? -1 : (at - g_origin).InNanoseconds(),
                       name.data(), name.size(), text.data(), text.size());
}

// LOG lines from our own files go on the timeline. Chromium's own stay where they went
// before, on stderr: nothing checks them for URLs or page content, and the timeline is
// written to disk.
bool HandleLog(int severity,
               const char* file,
               int line,
               size_t message_start,
               const std::string& str) {
  std::string_view path(file);
  if (severity >= logging::LOGGING_FATAL || severity < logging::LOGGING_INFO ||
      path.find("project_b/engine/") == std::string_view::npos) {
    return false;
  }
  const std::string text = base::StrCat(
      {path.substr(path.rfind('/') + 1), ":", base::NumberToString(line), " ",
       base::TrimWhitespaceASCII(std::string_view(str).substr(message_start),
                                 base::TRIM_TRAILING)});
  Record(severity == logging::LOGGING_INFO      ? PB_TIMELINE_INFO
         : severity == logging::LOGGING_WARNING ? PB_TIMELINE_WARN
                                                : PB_TIMELINE_ERROR,
         base::TimeTicks(), {}, text);
  return true;
}

}  // namespace

void Start(const base::FilePath& profile_dir, base::TimeDelta process_age) {
  Go().timeline_start(profile_dir.value().c_str(), process_age.InNanoseconds());
  // Go's time, read between two readings of ours.
  const base::TimeTicks before = base::TimeTicks::Now();
  const int64_t go_now = Go().timeline_now();
  const base::TimeTicks after = base::TimeTicks::Now();
  g_origin = before + (after - before) / 2 - base::Nanoseconds(go_now);

  for (const Early& e : EarlyEntries()) {
    Record(e.kind, e.at, e.name, {});
  }
  EarlyEntries().clear();
  logging::SetLogMessageHandler(HandleLog);
}

void Mark(std::string_view name, std::string_view text) {
  Record(PB_TIMELINE_MARK, base::TimeTicks(), name, text);
}

void MarkAt(std::string_view name, base::TimeTicks at) {
  Record(PB_TIMELINE_MARK, at, name, {});
}

void Begin(std::string_view name) {
  Record(PB_TIMELINE_BEGIN, base::TimeTicks(), name, {});
}

void End(std::string_view name) {
  Record(PB_TIMELINE_END, base::TimeTicks(), name, {});
}

void PageCommitted(std::string_view page, content::NavigationHandle* handle) {
  if (!handle->IsInPrimaryMainFrame() || !handle->HasCommitted() ||
      handle->IsSameDocument()) {
    return;
  }
  Go().timeline_page_origin(
      std::string(page).c_str(),
      (handle->NavigationStart() - g_origin).InNanoseconds());
  Mark(base::StrCat({page, ".committed"}));
}

}  // namespace timeline
}  // namespace project_b
