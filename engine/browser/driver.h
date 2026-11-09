#ifndef PROJECT_B_ENGINE_BROWSER_DRIVER_H_
#define PROJECT_B_ENGINE_BROWSER_DRIVER_H_

#include "base/files/file_path.h"

namespace project_b {

// Development only, enabled by --pb-driver=<fifo>. Reads commands from a FIFO and sends
// real NSEvents through the window, so tests go through the same hit test and focus
// path as a real mouse and keyboard. See tools/drive_first_app.py.
void StartDriver(const base::FilePath& fifo);

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_DRIVER_H_
