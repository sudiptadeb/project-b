#include "project_b/engine/browser/hooks.h"

#include <array>
#include <cstdlib>
#include <memory>
#include <utility>

#include "base/check.h"
#include "base/supports_user_data.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/hooks_gen.h"

namespace project_b {
namespace hooks {

namespace {

std::array<bool, kHookEnd> g_subscribed = {};

struct TabTag : public base::SupportsUserData::Data {
  explicit TabTag(int64_t tab) : tab(tab) {}
  const int64_t tab;
};
const void* const kTabTagKey = &kTabTagKey;

}  // namespace

Args::Args() {
  values_.reserve(16);
}

Args::~Args() = default;

void Args::Int(int64_t v) {
  values_.push_back(PBValue{.i = v});
}

void Args::Bool(bool v) {
  values_.push_back(PBValue{.i = v ? 1 : 0});
}

void Args::Double(double v) {
  values_.push_back(PBValue{.d = v});
}

void Args::Str(std::string v) {
  strings_.push_back(std::move(v));
  const std::string& kept = strings_.back();
  values_.push_back(PBValue{.s = kept.data(), .len = kept.size()});
}

void Args::Zeros(int count) {
  values_.insert(values_.end(), count, PBValue{});
}

bool Subscribed(int32_t hook) {
  return g_subscribed[hook];
}

void SetSubscribed(int32_t hook, bool on) {
  CHECK(hook >= kFirstHook && hook < kHookEnd) << hook;
  g_subscribed[hook] = on;
}

void Fire(int32_t hook, const Args& args) {
  CHECK(content::BrowserThread::CurrentlyOn(content::BrowserThread::UI)) << hook;
  PBValue unused;
  Go().hook(hook, args.data(), args.size(), &unused);
}

bool Ask(int32_t hook, const Args& args, PBValue* result) {
  CHECK(content::BrowserThread::CurrentlyOn(content::BrowserThread::UI)) << hook;
  *result = PBValue{};
  return Go().hook(hook, args.data(), args.size(), result) != 0;
}

std::string TakeString(const PBValue& result) {
  if (!result.s) {
    return std::string();
  }
  std::string s(result.s, result.len);
  free(const_cast<char*>(result.s));
  return s;
}

void TagTab(content::WebContents* contents, int64_t tab) {
  contents->SetUserData(kTabTagKey, std::make_unique<TabTag>(tab));
}

int64_t TabOf(content::WebContents* contents) {
  if (!contents) {
    return 0;
  }
  auto* tag = static_cast<TabTag*>(contents->GetUserData(kTabTagKey));
  return tag ? tag->tab : 0;
}

}  // namespace hooks
}  // namespace project_b
