#ifndef PROJECT_B_ENGINE_BROWSER_MAC_APP_H_
#define PROJECT_B_ENGINE_BROWSER_MAC_APP_H_

namespace project_b {

// Creates the NSApplication content's message pump needs.
void InitMacApp();

// Adds the menu that gives us Quit, Cut, Copy, Paste and Select All.
void InstallMainMenu();

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_MAC_APP_H_
