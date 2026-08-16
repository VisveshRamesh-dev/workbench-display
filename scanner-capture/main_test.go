//go:build windows

package main

import (
	"regexp"
	"testing"
	"time"
)

// feed replays a key sequence with a fixed gap between keys and returns the
// candidate produced by the terminating key (if any).
func feed(seq []uint32, gap time.Duration) string {
	scanBuf = scanBuf[:0]
	t := time.Unix(0, 0)
	out := ""
	for _, vk := range seq {
		t = t.Add(gap)
		if r := onKey(vk, t); r != "" {
			out = r
		}
	}
	return out
}

var partRe = regexp.MustCompile(`P([0-9A-Z]{13})`)

func TestFastScanCaptured(t *testing.T) {
	seq := []uint32{'P', '8', '7', '3', '1', '0', 'B', 'M', '0', '0', '0', 'A', 'B', 'P', vkReturn}
	got := feed(seq, 10*time.Millisecond)
	if got != "P87310BM000ABP" {
		t.Fatalf("reconstructed = %q, want P87310BM000ABP", got)
	}
	m := partRe.FindStringSubmatch(got)
	if m == nil || m[1] != "87310BM000ABP" {
		t.Fatalf("part extraction = %v, want 87310BM000ABP", m)
	}
}

func TestSlowHumanTypingIgnored(t *testing.T) {
	// The very same characters, but typed at human speed (200ms apart): each key
	// exceeds maxGap, so the buffer never accumulates and no scan is produced.
	seq := []uint32{'8', '7', '3', '1', '0', vkReturn}
	if got := feed(seq, 200*time.Millisecond); got != "" {
		t.Fatalf("slow typing produced a scan %q, want none", got)
	}
}

func TestFastNonPartRejectedByPattern(t *testing.T) {
	// A fast burst that is not a part code is reconstructed but rejected by the
	// pattern (this is the check poster() applies before forwarding).
	seq := []uint32{'H', 'E', 'L', 'L', 'O', vkReturn}
	got := feed(seq, 10*time.Millisecond)
	if got == "" {
		t.Fatalf("expected the fast burst to be reconstructed")
	}
	if partRe.MatchString(got) {
		t.Fatalf("%q should not match the part pattern", got)
	}
}

func TestFullIsoLabelExtractsPart(t *testing.T) {
	// Control characters/symbols in a real label are not character keys, so they
	// are skipped; the letters+digits that survive still yield the part code.
	seq := []uint32{'0', '6', 'V', 'T', 'E', 'I', 'H', 'P', '8', '7', '3', '1', '0', 'B', 'M', '0', '0', '0', 'A', 'B', 'P', vkReturn}
	got := feed(seq, 8*time.Millisecond)
	m := partRe.FindStringSubmatch(got)
	if m == nil || m[1] != "87310BM000ABP" {
		t.Fatalf("part extraction = %v (from %q), want 87310BM000ABP", m, got)
	}
}

func TestAutoFlushWithoutEnter(t *testing.T) {
	// A real scan that does NOT end in Enter: the characters arrive fast, then
	// the keyboard goes silent. After flushGap of silence it must auto-complete.
	scanBuf = scanBuf[:0]
	tk := time.Unix(0, 0)
	for _, vk := range []uint32{'P', '8', '7', '3', '1', '0', 'G', 'B', '0', '0', '0', 'A', 'B', 'P'} {
		tk = tk.Add(10 * time.Millisecond)
		onKey(vk, tk)
	}
	got := maybeAutoFlush(tk.Add(flushGap + 50*time.Millisecond))
	if got != "P87310GB000ABP" {
		t.Fatalf("auto-flush = %q, want P87310GB000ABP", got)
	}
	if m := partRe.FindStringSubmatch(got); m == nil || m[1] != "87310GB000ABP" {
		t.Fatalf("part = %v, want 87310GB000ABP", m)
	}
	// buffer is now empty; a second flush yields nothing
	if got := maybeAutoFlush(tk.Add(time.Second)); got != "" {
		t.Fatalf("second auto-flush = %q, want empty", got)
	}
}

func TestNoAutoFlushBeforeSilence(t *testing.T) {
	// Mid-burst (silence shorter than flushGap) must NOT flush early.
	scanBuf = scanBuf[:0]
	tk := time.Unix(0, 0)
	for _, vk := range []uint32{'P', '8', '7', '3', '1', '0'} {
		tk = tk.Add(10 * time.Millisecond)
		onKey(vk, tk)
	}
	if got := maybeAutoFlush(tk.Add(flushGap / 2)); got != "" {
		t.Fatalf("flushed too early: %q", got)
	}
}

func TestShortBurstIgnored(t *testing.T) {
	seq := []uint32{'8', '7', vkReturn} // below minLen
	if got := feed(seq, 10*time.Millisecond); got != "" {
		t.Fatalf("short burst produced %q, want none", got)
	}
}
