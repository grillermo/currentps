package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const pollInterval = 2 * time.Second

type procEntry struct {
	name  string
	cmd   string
	cpu   float64
	ports []int
	pid   string
}

type model struct {
	cumulative   map[string]float64
	sampleCount  map[string]int
	latestPorts  map[string][]int
	latestPID    map[string]string
	latestCmd    map[string]string
	excluded     map[string]struct{}
	excludedPath string
	filter       string
	filtering    bool
	cursor       int
	offset       int
	width        int
	height       int
	selected     string
	displayList  []procEntry
}

var (
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
	headerStyle   = lipgloss.NewStyle().Bold(true)
	dividerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func newModel(excluded map[string]struct{}, excludedPath string) model {
	return model{
		cumulative:   make(map[string]float64),
		sampleCount:  make(map[string]int),
		latestPorts:  make(map[string][]int),
		latestPID:    make(map[string]string),
		latestCmd:    make(map[string]string),
		excluded:     excluded,
		excludedPath: excludedPath,
		width:        80,
		height:       24,
	}
}

func (m model) viewHeight() int {
	const fixedRows = 5 // title + top divider + column header + bottom divider + help
	h := m.height - fixedRows
	if h < 1 {
		return 1
	}
	return h
}

func (m model) syncedOffset() int {
	vh := m.viewHeight()
	if m.cursor < m.offset {
		return m.cursor
	}
	if m.cursor >= m.offset+vh {
		return m.cursor - vh + 1
	}
	return m.offset
}

func (m model) Init() tea.Cmd {
	return pollCmd(pollInterval)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.offset = m.syncedOffset()
		return m, nil

	case tickMsg:
		nextPorts := make(map[string][]int, len(msg.entries))
		nextPID := make(map[string]string, len(msg.entries))
		nextCmd := make(map[string]string, len(msg.entries))
		for _, e := range msg.entries {
			m.cumulative[e.name] += e.cpu
			m.sampleCount[e.name]++
			if len(e.ports) > 0 {
				nextPorts[e.name] = e.ports
			}
			nextPID[e.name] = e.pid
			nextCmd[e.name] = e.cmd
		}
		m.latestPorts = nextPorts
		m.latestPID = nextPID
		m.latestCmd = nextCmd
		m.displayList = m.buildDisplayList()
		m.cursor = clamp(m.cursor, 0, len(m.displayList)-1)
		m.offset = m.syncedOffset()
		return m, pollCmd(pollInterval)

	case tea.KeyMsg:
		if m.filtering {
			return m.updateFiltering(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
			m.offset = m.syncedOffset()
		}

	case tea.KeyDown:
		if m.cursor < len(m.displayList)-1 {
			m.cursor++
			m.offset = m.syncedOffset()
		}

	case tea.KeyEnter:
		if len(m.displayList) > 0 {
			m.selected = m.displayList[m.cursor].name
		}

	case tea.KeyF2:
		if m.selected != "" {
			if pid, err := strconv.Atoi(m.latestPID[m.selected]); err == nil {
				syscall.Kill(pid, syscall.SIGKILL)
			}
			delete(m.cumulative, m.selected)
			delete(m.sampleCount, m.selected)
			delete(m.latestPorts, m.selected)
			delete(m.latestPID, m.selected)
			m.selected = ""
			m.displayList = m.buildDisplayList()
			m.cursor = clamp(m.cursor, 0, len(m.displayList)-1)
			m.offset = m.syncedOffset()
		}

	case tea.KeyF1:
		if m.selected != "" {
			next := make(map[string]struct{}, len(m.excluded)+1)
			for k, v := range m.excluded {
				next[k] = v
			}
			next[m.selected] = struct{}{}
			m.excluded = next
			if err := appendExclusion(m.excludedPath, m.selected); err != nil {
				fmt.Fprintf(os.Stderr, "currentps: failed to persist exclusion: %v\n", err)
			}
			m.selected = ""
			m.displayList = m.buildDisplayList()
			m.cursor = clamp(m.cursor, 0, len(m.displayList)-1)
			m.offset = m.syncedOffset()
		}

	case tea.KeyEsc:
		if m.selected != "" {
			m.selected = ""
		}

	case tea.KeyRunes:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "/":
			m.filtering = true
		}
	}
	return m, nil
}

func (m model) updateFiltering(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyEsc:
		m.filtering = false
		m.filter = ""
		m.displayList = m.buildDisplayList()
		m.cursor = clamp(m.cursor, 0, len(m.displayList)-1)
		m.offset = m.syncedOffset()

	case tea.KeyEnter:
		if len(m.displayList) > 0 {
			m.selected = m.displayList[m.cursor].name
			m.filtering = false
		}

	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
			m.offset = m.syncedOffset()
		}

	case tea.KeyDown:
		if m.cursor < len(m.displayList)-1 {
			m.cursor++
			m.offset = m.syncedOffset()
		}

	case tea.KeyCtrlW:
		if len(m.filter) > 0 {
			m.filter = ""
			m.displayList = m.buildDisplayList()
			m.cursor = clamp(m.cursor, 0, len(m.displayList)-1)
			m.offset = m.syncedOffset()
		}

	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			r := []rune(m.filter)
			m.filter = string(r[:len(r)-1])
			m.displayList = m.buildDisplayList()
			m.cursor = clamp(m.cursor, 0, len(m.displayList)-1)
			m.offset = m.syncedOffset()
		}

	case tea.KeyRunes:
		m.filter += msg.String()
		m.displayList = m.buildDisplayList()
		m.cursor = 0
		m.offset = 0
	}
	return m, nil
}

func (m model) View() string {
	var sb strings.Builder

	title := "currentps"
	if m.filtering {
		title += fmt.Sprintf("   [filter: %s_]", m.filter)
	}
	if m.selected != "" {
		title += fmt.Sprintf("   selected: %s", selectedStyle.Render(m.selected))
	}
	title += fmt.Sprintf("   excluded: %d", len(m.excluded))
	sb.WriteString(headerStyle.Render(title))
	sb.WriteString("\n")
	sb.WriteString(dividerStyle.Render(strings.Repeat("─", 52)))
	sb.WriteString("\n")

	const (
		nameWidth = 25
		// prefix(2) + pos(8) + sep(2) + cpu(9) + sep(2) + pid(7) + sep(2) + port(20) + sep(2) + name(25) + sep(2)
		fixedWidth = 2 + 8 + 2 + 9 + 2 + 7 + 2 + 20 + 2 + nameWidth + 2
	)
	cmdWidth := m.width - fixedWidth
	if cmdWidth < 10 {
		cmdWidth = 10
	}

	header := fmt.Sprintf("  %-8s  %-9s  %-7s  %-20s  %-*s  %s", "Position", "Avg CPU%", "PID", "Port", nameWidth, "Process Name", "Command")
	sb.WriteString(headerStyle.Render(header))
	sb.WriteString("\n")

	end := m.offset + m.viewHeight()
	if end > len(m.displayList) {
		end = len(m.displayList)
	}
	for i := m.offset; i < end; i++ {
		p := m.displayList[i]
		prefix := "  "
		line := fmt.Sprintf("%-8d  %8.1f%%  %-7s  %-20s  %-*s  %s", i+1, p.cpu, p.pid, formatPorts(p.ports), nameWidth, p.name, truncateLeft(p.cmd, cmdWidth))
		switch {
		case p.name == m.selected:
			prefix = "★ "
			line = selectedStyle.Render(line)
		case i == m.cursor:
			prefix = "▶ "
			line = cursorStyle.Render(line)
		}
		sb.WriteString(prefix + line + "\n")
	}

	sb.WriteString(dividerStyle.Render(strings.Repeat("─", 52)))
	sb.WriteString("\n")
	var help string
	switch {
	case m.selected != "":
		help = "↑↓ navigate  F1 exclude  F2 kill  Esc deselect  q quit"
	case m.filtering:
		help = "↑↓ navigate  Enter select  type to filter  Backspace  Esc exit filter"
	default:
		help = "↑↓ navigate  Enter select  / filter  q quit"
	}
	sb.WriteString(helpStyle.Render(help))

	return sb.String()
}

func (m model) buildDisplayList() []procEntry {
	type kv struct {
		name  string
		cmd   string
		cpu   float64
		ports []int
		pid   string
	}
	filterLower := strings.ToLower(m.filter)
	all := make([]kv, 0, len(m.cumulative))
	for name, sum := range m.cumulative {
		if _, ok := m.excluded[name]; ok {
			continue
		}
		ports := m.latestPorts[name]
		if filterLower != "" && !matchesFilter(name, ports, filterLower) {
			continue
		}
		cpu := sum / float64(m.sampleCount[name])
		all = append(all, kv{name, m.latestCmd[name], cpu, ports, m.latestPID[name]})
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].cpu > all[j].cpu
	})
	result := make([]procEntry, len(all))
	for i, kv := range all {
		result[i] = procEntry{name: kv.name, cmd: kv.cmd, cpu: kv.cpu, ports: kv.ports, pid: kv.pid}
	}
	return result
}

func matchesFilter(name string, ports []int, filterLower string) bool {
	if strings.Contains(strings.ToLower(name), filterLower) {
		return true
	}
	for _, p := range ports {
		if strings.Contains(fmt.Sprintf("%d", p), filterLower) {
			return true
		}
	}
	return false
}

func truncateLeft(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return "…" + string(r[len(r)-maxLen+1:])
}

func clamp(v, lo, hi int) int {
	if hi < 0 {
		return 0
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
