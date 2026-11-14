// What the shell needs about Go's tab groups (docs/groups.md).

// The colours a group can take, Go's GroupColors: the workspaces' seven.
export const colors = ['#3470d0', '#d03434', '#4f9e9e', '#e8a317', '#8a4fd0', '#2e9d5b', '#34404f'];

export function groupOf(groups, tab) {
  return groups.find((g) => g.tabs.includes(tab)) || null;
}

// What a group is called: its name, else how many tabs it has.
export function groupLabel(g) {
  return g.name || `${g.tabs.length} tabs`;
}
