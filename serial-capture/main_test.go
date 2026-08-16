//go:build windows

package main

import (
	"regexp"
	"testing"
)

var partRe = regexp.MustCompile(`P([0-9A-Z]{13})`)

func TestLineAssemblerCRLF(t *testing.T) {
	var a lineAssembler
	got := a.feed([]byte("[).06VTEIHP82301GB030MWUSB03WT2608160101AB8GXG0001\r\n"))
	if len(got) != 1 {
		t.Fatalf("got %d lines, want 1: %v", len(got), got)
	}
	m := partRe.FindStringSubmatch(got[0])
	if m == nil || m[1] != "82301GB030MWU" {
		t.Fatalf("part = %v (from %q), want 82301GB030MWU", m, got[0])
	}
}

func TestLineAssemblerSplitAcrossReads(t *testing.T) {
	var a lineAssembler
	if got := a.feed([]byte("[).06VTEIHP82301GB03")); len(got) != 0 {
		t.Fatalf("should not emit a partial line, got %v", got)
	}
	got := a.feed([]byte("0MWUSB03W\r"))
	if len(got) != 1 || got[0] != "[).06VTEIHP82301GB030MWUSB03W" {
		t.Fatalf("reassembled line wrong: %v", got)
	}
}

func TestLineAssemblerMultipleLines(t *testing.T) {
	var a lineAssembler
	got := a.feed([]byte("P82301GB030MWU\rP86600DY740ABP\n"))
	if len(got) != 2 || got[0] != "P82301GB030MWU" || got[1] != "P86600DY740ABP" {
		t.Fatalf("got %v", got)
	}
}

func TestFlushPendingNoTerminator(t *testing.T) {
	var a lineAssembler
	a.feed([]byte("P82301GB030MWU")) // no CR/LF
	if s := a.flushPending(); s != "P82301GB030MWU" {
		t.Fatalf("flushPending = %q", s)
	}
	if s := a.flushPending(); s != "" {
		t.Fatalf("second flush = %q, want empty", s)
	}
}

func TestControlBytesDropped(t *testing.T) {
	var a lineAssembler
	// leading control bytes (e.g. stripped separators) must not corrupt the line
	got := a.feed([]byte{0x1d, 'P', '8', '2', '3', '0', '1', 'G', 'B', '0', '3', '0', 'M', 'W', 'U', '\r'})
	if len(got) != 1 || got[0] != "P82301GB030MWU" {
		t.Fatalf("got %v", got)
	}
}
