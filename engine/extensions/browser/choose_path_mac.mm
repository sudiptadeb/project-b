#include "project_b/engine/extensions/browser/choose_path.h"

#import <Cocoa/Cocoa.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>

#include <utility>

#include "base/apple/foundation_util.h"
#include "project_b/engine/abi/pb_extensions_abi.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/window.h"

namespace project_b {

void ShowPathChooser(Window* window,
                     int kind,
                     base::OnceCallback<void(const base::FilePath&)> done) {
  NSOpenPanel* panel = [NSOpenPanel openPanel];
  panel.allowsMultipleSelection = NO;
  panel.canChooseDirectories = kind == PB_PATH_FOLDER;
  panel.canChooseFiles = kind == PB_PATH_PACKAGE;
  if (kind == PB_PATH_PACKAGE) {
    panel.allowedContentTypes = @[ [UTType typeWithFilenameExtension:@"crx"] ];
  }
  __block base::OnceCallback<void(const base::FilePath&)> answer =
      std::move(done);
  [panel beginSheetModalForWindow:window->native_window().GetNativeNSWindow()
                completionHandler:^(NSModalResponse response) {
                  std::move(answer).Run(
                      response == NSModalResponseOK
                          ? base::apple::NSURLToFilePath(panel.URL)
                          : base::FilePath());
                }];
}

}  // namespace project_b
