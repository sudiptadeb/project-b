// The Mac's part of downloads: the Save panel as a sheet on the page's window, and Finder.

#import <AppKit/AppKit.h>

#include <utility>

#include "base/apple/foundation_util.h"
#include "base/functional/callback.h"
#include "base/strings/sys_string_conversions.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/downloads.h"
#include "ui/gfx/native_ui_types.h"

namespace project_b {

void ChooseSavePath(content::WebContents* contents,
                    const base::FilePath& suggested,
                    base::OnceCallback<void(const base::FilePath&)> chosen) {
  NSSavePanel* panel = [NSSavePanel savePanel];
  panel.directoryURL = base::apple::FilePathToNSURL(suggested.DirName());
  panel.nameFieldStringValue = base::SysUTF8ToNSString(suggested.BaseName().AsUTF8Unsafe());
  panel.canCreateDirectories = YES;
  // The block may run once, and only once; the callback moves into it.
  __block base::OnceCallback<void(const base::FilePath&)> answer = std::move(chosen);
  void (^done)(NSModalResponse) = ^(NSModalResponse response) {
    base::FilePath path;
    if (response == NSModalResponseOK) {
      path = base::apple::NSURLToFilePath(panel.URL);
    }
    std::move(answer).Run(path);
  };
  NSWindow* window = contents->GetTopLevelNativeWindow().GetNativeNSWindow();
  if (window) {
    [panel beginSheetModalForWindow:window completionHandler:done];
  } else {
    [panel beginWithCompletionHandler:done];
  }
}

void OpenFile(const base::FilePath& path) {
  [NSWorkspace.sharedWorkspace openURL:base::apple::FilePathToNSURL(path)];
}

void RevealFile(const base::FilePath& path) {
  [NSWorkspace.sharedWorkspace activateFileViewerSelectingURLs:@[ base::apple::FilePathToNSURL(path) ]];
}

}  // namespace project_b
