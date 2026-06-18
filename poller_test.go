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
	if entries[0].cpu != 1.5 || entries[0].key != "101" || entries[0].name != "firefox" || entries[0].pid != "101" {
		t.Errorf("entry 0: got cpu=%v key=%q name=%q pid=%q, want cpu=1.5 key=\"101\" name=\"firefox\" pid=\"101\"", entries[0].cpu, entries[0].key, entries[0].name, entries[0].pid)
	}
	if entries[1].cpu != 23.4 || entries[1].key != "202" || entries[1].name != "node" || entries[1].pid != "202" {
		t.Errorf("entry 1: got cpu=%v key=%q name=%q pid=%q, want cpu=23.4 key=\"202\" name=\"node\" pid=\"202\"", entries[1].cpu, entries[1].key, entries[1].name, entries[1].pid)
	}
	if entries[2].cpu != 0.0 || entries[2].key != "303" || entries[2].name != "ps" || entries[2].pid != "303" {
		t.Errorf("entry 2: got cpu=%v key=%q name=%q pid=%q, want cpu=0.0 key=\"303\" name=\"ps\" pid=\"303\"", entries[2].cpu, entries[2].key, entries[2].name, entries[2].pid)
	}
}

func TestParsePSUsesArgv0Basename(t *testing.T) {
	input := `%CPU   PID ARGS
 10.0   42 my_worker script.py --flag`

	entries := parsePS(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].name != "my_worker" {
		t.Errorf("expected \"my_worker\", got %q", entries[0].name)
	}
}

func TestParsePSStripsPathFromArgv0(t *testing.T) {
	input := `%CPU   PID ARGS
 15.0   99 /usr/local/bin/python3 script.py`

	entries := parsePS(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].name != "python3" {
		t.Errorf("expected \"python3\", got %q", entries[0].name)
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
	if entries[0].name != "node" {
		t.Errorf("expected \"node\", got %q", entries[0].name)
	}
}

func TestParsePSUsesSetProcTitleWhenPresent(t *testing.T) {
	input := `%CPU   PID ARGS
  0.0  555 puma 7.2.0 (tcp://0.0.0.0:3001) [auto-email-classifier]`

	entries := parsePS(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].name != "[auto-email-classifier]" {
		t.Errorf("expected \"[auto-email-classifier]\", got %q", entries[0].name)
	}
}

func TestParsePSUsesSetProcTitleWithSpaces(t *testing.T) {
	input := `%CPU   PID ARGS
  0.0  777 ruby worker.rb [stress test]`

	entries := parsePS(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].name != "[stress test]" {
		t.Errorf("expected \"[stress test]\", got %q", entries[0].name)
	}
}

func TestParsePSUsesCommWhenAvailable(t *testing.T) {
	input := `%CPU   PID ARGS
  0.0  555 ruby worker.rb`

	entries := parsePSWithComms(input, map[string]string{"555": "ruby"})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].name != "ruby" {
		t.Errorf("expected comm name \"ruby\", got %q", entries[0].name)
	}
}

func TestParsePSPrefersCustomThreadNameOverComm(t *testing.T) {
	input := `%CPU   PID ARGS
  0.0  555 ruby worker.rb [auto-email-classifier]`

	entries := parsePSWithComms(input, map[string]string{"555": "ruby"})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].name != "[auto-email-classifier]" {
		t.Errorf("expected custom thread name \"[auto-email-classifier]\", got %q", entries[0].name)
	}
}

func TestParseProcComms(t *testing.T) {
	input := "    1 launchd\n  202 custom worker\n"

	comms := parseProcComms(input)
	if comms["1"] != "launchd" {
		t.Errorf("expected pid 1 comm launchd, got %q", comms["1"])
	}
	if comms["202"] != "custom worker" {
		t.Errorf("expected pid 202 comm custom worker, got %q", comms["202"])
	}
}
