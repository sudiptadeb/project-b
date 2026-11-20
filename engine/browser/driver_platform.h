#ifndef PROJECT_B_ENGINE_BROWSER_DRIVER_PLATFORM_H_
#define PROJECT_B_ENGINE_BROWSER_DRIVER_PLATFORM_H_

#include <cstdint>
#include <string>

#include "base/functional/callback.h"
#include "base/time/time.h"
#include "ui/gfx/geometry/rect_f.h"
#include "ui/gfx/native_ui_types.h"

namespace content {
class WebContents;
}

namespace project_b {

class Window;

// What the driver (driver.cc) needs of the OS's windows and input: driver_mac.mm sends real
// NSEvents through the window, driver_linux.cc real ui::Events through the Aura tree. Points are
// shell coordinates, y from the top, in the window the driver acts on.
namespace driver {

enum class Mouse { kMove, kDown, kUp };

// A key as the tests name it: one character, or enter, escape, tab, up or down, with the
// modifiers held. cmd is the app's own modifier: Command on the Mac, Ctrl on Linux.
struct Key {
  std::u16string character;  // Empty for a named key.
  std::string name;
  bool cmd = false, ctrl = false, shift = false, opt = false;
};

// Reads the driver's commands, one per line, from the FIFO at path, off the main thread, and
// runs each on the main thread; and quits the app once the process that started it is gone.
void StartReading(const std::string& path,
                  base::RepeatingCallback<void(const std::string&)> run,
                  base::OnceClosure quit);

// Who would get a mouse event here: shell, page, popup, container, frame, or a native part.
std::string Hit(Window* window, double x, double y);
// Sends one mouse event, as the OS would, and says who got it, as Hit does.
std::string SendMouse(Window* window, Mouse kind, double x, double y, int clicks);
// What a press here does to the window: "moves" when the app moves the window from it,
// "titlebar" when the OS does, otherwise who gets it.
std::string Press(Window* window, double x, double y);
// What the window may be dragged by, as the OS describes it, or "unknown".
std::string DragRegion(Window* window);
// Presses and lets go of the key, at the window's focused view.
void SendKey(Window* window, const Key& key);

// The window's own parts, as the OS has them.
std::string Title(Window* window);
bool IsKey(Window* window);
bool IsVisible(Window* window);
// "key":1,"main":1,"visible":1,"occlusionVisible":1, for stats.
std::string Flags(Window* window);
void ResizeContent(Window* window, double width, double height);
double ContentWidth(Window* window);
// The colour the window paints under everything now, 0xRRGGBB.
uint32_t BackgroundColor(Window* window);
// The band drawn over everything at the window's edge: {"radius","width","color"}.
std::string Frame(Window* window);
// Where a page's view is in the window, in shell coordinates, and whether it is hidden.
gfx::RectF ViewFrame(Window* window, gfx::NativeView view);
bool ViewHidden(gfx::NativeView view);
void SetViewHidden(gfx::NativeView view, bool hidden);
// The page's frame and the window's buttons, for "bounds", around reported, where the layout
// put the page.
std::string Bounds(Window* window, const gfx::RectF& reported, content::WebContents* page);
// Closes the window as its close button does.
void PressClose(Window* window);

// Tells the app which modifiers are held, as the OS does when they change. flags are the
// Mac's NSEventModifierFlags, as the tests write them: Shift 1 << 17, Control 1 << 18,
// Option 1 << 19, Command 1 << 20.
void SendModifiers(uint64_t flags);

// The app's menu bar as the user sees it, and choosing one of its items ("<submenu> > <item>");
// the colour of the Dock icon's dot. Nothing on Linux, which has neither.
std::string MenuJson();
bool ChooseMenu(const std::string& menu, const std::string& path);
std::string DockDot();

}  // namespace driver
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_DRIVER_PLATFORM_H_
