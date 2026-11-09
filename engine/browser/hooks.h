// How a content hook reaches Go: one numbered call with a flat list of values.
//
// Go subscribes to a hook by number (SetSubscribed). A generated override
// (hooks_gen.h) checks that bit, and when it is set copies its arguments into plain
// values and crosses once. An unsubscribed hook costs the check and nothing else.
// The adapter's own events (PBEvent in pb_abi.h) use the same call without a check.

#ifndef PROJECT_B_ENGINE_BROWSER_HOOKS_H_
#define PROJECT_B_ENGINE_BROWSER_HOOKS_H_

#include <cstdint>
#include <deque>
#include <string>
#include <vector>

#include "project_b/engine/abi/pb_abi.h"

namespace content {
class WebContents;
}

namespace project_b {
namespace hooks {

// The values of one call, in order. Strings live here until the call returns.
class Args {
 public:
  Args();
  Args(const Args&) = delete;
  Args& operator=(const Args&) = delete;
  ~Args();

  void Int(int64_t v);
  void Bool(bool v);
  void Double(double v);
  void Str(std::string v);
  // What a null object becomes: as many zero values as its copy would have had.
  void Zeros(int count);

  const PBValue* data() const { return values_.data(); }
  size_t size() const { return values_.size(); }

 private:
  std::vector<PBValue> values_;
  std::deque<std::string> strings_;
};

bool Subscribed(int32_t hook);
void SetSubscribed(int32_t hook, bool on);

// Fire crosses for a hook that only informs. Ask crosses for one that wants an answer
// and returns whether Go gave one; a string answer is read with TakeString, which frees
// it. Both must be on the main thread.
void Fire(int32_t hook, const Args& args);
bool Ask(int32_t hook, const Args& args, PBValue* result);
std::string TakeString(const PBValue& result);

// The tab id a WebContents belongs to, 0 for the shell or none. Set once at creation.
void TagTab(content::WebContents* contents, int64_t tab);
int64_t TabOf(content::WebContents* contents);

}  // namespace hooks
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_HOOKS_H_
