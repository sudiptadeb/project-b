#ifndef PROJECT_B_ENGINE_ANDROID_DESCRIPTORS_H_
#define PROJECT_B_ENGINE_ANDROID_DESCRIPTORS_H_

#include "content/public/common/content_descriptors.h"

namespace project_b {

// A renderer has a different user and can't read files, so it gets the pak as an open file.
enum { kPakDescriptor = kContentIPCDescriptorMax + 1 };

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_ANDROID_DESCRIPTORS_H_
