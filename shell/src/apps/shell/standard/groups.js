// Go's tab groups (docs/groups.md) as the standard shell lays them out: each group once,
// where its first tab is in the window's order, holding its tabs in the group's own order.
// A collapsed group shows only its active tab, if it holds the active one.

// The colours a group can take: the workspaces' seven, as Go gives them.
export const colors = ['#3470d0', '#d03434', '#4f9e9e', '#e8a317', '#8a4fd0', '#2e9d5b', '#34404f'];

// groupLabel is what a group's header says: its name, or how many tabs it has.
export function groupLabel(g) {
  return g.name || `${g.tabs.length} ${g.tabs.length === 1 ? 'tab' : 'tabs'}`;
}

// entries turns the window's tabs into what the list draws, in order: { tab } for a tab in
// no group, { group, tabs } for a group, tabs being the ones it shows. With workspace, only
// that workspace's tabs; a group is in one workspace, so it comes whole or not at all.
export function entries(state, workspace = null) {
  const shown = workspace === null ? state.tabs : state.tabs.filter((t) => t.workspace === workspace);
  const byId = new Map(shown.map((t) => [t.id, t]));
  const groups = new Map((state.groups || []).map((g) => [g.id, g]));
  const done = new Set();
  const out = [];
  for (const tab of shown) {
    const g = tab.group && groups.get(tab.group);
    if (!g) {
      out.push({ tab });
      continue;
    }
    if (done.has(g.id)) continue;
    done.add(g.id);
    const all = g.tabs.map((id) => byId.get(id)).filter(Boolean);
    const tabs = g.collapsed ? all.filter((t) => t.id === state.active) : all;
    out.push({ group: g, tabs });
  }
  return out;
}
