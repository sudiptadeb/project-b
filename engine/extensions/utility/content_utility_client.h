#ifndef PROJECT_B_ENGINE_EXTENSIONS_UTILITY_CONTENT_UTILITY_CLIENT_H_
#define PROJECT_B_ENGINE_EXTENSIONS_UTILITY_CONTENT_UTILITY_CLIENT_H_

#include "content/public/utility/content_utility_client.h"

namespace project_b {

// The one service of ours a utility process runs: unzipping, which unpacking an
// extension package asks for in a sandboxed process.
class ContentUtilityClient : public content::ContentUtilityClient {
 public:
  ContentUtilityClient();
  ContentUtilityClient(const ContentUtilityClient&) = delete;
  ContentUtilityClient& operator=(const ContentUtilityClient&) = delete;
  ~ContentUtilityClient() override;

 private:
  void RegisterMainThreadServices(mojo::ServiceFactory& services) override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_UTILITY_CONTENT_UTILITY_CLIENT_H_
