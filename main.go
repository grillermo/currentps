package main

import (
	"fmt"
	"os"

	"github.com/grillermo/chicle"
)

const excludedFilePath = "./currentps_excluded.txt"

// cpuColumnWidth must match the "Avg CPU%" column's Width below — rowsLocked
// right-pads the CPU string to this width itself, since chicle's pad() only
// ever left-justifies.
const cpuColumnWidth = 9

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "kill":
			os.Exit(runKill(os.Args[2:]))
		default:
			fmt.Fprintf(os.Stderr, "currentps: unknown command %q\n\nusage:\n  currentps\n  currentps kill <port>\n", os.Args[1])
			os.Exit(2)
		}
	}

	excluded, err := loadExclusions(excludedFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load exclusions: %v\n", err)
		excluded = make(map[string]struct{})
	}

	st := newState(excluded, excludedFilePath)

	// Nothing about this program produces a picked value: it runs until
	// quit. The result is ignored on success, same as any other error path.
	if _, err := chicle.Run(chicle.Config{
		Title: "currentps",
		Columns: []chicle.Column{
			{Title: "Avg CPU%", Width: cpuColumnWidth},
			{Title: "PID", Width: 7},
			{Title: "Port", Width: portColumnWidth},
			{Title: "Process Name", Width: 25},
			{Title: "Command"},
		},
		MultiSelect: true,
		Actions:     actions(st),
		Updates:     pollLoop(st),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
