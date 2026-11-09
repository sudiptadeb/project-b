#include "project_b/engine/common/content_client.h"

#include "base/memory/ref_counted_memory.h"
#include "ui/base/resource/resource_bundle.h"

namespace project_b {

ContentClient::ContentClient() = default;
ContentClient::~ContentClient() = default;

std::string_view ContentClient::GetDataResource(
    int resource_id,
    ui::ResourceScaleFactor scale_factor) {
  return ui::ResourceBundle::GetSharedInstance().GetRawDataResourceForScale(
      resource_id, scale_factor);
}

scoped_refptr<base::RefCountedMemory> ContentClient::GetDataResourceBytes(
    int resource_id) {
  return ui::ResourceBundle::GetSharedInstance().LoadDataResourceBytes(
      resource_id);
}

}  // namespace project_b
