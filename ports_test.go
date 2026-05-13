package main

import (
	"reflect"
	"testing"
)

func TestParseLsofSinglePidSinglePort(t *testing.T) {
	input := "p1234\nn127.0.0.1:3000\n"
	got := parseLsof(input)
	want := map[string][]int{"1234": {3000}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseLsofSinglePidMultiPortSortedDeduped(t *testing.T) {
	input := "p1234\nn127.0.0.1:8080\nn127.0.0.1:3000\nn127.0.0.1:3000\nn0.0.0.0:5000\n"
	got := parseLsof(input)
	want := map[string][]int{"1234": {3000, 5000, 8080}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseLsofMultiplePids(t *testing.T) {
	input := "p1234\nn127.0.0.1:3000\np5678\nn0.0.0.0:8080\n"
	got := parseLsof(input)
	want := map[string][]int{
		"1234": {3000},
		"5678": {8080},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseLsofIPv6(t *testing.T) {
	input := "p99\nn[::1]:3000\nn[fe80::1]:8080\n"
	got := parseLsof(input)
	want := map[string][]int{"99": {3000, 8080}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseLsofWildcardAddress(t *testing.T) {
	input := "p42\nn*:8080\n"
	got := parseLsof(input)
	want := map[string][]int{"42": {8080}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseLsofSkipsMalformed(t *testing.T) {
	input := "p1234\nnnoport\nn127.0.0.1:abc\nn127.0.0.1:3000\n"
	got := parseLsof(input)
	want := map[string][]int{"1234": {3000}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseLsofEmpty(t *testing.T) {
	got := parseLsof("")
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestParseLsofIgnoresOtherFieldTypes(t *testing.T) {
	input := "p1234\ncnode\nfu0t0\nPTCP\nn127.0.0.1:3000\nTST=LISTEN\n"
	got := parseLsof(input)
	want := map[string][]int{"1234": {3000}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFormatPortsEmpty(t *testing.T) {
	if got := formatPorts(nil); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
	if got := formatPorts([]int{}); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatPortsSingle(t *testing.T) {
	got := formatPorts([]int{3000})
	if got != "3000" {
		t.Errorf("expected \"3000\", got %q", got)
	}
}

func TestFormatPortsMultiple(t *testing.T) {
	got := formatPorts([]int{3000, 8080})
	if got != "3000,8080" {
		t.Errorf("expected \"3000,8080\", got %q", got)
	}
}

func TestFormatPortsTruncatesPast20Chars(t *testing.T) {
	got := formatPorts([]int{3000, 3001, 8080, 9090, 5432})
	// "3000,3001,8080,9090,5432" = 24 chars; truncate to 19 + "…" = 20 runes
	if len([]rune(got)) != 20 {
		t.Errorf("expected 20 runes, got %d (%q)", len([]rune(got)), got)
	}
	if !contains(got, "…") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
