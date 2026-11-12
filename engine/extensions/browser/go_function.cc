#include "project_b/engine/extensions/browser/go_function.h"

#include <string_view>
#include <utility>
#include <vector>

#include "base/containers/fixed_flat_map.h"
#include "base/json/json_reader.h"
#include "base/json/json_writer.h"
#include "base/no_destructor.h"
#include "base/strings/strcat.h"
#include "extensions/browser/extension_function_registry.h"
#include "extensions/common/extension_api.h"
#include "project_b/engine/abi/pb_extensions_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/extensions/browser/engine_functions.h"

namespace project_b {

namespace {

// The APIs Go answers, from engine/BUILD.gn.
constexpr const char* kGoAPIs[] = {
#include "project_b/engine/extensions/go_apis.inc"
};

template <typename T>
scoped_refptr<ExtensionFunction> New() {
  return base::MakeRefCounted<T>();
}

// The functions the engine does its part of before Go answers, or in place of Go.
constexpr auto kEngineFunctions =
    base::MakeFixedFlatMap<std::string_view, ExtensionFunctionFactory>({
        {"action.setIcon", &New<ActionSetIconFunction>},
    });

}  // namespace

GoFunction::GoFunction() = default;
GoFunction::~GoFunction() = default;

ExtensionFunction::ResponseAction GoFunction::Run() {
  return RespondNow(Ask(args()));
}

ExtensionFunction::ResponseValue GoFunction::Ask(const base::ListValue& args) {
  GoAnswer answer = AskGo(extension_id(), name(), args,
                          hooks::TabOf(GetSenderWebContents()));
  if (!answer.error.empty()) {
    return Error(std::move(answer.error));
  }
  if (!answer.result) {
    return NoArguments();
  }
  return WithArguments(std::move(*answer.result));
}

GoAnswer AskGo(const std::string& extension_id,
               const std::string& name,
               const base::ListValue& args,
               int64_t tab) {
  hooks::Args values;
  values.Str(extension_id);
  values.Str(name);
  values.Str(base::WriteJson(args).value_or("[]"));
  values.Int(tab);
  PBValue result{};
  hooks::Ask(PB_EVENT_EXTENSION_CALL, values, &result);
  // {"result": ...}, {} or {"error": "..."}.
  std::optional<base::DictValue> answer = base::JSONReader::ReadDict(
      hooks::TakeString(result), base::JSON_PARSE_RFC);
  GoAnswer out;
  if (const std::string* error = answer->FindString("error")) {
    out.error = *error;
  } else if (std::optional<base::Value> value = answer->Extract("result")) {
    out.result = std::move(value);
  }
  return out;
}

void GoAPIProvider::RegisterExtensionFunctions(
    ExtensionFunctionRegistry* registry) {
  // The registry keeps each name by pointer, so they live as long as the process. The
  // names come from the schemas, so nothing lists them twice.
  static base::NoDestructor<std::vector<std::string>> names;
  for (const char* api : kGoAPIs) {
    const base::DictValue* schema =
        extensions::ExtensionAPI::GetSharedInstance()->GetSchema(api);
    for (const base::Value& function : *schema->FindList("functions")) {
      names->push_back(base::StrCat(
          {api, ".", *function.GetDict().FindString("name")}));
    }
  }
  for (const std::string& name : *names) {
    auto engine = kEngineFunctions.find(name);
    registry->Register(
        {engine != kEngineFunctions.end() ? engine->second : &New<GoFunction>,
         name.c_str(), extensions::functions::UNKNOWN});
  }
}

}  // namespace project_b
