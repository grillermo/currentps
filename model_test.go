package main

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBuildDisplayListFiltersExcluded(t *testing.T) {
	m := newModel(map[string]struct{}{"firefox": {}}, "")
	m.cumulative = map[string]float64{
		"firefox": 50.0,
		"node":    30.0,
		"bash":    10.0,
	}
	m.sampleCount = map[string]int{"firefox": 1, "node": 1, "bash": 1}
	list := m.buildDisplayList()

	for _, p := range list {
		if p.name == "firefox" {
			t.Error("firefox should be excluded from display list")
		}
	}
	if len(list) != 2 {
		t.Errorf("expected 2 entries, got %d", len(list))
	}
}

func TestBuildDisplayListSortedDescending(t *testing.T) {
	m := newModel(make(map[string]struct{}), "")
	m.cumulative = map[string]float64{
		"a": 10.0,
		"b": 50.0,
		"c": 30.0,
	}
	m.sampleCount = map[string]int{"a": 1, "b": 1, "c": 1}
	list := m.buildDisplayList()

	if len(list) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(list))
	}
	if list[0].name != "b" {
		t.Errorf("expected b first (highest cpu), got %q", list[0].name)
	}
	if list[1].name != "c" {
		t.Errorf("expected c second, got %q", list[1].name)
	}
	if list[2].name != "a" {
		t.Errorf("expected a third, got %q", list[2].name)
	}
}

func TestBuildDisplayListFilterCaseInsensitive(t *testing.T) {
	m := newModel(make(map[string]struct{}), "")
	m.cumulative = map[string]float64{
		"Firefox": 50.0,
		"node":    30.0,
	}
	m.sampleCount = map[string]int{"Firefox": 1, "node": 1}
	m.filter = "fire"
	list := m.buildDisplayList()

	if len(list) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list))
	}
	if list[0].name != "Firefox" {
		t.Errorf("expected Firefox, got %q", list[0].name)
	}
}

func TestBuildDisplayListShowsAll(t *testing.T) {
	m := newModel(make(map[string]struct{}), "")
	m.cumulative = make(map[string]float64)
	m.sampleCount = make(map[string]int)
	for i := 0; i < 100; i++ {
		m.cumulative[fmt.Sprintf("proc%d", i)] = float64(i)
		m.sampleCount[fmt.Sprintf("proc%d", i)] = 1
	}
	list := m.buildDisplayList()
	if len(list) != 100 {
		t.Errorf("expected 100 entries (no display limit), got %d", len(list))
	}
}

func TestBuildDisplayListFilterByPort(t *testing.T) {
	m := newModel(make(map[string]struct{}), "")
	m.cumulative = map[string]float64{
		"node (123)":     50.0,
		"postgres (456)": 30.0,
		"sshd (789)":     10.0,
	}
	m.sampleCount = map[string]int{"node (123)": 1, "postgres (456)": 1, "sshd (789)": 1}
	m.latestCmd = map[string]string{
		"node (123)":     "node server.js",
		"postgres (456)": "postgres -D /usr/local/var/postgres",
		"sshd (789)":     "/usr/sbin/sshd -D",
	}
	m.latestPorts = map[string][]int{
		"node (123)":     {3000, 8080},
		"postgres (456)": {5432},
	}

	m.filter = "300"
	list := m.buildDisplayList()
	if len(list) != 1 || list[0].name != "node (123)" {
		t.Errorf("filter \"300\" expected node (123), got %v", list)
	}

	m.filter = "5432"
	list = m.buildDisplayList()
	if len(list) != 1 || list[0].name != "postgres (456)" {
		t.Errorf("filter \"5432\" expected postgres (456), got %v", list)
	}

	m.filter = "node"
	list = m.buildDisplayList()
	if len(list) != 1 || list[0].name != "node (123)" {
		t.Errorf("filter \"node\" expected node (123), got %v", list)
	}

	m.filter = "server.js"
	list = m.buildDisplayList()
	if len(list) != 1 || list[0].name != "node (123)" {
		t.Errorf("filter \"server.js\" expected node (123), got %v", list)
	}

	m.filter = "9999"
	list = m.buildDisplayList()
	if len(list) != 0 {
		t.Errorf("filter \"9999\" expected empty, got %v", list)
	}
}

func TestBuildDisplayListFilterIgnoresPID(t *testing.T) {
	m := newModel(make(map[string]struct{}), "")
	m.cumulative = map[string]float64{
		"node (123)": 50.0,
	}
	m.sampleCount = map[string]int{"node (123)": 1}
	m.latestPorts = map[string][]int{"node (123)": {3000}}
	m.latestPID = map[string]string{"node (123)": "123"}

	m.filter = "123"
	list := m.buildDisplayList()

	if len(list) != 0 {
		t.Errorf("filter \"123\" expected empty because it only matches PID, got %v", list)
	}
}

func TestBuildDisplayListProjectsPorts(t *testing.T) {
	m := newModel(make(map[string]struct{}), "")
	m.cumulative = map[string]float64{"node (123)": 50.0}
	m.sampleCount = map[string]int{"node (123)": 1}
	m.latestPorts = map[string][]int{"node (123)": {3000, 8080}}

	list := m.buildDisplayList()
	if len(list) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list))
	}
	if len(list[0].ports) != 2 || list[0].ports[0] != 3000 || list[0].ports[1] != 8080 {
		t.Errorf("expected ports [3000 8080], got %v", list[0].ports)
	}
}

func TestBuildDisplayListEmptyCumulative(t *testing.T) {
	m := newModel(make(map[string]struct{}), "")
	list := m.buildDisplayList()
	if len(list) != 0 {
		t.Errorf("expected 0 entries for empty cumulative, got %d", len(list))
	}
}

func TestTickPrunesProcessesMissingFromLatestPoll(t *testing.T) {
	m := newModel(make(map[string]struct{}), "")

	updated, _ := m.Update(tickMsg{entries: []rawEntry{
		{key: "123", name: "node", pid: "123", cmd: "node server.js", cpu: 10},
		{key: "456", name: "ruby", pid: "456", cmd: "ruby app.rb", cpu: 20},
	}})
	m = updated.(model)

	updated, _ = m.Update(tickMsg{entries: []rawEntry{
		{key: "123", name: "node", pid: "123", cmd: "node server.js", cpu: 30},
	}})
	m = updated.(model)

	if _, ok := m.cumulative["456"]; ok {
		t.Fatal("expected ruby process to be pruned after disappearing from poll")
	}
	if _, ok := m.latestPID["456"]; ok {
		t.Fatal("expected ruby pid to be removed after disappearing from poll")
	}
	if len(m.displayList) != 1 || m.displayList[0].name != "node" {
		t.Fatalf("expected only live node process in display list, got %+v", m.displayList)
	}
	if m.displayList[0].pid != "123" {
		t.Fatalf("expected node pid to remain populated, got %q", m.displayList[0].pid)
	}
}

func TestTickClearsSelectionWhenProcessDisappears(t *testing.T) {
	m := newModel(make(map[string]struct{}), "")
	m.selected = "456"

	updated, _ := m.Update(tickMsg{entries: []rawEntry{
		{key: "123", name: "node", pid: "123", cmd: "node server.js", cpu: 30},
	}})
	m = updated.(model)

	if m.selected != "" {
		t.Fatalf("expected selection to clear when selected process disappears, got %q", m.selected)
	}
}

func TestTickKeepsSameNameProcessesSeparate(t *testing.T) {
	m := newModel(make(map[string]struct{}), "")

	updated, _ := m.Update(tickMsg{entries: []rawEntry{
		{key: "123", name: "node", pid: "123", cmd: "node server.js", cpu: 10},
		{key: "456", name: "node", pid: "456", cmd: "node worker.js", cpu: 30},
	}})
	m = updated.(model)

	if len(m.displayList) != 2 {
		t.Fatalf("expected same-name processes to stay separate, got %+v", m.displayList)
	}
	if m.displayList[0].pid != "456" || m.displayList[1].pid != "123" {
		t.Fatalf("expected rows to retain distinct pids sorted by cpu, got %+v", m.displayList)
	}
}

func TestClamp(t *testing.T) {
	if clamp(5, 0, 10) != 5 {
		t.Error("clamp(5,0,10) should be 5")
	}
	if clamp(-1, 0, 10) != 0 {
		t.Error("clamp(-1,0,10) should be 0")
	}
	if clamp(15, 0, 10) != 10 {
		t.Error("clamp(15,0,10) should be 10")
	}
	if clamp(5, 0, -1) != 0 {
		t.Error("clamp(5,0,-1) should be 0 (empty list case)")
	}
}

func TestKillRemovesLatestCmd(t *testing.T) {
	m := newModel(make(map[string]struct{}), "")
	m.selected = "node (123)"
	m.cumulative["node (123)"] = 10
	m.sampleCount["node (123)"] = 1
	m.latestPID["node (123)"] = "not-a-pid"
	m.latestCmd["node (123)"] = "node server.js"
	m.displayList = []procEntry{{name: "node (123)"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	m = updated.(model)

	if _, ok := m.latestCmd["node (123)"]; ok {
		t.Fatal("expected latest command to be removed on kill action")
	}
}
