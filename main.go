package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const excludedFilePath = "./currentps_excluded.txt"

func main() {
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
