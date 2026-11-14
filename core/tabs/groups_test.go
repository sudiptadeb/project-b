package tabs

import (
	"slices"
	"testing"

	"projectb/engine"
)

func groupTabs(m *Manager, w engine.WindowID) [][]engine.TabID {
	var out [][]engine.TabID
	for _, g := range m.GroupsIn(w) {
		out = append(out, g.Tabs)
	}
	return out
}

func TestAGroupKeepsItsTabsAndEndsBelowTwo(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	a, b, c := m.Create(1), m.Create(1), m.Create(1)
	lists := 0
	m.OnListChanged(func() { lists++ })
	g := m.CreateGroup([]engine.TabID{a, b}, "", "")
	gs := m.GroupsIn(1)
	if g == "" || len(gs) != 1 || gs[0].Color != GroupColors[0] || !slices.Equal(gs[0].Tabs, []engine.TabID{a, b}) || lists != 1 {
		t.Fatalf("groups %+v, lists %d", gs, lists)
	}
	if x, _ := m.Get(a); x.Group != g {
		t.Fatalf("tab a is in %q", x.Group)
	}
	// The next group takes the next colour.
	m.CreateGroup([]engine.TabID{c}, "Solo", "")
	if gs := m.GroupsIn(1); gs[1].Color != GroupColors[1] || gs[1].Name != "Solo" {
		t.Fatalf("second group %+v", gs[1])
	}
	// c joins the first group, which ends its own, a group of one left with none.
	m.AddToGroup(c, g)
	if got := groupTabs(m, 1); len(got) != 1 || !slices.Equal(got[0], []engine.TabID{a, b, c}) {
		t.Fatalf("groups %v", got)
	}
	m.RemoveFromGroup(b)
	m.Close(c)
	if got := m.GroupsIn(1); len(got) != 0 {
		t.Fatalf("a group of one is left: %+v", got)
	}
	if x, _ := m.Get(a); x.Group != "" {
		t.Fatalf("tab a is still in %q", x.Group)
	}
}

func TestAGroupHoldsOneWorkspaceOfOneWindow(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	m.AddWindow(2)
	a, b := m.CreateIn(1, ""), m.CreateIn(1, "")
	work := m.CreateIn(1, "w1")
	other := m.CreateIn(2, "")
	g := m.CreateGroup([]engine.TabID{a, work, b, other}, "", "")
	m.AddToGroup(work, g)
	m.AddToGroup(other, g)
	if got := groupTabs(m, 1); len(got) != 1 || !slices.Equal(got[0], []engine.TabID{a, b}) {
		t.Fatalf("groups %v", got)
	}
	// Moving to another window takes a tab out, and the group of one left ends.
	m.Move(b, 2, 0)
	if got := m.GroupsIn(1); len(got) != 0 {
		t.Fatalf("groups %+v", got)
	}
}

func TestANewTabWithATabJoinsOrStartsItsGroup(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	a := m.CreateIn(1, "w1")
	b := m.CreateWith(a, true)
	gs := m.GroupsIn(1)
	if len(gs) != 1 || !slices.Equal(gs[0].Tabs, []engine.TabID{a, b}) || m.ActiveIn(1) != b {
		t.Fatalf("groups %+v, active %d", gs, m.ActiveIn(1))
	}
	if x, _ := m.Get(b); x.Workspace != "w1" || x.Group != gs[0].ID {
		t.Fatalf("new tab %+v", x)
	}
	c := m.CreateWith(a, true)
	if got := groupTabs(m, 1); !slices.Equal(got[0], []engine.TabID{a, b, c}) {
		t.Fatalf("groups %v", got)
	}
}

func TestATabAPageOpensJoinsItsOpenersGroupAfterIt(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	a, b := m.Create(1), m.Create(1)
	g := m.CreateGroup([]engine.TabID{a, b}, "", "")
	e.next++
	opener, _ := m.Get(a)
	m.HandleTabOpened(a, e.next, "https://opened.test/", opener.Workspace, true)
	if got := groupTabs(m, 1); !slices.Equal(got[0], []engine.TabID{a, e.next, b}) {
		t.Fatalf("groups %v", got)
	}
	if x, _ := m.Get(e.next); x.Group != g {
		t.Fatalf("opened tab %+v", x)
	}
}

func TestEditUngroupAndCloseAGroup(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	a, b, c := m.Create(1), m.Create(1), m.Create(1)
	g := m.CreateGroup([]engine.TabID{a, b}, "", "#123456")
	name, bad, yes := "Release", "red", true
	m.EditGroup(g, &name, &bad, &yes)
	if gs := m.GroupsIn(1); gs[0].Name != "Release" || gs[0].Color != "#123456" || !gs[0].Collapsed {
		t.Fatalf("group %+v", gs[0])
	}
	m.Ungroup(g)
	if x, _ := m.Get(b); len(m.GroupsIn(1)) != 0 || x.Group != "" {
		t.Fatalf("groups %+v, b %+v", m.GroupsIn(1), x)
	}
	g = m.CreateGroup([]engine.TabID{b, c}, "", "")
	m.CloseGroup(g)
	if got := ids(m.AllIn(1)); !slices.Equal(got, []engine.TabID{a}) || len(m.GroupsIn(1)) != 0 {
		t.Fatalf("tabs %v, groups %+v", got, m.GroupsIn(1))
	}
}

func TestANewTabInAGroupCanOpenBehind(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	a := m.Create(1)
	b := m.CreateWith(a, false)
	if m.ActiveIn(1) != a || len(m.GroupsIn(1)) != 1 || !slices.Equal(m.GroupsIn(1)[0].Tabs, []engine.TabID{a, b}) {
		t.Fatalf("active %d, groups %+v", m.ActiveIn(1), m.GroupsIn(1))
	}
	if c := m.Add(1, "", false); m.ActiveIn(1) != a || len(m.AllIn(1)) != 3 || c == 0 {
		t.Fatalf("active %d, tabs %v", m.ActiveIn(1), ids(m.AllIn(1)))
	}
}
