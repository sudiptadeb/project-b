#include "project_b/engine/extensions/renderer/tabs_hooks.h"

#include <optional>
#include <utility>

#include "extensions/common/api/messaging/message.h"
#include "extensions/common/api/messaging/messaging_util.h"
#include "extensions/common/mojom/message_port.mojom-shared.h"
#include "extensions/renderer/api/messaging/message_target.h"
#include "extensions/renderer/api/messaging/messaging_util.h"
#include "extensions/renderer/api/messaging/native_renderer_messaging_service.h"
#include "extensions/renderer/bindings/api_binding_types.h"
#include "extensions/renderer/get_script_context.h"
#include "extensions/renderer/script_context.h"

namespace project_b {

using extensions::APIBindingHooks;
using extensions::MessageTarget;
using extensions::ScriptContext;
using extensions::mojom::ChannelType;
using RequestResult = APIBindingHooks::RequestResult;
namespace messaging_util = extensions::messaging_util;

TabsHooks::TabsHooks(extensions::NativeRendererMessagingService* messaging)
    : messaging_(messaging) {}
TabsHooks::~TabsHooks() = default;

RequestResult TabsHooks::HandleRequest(
    const std::string& method_name,
    const extensions::APISignature* signature,
    v8::Local<v8::Context> context,
    v8::LocalVector<v8::Value>* arguments,
    const extensions::APITypeReferenceMap& refs) {
  const bool send = method_name == "tabs.sendMessage";
  if (!send && method_name != "tabs.connect") {
    return RequestResult(RequestResult::NOT_HANDLED);
  }
  extensions::APISignature::V8ParseResult parsed =
      signature->ParseArgumentsToV8(context, *arguments, refs);
  if (!parsed.succeeded()) {
    RequestResult result(RequestResult::INVALID_INVOCATION);
    result.error = std::move(*parsed.error);
    return result;
  }
  ScriptContext* script_context =
      extensions::GetScriptContextFromV8ContextChecked(context);
  return send ? SendMessage(script_context, parsed)
              : Connect(script_context, parsed);
}

// tabs.sendMessage(tabId, message, options, callback).
RequestResult TabsHooks::SendMessage(
    ScriptContext* script_context,
    const extensions::APISignature::V8ParseResult& parsed) {
  const v8::LocalVector<v8::Value>& arguments = *parsed.arguments;
  int tab_id = messaging_util::ExtractIntegerId(arguments[0]);
  messaging_util::MessageOptions options;
  if (!arguments[2]->IsNull()) {
    options = messaging_util::ParseMessageOptions(
        script_context->v8_context(), arguments[2].As<v8::Object>(),
        messaging_util::PARSE_FRAME_ID);
  }
  std::string error;
  std::optional<extensions::Message> message = messaging_util::MessageFromV8(
      script_context->v8_context(), arguments[1],
      messaging_util::GetSerializationFormat(script_context->extension(),
                                             ChannelType::kSendMessage),
      &error);
  if (!message) {
    RequestResult result(RequestResult::INVALID_INVOCATION);
    result.error = std::move(error);
    return result;
  }
  v8::Local<v8::Function> callback;
  if (!arguments[3]->IsNull()) {
    callback = arguments[3].As<v8::Function>();
  }
  v8::Local<v8::Promise> promise = messaging_->SendOneTimeMessage(
      script_context,
      MessageTarget::ForTab(tab_id, options.frame_id, options.document_id),
      ChannelType::kSendMessage, std::move(*message), parsed.async_type,
      callback);
  RequestResult result(RequestResult::HANDLED);
  if (parsed.async_type == extensions::binding::AsyncResponseType::kPromise) {
    result.return_value = promise;
  }
  return result;
}

// tabs.connect(tabId, options): a port.
RequestResult TabsHooks::Connect(
    ScriptContext* script_context,
    const extensions::APISignature::V8ParseResult& parsed) {
  const v8::LocalVector<v8::Value>& arguments = *parsed.arguments;
  int tab_id = messaging_util::ExtractIntegerId(arguments[0]);
  messaging_util::MessageOptions options;
  if (!arguments[1]->IsNull()) {
    options = messaging_util::ParseMessageOptions(
        script_context->v8_context(), arguments[1].As<v8::Object>(),
        messaging_util::PARSE_FRAME_ID | messaging_util::PARSE_CHANNEL_NAME);
  }
  extensions::GinPort* port = messaging_->Connect(
      script_context,
      MessageTarget::ForTab(tab_id, options.frame_id, options.document_id),
      options.channel_name,
      messaging_util::GetSerializationFormat(script_context->extension(),
                                             ChannelType::kConnect));
  RequestResult result(RequestResult::HANDLED);
  result.return_value =
      port->GetWrapper(script_context->isolate()).ToLocalChecked();
  return result;
}

}  // namespace project_b
