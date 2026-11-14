#ifndef PROJECT_B_ENGINE_COMMON_CONTENT_CLIENT_H_
#define PROJECT_B_ENGINE_COMMON_CONTENT_CLIENT_H_

#include <string_view>

#include "content/public/common/content_client.h"
#include "project_b/engine/buildflags/buildflags.h"

namespace project_b {

// Hands content its resources from project_b.pak.
class ContentClient : public content::ContentClient {
 public:
  ContentClient();
  ContentClient(const ContentClient&) = delete;
  ContentClient& operator=(const ContentClient&) = delete;
  ~ContentClient() override;

 private:
#if BUILDFLAG(PB_EXTENSIONS)
  void AddAdditionalSchemes(Schemes* schemes) override;
#endif
  std::string_view GetDataResource(
      int resource_id,
      ui::ResourceScaleFactor scale_factor) override;
  scoped_refptr<base::RefCountedMemory> GetDataResourceBytes(
      int resource_id) override;
  // The words content and Blink show in pages, such as a file input's Choose File, from
  // the strings in our pak.
  std::u16string GetLocalizedString(int message_id) override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_COMMON_CONTENT_CLIENT_H_
