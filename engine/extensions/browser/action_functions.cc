#include <string>
#include <utility>

#include "base/base64.h"
#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/strings/strcat.h"
#include "base/strings/string_number_conversions.h"
#include "base/task/thread_pool.h"
#include "extensions/common/extension.h"
#include "extensions/common/extension_resource.h"
#include "project_b/engine/extensions/browser/engine_functions.h"

namespace project_b {

namespace {

// Finding the file touches the disk, as reading it does, so this runs off the main
// thread.
std::string ReadAsDataURL(const extensions::ExtensionResource& resource) {
  std::string bytes;
  if (!base::ReadFileToString(resource.GetFilePath(), &bytes)) {
    return std::string();
  }
  return base::StrCat({"data:image/png;base64,", base::Base64Encode(bytes)});
}

}  // namespace

// details.path is one path or a table of them by size; the biggest is read.
ExtensionFunction::ResponseAction ActionSetIconFunction::Run() {
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
      base::BindOnce(&ActionSetIconFunction::OnIconRead, this));
  return RespondLater();
}

void ActionSetIconFunction::OnIconRead(std::string data_url) {
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

}  // namespace project_b
