package main

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

func loadExclusions(path string) (map[string]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]struct{}), nil
		}
		return nil, err
	}
	defer f.Close()

	excluded := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			excluded[name] = struct{}{}
		}
	}
	return excluded, scanner.Err()
}

func appendExclusion(path string, name string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(name + "\n")
	return err
}
