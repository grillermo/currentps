package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
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

func fetchListeningPorts() map[string][]int {
	out, err := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-nP", "-P", "-F", "pn").Output()
	if err != nil {
		// lsof returns exit 1 when no matches; still has stdout we can use.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 && len(out) > 0 {
			return parseLsof(string(out))
		}
		lsofWarnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "top_cpu: lsof error: %v (port column will be empty)\n", err)
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
