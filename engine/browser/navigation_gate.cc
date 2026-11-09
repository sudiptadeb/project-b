#include "project_b/engine/browser/navigation_gate.h"

#include "content/public/browser/navigation_handle.h"
#include "project_b/engine/browser/go.h"

namespace project_b {

NavigationGate::NavigationGate(content::NavigationThrottleRegistry& registry,
                               int64_t tab)
    : content::NavigationThrottle(registry), tab_(tab) {}

NavigationGate::~NavigationGate() = default;

content::NavigationThrottle::ThrottleCheckResult
NavigationGate::WillStartRequest() {
  return Ask();
}

content::NavigationThrottle::ThrottleCheckResult
NavigationGate::WillRedirectRequest() {
  return Ask();
}

const char* NavigationGate::GetNameForLogging() {
  return "project_b::NavigationGate";
}

content::NavigationThrottle::ThrottleCheckResult NavigationGate::Ask() {
  const std::string& url = navigation_handle()->GetURL().spec();
  return Go().navigation_starting(tab_, url.c_str()) ? PROCEED : BLOCK_REQUEST;
}

}  // namespace project_b
