//go:build windows

// serial-capture reads a 2D barcode scanner that is in SERIAL (COM port) mode
// and forwards each scan to the display server's /api/scan.
//
// Use this instead of the keyboard-hook helper (scanner-capture) when the
// scanner sends data over a COM port rather than acting as a keyboard (i.e.
// nothing appears in Notepad when you scan).
//
// Sharing with the client's existing software: a COM port can be opened by only
// one program at a time, so to let both their software and this reader see the
// scans, a COM-port splitter (e.g. com0com + hub4com) mirrors the physical
// scanner port to two virtual ports. Their software reads one; this reader
// reads the other. Set "serial_port" in config.json to OUR virtual port.
//
// Build: go build -o serial-capture.exe ./serial-capture
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"syscall"
	"time"
	"unsafe"
)

const idleFlush = 200 * time.Millisecond // flush a pending line after this much silence

// ---- config ----

type stationCfg struct {
	ID string `json:"id"`
}

type appCfg struct {
	CodePattern string       `json:"code_pattern"`
	ServerPort  int          `json:"server_port"`
	SerialPort  string       `json:"serial_port"`
	SerialMode  string       `json:"serial_mode"`
	Stations    []stationCfg `json:"stations"`
}

func loadCfg() appCfg {
	c := appCfg{
		CodePattern: `P([0-9A-Z]{13})`,
		ServerPort:  8080,
		SerialPort:  "COM3",
		SerialMode:  "baud=9600 parity=N data=8 stop=1",
	}
	if data, err := os.ReadFile("config.json"); err == nil {
		_ = json.Unmarshal(data, &c)
	} else {
		log.Printf("config.json not read (%v); using defaults", err)
	}
	if c.ServerPort == 0 {
		c.ServerPort = 8080
	}
	if c.CodePattern == "" {
		c.CodePattern = `P([0-9A-Z]{13})`
	}
	if c.SerialPort == "" {
		c.SerialPort = "COM3"
	}
	if c.SerialMode == "" {
		c.SerialMode = "baud=9600 parity=N data=8 stop=1"
	}
	return c
}

// ---- line framing (pure logic, unit-tested) ----

// lineAssembler turns a stream of bytes into complete scan lines, split on
// CR/LF. A scan that arrives without a terminator is flushed by the read loop
// after a short silence (flushPending).
type lineAssembler struct{ buf []byte }

func (a *lineAssembler) feed(b []byte) []string {
	var out []string
	for _, c := range b {
		if c == '\r' || c == '\n' {
			if len(a.buf) > 0 {
				out = append(out, string(a.buf))
				a.buf = a.buf[:0]
			}
			continue
		}
		// keep printable bytes only; drop stray control bytes
		if c >= 0x20 && c < 0x7f {
			a.buf = append(a.buf, c)
		}
	}
	return out
}

func (a *lineAssembler) flushPending() string {
	if len(a.buf) == 0 {
		return ""
	}
	s := string(a.buf)
	a.buf = a.buf[:0]
	return s
}

// ---- Windows serial I/O ----

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procCreateFileW     = kernel32.NewProc("CreateFileW")
	procGetCommState    = kernel32.NewProc("GetCommState")
	procBuildCommDCBW   = kernel32.NewProc("BuildCommDCBW")
	procSetCommState    = kernel32.NewProc("SetCommState")
	procSetCommTimeouts = kernel32.NewProc("SetCommTimeouts")
	procReadFile        = kernel32.NewProc("ReadFile")
	procCloseHandle     = kernel32.NewProc("CloseHandle")
)

const (
	genericRead   = 0x80000000
	genericWrite  = 0x40000000
	openExisting  = 3
	invalidHandle = ^uintptr(0)
)

// DCB — device control block (see Win32 docs). Size must be exact.
type dcb struct {
	DCBlength  uint32
	BaudRate   uint32
	Flags      uint32
	wReserved  uint16
	XonLim     uint16
	XoffLim    uint16
	ByteSize   byte
	Parity     byte
	StopBits   byte
	XonChar    byte
	XoffChar   byte
	ErrorChar  byte
	EofChar    byte
	EvtChar    byte
	wReserved1 uint16
}

type commTimeouts struct {
	ReadIntervalTimeout         uint32
	ReadTotalTimeoutMultiplier  uint32
	ReadTotalTimeoutConstant    uint32
	WriteTotalTimeoutMultiplier uint32
	WriteTotalTimeoutConstant   uint32
}

func openSerial(port, mode string) (uintptr, error) {
	name := `\\.\` + port
	np, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	h, _, callErr := procCreateFileW.Call(
		uintptr(unsafe.Pointer(np)),
		genericRead|genericWrite,
		0, 0, openExisting, 0, 0)
	if h == invalidHandle {
		return 0, fmt.Errorf("open %s: %v", name, callErr)
	}

	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	procGetCommState.Call(h, uintptr(unsafe.Pointer(&d)))

	mp, err := syscall.UTF16PtrFromString(mode)
	if err != nil {
		procCloseHandle.Call(h)
		return 0, err
	}
	if r, _, e := procBuildCommDCBW.Call(uintptr(unsafe.Pointer(mp)), uintptr(unsafe.Pointer(&d))); r == 0 {
		procCloseHandle.Call(h)
		return 0, fmt.Errorf("BuildCommDCB(%q): %v", mode, e)
	}
	if r, _, e := procSetCommState.Call(h, uintptr(unsafe.Pointer(&d))); r == 0 {
		procCloseHandle.Call(h)
		return 0, fmt.Errorf("SetCommState: %v", e)
	}

	// Return from ReadFile quickly (max ~100ms) so we can poll and pause-flush.
	to := commTimeouts{ReadIntervalTimeout: 50, ReadTotalTimeoutConstant: 100}
	procSetCommTimeouts.Call(h, uintptr(unsafe.Pointer(&to)))
	return h, nil
}

func readLoop(h uintptr, onScan func(string)) {
	buf := make([]byte, 512)
	var asm lineAssembler
	lastData := time.Now()
	for {
		var n uint32
		r, _, _ := procReadFile.Call(h,
			uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
			uintptr(unsafe.Pointer(&n)), 0)
		if r == 0 {
			log.Printf("serial read error; retrying")
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if n > 0 {
			for _, line := range asm.feed(buf[:n]) {
				onScan(line)
			}
			lastData = time.Now()
			continue
		}
		// timeout with no data: complete a scan that had no CR/LF terminator
		if time.Since(lastData) > idleFlush {
			if s := asm.flushPending(); s != "" {
				onScan(s)
			}
			lastData = time.Now()
		}
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
	url := fmt.Sprintf("http://localhost:%d/api/scan", cfg.ServerPort)
	client := &http.Client{Timeout: 3 * time.Second}

	h, err := openSerial(cfg.SerialPort, cfg.SerialMode)
	if err != nil {
		log.Fatalf("serial: %v", err)
	}
	defer procCloseHandle.Call(h)

	log.Printf("serial-capture running.")
	log.Printf("  reading %s (%s)", cfg.SerialPort, cfg.SerialMode)
	log.Printf("  forwarding scans to %s (station %s)", url, station)

	readLoop(h, func(line string) {
		if !re.MatchString(line) {
			return // not a part scan (noise / partial): ignore
		}
		body, _ := json.Marshal(map[string]string{"raw": line, "station_id": station})
		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("forward failed: %v", err)
			return
		}
		resp.Body.Close()
		log.Printf("scan forwarded: %q", line)
	})
}
