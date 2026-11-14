#ifndef PROJECT_B_ENGINE_BROWSER_FILE_CHOOSER_H_
#define PROJECT_B_ENGINE_BROWSER_FILE_CHOOSER_H_

#include <memory>
#include <vector>

#include "base/files/file_path.h"
#include "base/memory/ref_counted.h"
#include "base/memory/weak_ptr.h"
#include "content/public/browser/file_select_listener.h"
#include "net/base/directory_lister.h"
#include "third_party/blink/public/mojom/choosers/file_chooser.mojom.h"
#include "ui/shell_dialogs/select_file_dialog.h"

namespace content {
class RenderFrameHost;
class WebContents;
}  // namespace content

namespace project_b {

// A page's <input type=file> (docs/daily-driver.md): Go says whether it may ask, and in
// which folder the panel opens; the panel is the system's own, as a sheet on the page's
// window. A folder upload hands the page every file under the folder chosen. It keeps
// itself alive until the page has its answer, as content shell's helper does.
class FileChooser : public base::RefCounted<FileChooser>,
                    public ui::SelectFileDialog::Listener,
                    public net::DirectoryLister::DirectoryListerDelegate {
 public:
  static void Run(int64_t tab,
                  content::RenderFrameHost* frame,
                  scoped_refptr<content::FileSelectListener> listener,
                  const blink::mojom::FileChooserParams& params);

  FileChooser(const FileChooser&) = delete;
  FileChooser& operator=(const FileChooser&) = delete;

 private:
  friend class base::RefCounted<FileChooser>;

  FileChooser(int64_t tab, scoped_refptr<content::FileSelectListener> listener);
  ~FileChooser() override;

  void Open(content::WebContents* contents,
            const blink::mojom::FileChooserParams& params,
            const base::FilePath& folder);
  // Answers the page with files, or with nothing when files is empty, and lets go.
  void Answer(std::vector<base::FilePath> files, const base::FilePath& base_dir);

  // ui::SelectFileDialog::Listener:
  void FileSelected(const ui::SelectedFileInfo& file, int index) override;
  void MultiFilesSelected(const std::vector<ui::SelectedFileInfo>& files) override;
  void FileSelectionCanceled() override;

  // net::DirectoryLister::DirectoryListerDelegate:
  void OnListFile(const net::DirectoryLister::DirectoryListerData& data) override;
  void OnListDone(int error) override;

  const int64_t tab_;
  scoped_refptr<content::FileSelectListener> listener_;
  scoped_refptr<ui::SelectFileDialog> dialog_;
  std::unique_ptr<ui::SelectFileDialog::FileTypeInfo> types_;
  blink::mojom::FileChooserParams::Mode mode_ =
      blink::mojom::FileChooserParams::Mode::kOpen;
  // A folder upload's listing, while it runs.
  std::unique_ptr<net::DirectoryLister> lister_;
  base::FilePath folder_;
  std::vector<base::FilePath> listed_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_FILE_CHOOSER_H_
