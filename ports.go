package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var lsofWarnOnce sync.Once

func parseLsof(out string) map[string][]int {
	result := make(map[string][]int)
	if out == "" {
		return result
	}
	var currentPid string
	seen := make(map[string]map[int]struct{})
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			currentPid = line[1:]
			if _, ok := seen[currentPid]; !ok {
				seen[currentPid] = make(map[int]struct{})
			}
		case 'n':
			if currentPid == "" {
				continue
			}
			addr := line[1:]
			i := strings.LastIndex(addr, ":")
			if i == -1 || i == len(addr)-1 {
				continue
			}
			port, err := strconv.Atoi(addr[i+1:])
			if err != nil {
				continue
			}
			seen[currentPid][port] = struct{}{}
		}
	}
	for pid, ports := range seen {
		if len(ports) == 0 {
			continue
		}
		sorted := make([]int, 0, len(ports))
		for p := range ports {
			sorted = append(sorted, p)
		}
		sort.Ints(sorted)
		result[pid] = sorted
	}
	return result
}

// lsofTimeout bounds the port scan: a single wedged process (hung mount, stuck
// kernel fd) can make lsof block indefinitely, which would otherwise leave the
// port column loading forever.
const lsofTimeout = 5 * time.Second

func fetchListeningPorts() map[string][]int {
	ctx, cancel := context.WithTimeout(context.Background(), lsofTimeout)
	defer cancel()

	// -b avoids kernel calls that can block on wedged processes; -w silences
	// the warnings that mode produces.
	cmd := exec.CommandContext(ctx, "lsof", "-b", "-w", "-iTCP", "-sTCP:LISTEN", "-nP", "-F", "pn")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	out := buf.Bytes()

	if ctx.Err() != nil {
		// Partial output is still useful; report the stall once.
		lsofWarnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "currentps: lsof timed out after %s (ports may be incomplete)\n", lsofTimeout)
		})
		return parseLsof(string(out))
	}
	if err != nil {
		// lsof returns exit 1 when no matches; still has stdout we can use.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 && len(out) > 0 {
			return parseLsof(string(out))
		}
		lsofWarnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "currentps: lsof error: %v (port column will be empty)\n", err)
		})
		return map[string][]int{}
	}
	return parseLsof(string(out))
}

func formatPorts(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = strconv.Itoa(p)
	}
	joined := strings.Join(parts, ",")
	const maxWidth = 20
	if len([]rune(joined)) <= maxWidth {
		return joined
	}
	runes := []rune(joined)
	return string(runes[:maxWidth-1]) + "…"
}
