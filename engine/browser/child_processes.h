#ifndef PROJECT_B_ENGINE_BROWSER_CHILD_PROCESSES_H_
#define PROJECT_B_ENGINE_BROWSER_CHILD_PROCESSES_H_

#include <map>

#include "base/process/process_handle.h"
#include "base/scoped_multi_source_observation.h"
#include "content/public/browser/browser_child_process_observer.h"
#include "content/public/browser/render_process_host.h"
#include "content/public/browser/render_process_host_creation_observer.h"
#include "content/public/browser/render_process_host_observer.h"

namespace project_b {

// Tells Go about every child process that crashes: renderers, and the GPU and utility
// processes. Content forgets a process's pid before it says the process died, so this
// keeps each one from its launch.
class ChildProcesses : public content::BrowserChildProcessObserver,
                       public content::RenderProcessHostCreationObserver,
                       public content::RenderProcessHostObserver {
 public:
  ChildProcesses();
  ChildProcesses(const ChildProcesses&) = delete;
  ChildProcesses& operator=(const ChildProcesses&) = delete;
  ~ChildProcesses() override;

 private:
  // content::BrowserChildProcessObserver
  void BrowserChildProcessLaunchedAndConnected(
      const content::ChildProcessData& data,
      const base::Process& process) override;
  void BrowserChildProcessCrashed(
      const content::ChildProcessData& data,
      const content::ChildProcessTerminationInfo& info) override;
  void BrowserChildProcessKilled(
      const content::ChildProcessData& data,
      const content::ChildProcessTerminationInfo& info) override;

  // content::RenderProcessHostCreationObserver
  void OnRenderProcessHostCreated(content::RenderProcessHost* host) override;

  // content::RenderProcessHostObserver
  void RenderProcessReady(content::RenderProcessHost* host) override;
  void RenderProcessExited(
      content::RenderProcessHost* host,
      const content::ChildProcessTerminationInfo& info) override;
  void RenderProcessHostDestroyed(content::RenderProcessHost* host) override;

  void ChildGone(const content::ChildProcessData& data,
                 const content::ChildProcessTerminationInfo& info);

  // By content's child process id, which renderers share with the others.
  std::map<int, base::ProcessId> pids_;
  base::ScopedMultiSourceObservation<content::RenderProcessHost,
                                     content::RenderProcessHostObserver>
      hosts_{this};
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_CHILD_PROCESSES_H_
