package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/grillermo/chicle"
)

// Styles shared with kill.go's standalone `currentps kill <port>` picker,
// which predates chicle and still draws its own small bubbletea program (see
// kill.go for why: it needs a plain checkbox list keyed by port, independent
// of anything chicle owns).
var (
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	headerStyle  = lipgloss.NewStyle().Bold(true)
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// truncateLeft keeps the *end* of a long command line visible (the binary
// name up front matters less than the flags/args trailing off), used by
// kill.go's own table too.
func truncateLeft(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return "…" + string(r[len(r)-maxLen+1:])
}

// state is the live process/ports data. Both poller.go's two ticking
// goroutines and chicle's Action.Run callbacks (which run on the bubbletea
// event loop's own goroutine) read and mutate it, so every access goes
// through mu.
type state struct {
	mu  sync.Mutex
	out chan []chicle.Row // set by pollLoop once the channel exists; publish drains+refills it, so it must stay bidirectional here

	cumulative  map[string]float64
	sampleCount map[string]int
	latestPID   map[string]string
	latestCmd   map[string]string
	latestName  map[string]string
	portsByPID  map[string][]int
	portsLoaded bool

	excluded     map[string]struct{}
	excludedPath string
}

func newState(excluded map[string]struct{}, excludedPath string) *state {
	return &state{
		cumulative:   make(map[string]float64),
		sampleCount:  make(map[string]int),
		latestPID:    make(map[string]string),
		latestCmd:    make(map[string]string),
		latestName:   make(map[string]string),
		portsByPID:   make(map[string][]int),
		excluded:     excluded,
		excludedPath: excludedPath,
	}
}

// applyTick folds in a fresh ps sample. Entries missing from this round are
// dropped from every map, exactly as the old model's tickMsg handler did —
// that's how a process that has exited disappears.
func (s *state) applyTick(entries []rawEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nextCumulative := make(map[string]float64, len(entries))
	nextSampleCount := make(map[string]int, len(entries))
	nextPID := make(map[string]string, len(entries))
	nextCmd := make(map[string]string, len(entries))
	nextName := make(map[string]string, len(entries))
	for _, e := range entries {
		key := e.key
		if key == "" {
			key = e.pid
		}
		if key == "" {
			key = e.name
		}
		nextCumulative[key] = s.cumulative[key] + e.cpu
		nextSampleCount[key] = s.sampleCount[key] + 1
		nextPID[key] = e.pid
		nextCmd[key] = e.cmd
		nextName[key] = e.name
	}
	s.cumulative = nextCumulative
	s.sampleCount = nextSampleCount
	s.latestPID = nextPID
	s.latestCmd = nextCmd
	s.latestName = nextName
}

// applyPorts folds in a fresh lsof sample, keyed by PID.
func (s *state) applyPorts(ports map[string][]int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.portsByPID = ports
	s.portsLoaded = true
}

// exclude adds name to the exclusion set, persists it, and drops any entries
// currently keyed under that name so the very next published snapshot omits
// it rather than waiting a full poll cycle.
func (s *state) exclude(name string) error {
	s.mu.Lock()
	if _, ok := s.excluded[name]; !ok {
		next := make(map[string]struct{}, len(s.excluded)+1)
		for k, v := range s.excluded {
			next[k] = v
		}
		next[name] = struct{}{}
		s.excluded = next
	}
	for key, n := range s.latestName {
		if n == name {
			delete(s.cumulative, key)
			delete(s.sampleCount, key)
			delete(s.latestPID, key)
			delete(s.latestCmd, key)
			delete(s.latestName, key)
		}
	}
	out := s.out
	rows := s.rowsLocked()
	s.mu.Unlock()

	if out != nil {
		publish(out, rows)
	}
	return appendExclusion(s.excludedPath, name)
}

// forget drops a single key's tracked data, e.g. right after killing it so a
// still-exiting process doesn't reappear for one more tick.
func (s *state) forget(key string) {
	s.mu.Lock()
	delete(s.cumulative, key)
	delete(s.sampleCount, key)
	delete(s.latestPID, key)
	delete(s.latestCmd, key)
	delete(s.latestName, key)
	out := s.out
	rows := s.rowsLocked()
	s.mu.Unlock()

	if out != nil {
		publish(out, rows)
	}
}

// rows builds the current sorted, exclusion-filtered snapshot as chicle.Rows.
func (s *state) rows() []chicle.Row {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rowsLocked()
}

// rowsLocked is rows() for callers that already hold mu.
func (s *state) rowsLocked() []chicle.Row {
	type kv struct {
		key, name, cmd, pid string
		cpu                 float64
		ports               []int
	}
	all := make([]kv, 0, len(s.cumulative))
	for key, sum := range s.cumulative {
		name := s.latestName[key]
		if name == "" {
			name = key
		}
		if _, ok := s.excluded[name]; ok {
			continue
		}
		pid := s.latestPID[key]
		all = append(all, kv{
			key:   key,
			name:  name,
			cmd:   s.latestCmd[key],
			pid:   pid,
			cpu:   sum / float64(s.sampleCount[key]),
			ports: s.portsByPID[pid],
		})
	}
	// CPU desc, then name, then pid — same order as the old buildDisplayList.
	sort.Slice(all, func(i, j int) bool {
		if all[i].cpu == all[j].cpu {
			if all[i].name == all[j].name {
				return all[i].pid < all[j].pid
			}
			return all[i].name < all[j].name
		}
		return all[i].cpu > all[j].cpu
	})

	rows := make([]chicle.Row, len(all))
	for i, e := range all {
		ports := "…"
		if s.portsLoaded {
			ports = formatPorts(e.ports)
		}
		rows[i] = chicle.Row{
			Key:  e.key,
			Cols: []string{fmt.Sprintf("%.1f%%", e.cpu), e.pid, ports, e.name, e.cmd},
		}
	}
	return rows
}

// targets is what an action operates on: every ticked row, or just the
// cursor row when nothing is ticked, so the common single-target case needs
// no extra keypress.
func targets(sel chicle.Selection) []chicle.Row {
	if len(sel.Ticked) > 0 {
		return sel.Ticked
	}
	if sel.Cursor.Key == "" {
		return nil
	}
	return []chicle.Row{sel.Cursor}
}

// Row.Cols index, matching the Columns order built in main.go.
const (
	colCPU = iota
	colPID
	colPort
	colName
	colCmd
)

func actions(st *state) []chicle.Action {
	return []chicle.Action{
		{Label: "Exclude", Key: "f1", Run: excludeAction(st)},
		{Label: "Kill", Key: "f2", Confirm: killConfirm, Run: killAction(st)},
		{Label: "Copy cmd", Key: "f3", Run: copyAction},
		{Label: "Quit"},
	}
}

func excludeAction(st *state) func(chicle.Selection) chicle.Outcome {
	return func(sel chicle.Selection) chicle.Outcome {
		ts := targets(sel)
		if len(ts) == 0 {
			return chicle.Outcome{}
		}
		seen := make(map[string]struct{}, len(ts))
		var names []string
		for _, r := range ts {
			name := r.Cols[colName]
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			if err := st.exclude(name); err != nil {
				fmt.Fprintf(os.Stderr, "currentps: failed to persist exclusion: %v\n", err)
			}
			names = append(names, name)
		}
		return chicle.Outcome{Status: "Excluded: " + strings.Join(names, ", ")}
	}
}

func killConfirm(sel chicle.Selection) string {
	ts := targets(sel)
	switch len(ts) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("Kill %s (pid %s)?", ts[0].Cols[colName], ts[0].Cols[colPID])
	default:
		return fmt.Sprintf("Kill %d processes?", len(ts))
	}
}

func killAction(st *state) func(chicle.Selection) chicle.Outcome {
	return func(sel chicle.Selection) chicle.Outcome {
		ts := targets(sel)
		var killed, failed []string
		for _, r := range ts {
			pid, err := strconv.Atoi(r.Cols[colPID])
			if err != nil {
				failed = append(failed, r.Cols[colName])
				continue
			}
			if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
				failed = append(failed, r.Cols[colName])
				continue
			}
			st.forget(r.Key)
			killed = append(killed, r.Cols[colName])
		}
		var status string
		if len(killed) > 0 {
			status = "Killed: " + strings.Join(killed, ", ")
		}
		if len(failed) > 0 {
			if status != "" {
				status += "; "
			}
			status += "failed: " + strings.Join(failed, ", ")
		}
		return chicle.Outcome{Status: status}
	}
}

func copyAction(sel chicle.Selection) chicle.Outcome {
	ts := targets(sel)
	if len(ts) == 0 {
		return chicle.Outcome{}
	}
	var cmds []string
	for _, r := range ts {
		if r.Cols[colCmd] != "" {
			cmds = append(cmds, r.Cols[colCmd])
		}
	}
	if len(cmds) == 0 {
		return chicle.Outcome{Status: "Nothing to copy"}
	}
	c := exec.Command("pbcopy")
	c.Stdin = strings.NewReader(strings.Join(cmds, "\n"))
	if err := c.Run(); err != nil {
		return chicle.Outcome{Status: fmt.Sprintf("copy failed: %v", err)}
	}
	return chicle.Outcome{Status: fmt.Sprintf("Copied %d command(s)", len(cmds))}
}
