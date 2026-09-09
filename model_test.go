package main

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
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

// TestKillActionCallsKillerWithRealPidAndForgetsOnSuccess exercises
// killAction's own logic — parsing Cols[colPID], calling the injected killer,
// and forgetting the entry only once the kill succeeds — without signaling a
// real process. state.kill is swapped for a fake that just records calls;
// the real syscall.Kill wrapper is a one-line pass-through (see newState) and
// is exercised by TestRealKillerSignalsARealChildProcess below instead of
// here, where a wrong or accidental pid would be a real footgun.
func TestKillActionCallsKillerWithRealPidAndForgetsOnSuccess(t *testing.T) {
	s := newState(make(map[string]struct{}), "")
	s.applyTick([]rawEntry{{key: "node (123)", name: "node (123)", pid: "123", cmd: "node server.js", cpu: 10}})

	var gotPID int
	var calls int
	s.kill = func(pid int) error {
		calls++
		gotPID = pid
		return nil
	}

	outcome := killAction(s)(chicle.Selection{Cursor: mustRow(t, s, "node (123)")})

	if calls != 1 {
		t.Fatalf("expected killer to be called once, got %d", calls)
	}
	if gotPID != 123 {
		t.Fatalf("expected killer called with pid 123, got %d", gotPID)
	}
	if _, ok := s.latestCmd["node (123)"]; ok {
		t.Fatal("expected latest command to be removed after a successful kill")
	}
	if outcome.Status == "" || outcome.Done {
		t.Fatalf("expected a non-empty status and Done=false, got %+v", outcome)
	}
}

// TestKillActionDoesNotForgetOnKillerFailure guards the failure path: a
// process the killer failed to signal must not be silently dropped from
// tracking, or the user would lose the row without the kill having happened.
func TestKillActionDoesNotForgetOnKillerFailure(t *testing.T) {
	s := newState(make(map[string]struct{}), "")
	s.applyTick([]rawEntry{{key: "node (123)", name: "node (123)", pid: "123", cmd: "node server.js", cpu: 10}})
	s.kill = func(pid int) error { return fmt.Errorf("permission denied") }

	killAction(s)(chicle.Selection{Cursor: mustRow(t, s, "node (123)")})

	if _, ok := s.latestCmd["node (123)"]; !ok {
		t.Fatal("expected entry to remain tracked when the kill failed")
	}
}

// TestRealKillerSignalsARealChildProcess exercises the production kill field
// (syscall.Kill via newState) end to end, against a real short-lived child
// process spawned just for this test — never against pid 0 or any pid this
// test does not own.
func TestRealKillerSignalsARealChildProcess(t *testing.T) {
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start dummy child process: %v", err)
	}

	s := newState(make(map[string]struct{}), "")
	if err := s.kill(cmd.Process.Pid); err != nil {
		t.Fatalf("kill: %v", err)
	}

	err := cmd.Wait()
	if err == nil {
		t.Fatal("expected the killed child process to exit with an error status")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected an *exec.ExitError, got %T: %v", err, err)
	}
	if !exitErr.Sys().(syscall.WaitStatus).Signaled() {
		t.Fatalf("expected the child to have been killed by a signal, got status %v", exitErr.Sys())
	}
}

func mustRow(t *testing.T, s *state, key string) chicle.Row {
	t.Helper()
	row, ok := rowByKey(s.rows(), key)
	if !ok {
		t.Fatalf("expected a row for key %q", key)
	}
	return row
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
