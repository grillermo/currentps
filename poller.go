package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type rawEntry struct {
	cpu   float64
	name  string
	cmd   string
	pid   string
	ports []int
}

type tickMsg struct {
	entries []rawEntry
}

func parsePS(output string) []rawEntry {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil
	}
	var entries []rawEntry
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		cpu, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		pid := fields[1]
		argsStart := strings.Index(line, fields[2])
		if argsStart == -1 {
			continue
		}
		args := strings.TrimSpace(line[argsStart:])
		name := processDisplayName(args, pid)
		entries = append(entries, rawEntry{cpu: cpu, name: name, cmd: args, pid: pid})
	}
	return entries
}

func processDisplayName(args, pid string) string {
	if strings.HasSuffix(args, "]") {
		if i := strings.LastIndex(args, "["); i != -1 {
			title := strings.TrimSpace(args[i:])
			if len(title) >= 3 {
			return fmt.Sprintf("%s (%s)", title, pid)
			}
		}
	}

	argv := strings.Fields(args)
	if len(argv) == 0 {
		return fmt.Sprintf("unknown (%s)", pid)
	}
	return fmt.Sprintf("%s (%s)", filepath.Base(argv[0]), pid)
}

func fetchProcesses() tickMsg {
	var (
		wg       sync.WaitGroup
		psOut    string
		psErr    error
		portsMap map[string][]int
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		out, err := exec.Command("ps", "-eo", "%cpu,pid,args").Output()
		psOut, psErr = string(out), err
	}()
	go func() {
		defer wg.Done()
		portsMap = fetchListeningPorts()
	}()
	wg.Wait()

	if psErr != nil {
		fmt.Fprintf(os.Stderr, "top_cpu: ps error: %v\n", psErr)
		return tickMsg{}
	}
	entries := parsePS(psOut)
	for i := range entries {
		entries[i].ports = portsMap[entries[i].pid]
	}
	return tickMsg{entries: entries}
}

func pollCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return fetchProcesses()
	})
}
