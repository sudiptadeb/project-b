#ifndef PROJECT_B_ENGINE_EXTENSIONS_BROWSER_CHOOSE_PATH_H_
#define PROJECT_B_ENGINE_EXTENSIONS_BROWSER_CHOOSE_PATH_H_

#include "base/files/file_path.h"
#include "base/functional/callback.h"

namespace project_b {

// Asks the user for a folder or an extension package (PBPathKind), in the system's own
// panel on the window. done runs with the choice, or an empty path for none.
void ShowPathChooser(int kind,
                     base::OnceCallback<void(const base::FilePath&)> done);

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_BROWSER_CHOOSE_PATH_H_
