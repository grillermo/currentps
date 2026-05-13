package main

import (
	"testing"
)

func TestParsePSNormalOutput(t *testing.T) {
	input := `%CPU   PID ARGS
  1.5   101 firefox
 23.4   202 node server.js
  0.0   303 /usr/bin/ps -eo %cpu,pid,args`

	entries := parsePS(input)

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].cpu != 1.5 || entries[0].name != "firefox (101)" || entries[0].pid != "101" {
		t.Errorf("entry 0: got cpu=%v name=%q pid=%q, want cpu=1.5 name=\"firefox (101)\" pid=\"101\"", entries[0].cpu, entries[0].name, entries[0].pid)
	}
	if entries[1].cpu != 23.4 || entries[1].name != "node (202)" || entries[1].pid != "202" {
		t.Errorf("entry 1: got cpu=%v name=%q pid=%q, want cpu=23.4 name=\"node (202)\" pid=\"202\"", entries[1].cpu, entries[1].name, entries[1].pid)
	}
	if entries[2].cpu != 0.0 || entries[2].name != "ps (303)" || entries[2].pid != "303" {
		t.Errorf("entry 2: got cpu=%v name=%q pid=%q, want cpu=0.0 name=\"ps (303)\" pid=\"303\"", entries[2].cpu, entries[2].name, entries[2].pid)
	}
}

func TestParsePSUsesArgv0Basename(t *testing.T) {
	input := `%CPU   PID ARGS
 10.0   42 my_worker script.py --flag`

	entries := parsePS(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].name != "my_worker (42)" {
		t.Errorf("expected \"my_worker (42)\", got %q", entries[0].name)
	}
}

func TestParsePSStripsPathFromArgv0(t *testing.T) {
	input := `%CPU   PID ARGS
 15.0   99 /usr/local/bin/python3 script.py`

	entries := parsePS(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].name != "python3 (99)" {
		t.Errorf("expected \"python3 (99)\", got %q", entries[0].name)
	}
}

func TestParsePSHeaderOnly(t *testing.T) {
	entries := parsePS("%CPU   PID ARGS\n")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for header-only output, got %d", len(entries))
	}
}

func TestParsePSEmpty(t *testing.T) {
	entries := parsePS("")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty output, got %d", len(entries))
	}
}

func TestParsePSSkipsMalformedLines(t *testing.T) {
	input := `%CPU   PID ARGS
  notanumber 101 firefox
 10.0   202 node server.js`

	entries := parsePS(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(entries))
	}
	if entries[0].name != "node (202)" {
		t.Errorf("expected \"node (202)\", got %q", entries[0].name)
	}
}

func TestParsePSUsesSetProcTitleWhenPresent(t *testing.T) {
	input := `%CPU   PID ARGS
  0.0  555 puma 7.2.0 (tcp://0.0.0.0:3001) [auto-email-classifier]`

	entries := parsePS(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].name != "[auto-email-classifier] (555)" {
		t.Errorf("expected \"[auto-email-classifier] (555)\", got %q", entries[0].name)
	}
}

func TestParsePSUsesSetProcTitleWithSpaces(t *testing.T) {
	input := `%CPU   PID ARGS
  0.0  777 ruby worker.rb [stress test]`

	entries := parsePS(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].name != "[stress test] (777)" {
		t.Errorf("expected \"[stress test] (777)\", got %q", entries[0].name)
	}
}
