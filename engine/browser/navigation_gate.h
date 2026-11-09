#ifndef PROJECT_B_ENGINE_BROWSER_NAVIGATION_GATE_H_
#define PROJECT_B_ENGINE_BROWSER_NAVIGATION_GATE_H_

#include <cstdint>

#include "content/public/browser/navigation_throttle.h"

namespace project_b {

// Asks the Go core about a navigation at start and on each redirect. Go answers
// before this returns.
class NavigationGate : public content::NavigationThrottle {
 public:
  NavigationGate(content::NavigationThrottleRegistry& registry, int64_t tab);
  ~NavigationGate() override;

 private:
  ThrottleCheckResult WillStartRequest() override;
  ThrottleCheckResult WillRedirectRequest() override;
  const char* GetNameForLogging() override;

  ThrottleCheckResult Ask();

  const int64_t tab_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_NAVIGATION_GATE_H_
