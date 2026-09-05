package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const excludedFilePath = "./currentps_excluded.txt"

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

	m := newModel(excluded, excludedFilePath)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
