#include "project_b/engine/extensions/renderer/action_hooks.h"

#include <optional>
#include <utility>
#include <vector>

#include "base/base64.h"
#include "base/strings/strcat.h"
#include "extensions/renderer/bindings/api_signature.h"
#include "gin/converter.h"
#include "ui/gfx/codec/png_codec.h"
#include "ui/gfx/geometry/size.h"
#include "v8/include/v8-container.h"
#include "v8/include/v8-context.h"
#include "v8/include/v8-isolate.h"
#include "v8/include/v8-object.h"
#include "v8/include/v8-typed-array.h"

namespace project_b {

namespace {

using RequestResult = extensions::APIBindingHooks::RequestResult;

v8::Local<v8::Value> Get(v8::Local<v8::Context> context,
                         v8::Local<v8::Object> object,
                         const char* key) {
  v8::Local<v8::Value> value;
  if (!object->Get(context, gin::StringToV8(v8::Isolate::GetCurrent(), key))
           .ToLocal(&value)) {
    return v8::Undefined(v8::Isolate::GetCurrent());
  }
  return value;
}

// An ImageData as a PNG data: URL, or nothing when it isn't one.
std::optional<std::string> ToPNG(v8::Local<v8::Context> context,
                                 v8::Local<v8::Value> value) {
  if (!value->IsObject()) {
    return std::nullopt;
  }
  v8::Local<v8::Object> image = value.As<v8::Object>();
  v8::Local<v8::Value> width = Get(context, image, "width");
  v8::Local<v8::Value> height = Get(context, image, "height");
  v8::Local<v8::Value> data = Get(context, image, "data");
  if (!width->IsUint32() || !height->IsUint32() ||
      !data->IsUint8ClampedArray()) {
    return std::nullopt;
  }
  const int w = width.As<v8::Uint32>()->Value();
  const int h = height.As<v8::Uint32>()->Value();
  v8::Local<v8::Uint8ClampedArray> pixels = data.As<v8::Uint8ClampedArray>();
  if (w <= 0 || h <= 0 || pixels->ByteLength() != static_cast<size_t>(w) * h * 4) {
    return std::nullopt;
  }
  std::vector<uint8_t> rgba(pixels->ByteLength());
  pixels->CopyContents(rgba.data(), rgba.size());
  std::optional<std::vector<uint8_t>> png = gfx::PNGCodec::Encode(
      rgba.data(), gfx::PNGCodec::FORMAT_RGBA, gfx::Size(w, h), w * 4,
      /*discard_transparency=*/false, {});
  if (!png) {
    return std::nullopt;
  }
  return base::StrCat({"data:image/png;base64,", base::Base64Encode(*png)});
}

// imageData is one ImageData or a table of them by size; the biggest is used.
std::optional<std::string> IconOf(v8::Local<v8::Context> context,
                                  v8::Local<v8::Value> image_data) {
  if (std::optional<std::string> one = ToPNG(context, image_data)) {
    return one;
  }
  if (!image_data->IsObject()) {
    return std::nullopt;
  }
  v8::Local<v8::Object> by_size = image_data.As<v8::Object>();
  v8::Local<v8::Array> sizes;
  if (!by_size->GetOwnPropertyNames(context).ToLocal(&sizes)) {
    return std::nullopt;
  }
  std::optional<std::string> best;
  uint32_t best_size = 0;
  for (uint32_t i = 0; i < sizes->Length(); ++i) {
    v8::Local<v8::Value> key;
    uint32_t size = 0;
    if (!sizes->Get(context, i).ToLocal(&key) ||
        !key->Uint32Value(context).To(&size) || size <= best_size) {
      continue;
    }
    v8::Local<v8::Value> image;
    if (by_size->Get(context, key).ToLocal(&image)) {
      if (std::optional<std::string> png = ToPNG(context, image)) {
        best = std::move(png);
        best_size = size;
      }
    }
  }
  return best;
}

}  // namespace

RequestResult ActionHooks::HandleRequest(
    const std::string& method_name,
    const extensions::APISignature* signature,
    v8::Local<v8::Context> context,
    v8::LocalVector<v8::Value>* arguments,
    const extensions::APITypeReferenceMap& refs) {
  if (method_name != "action.setIcon") {
    return RequestResult(RequestResult::NOT_HANDLED);
  }
  extensions::APISignature::V8ParseResult parsed =
      signature->ParseArgumentsToV8(context, *arguments, refs);
  if (!parsed.succeeded()) {
    RequestResult result(RequestResult::INVALID_INVOCATION);
    result.error = std::move(*parsed.error);
    return result;
  }
  v8::Local<v8::Object> details = (*parsed.arguments)[0].As<v8::Object>();
  v8::Local<v8::Value> image_data = Get(context, details, "imageData");
  if (image_data->IsUndefined() || image_data->IsNull()) {
    return RequestResult(RequestResult::NOT_HANDLED);
  }
  std::optional<std::string> icon = IconOf(context, image_data);
  if (!icon) {
    RequestResult result(RequestResult::INVALID_INVOCATION);
    result.error = "imageData is not an ImageData, or a table of them by size";
    return result;
  }
  // The details Go sees: the icon, and the tab when one is named.
  v8::Isolate* isolate = v8::Isolate::GetCurrent();
  v8::Local<v8::Object> with_icon = v8::Object::New(isolate);
  v8::Local<v8::Value> tab = Get(context, details, "tabId");
  if (!tab->IsUndefined() && !tab->IsNull() &&
      with_icon->Set(context, gin::StringToV8(isolate, "tabId"), tab)
          .IsNothing()) {
    return RequestResult(RequestResult::THROWN);
  }
  if (with_icon
          ->Set(context, gin::StringToV8(isolate, "icon"),
                gin::StringToV8(isolate, *icon))
          .IsNothing()) {
    return RequestResult(RequestResult::THROWN);
  }
  (*parsed.arguments)[0] = with_icon;
  *arguments = std::move(*parsed.arguments);
  return RequestResult(RequestResult::ARGUMENTS_UPDATED);
}

}  // namespace project_b
