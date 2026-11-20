// Linux's part of printing, until its dialog is chosen (docs/research/linux.md, open item 1):
// A4, and no panel yet, so a print says so in the log and ends there.

#include "base/memory/ref_counted_memory.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/printing.h"
#include "base/logging.h"

namespace project_b {
namespace printing {

gfx::SizeF PaperInches() {
  return gfx::SizeF(8.27, 11.69);
}

void ShowPanel(content::WebContents* contents,
               const std::u16string& title,
               scoped_refptr<base::RefCountedMemory> pdf) {
  LOG(ERROR) << "print: no print dialog on Linux yet";
}

}  // namespace printing
}  // namespace project_b
