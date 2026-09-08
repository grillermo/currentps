package main

import (
	"fmt"
	"testing"

	"github.com/grillermo/chicle"
)

func rowByKey(rows []chicle.Row, key string) (chicle.Row, bool) {
	for _, r := range rows {
		if r.Key == key {
			return r, true
		}
	}
	return chicle.Row{}, false
}

func TestRowsFiltersExcluded(t *testing.T) {
	s := newState(map[string]struct{}{"firefox": {}}, "")
	s.applyTick([]rawEntry{
		{key: "firefox", name: "firefox", pid: "1", cpu: 50},
		{key: "node", name: "node", pid: "2", cpu: 30},
		{key: "bash", name: "bash", pid: "3", cpu: 10},
	})

	rows := s.rows()
	if _, ok := rowByKey(rows, "firefox"); ok {
		t.Error("firefox should be excluded from rows")
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
}

func TestRowsSortedDescendingByCPU(t *testing.T) {
	s := newState(make(map[string]struct{}), "")
	s.applyTick([]rawEntry{
		{key: "a", name: "a", pid: "1", cpu: 10},
		{key: "b", name: "b", pid: "2", cpu: 50},
		{key: "c", name: "c", pid: "3", cpu: 30},
	})

	rows := s.rows()
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0].Cols[colName] != "b" {
		t.Errorf("expected b first (highest cpu), got %q", rows[0].Cols[colName])
	}
	if rows[1].Cols[colName] != "c" {
		t.Errorf("expected c second, got %q", rows[1].Cols[colName])
	}
	if rows[2].Cols[colName] != "a" {
		t.Errorf("expected a third, got %q", rows[2].Cols[colName])
	}
}

func TestRowsShowsAllNoDisplayLimit(t *testing.T) {
	s := newState(make(map[string]struct{}), "")
	entries := make([]rawEntry, 0, 100)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("proc%d", i)
		entries = append(entries, rawEntry{key: key, name: key, pid: key, cpu: float64(i)})
	}
	s.applyTick(entries)

	rows := s.rows()
	if len(rows) != 100 {
		t.Errorf("expected 100 rows (no display limit), got %d", len(rows))
	}
}

func TestRowsProjectsPorts(t *testing.T) {
	s := newState(make(map[string]struct{}), "")
	s.applyTick([]rawEntry{{key: "node", name: "node", pid: "123", cpu: 50}})
	s.applyPorts(map[string][]int{"123": {3000, 8080}})

	rows := s.rows()
	row, ok := rowByKey(rows, "node")
	if !ok {
		t.Fatal("expected node row")
	}
	if row.Cols[colPort] != formatPorts([]int{3000, 8080}) {
		t.Errorf("expected formatted ports, got %q", row.Cols[colPort])
	}
}

func TestRowsShowsLoadingPortsBeforeFirstLsofPoll(t *testing.T) {
	s := newState(make(map[string]struct{}), "")
	s.applyTick([]rawEntry{{key: "node", name: "node", pid: "123", cpu: 50}})

	rows := s.rows()
	row, ok := rowByKey(rows, "node")
	if !ok {
		t.Fatal("expected node row")
	}
	if row.Cols[colPort] != "…" {
		t.Errorf("expected loading indicator before first lsof poll, got %q", row.Cols[colPort])
	}
}

func TestApplyTickPrunesProcessesMissingFromLatestPoll(t *testing.T) {
	s := newState(make(map[string]struct{}), "")
	s.applyTick([]rawEntry{
		{key: "123", name: "node", pid: "123", cmd: "node server.js", cpu: 10},
		{key: "456", name: "ruby", pid: "456", cmd: "ruby app.rb", cpu: 20},
	})
	s.applyTick([]rawEntry{
		{key: "123", name: "node", pid: "123", cmd: "node server.js", cpu: 30},
	})

	if _, ok := s.latestPID["456"]; ok {
		t.Fatal("expected ruby pid to be removed after disappearing from poll")
	}
	rows := s.rows()
	if len(rows) != 1 || rows[0].Cols[colName] != "node" {
		t.Fatalf("expected only live node process in rows, got %+v", rows)
	}
}

func TestApplyTickKeepsSameNameProcessesSeparate(t *testing.T) {
	s := newState(make(map[string]struct{}), "")
	s.applyTick([]rawEntry{
		{key: "123", name: "node", pid: "123", cmd: "node server.js", cpu: 10},
		{key: "456", name: "node", pid: "456", cmd: "node worker.js", cpu: 30},
	})

	rows := s.rows()
	if len(rows) != 2 {
		t.Fatalf("expected same-name processes to stay separate, got %+v", rows)
	}
	if rows[0].Cols[colPID] != "456" || rows[1].Cols[colPID] != "123" {
		t.Fatalf("expected rows to retain distinct pids sorted by cpu, got %+v", rows)
	}
}

func TestKillActionRemovesEntryAndSyscallsKill(t *testing.T) {
	s := newState(make(map[string]struct{}), "")
	// Use pid 0 so the syscall.Kill in killAction is a harmless no-op signal
	// to the caller's own process group rather than a real target; the point
	// of this test is that state.forget removes the tracked entry, not that
	// the kill syscall itself succeeds against a real pid.
	s.applyTick([]rawEntry{{key: "node (123)", name: "node (123)", pid: "0", cmd: "node server.js", cpu: 10}})

	if _, ok := s.latestCmd["node (123)"]; !ok {
		t.Fatal("expected latestCmd to be populated before kill")
	}

	s.forget("node (123)")

	if _, ok := s.latestCmd["node (123)"]; ok {
		t.Fatal("expected latest command to be removed after forget")
	}
}

func TestExcludeRemovesRowImmediately(t *testing.T) {
	s := newState(make(map[string]struct{}), t.TempDir()+"/excluded.txt")
	s.applyTick([]rawEntry{
		{key: "firefox", name: "firefox", pid: "1", cpu: 50},
		{key: "node", name: "node", pid: "2", cpu: 30},
	})

	if err := s.exclude("firefox"); err != nil {
		t.Fatalf("exclude: %v", err)
	}

	rows := s.rows()
	if _, ok := rowByKey(rows, "firefox"); ok {
		t.Error("expected firefox to disappear from rows immediately after exclude")
	}
	if len(rows) != 1 {
		t.Errorf("expected 1 row remaining, got %d", len(rows))
	}
}

func TestTargetsFallsBackToCursorWhenNothingTicked(t *testing.T) {
	cursor := chicle.Row{Key: "node", Cols: []string{"1.0%", "1", "", "node", "node server.js"}}
	sel := chicle.Selection{Cursor: cursor}

	ts := targets(sel)
	if len(ts) != 1 || ts[0].Key != "node" {
		t.Fatalf("expected fallback to cursor row, got %+v", ts)
	}
}

func TestTargetsUsesTickedOverCursor(t *testing.T) {
	cursor := chicle.Row{Key: "a"}
	ticked := []chicle.Row{{Key: "b"}, {Key: "c"}}
	sel := chicle.Selection{Cursor: cursor, Ticked: ticked}

	ts := targets(sel)
	if len(ts) != 2 || ts[0].Key != "b" || ts[1].Key != "c" {
		t.Fatalf("expected ticked rows to win over cursor, got %+v", ts)
	}
}
