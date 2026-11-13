#ifndef PROJECT_B_ENGINE_BROWSER_MAC_APP_H_
#define PROJECT_B_ENGINE_BROWSER_MAC_APP_H_

#include <cstdint>
#include <string>

namespace project_b {

// Creates the NSApplication content's message pump needs.
void InitMacApp();

// Replaces the menu bar with Go's engine.Menu, as JSON. Choosing an item tells Go.
void SetMainMenu(const std::string& json);

// Tells Go which modifiers are held now, from NSEventModifierFlags.
void ReportModifiers(uint64_t flags);

// Runs a PBAppCommand.
void RunAppCommand(int32_t command);

// Draws the Dock icon as the app's own with a small dot in rgb, 0xRRGGBB, at its bottom
// right.
void SetDockColor(uint32_t rgb);

// Tells Go the keys macOS App Shortcuts gives our menu items, now and on every change.
void WatchOSKeyBindings();

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_MAC_APP_H_
