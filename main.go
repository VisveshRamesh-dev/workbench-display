package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ---------- config types ----------

type PartSpec struct {
	Image       string `json:"image"`
	Description string `json:"description"`
}

type StationConfig struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

type Config struct {
	CodePattern         string              `json:"code_pattern"`
	IdleTimeoutSeconds  int                 `json:"idle_timeout_seconds"`
	CompletionMinGapMs  int                 `json:"completion_min_gap_ms"`
	CompletionFlashMs   int                 `json:"completion_flash_ms"`
	ServerPort          int                 `json:"server_port"`
	Stations            []StationConfig     `json:"stations"`
	Parts               map[string]PartSpec `json:"parts"`
}

// ---------- station state machine ----------

type StationState string

const (
	StateIdle      StationState = "idle"
	StateDisplay   StationState = "display"
	StateComplete  StationState = "complete"
)

// Event is what we push to browsers over SSE.
type Event struct {
	Type        string    `json:"type"`        // "state", "error"
	State       string    `json:"state,omitempty"`
	PartCode    string    `json:"part_code,omitempty"`
	Image       string    `json:"image,omitempty"`
	Description string    `json:"description,omitempty"`
	Message     string    `json:"message,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Station holds one workbench's live state + its SSE subscribers.
type Station struct {
	Config StationConfig

	mu           sync.Mutex
	state        StationState
	currentCode  string
	currentImage string
	currentDesc  string
	startedAt    time.Time
	timer        *time.Timer

	// SSE subscribers
	subMu       sync.Mutex
	subscribers map[chan Event]bool
}

func NewStation(sc StationConfig) *Station {
	return &Station{
		Config:      sc,
		state:       StateIdle,
		subscribers: make(map[chan Event]bool),
	}
}

// Subscribe returns a channel that receives events for this station.
func (s *Station) Subscribe() chan Event {
	ch := make(chan Event, 8)
	s.subMu.Lock()
	s.subscribers[ch] = true
	s.subMu.Unlock()

	// Send current state immediately so a fresh subscriber renders correctly.
	s.mu.Lock()
	ev := s.buildStateEvent()
	s.mu.Unlock()
	ch <- ev

	return ch
}

func (s *Station) Unsubscribe(ch chan Event) {
	s.subMu.Lock()
	if _, ok := s.subscribers[ch]; ok {
		delete(s.subscribers, ch)
		close(ch)
	}
	s.subMu.Unlock()
}

func (s *Station) broadcast(ev Event) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for ch := range s.subscribers {
		select {
		case ch <- ev:
		default:
			// slow subscriber, drop event rather than block
		}
	}
}

// buildStateEvent is called with s.mu held.
func (s *Station) buildStateEvent() Event {
	return Event{
		Type:        "state",
		State:       string(s.state),
		PartCode:    s.currentCode,
		Image:       s.currentImage,
		Description: s.currentDesc,
		Timestamp:   time.Now(),
	}
}

// Snapshot for supervisor view.
type StationSnapshot struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	PartCode    string `json:"part_code,omitempty"`
	Description string `json:"description,omitempty"`
	SecondsIn   int    `json:"seconds_in_state"`
}

func (s *Station) Snapshot() StationSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	secs := 0
	if !s.startedAt.IsZero() {
		secs = int(time.Since(s.startedAt).Seconds())
	}
	return StationSnapshot{
		ID:          s.Config.ID,
		Name:        s.Config.Name,
		State:       string(s.state),
		PartCode:    s.currentCode,
		Description: s.currentDesc,
		SecondsIn:   secs,
	}
}

// HandleScan applies the state machine for a scan at this station.
// Behaviour: the scanned part's spec is shown and STAYS on screen until the
// next scan. Re-scanning the same code re-shows the same spec (never skipped);
// a different code replaces it. There is no auto-complete, and with
// idle_timeout_seconds <= 0 there is no auto-return to idle either.
// Returns the outcome for logging.
func (s *Station) HandleScan(partCode, image, description string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	prevState := s.state
	prevCode := s.currentCode

	s.state = StateDisplay
	s.currentCode = partCode
	s.currentImage = image
	s.currentDesc = description
	s.startedAt = time.Now()
	s.armIdleTimerLocked() // no-op when idle_timeout_seconds <= 0
	go s.broadcast(s.buildStateEvent())

	switch {
	case prevState != StateDisplay:
		return "STARTED"
	case prevCode == partCode:
		return "RESCANNED"
	default:
		return "REPLACED"
	}
}

// HandleError broadcasts an error event without changing state.
func (s *Station) HandleError(message, partCode string) {
	go s.broadcast(Event{
		Type:      "error",
		Message:   message,
		PartCode:  partCode,
		Timestamp: time.Now(),
	})
}

func (s *Station) armIdleTimerLocked() {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if cfg.IdleTimeoutSeconds <= 0 {
		return // persistent display: the spec stays until the next scan
	}
	s.timer = time.AfterFunc(time.Duration(cfg.IdleTimeoutSeconds)*time.Second, func() {
		s.goIdle()
	})
}

func (s *Station) cancelTimerLocked() {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

func (s *Station) goIdle() {
	s.mu.Lock()
	s.state = StateIdle
	s.currentCode = ""
	s.currentImage = ""
	s.currentDesc = ""
	s.startedAt = time.Time{}
	s.cancelTimerLocked()
	ev := s.buildStateEvent()
	s.mu.Unlock()
	s.broadcast(ev)
}

// ---------- globals ----------

var (
	cfg         Config
	codeRegex   *regexp.Regexp
	stations    = make(map[string]*Station)
	logMu       sync.Mutex
	logFilePath string
)

// ---------- main ----------

func main() {
	if err := loadConfig("config.json"); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	re, err := regexp.Compile(cfg.CodePattern)
	if err != nil {
		log.Fatalf("invalid code_pattern: %v", err)
	}
	codeRegex = re

	for _, sc := range cfg.Stations {
		stations[sc.ID] = NewStation(sc)
		log.Printf("station registered: %s (%s)", sc.ID, sc.Name)
	}

	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatalf("logs dir: %v", err)
	}
	logFilePath = filepath.Join("logs", fmt.Sprintf("scans-%s.csv", time.Now().Format("2006-01-02")))
	ensureLogHeader()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/station/", handleStationPage)
	mux.HandleFunc("/events/", handleSSE)
	mux.HandleFunc("/simulator", handleSimulatorPage)
	mux.HandleFunc("/supervisor", handleSupervisorPage)
	mux.HandleFunc("/api/scan", handleAPIScan)
	mux.HandleFunc("/api/config", handleAPIConfig)
	mux.HandleFunc("/api/snapshots", handleAPISnapshots)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	log.Printf("Workbench Display Controller listening on http://localhost%s", addr)
	log.Printf("Stations: %d | Parts: %d | Log: %s", len(stations), len(cfg.Parts), logFilePath)
	log.Printf("Open http://localhost%s/simulator to test", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if cfg.ServerPort == 0 {
		cfg.ServerPort = 8080
	}
	// idle_timeout_seconds is intentionally NOT defaulted: 0 (or omitted) means
	// "persistent display" — the spec stays on screen until the next scan.
	if cfg.CompletionMinGapMs == 0 {
		cfg.CompletionMinGapMs = 3000
	}
	if cfg.CompletionFlashMs == 0 {
		cfg.CompletionFlashMs = 1800
	}
	if cfg.CodePattern == "" {
		cfg.CodePattern = `SB\d{3}[A-Z]{3}`
	}
	return nil
}

// ---------- HTTP handlers ----------

// serveNoCache serves an HTML page with revalidation forced, so an updated
// page (or a behaviour change like this one) always takes effect on the kiosk
// instead of a stale cached copy being shown.
func serveNoCache(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	http.ServeFile(w, r, path)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	// Landing page: list stations + links
	serveNoCache(w, r, filepath.Join("static", "index.html"))
}

func handleStationPage(w http.ResponseWriter, r *http.Request) {
	// URL: /station/{id}
	id := strings.TrimPrefix(r.URL.Path, "/station/")
	if _, ok := stations[id]; !ok {
		http.Error(w, "unknown station", http.StatusNotFound)
		return
	}
	serveNoCache(w, r, filepath.Join("static", "station.html"))
}

func handleSimulatorPage(w http.ResponseWriter, r *http.Request) {
	serveNoCache(w, r, filepath.Join("static", "simulator.html"))
}

func handleSupervisorPage(w http.ResponseWriter, r *http.Request) {
	serveNoCache(w, r, filepath.Join("static", "supervisor.html"))
}

func handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	out := struct {
		IdleTimeoutSeconds int             `json:"idle_timeout_seconds"`
		CompletionFlashMs  int             `json:"completion_flash_ms"`
		Stations           []StationConfig `json:"stations"`
	}{
		IdleTimeoutSeconds: cfg.IdleTimeoutSeconds,
		CompletionFlashMs:  cfg.CompletionFlashMs,
		Stations:           cfg.Stations,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func handleAPISnapshots(w http.ResponseWriter, r *http.Request) {
	snaps := make([]StationSnapshot, 0, len(cfg.Stations))
	for _, sc := range cfg.Stations {
		if st, ok := stations[sc.ID]; ok {
			snaps = append(snaps, st.Snapshot())
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snaps)
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
	// URL: /events/{stationID}
	id := strings.TrimPrefix(r.URL.Path, "/events/")
	station, ok := stations[id]
	if !ok {
		http.Error(w, "unknown station", http.StatusNotFound)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := station.Subscribe()
	defer station.Unsubscribe(ch)

	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-keepAlive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func handleAPIScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Raw       string `json:"raw"`
		StationID string `json:"station_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	// If station_id is missing, try to infer from a prefix in raw (real scanner path)
	stationID := req.StationID
	raw := req.Raw
	if stationID == "" {
		stationID, raw = inferStationFromPrefix(raw)
	}
	station, ok := stations[stationID]
	if !ok {
		http.Error(w, "unknown or missing station_id", http.StatusBadRequest)
		return
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		station.HandleError("empty scan", "")
		logScan(stationID, raw, "", "ERROR", "empty scan")
		writeJSON(w, map[string]any{"ok": false, "error": "empty scan"})
		return
	}
	match := extractCode(trimmed)
	if match == "" {
		station.HandleError("no part code found in scan", "")
		logScan(stationID, raw, "", "ERROR", "no part code found in scan")
		writeJSON(w, map[string]any{"ok": false, "error": "no part code found"})
		return
	}
	spec, found := cfg.Parts[match]
	if !found {
		station.HandleError(fmt.Sprintf("unknown part code: %s", match), match)
		logScan(stationID, raw, match, "ERROR", "unknown part code")
		writeJSON(w, map[string]any{"ok": false, "part_code": match, "error": "unknown part"})
		return
	}

	outcome := station.HandleScan(match, spec.Image, spec.Description)
	logScan(stationID, raw, match, outcome, "")
	writeJSON(w, map[string]any{"ok": true, "part_code": match, "outcome": outcome})
}

// extractCode applies codeRegex to the raw scan and returns the part code.
// If code_pattern has a capture group, group 1 is returned (so a structured
// ISO 15434 / H-KMC label can be parsed down to just the part number in the
// "P" field). Otherwise the whole match is returned, so simple patterns still
// work unchanged.
func extractCode(s string) string {
	m := codeRegex.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	if len(m) > 1 && m[1] != "" {
		return m[1]
	}
	return m[0]
}

// inferStationFromPrefix looks for a leading "N|" or "SN|" style prefix
// (configured on the physical scanner). Returns stationID and stripped raw.
func inferStationFromPrefix(raw string) (string, string) {
	trimmed := strings.TrimLeft(raw, " \t")
	if len(trimmed) < 2 {
		return "", raw
	}
	// Look for pattern "<prefix>|" at start
	idx := strings.Index(trimmed, "|")
	if idx <= 0 || idx > 4 {
		return "", raw
	}
	prefix := trimmed[:idx]
	for _, sc := range cfg.Stations {
		if sc.Prefix == prefix {
			return sc.ID, trimmed[idx+1:]
		}
	}
	return "", raw
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// ---------- logging ----------

func ensureLogHeader() {
	logMu.Lock()
	defer logMu.Unlock()
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		f, err := os.Create(logFilePath)
		if err != nil {
			log.Printf("log create: %v", err)
			return
		}
		defer f.Close()
		f.WriteString("timestamp,station,raw_scan,part_code,outcome,error\n")
	}
}

func logScan(stationID, raw, partCode, outcome, errMsg string) {
	logMu.Lock()
	defer logMu.Unlock()

	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Printf("log open: %v", err)
		return
	}
	defer f.Close()
	rawEsc := strings.ReplaceAll(raw, "\"", "\"\"")
	line := fmt.Sprintf("%s,%s,\"%s\",%s,%s,%s\n",
		time.Now().Format(time.RFC3339),
		stationID,
		rawEsc,
		partCode,
		outcome,
		errMsg,
	)
	f.WriteString(line)
}
