package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type rawEntry struct {
	cpu   float64
	key   string
	name  string
	cmd   string
	pid   string
}

type tickMsg struct {
	entries []rawEntry
}

var commWarnOnce sync.Once

func parsePS(output string) []rawEntry {
	return parsePSWithComms(output, nil)
}

func parsePSWithComms(output string, comms map[string]string) []rawEntry {
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
		name := processDisplayName(comms[pid], args)
		entries = append(entries, rawEntry{cpu: cpu, key: pid, name: name, cmd: args, pid: pid})
	}
	return entries
}

func processDisplayName(comm, args string) string {
	if title := customThreadName(args); title != "" {
		return title
	}

	if comm = strings.TrimSpace(comm); comm != "" {
		return comm
	}

	return argv0Basename(args)
}

func customThreadName(args string) string {
	if strings.HasSuffix(args, "]") {
		if i := strings.LastIndex(args, "["); i != -1 {
			title := strings.TrimSpace(args[i:])
			if len(title) >= 3 {
				return title
			}
		}
	}
	return ""
}

func argv0Basename(args string) string {
	argv := strings.Fields(args)
	if len(argv) == 0 {
		return "unknown"
	}
	return filepath.Base(argv[0])
}

func parseProcComms(output string) map[string]string {
	comms := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid := fields[0]
		commStart := strings.Index(line, fields[1])
		if commStart == -1 {
			continue
		}
		if comm := strings.TrimSpace(line[commStart:]); comm != "" {
			comms[pid] = comm
		}
	}
	return comms
}

func procCommPSArgs() []string {
	if runtime.GOOS == "darwin" {
		return []string{"-axo", "pid=,ucomm="}
	}
	return []string{"-eo", "pid=,comm="}
}

func fetchProcesses() tickMsg {
	var (
		wg      sync.WaitGroup
		psOut   string
		psErr   error
		commOut string
		commErr error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		out, err := exec.Command("ps", "-eo", "%cpu,pid,args").Output()
		psOut, psErr = string(out), err
	}()
	go func() {
		defer wg.Done()
		args := procCommPSArgs()
		out, err := exec.Command("ps", args...).Output()
		commOut, commErr = string(out), err
	}()
	wg.Wait()

	if psErr != nil {
		fmt.Fprintf(os.Stderr, "top_cpu: ps error: %v\n", psErr)
		return tickMsg{}
	}
	var comms map[string]string
	if commErr != nil {
		commWarnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "top_cpu: ps comm error: %v (falling back to argv names)\n", commErr)
		})
	} else {
		comms = parseProcComms(commOut)
	}
	return tickMsg{entries: parsePSWithComms(psOut, comms)}
}

func pollCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return fetchProcesses()
	})
}

// pollNowCmd runs a fetch immediately instead of waiting a full interval.
func pollNowCmd() tea.Cmd {
	return func() tea.Msg { return fetchProcesses() }
}

// portsMsg carries listening ports keyed by PID. lsof is slow, so ports are
// loaded out of band from the process list.
type portsMsg struct {
	ports map[string][]int
}

func portsCmd() tea.Cmd {
	return func() tea.Msg {
		return portsMsg{ports: fetchListeningPorts()}
	}
}

func portsPollCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return portsMsg{ports: fetchListeningPorts()}
	})
}
