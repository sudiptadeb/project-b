#include "project_b/engine/extensions/utility/content_utility_client.h"

#include <memory>

#include "components/services/unzip/public/mojom/unzipper.mojom.h"
#include "components/services/unzip/unzipper_impl.h"
#include "mojo/public/cpp/bindings/pending_receiver.h"
#include "mojo/public/cpp/bindings/service_factory.h"

namespace project_b {

namespace {

auto RunUnzipper(mojo::PendingReceiver<unzip::mojom::Unzipper> receiver) {
  return std::make_unique<unzip::UnzipperImpl>(std::move(receiver));
}

}  // namespace

ContentUtilityClient::ContentUtilityClient() = default;
ContentUtilityClient::~ContentUtilityClient() = default;

void ContentUtilityClient::RegisterMainThreadServices(
    mojo::ServiceFactory& services) {
  services.Add(RunUnzipper);
}

}  // namespace project_b
