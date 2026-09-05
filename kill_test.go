package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFindPortTargetsMatchesListenersOnly(t *testing.T) {
	ports := map[string][]int{
		"100": {11434},
		"200": {3000, 11434},
		"300": {5432},
	}
	entries := []rawEntry{
		{pid: "100", name: "ollama", cmd: "ollama serve"},
		{pid: "200", name: "node", cmd: "node proxy.js"},
		{pid: "300", name: "postgres", cmd: "postgres -D /data"},
	}

	targets := findPortTargets(11434, ports, entries)

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %+v", targets)
	}
	if targets[0].pid != "100" || targets[1].pid != "200" {
		t.Fatalf("expected targets sorted by numeric pid, got %+v", targets)
	}
	if targets[0].name != "ollama" || targets[0].cmd != "ollama serve" {
		t.Errorf("expected ps details attached to target, got %+v", targets[0])
	}
	if len(targets[1].ports) != 2 {
		t.Errorf("expected all listening ports retained, got %v", targets[1].ports)
	}
}

func TestFindPortTargetsUnknownProcess(t *testing.T) {
	targets := findPortTargets(8080, map[string][]int{"999": {8080}}, nil)

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %+v", targets)
	}
	if targets[0].name != "unknown" {
		t.Errorf("expected placeholder name for pid missing from ps, got %q", targets[0].name)
	}
}

func TestFindPortTargetsNoMatch(t *testing.T) {
	targets := findPortTargets(9999, map[string][]int{"100": {11434}}, nil)
	if len(targets) != 0 {
		t.Errorf("expected no targets, got %+v", targets)
	}
}

func TestKillModelSelectsAllByDefault(t *testing.T) {
	m := newKillModel(11434, []killTarget{{pid: "100"}, {pid: "200"}})

	if len(m.selectedTargets()) != 2 {
		t.Fatalf("expected every target selected by default, got %+v", m.selectedTargets())
	}
}

func TestKillModelSpaceDeselects(t *testing.T) {
	m := newKillModel(11434, []killTarget{{pid: "100"}, {pid: "200"}})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, _ = updated.(killModel).Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(killModel)

	selected := m.selectedTargets()
	if len(selected) != 1 || selected[0].pid != "100" {
		t.Fatalf("expected only pid 100 to remain selected, got %+v", selected)
	}
	if m.confirmed {
		t.Error("toggling should not confirm the kill")
	}
}

func TestKillModelToggleAll(t *testing.T) {
	m := newKillModel(11434, []killTarget{{pid: "100"}, {pid: "200"}})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = updated.(killModel)
	if len(m.selectedTargets()) != 0 {
		t.Fatalf("expected 'a' to clear a fully checked list, got %+v", m.selectedTargets())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = updated.(killModel)
	if len(m.selectedTargets()) != 2 {
		t.Fatalf("expected 'a' to re-check everything, got %+v", m.selectedTargets())
	}
}

func TestKillModelEnterConfirms(t *testing.T) {
	m := newKillModel(11434, []killTarget{{pid: "100"}})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(killModel)

	if !m.confirmed {
		t.Error("expected Enter to confirm")
	}
	if cmd == nil {
		t.Error("expected Enter to quit the program")
	}
}

func TestKillModelEscAndQCancel(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune("q")},
	} {
		m := newKillModel(11434, []killTarget{{pid: "100"}})
		updated, cmd := m.Update(key)
		m = updated.(killModel)

		if m.confirmed {
			t.Errorf("%v should not confirm the kill", key)
		}
		if cmd == nil {
			t.Errorf("%v should quit the program", key)
		}
	}
}

func TestKillModelViewShowsTargets(t *testing.T) {
	m := newKillModel(11434, []killTarget{{pid: "4321", name: "ollama", cmd: "ollama serve", ports: []int{11434}}})
	m.width = 100

	view := m.View()

	for _, want := range []string{"port 11434", "4321", "ollama serve", "[x]"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestKillModelViewEmptyOnceDone(t *testing.T) {
	m := newKillModel(11434, []killTarget{{pid: "100", name: "ollama"}})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if view := updated.(killModel).View(); view != "" {
		t.Errorf("expected the final frame to be blank so the kill summary stands alone, got %q", view)
	}
}

func TestParsePortArg(t *testing.T) {
	if port, err := parsePortArg("11434"); err != nil || port != 11434 {
		t.Errorf("expected 11434, got %d (%v)", port, err)
	}
	for _, bad := range []string{"", "abc", "0", "65536", "-1"} {
		if _, err := parsePortArg(bad); err == nil {
			t.Errorf("expected %q to be rejected", bad)
		}
	}
}

func TestTruncateRight(t *testing.T) {
	if got := truncateRight("short", 10); got != "short" {
		t.Errorf("expected untouched value, got %q", got)
	}
	if got := truncateRight("3000,4000,5000", 6); got != "3000,…" {
		t.Errorf("expected right truncation, got %q", got)
	}
}
