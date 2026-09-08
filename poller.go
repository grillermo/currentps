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

	"github.com/grillermo/chicle"
)

const (
	pollInterval      = 2 * time.Second
	portsPollInterval = 5 * time.Second
)

type rawEntry struct {
	cpu  float64
	key  string
	name string
	cmd  string
	pid  string
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

// pollLoop starts the two independently-ticking pollers this program has
// always had — ps every pollInterval, lsof every portsPollInterval — and
// merges their output into a single stream of sorted chicle.Row snapshots,
// each already filtered against st's exclusion set. That filtering has to
// happen here rather than via chicle's own "/" filter: a process that is
// excluded must never reach the channel at all, not be sent and then hidden,
// so it can never flash on screen even for one frame.
//
// Both goroutines fetch once immediately (mirroring the old model's Init,
// which ran pollNowCmd/portsCmd before the first tick) so the list has real
// data as soon as possible instead of waiting a full interval.
func pollLoop(st *state) <-chan []chicle.Row {
	out := make(chan []chicle.Row, 1)
	st.out = out

	go func() {
		st.applyTick(fetchProcesses().entries)
		publish(out, st.rows())
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for range ticker.C {
			st.applyTick(fetchProcesses().entries)
			publish(out, st.rows())
		}
	}()

	go func() {
		st.applyPorts(fetchListeningPorts())
		publish(out, st.rows())
		ticker := time.NewTicker(portsPollInterval)
		defer ticker.Stop()
		for range ticker.C {
			st.applyPorts(fetchListeningPorts())
			publish(out, st.rows())
		}
	}()

	return out
}

// publish replaces whatever snapshot is queued on out with rows, without
// blocking. out has room for exactly one pending snapshot, so a consumer that
// is momentarily behind only ever sees the latest one, never a backlog — this
// is also what lets state's exclude/forget push an out-of-band refresh from
// the bubbletea event-loop goroutine without risking a deadlock against
// chicle's own single-reader loop.
func publish(out chan []chicle.Row, rows []chicle.Row) {
	for {
		select {
		case out <- rows:
			return
		default:
		}
		select {
		case <-out:
		default:
		}
	}
}
