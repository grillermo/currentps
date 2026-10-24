package main

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	killPIDWidth   = 7
	killPortsWidth = 16
	killNameWidth  = 20
	// prefix(2) + checkbox(4) + pid + sep(2) + ports + sep(2) + name + sep(2)
	killFixedWidth = 2 + 4 + killPIDWidth + 2 + killPortsWidth + 2 + killNameWidth + 2
)

var (
	checkedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
)

type killTarget struct {
	pid   string
	name  string
	cmd   string
	ports []int
}

// findPortTargets pairs each PID listening on port with its ps entry so the
// confirmation list can show a name and command rather than a bare PID.
func findPortTargets(port int, portsByPID map[string][]int, entries []rawEntry) []killTarget {
	byPID := make(map[string]rawEntry, len(entries))
	for _, e := range entries {
		byPID[e.pid] = e
	}
	var targets []killTarget
	for pid, ports := range portsByPID {
		if !listensOn(ports, port) {
			continue
		}
		t := killTarget{pid: pid, ports: ports, name: "unknown"}
		if e, ok := byPID[pid]; ok {
			if e.name != "" {
				t.name = e.name
			}
			t.cmd = e.cmd
		}
		targets = append(targets, t)
	}
	sortTargets(targets)
	return targets
}

// findPatternTargets returns every process whose name or argv[0] basename
// matches the glob pattern, case-insensitively. argv[0] is checked too because
// some programs (e.g. claude) report a version string as their comm name.
// selfPID is skipped so `currentps kill 'curr*'` never kills itself.
func findPatternTargets(pattern string, portsByPID map[string][]int, entries []rawEntry, selfPID string) []killTarget {
	pattern = strings.ToLower(pattern)
	var targets []killTarget
	for _, e := range entries {
		if e.pid == selfPID || !globMatches(pattern, e.name, argv0Basename(e.cmd)) {
			continue
		}
		name := e.name
		if name == "" {
			name = "unknown"
		}
		targets = append(targets, killTarget{pid: e.pid, name: name, cmd: e.cmd, ports: portsByPID[e.pid]})
	}
	sortTargets(targets)
	return targets
}

// globMatches expects a lower-cased, already validated pattern.
func globMatches(pattern string, candidates ...string) bool {
	for _, c := range candidates {
		if ok, _ := path.Match(pattern, strings.ToLower(c)); ok {
			return true
		}
	}
	return false
}

func sortTargets(targets []killTarget) {
	sort.Slice(targets, func(i, j int) bool {
		a, aErr := strconv.Atoi(targets[i].pid)
		b, bErr := strconv.Atoi(targets[j].pid)
		if aErr != nil || bErr != nil || a == b {
			return targets[i].pid < targets[j].pid
		}
		return a < b
	})
}

func listensOn(ports []int, port int) bool {
	for _, p := range ports {
		if p == port {
			return true
		}
	}
	return false
}

type killModel struct {
	label     string
	targets   []killTarget
	checked   []bool
	cursor    int
	width     int
	height    int
	confirmed bool
	done      bool
}

// label describes what was matched, e.g. "port 11434" or `matching "claude*"`.
func newKillModel(label string, targets []killTarget) killModel {
	checked := make([]bool, len(targets))
	for i := range checked {
		checked[i] = true
	}
	return killModel{
		label:   label,
		targets: targets,
		checked: checked,
		width:   80,
		height:  24,
	}
}

func (m killModel) Init() tea.Cmd { return nil }

func (m killModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.done = true
			return m, tea.Quit

		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}

		case tea.KeyDown:
			if m.cursor < len(m.targets)-1 {
				m.cursor++
			}

		case tea.KeySpace:
			m.checked = m.toggled(m.cursor)

		case tea.KeyEnter:
			m.confirmed = true
			m.done = true
			return m, tea.Quit

		case tea.KeyRunes:
			switch msg.String() {
			case "q":
				m.done = true
				return m, tea.Quit
			case "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "j":
				if m.cursor < len(m.targets)-1 {
					m.cursor++
				}
			case " ":
				m.checked = m.toggled(m.cursor)
			case "a":
				m.checked = m.toggledAll()
			}
		}
	}
	return m, nil
}

// toggled returns a fresh slice so the value-receiver model never mutates state
// shared with the copy Bubble Tea still holds.
func (m killModel) toggled(i int) []bool {
	next := make([]bool, len(m.checked))
	copy(next, m.checked)
	if i >= 0 && i < len(next) {
		next[i] = !next[i]
	}
	return next
}

// toggledAll checks everything unless everything is already checked.
func (m killModel) toggledAll() []bool {
	all := true
	for _, c := range m.checked {
		if !c {
			all = false
			break
		}
	}
	next := make([]bool, len(m.checked))
	for i := range next {
		next[i] = !all
	}
	return next
}

func (m killModel) selectedTargets() []killTarget {
	var selected []killTarget
	for i, t := range m.targets {
		if i < len(m.checked) && m.checked[i] {
			selected = append(selected, t)
		}
	}
	return selected
}

func (m killModel) View() string {
	if m.done {
		return ""
	}

	var sb strings.Builder

	title := fmt.Sprintf("currentps kill   %s   %s", m.label, warnStyle.Render(fmt.Sprintf("%d of %d will be killed", len(m.selectedTargets()), len(m.targets))))
	sb.WriteString(headerStyle.Render(title))
	sb.WriteString("\n")
	sb.WriteString(dividerStyle.Render(strings.Repeat("─", 52)))
	sb.WriteString("\n")

	cmdWidth := m.width - killFixedWidth
	if cmdWidth < 10 {
		cmdWidth = 10
	}

	header := fmt.Sprintf("      %-*s  %-*s  %-*s  %s", killPIDWidth, "PID", killPortsWidth, "Ports", killNameWidth, "Process Name", "Command")
	sb.WriteString(headerStyle.Render(header))
	sb.WriteString("\n")

	for i, t := range m.targets {
		prefix := "  "
		if i == m.cursor {
			prefix = "▶ "
		}
		box := "[ ]"
		if m.checked[i] {
			box = checkedStyle.Render("[x]")
		}
		line := fmt.Sprintf("%-*s  %-*s  %-*s  %s",
			killPIDWidth, t.pid,
			killPortsWidth, truncateRight(joinPorts(t.ports), killPortsWidth),
			killNameWidth, truncateRight(t.name, killNameWidth),
			truncateLeft(t.cmd, cmdWidth))
		if i == m.cursor {
			line = cursorStyle.Render(line)
		}
		sb.WriteString(prefix + box + " " + line + "\n")
	}

	sb.WriteString(dividerStyle.Render(strings.Repeat("─", 52)))
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("↑↓ navigate  Space toggle  a toggle all  Enter kill  Esc/q cancel"))

	return sb.String()
}

func joinPorts(ports []int) string {
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ",")
}

func truncateRight(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen-1]) + "…"
}

func parsePortArg(arg string) (int, error) {
	port, err := strconv.Atoi(arg)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port %q", arg)
	}
	return port, nil
}

// isPortArg reports whether arg should be treated as a port rather than a
// process-name glob: any all-digit argument is a port, valid or not.
func isPortArg(arg string) bool {
	if arg == "" {
		return false
	}
	for _, r := range arg {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parsePatternArg(arg string) (string, error) {
	if _, err := path.Match(arg, ""); err != nil {
		return "", fmt.Errorf("invalid pattern %q", arg)
	}
	return arg, nil
}

// runKill drives `currentps kill <port|pattern>` and returns the process exit code.
func runKill(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: currentps kill <port|pattern>")
		return 2
	}
	arg := args[0]

	var (
		targets []killTarget
		label   string
	)
	if isPortArg(arg) {
		port, err := parsePortArg(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "currentps: %v\n", err)
			return 2
		}
		label = fmt.Sprintf("port %d", port)
		targets = findPortTargets(port, fetchListeningPorts(), fetchProcesses().entries)
		if len(targets) == 0 {
			fmt.Printf("No processes listening on port %d\n", port)
			return 0
		}
	} else {
		pattern, err := parsePatternArg(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "currentps: %v\n", err)
			return 2
		}
		label = fmt.Sprintf("matching %q", pattern)
		targets = findPatternTargets(pattern, fetchListeningPorts(), fetchProcesses().entries, strconv.Itoa(os.Getpid()))
		if len(targets) == 0 {
			fmt.Printf("No processes matching %q\n", pattern)
			return 0
		}
	}

	final, err := tea.NewProgram(newKillModel(label, targets)).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	m, ok := final.(killModel)
	if !ok || !m.confirmed {
		fmt.Println("Cancelled, nothing killed.")
		return 1
	}

	selected := m.selectedTargets()
	if len(selected) == 0 {
		fmt.Println("No processes selected, nothing killed.")
		return 0
	}
	return killTargets(selected)
}

func killTargets(targets []killTarget) int {
	failures := 0
	for _, t := range targets {
		pid, err := strconv.Atoi(t.pid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed %s (%s): bad pid\n", t.name, t.pid)
			failures++
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
			fmt.Fprintf(os.Stderr, "failed %s (%s): %v\n", t.name, t.pid, err)
			failures++
			continue
		}
		fmt.Printf("killed %s (%s)\n", t.name, t.pid)
	}
	if failures > 0 {
		return 1
	}
	return 0
}
