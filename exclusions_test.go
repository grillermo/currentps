package main

import (
	"os"
	"testing"
)

func TestLoadExclusionsFileNotExist(t *testing.T) {
	excluded, err := loadExclusions("/tmp/top_cpu_nonexistent_12345.txt")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(excluded) != 0 {
		t.Errorf("expected empty map, got %d entries", len(excluded))
	}
}

func TestLoadAndAppendExclusions(t *testing.T) {
	path := t.TempDir() + "/excluded.txt"

	excluded, err := loadExclusions(path)
	if err != nil {
		t.Fatalf("loadExclusions on empty: %v", err)
	}
	if len(excluded) != 0 {
		t.Errorf("expected empty, got %d", len(excluded))
	}

	if err := appendExclusion(path, "firefox"); err != nil {
		t.Fatalf("appendExclusion firefox: %v", err)
	}
	if err := appendExclusion(path, "node"); err != nil {
		t.Fatalf("appendExclusion node: %v", err)
	}

	excluded, err = loadExclusions(path)
	if err != nil {
		t.Fatalf("loadExclusions after append: %v", err)
	}
	if _, ok := excluded["firefox"]; !ok {
		t.Error("expected firefox in excluded")
	}
	if _, ok := excluded["node"]; !ok {
		t.Error("expected node in excluded")
	}
	if len(excluded) != 2 {
		t.Errorf("expected 2 entries, got %d", len(excluded))
	}
}

func TestLoadExclusionsDeduplicate(t *testing.T) {
	path := t.TempDir() + "/excluded.txt"
	if err := os.WriteFile(path, []byte("firefox\nnode\nfirefox\n"), 0644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	excluded, err := loadExclusions(path)
	if err != nil {
		t.Fatalf("loadExclusions: %v", err)
	}
	if len(excluded) != 2 {
		t.Errorf("expected 2 unique entries, got %d", len(excluded))
	}
}

func TestLoadExclusionsIgnoresBlankLines(t *testing.T) {
	path := t.TempDir() + "/excluded.txt"
	if err := os.WriteFile(path, []byte("firefox\n\nnode\n\n"), 0644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	excluded, err := loadExclusions(path)
	if err != nil {
		t.Fatalf("loadExclusions: %v", err)
	}
	if len(excluded) != 2 {
		t.Errorf("expected 2 entries, got %d", len(excluded))
	}
}
