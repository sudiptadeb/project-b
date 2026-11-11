#include "project_b/engine/extensions/browser/go_function.h"

#include <utility>
#include <vector>

#include "base/base64.h"
#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/json/json_reader.h"
#include "base/json/json_writer.h"
#include "base/no_destructor.h"
#include "base/strings/strcat.h"
#include "base/strings/string_number_conversions.h"
#include "base/task/thread_pool.h"
#include "extensions/browser/extension_function.h"
#include "extensions/browser/extension_function_registry.h"
#include "extensions/common/extension.h"
#include "extensions/common/extension_api.h"
#include "extensions/common/extension_resource.h"
#include "project_b/engine/abi/pb_extensions_abi.h"
#include "project_b/engine/browser/hooks.h"

namespace project_b {

namespace {

// The APIs Go answers. Their schemas are Chrome's (engine/BUILD.gn).
constexpr const char* kGoAPIs[] = {"action", "commands", "tabs", "windows"};

// Finding the file touches the disk, as reading it does, so this runs off the main
// thread.
std::string ReadAsDataURL(const extensions::ExtensionResource& resource) {
  std::string bytes;
  if (!base::ReadFileToString(resource.GetFilePath(), &bytes)) {
    return std::string();
  }
  return base::StrCat({"data:image/png;base64,", base::Base64Encode(bytes)});
}

// Every function of those APIs. It has no logic of its own: the function's name and
// arguments go to Go as they came, and Go's answer goes back.
class GoFunction : public ExtensionFunction {
 public:
  GoFunction() = default;

 private:
  ~GoFunction() override = default;

  ResponseAction Run() override {
    // An icon is named by a file inside the extension, which Go can't read through the
    // engine: it gets the picture itself, as a data: URL under "icon".
    if (std::string_view(name()) == "action.setIcon") {
      return SetIcon();
    }
    return RespondNow(Ask(args()));
  }

  ResponseValue Ask(const base::ListValue& args) {
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

  // details.path is one path or a table of them by size; the biggest is read.
  ResponseAction SetIcon() {
    const base::DictValue* details =
        args().empty() ? nullptr : args()[0].GetIfDict();
    std::string path;
    if (details) {
      if (const std::string* one = details->FindString("path")) {
        path = *one;
      } else if (const base::DictValue* by_size = details->FindDict("path")) {
        int best = 0;
        for (const auto [size, value] : *by_size) {
          int n = 0;
          if (base::StringToInt(size, &n) && n > best && value.is_string()) {
            best = n;
            path = value.GetString();
          }
        }
      }
    }
    if (path.empty()) {
      return RespondNow(
          Error("action.setIcon takes the icon as a path in Project B."));
    }
    base::ThreadPool::PostTaskAndReplyWithResult(
        FROM_HERE, {base::MayBlock()},
        base::BindOnce(&ReadAsDataURL, extension()->GetResource(path)),
        base::BindOnce(&GoFunction::OnIconRead, this));
    return RespondLater();
  }

  void OnIconRead(std::string data_url) {
    if (data_url.empty()) {
      Respond(Error("The icon could not be read."));
      return;
    }
    base::DictValue details = args()[0].GetDict().Clone();
    details.Remove("path");
    details.Set("icon", std::move(data_url));
    base::ListValue with_icon;
    with_icon.Append(std::move(details));
    Respond(Ask(with_icon));
  }
};

scoped_refptr<ExtensionFunction> NewGoFunction() {
  return base::MakeRefCounted<GoFunction>();
}

}  // namespace

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
  // The registry keeps each name by pointer, so they live as long as the process.
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
    registry->Register({&NewGoFunction, name.c_str(),
                        extensions::functions::UNKNOWN});
  }
}

}  // namespace project_b
