# Workbench Display Controller

Display controller for a single-station door garnish assembly line.
One industrial PC reads **one Bluetooth barcode scanner** and drives **one
secondary monitor**. When the operator scans a part, the matching work
specification appears full-screen on the secondary monitor. Scanning the
same part again signals completion.

---

## Architecture

```
   ┌──────────────── INDUSTRIAL PC (Windows) ────────────────┐
   │                                                          │
 [BT Scanner] ══Bluetooth HID══▶ Chrome kiosk (2nd monitor)   │
   (types code)                    │  captures keystrokes     │
                                    │  POST /api/scan          │
                                    ▼                          │
                              Go server (state machine,        │
                              CSV log, SSE broadcast) ─────────┼─▶ same kiosk
                                    │                          │   shows image
                                    └─▶ config.json, assets/, logs/
   └──────────────────────────────────────────────────────────┘
```

The Bluetooth scanner pairs with Windows as an **HID keyboard**: it "types"
the barcode text followed by Enter into whatever window has focus. The
full-screen Chrome kiosk on the secondary monitor is that focused window;
its page captures the keystrokes, POSTs them to the local Go server, and
the server pushes the resulting display state back over Server-Sent Events.

> **Operational requirement:** the kiosk window must stay focused for scans
> to land on it. On a dedicated kiosk PC (nothing else running, kiosk
> full-screen on the secondary monitor) this is automatic.

---

## Pages

Once the server is running (default `http://localhost:8080/`):

| Path | Purpose |
|---|---|
| `/` | Landing page with links to everything below. |
| `/station/WB-01` | Full-screen operator display. Captures scanner keystrokes. Runs on the secondary monitor in production. |
| `/supervisor` | Live status of the station. Optional — for a supervisor PC or wall monitor. |
| `/simulator` | Send mock scans without hardware. Used for testing and demos. |
| `/api/scan` | POST endpoint. The kiosk page calls this on each scan. |
| `/api/snapshots` | JSON snapshot of station state. Used by the supervisor view. |
| `/events/WB-01` | Server-Sent Events stream for the station. The display subscribes here. |

---

## Behaviour

The scanned part's spec is shown and **stays on screen until the next scan** —
there is no auto-complete and no idle timeout.

```
       ┌──────┐   scan part X    ┌─────────┐
       │ IDLE │─────────────────▶│ DISPLAY │──┐ scan same X  → re-show X (RESCANNED)
       └──────┘  (first scan)    └─────────┘◀─┘ scan diff Y  → show Y     (REPLACED)
       (initial only)                 │
                                       └─ unknown code → brief error flash,
                                          then returns to the current spec
```

`idle_timeout_seconds: 0` means the display is persistent (the spec never
clears on its own). Set it to a positive number only if you want an automatic
return to the waiting screen after N seconds of no scans.

Every scan is logged to `logs/scans-YYYY-MM-DD.csv` with the timestamp,
station, raw scan text, extracted part code, and outcome (STARTED,
RESCANNED, REPLACED, ERROR).

---

## Build & run

```
go build -o workbench-display.exe
workbench-display.exe
```

Then open `http://localhost:8080/simulator` to send test scans, and
`http://localhost:8080/station/WB-01` in another window to watch the display.

---

## Configuration

Edit `config.json`:

```json
{
  "code_pattern": "SB\\d{3}[A-Z]{3}",
  "idle_timeout_seconds": 30,
  "completion_min_gap_ms": 3000,
  "completion_flash_ms": 1800,
  "server_port": 8080,
  "stations": [
    { "id": "WB-01", "name": "Assembly Station" }
  ],
  "parts": {
    "SB390GKG": { "image": "assets/SB390GKG.png", "description": "..." }
  }
}
```

| Field | Purpose |
|---|---|
| `code_pattern` | Regex used to extract the part code from a raw scan. |
| `idle_timeout_seconds` | How long a display stays before falling back to idle. |
| `completion_min_gap_ms` | Minimum time between the two scans that trigger completion. Protects against double-scan bounce. |
| `completion_flash_ms` | Duration of the green "Complete" screen. |
| `parts` | Map of part code → image path + description. Update this when specs change. |

**Updating an image:** replace the PNG in `assets/`. No rebuild.
**Adding a part:** add an entry to `parts` and drop the image in `assets/`.

---

## Deploy to the industrial PC

1. Build the binary: `go build -o workbench-display.exe`.
2. Copy the whole folder to the target PC, e.g. `C:\workbench-display\`.
3. Pair the Bluetooth scanner with Windows (Settings > Bluetooth & devices).
   Confirm it is in **HID keyboard** mode — scan a label into Notepad and
   check the text appears followed by a new line.
4. Arrange the displays in Windows Display Settings and note the **secondary
   monitor's top-left position**. Open `start.bat` and set `MONITOR_X` /
   `MONITOR_Y` to that position.
5. Run `install.bat` as Administrator to register autostart.
6. Reboot, or double-click `start.bat` to launch now.

Chrome opens one kiosk window full-screen on the secondary monitor showing
the operator display.

To exit the kiosk window for maintenance: `Alt+F4`.
To remove autostart: run `uninstall.bat`.

---

## Serial / COM-port scanner (serial-capture)

If the scanner is in **serial mode** (it sends bytes over a COM port and types
nothing into Notepad), use **`serial-capture`** instead of the keyboard hook.

- It opens the COM port set by `serial_port` / `serial_mode` in `config.json`
  (default `COM3`, `baud=9600 parity=N data=8 stop=1`), frames each scan on
  CR/LF (with a short-silence fallback), extracts the part with `code_pattern`,
  and POSTs to `/api/scan`.

```
go build -o serial-capture.exe ./serial-capture
```

**Sharing a serial scanner** (a COM port can be opened by only one program):
run a COM-port splitter so both the client's existing software and this reader
see the scans. With the free **com0com + hub4com** tools: `hub4com` reads the
physical scanner port and mirrors it to two virtual `com0com` ports — point the
client's software at one and `serial_port` at the other. (Commercial splitters
such as Eltima Virtual Serial Port Driver do the same.)

`start.bat` launches `serial-capture.exe` if present (otherwise falls back to
the keyboard `scanner-capture.exe`).

## Sharing the scanner with other software (scanner-capture)

By default the display captures scans inside its own browser window, which only
works while that window is focused. If the same scanner must ALSO feed another
program (e.g. an existing MES/production app on the primary monitor), run the
**`scanner-capture`** helper instead.

It installs a Windows low-level keyboard hook that:

- **passes every keystroke through untouched** — the focused app still receives
  the scan exactly as before, so their software is unaffected;
- **mirrors** each scan to this display by POSTing to `/api/scan`, regardless of
  which window is focused;
- **ignores hand typing** via two guards: scan speed (only fast bursts) and the
  `code_pattern` match (must be a real part code).

```
go build -o scanner-capture.exe ./scanner-capture   # build the helper
```

`start.bat` launches it automatically if `scanner-capture.exe` is present. Note:
because it watches all keyboard input, some antivirus/endpoint tools may flag it
for review the first time — it only forwards barcode scans locally, and IT may
need to whitelist it. It does not require Administrator, but if the other app
runs elevated, run the helper elevated too.

## How scans reach the server

The scanner is an HID keyboard, so the kiosk page (`static/station.html`)
listens for keystrokes: it buffers the characters the scanner types and,
on the terminating Enter, POSTs the raw string to `/api/scan`:

```
POST /api/scan
{ "raw": "SB390GKG", "station_id": "WB-01" }
```

A gap longer than 500 ms between keystrokes resets the buffer, so a stray
keypress can never prepend to a real scan. The server extracts the part
code with `code_pattern`, looks it up in `parts`, runs the state machine,
logs the scan, and pushes the new display state over SSE.

> **Validate the regex before go-live:** scan one real DataMatrix label into
> Notepad and confirm the code matches `SB\d{3}[A-Z]{3}`. Adjust
> `code_pattern` in `config.json` if the real output differs.

---

## File layout

```
workbench-display/
├── workbench-display.exe        (built binary)
├── scanner-capture.exe          (optional: shares the scanner with other apps)
├── main.go                      (server source)
├── scanner-capture/             (keyboard-hook helper source)
│   ├── main.go
│   └── main_test.go
├── go.mod
├── config.json
├── static/
│   ├── index.html               (landing)
│   ├── station.html             (kiosk display + scanner capture)
│   ├── simulator.html           (mock scanner UI)
│   ├── supervisor.html          (live status)
│   └── style.css
├── assets/
│   ├── SB390GKG.png             (spec images)
│   ├── SB391GKG.png
│   ├── SB392GKG.png
│   └── SB393GKG.png
├── logs/
│   └── scans-YYYY-MM-DD.csv     (created at runtime)
├── start.bat                    (launcher: server + 1 Chrome kiosk)
├── install.bat                  (autostart registration)
└── uninstall.bat
```
