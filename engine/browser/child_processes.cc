#include "project_b/engine/browser/child_processes.h"

#include <string>

#include "base/process/kill.h"
#include "base/process/process.h"
#include "content/public/browser/browser_child_process_host.h"
#include "content/public/browser/browser_child_process_host_iterator.h"
#include "content/public/browser/child_process_data.h"
#include "content/public/browser/child_process_termination_info.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_process_host.h"
#include "content/public/browser/web_contents.h"
#include "content/public/common/process_type.h"
#include "project_b/engine/browser/hooks.h"

namespace project_b {

namespace {

void Fire(const std::string& type,
          const content::ChildProcessTerminationInfo& info,
          base::ProcessId pid,
          const std::string& url) {
  hooks::Args args;
  args.Str(type);
  args.Int(info.status);
  args.Int(info.exit_code);
  args.Int(pid);
  args.Str(url);
  hooks::Fire(PB_EVENT_PROCESS_CRASHED, args);
}

}  // namespace

ChildProcesses::ChildProcesses() {
  content::BrowserChildProcessObserver::Add(this);
  // The GPU process and the shell's renderer start before Go, so before this.
  for (content::BrowserChildProcessHostIterator it; !it.Done(); ++it) {
    const base::Process& process =
        content::BrowserChildProcessHost::FromID(it.GetData().id)->GetProcess();
    if (process.IsValid()) {
      pids_[it.GetData().id] = process.Pid();
    }
  }
  for (auto it = content::RenderProcessHost::AllHostsIterator(); !it.IsAtEnd();
       it.Advance()) {
    content::RenderProcessHost* host = it.GetCurrentValue();
    hosts_.AddObservation(host);
    if (host->IsReady()) {
      RenderProcessReady(host);
    }
  }
}

ChildProcesses::~ChildProcesses() {
  content::BrowserChildProcessObserver::Remove(this);
}

void ChildProcesses::BrowserChildProcessLaunchedAndConnected(
    const content::ChildProcessData& data,
    const base::Process& process) {
  pids_[data.id] = process.Pid();
}

void ChildProcesses::BrowserChildProcessCrashed(
    const content::ChildProcessData& data,
    const content::ChildProcessTerminationInfo& info) {
  ChildGone(data, info);
}

void ChildProcesses::BrowserChildProcessKilled(
    const content::ChildProcessData& data,
    const content::ChildProcessTerminationInfo& info) {
  ChildGone(data, info);
}

void ChildProcesses::ChildGone(
    const content::ChildProcessData& data,
    const content::ChildProcessTerminationInfo& info) {
  std::string type = content::GetProcessTypeNameInEnglish(data.process_type);
  if (data.process_type == content::PROCESS_TYPE_UTILITY) {
    type += " " + data.metrics_name;
  }
  Fire(type, info, pids_[data.id], "");
  pids_.erase(data.id);
}

void ChildProcesses::OnRenderProcessHostCreated(
    content::RenderProcessHost* host) {
  hosts_.AddObservation(host);
}

void ChildProcesses::RenderProcessReady(content::RenderProcessHost* host) {
  pids_[host->GetDeprecatedID()] = host->GetProcess().Pid();
}

void ChildProcesses::RenderProcessExited(
    content::RenderProcessHost* host,
    const content::ChildProcessTerminationInfo& info) {
  const int id = host->GetDeprecatedID();
  const base::ProcessId pid = pids_[id];
  pids_.erase(id);
  // A renderer that is no longer needed, as for a closed tab, ends normally.
  if (info.status == base::TERMINATION_STATUS_NORMAL_TERMINATION) {
    return;
  }
  // The frame has already forgotten its URL; its tab's history hasn't.
  std::string url;
  host->ForEachRenderFrameHost([&url](content::RenderFrameHost* frame) {
    if (url.empty() && frame->IsInPrimaryMainFrame()) {
      url = content::WebContents::FromRenderFrameHost(frame)
                ->GetLastCommittedURL()
                .spec();
    }
  });
  Fire("renderer", info, pid, url);
}

void ChildProcesses::RenderProcessHostDestroyed(
    content::RenderProcessHost* host) {
  hosts_.RemoveObservation(host);
}

}  // namespace project_b
