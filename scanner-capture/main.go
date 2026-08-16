//go:build windows

// scanner-capture is a small background helper for the single-station display.
//
// It installs a Windows low-level keyboard hook (WH_KEYBOARD_LL) so the barcode
// scanner (which acts as a keyboard) can be shared: every scan STILL reaches
// whatever application is focused on the primary monitor (their existing
// software) exactly as before — this helper only OBSERVES the keystrokes and
// passes them through untouched. In parallel it reconstructs the scanned code
// and POSTs it to the display server's /api/scan, so our secondary-monitor
// display mirrors the scan.
//
// It deliberately does NOT react to ordinary hand typing, using two guards:
//   1. Speed  — a real scan arrives as a fast burst of characters; keystrokes
//               spaced further apart than maxGap are treated as human typing
//               and restart the buffer.
//   2. Pattern— the reconstructed string must match config.json's code_pattern
//               (i.e. contain a real 87### part code) or it is dropped silently.
//
// Build:  go build -o scanner-capture.exe ./scanner-capture
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// ---- tuning (safe to adjust) ----
const (
	maxGap   = 100 * time.Millisecond // max spacing between keys within one scan
	flushGap = 200 * time.Millisecond // silence after the last key => scan done
	maxTotal = 3 * time.Second        // max time from first key to completion
	minLen   = 4                      // ignore trivially short bursts
)

// ---- Windows constants / types ----
const (
	whKeyboardLL = 13
	wmKeyDown    = 0x0100
	wmSysKeyDown = 0x0104
	vkReturn     = 0x0D
)

type kbdllhookstruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type msgStruct struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procSetWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procGetMessage          = user32.NewProc("GetMessageW")
)

var scanCh chan string

// ---- scan reconstruction (pure logic, unit-tested) ----

var (
	mu         sync.Mutex // guards the buffer (hook thread + auto-flush goroutine)
	scanBuf    []rune
	lastKeyAt  time.Time
	firstKeyAt time.Time
)

// onKey feeds one key-down into the reconstruction buffer. It returns the
// completed candidate string when a terminating Enter finishes a fast burst,
// otherwise "". Non-character keys (Shift, Ctrl, arrows, …) are ignored without
// disturbing the buffer.
func onKey(vk uint32, now time.Time) string {
	mu.Lock()
	defer mu.Unlock()
	if vk == vkReturn {
		return flushLocked(now)
	}
	ch, ok := vkToChar(vk)
	if !ok {
		return "" // modifier / non-character key: leave the buffer as-is
	}
	if len(scanBuf) == 0 || now.Sub(lastKeyAt) > maxGap {
		// new sequence, or a human-speed gap: restart the buffer here
		scanBuf = scanBuf[:0]
		firstKeyAt = now
	}
	scanBuf = append(scanBuf, ch)
	lastKeyAt = now
	return ""
}

// flushLocked returns the buffered burst if it qualifies as a scan, and clears
// the buffer. Caller must hold mu.
func flushLocked(now time.Time) string {
	candidate := ""
	if len(scanBuf) >= minLen && now.Sub(firstKeyAt) <= maxTotal {
		candidate = string(scanBuf)
	}
	scanBuf = scanBuf[:0]
	return candidate
}

// maybeAutoFlush completes a scan that did NOT end in Enter: once the keyboard
// has been silent for flushGap since the last character, the buffered burst is
// treated as a finished scan. This makes the scanner's Enter/CR suffix optional.
func maybeAutoFlush(now time.Time) string {
	mu.Lock()
	defer mu.Unlock()
	if len(scanBuf) > 0 && now.Sub(lastKeyAt) > flushGap {
		return flushLocked(now)
	}
	return ""
}

// vkToChar maps the virtual-key codes we care about to characters. Letters are
// returned uppercase regardless of Shift — case is irrelevant to the part
// pattern, which only needs the digits and a non-digit boundary.
func vkToChar(vk uint32) (rune, bool) {
	switch {
	case vk >= '0' && vk <= '9': // 0x30-0x39
		return rune(vk), true
	case vk >= 0x60 && vk <= 0x69: // numpad 0-9
		return rune('0' + (vk - 0x60)), true
	case vk >= 'A' && vk <= 'Z': // 0x41-0x5A
		return rune(vk), true
	}
	return 0, false
}

// ---- hook plumbing ----

func hookProc(nCode uintptr, wParam uintptr, lParam uintptr) uintptr {
	if nCode == 0 && (wParam == wmKeyDown || wParam == wmSysKeyDown) {
		kb := (*kbdllhookstruct)(unsafe.Pointer(lParam))
		if c := onKey(kb.VkCode, time.Now()); c != "" {
			select {
			case scanCh <- c:
			default: // poster busy; drop rather than block the keyboard
			}
		}
	}
	// Always pass the keystroke through so the focused app still receives it.
	ret, _, _ := procCallNextHookEx.Call(0, nCode, wParam, lParam)
	return ret
}

// ---- config ----

type stationCfg struct {
	ID string `json:"id"`
}

type appCfg struct {
	CodePattern string       `json:"code_pattern"`
	ServerPort  int          `json:"server_port"`
	Stations    []stationCfg `json:"stations"`
}

func loadCfg() appCfg {
	c := appCfg{CodePattern: `P([0-9A-Z]{13})`, ServerPort: 8080}
	data, err := os.ReadFile("config.json")
	if err != nil {
		log.Printf("config.json not read (%v); using defaults", err)
		return c
	}
	_ = json.Unmarshal(data, &c)
	if c.ServerPort == 0 {
		c.ServerPort = 8080
	}
	if c.CodePattern == "" {
		c.CodePattern = `P([0-9A-Z]{13})`
	}
	return c
}

// ---- forwarding ----

// autoFlusher periodically completes a scan that ended in a pause rather than
// an Enter (see maybeAutoFlush).
func autoFlusher() {
	t := time.NewTicker(40 * time.Millisecond)
	defer t.Stop()
	for range t.C {
		if c := maybeAutoFlush(time.Now()); c != "" {
			select {
			case scanCh <- c:
			default:
			}
		}
	}
}

func poster(url, station string, re *regexp.Regexp) {
	client := &http.Client{Timeout: 3 * time.Second}
	for raw := range scanCh {
		if !re.MatchString(raw) {
			continue // hand-typed / non-part input: ignore silently
		}
		body, _ := json.Marshal(map[string]string{"raw": raw, "station_id": station})
		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("forward failed: %v", err)
			continue
		}
		resp.Body.Close()
		log.Printf("scan forwarded: %q", raw)
	}
}

func main() {
	cfg := loadCfg()
	re, err := regexp.Compile(cfg.CodePattern)
	if err != nil {
		log.Fatalf("invalid code_pattern %q: %v", cfg.CodePattern, err)
	}
	station := "WB-01"
	if len(cfg.Stations) > 0 && cfg.Stations[0].ID != "" {
		station = cfg.Stations[0].ID
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/scan", cfg.ServerPort)

	scanCh = make(chan string, 16)
	go poster(url, station, re)
	go autoFlusher()

	// The hook must be set on a thread that pumps a message loop.
	runtime.LockOSThread()
	hook, _, callErr := procSetWindowsHookEx.Call(uintptr(whKeyboardLL), syscall.NewCallback(hookProc), 0, 0)
	if hook == 0 {
		log.Fatalf("SetWindowsHookEx failed: %v", callErr)
	}
	defer procUnhookWindowsHookEx.Call(hook)

	log.Printf("scanner-capture running.")
	log.Printf("  forwarding scans to %s (station %s)", url, station)
	log.Printf("  scans are mirrored here AND still reach the focused app; hand typing is ignored.")

	var msg msgStruct
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
			break
		}
	}
}
