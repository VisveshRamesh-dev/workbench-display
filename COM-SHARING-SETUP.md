# Sharing one serial scanner between two programs (com0com + hub4com)

**Audience:** the technician / IT person setting up the PC. Requires **administrator rights**.

**Goal:** let the **existing plant software** and this **spec-display system** both read the
**same** serial barcode scanner, with **no change to the existing software's port setting**.

## How it works (the "rename trick")

The scanner's data physically arrives at one COM port, and Windows lets only one program open
it. We insert a small "copier" (**hub4com**) that reads the scanner and mirrors every scan to
**two virtual ports** (created by **com0com**). We rename the real port out of the way and give
the existing software's expected name (`COM1`) to a virtual port, so that software doesn't change.

```
[Scanner] -> COM20 (real, renamed) -> hub4com -+-> COM1  (virtual) -> existing plant software
                                               +-> COM11 (virtual) -> this display (serial-capture)
```

| Port | What it is | Read by |
|------|------------|---------|
| `COM20` | the REAL scanner port, renamed from COM1 | hub4com only |
| `COM1`  | virtual (com0com `CNCB0`), fed by hub4com | existing plant software (unchanged) |
| `COM11` | virtual (com0com `CNCB1`), fed by hub4com | this display (`serial-capture`) |

> Adjust the numbers to whatever is free on the machine — just keep them consistent across the
> steps below and in `config.json` / `start-scanner-copier.bat`.

---

## Step 1 — Install com0com (signed build)

1. Download com0com from https://sourceforge.net/projects/com0com/files/
2. **Use the SIGNED build** (file name contains `signed`, e.g. `com0com-3.0.0.0-i386-and-x64-signed.zip`).
   The unsigned build will NOT install on 64-bit Windows.
3. Run the installer as Administrator. Accept the driver-install prompt.

## Step 2 — Download hub4com

1. From the same com0com files page, download **hub4com** (e.g. `hub4com-2.1.0.0-386.zip`).
2. Unzip it to a simple folder, e.g. `C:\hub4com\` (so `C:\hub4com\hub4com.exe` exists).

## Step 3 — Rename the real scanner port to COM20

1. Open **Device Manager** -> **Ports (COM & LPT)**.
2. Find the scanner's port (currently `COM1`). If the plant software is running, close it first
   (it is holding the port).
3. Right-click it -> **Properties** -> **Port Settings** tab -> **Advanced...**
4. Change **COM Port Number** to **COM20** -> OK. (If Windows says it's in use, pick another free
   number and use that everywhere instead.)

## Step 4 — Create the two virtual port pairs

1. Launch the com0com setup GUI: **Start menu -> "Setup for com0com"** (`setupg.exe`).
2. Click **Add Pair**. A pair appears as `CNCA0 <-> CNCB0`.
3. Select **CNCB0**, tick **"use Ports class"**, and set its **COM Port Name** to **COM1**
   (this is the port the existing software will read). Click **Apply**.
4. Click **Add Pair** again -> `CNCA1 <-> CNCB1`.
5. Select **CNCB1**, tick **"use Ports class"**, set **COM Port Name** to **COM11**
   (this is the port the display reads). Click **Apply**.

You now have: `CNCA0<->COM1` and `CNCA1<->COM11`.

## Step 5 — Configure the copier

Open **`start-scanner-copier.bat`** (in this folder) and confirm these match your machine:

```bat
set HUB4COM="C:\hub4com\hub4com.exe"
set REAL_PORT=COM20
set VIRT_A=CNCA0
set VIRT_B=CNCA1
set BAUD=9600
```

That runs, in effect:

```
hub4com --baud=9600 --octs=off --route=0:1,2 \\.\COM20 \\.\CNCA0 \\.\CNCA1
```

which reads the scanner (COM20) and copies its data into both pairs. `--route=0:1,2` = "send data
from port 0 to ports 1 and 2." `--octs=off` disables hardware flow control (scanners rarely use it).

## Step 6 — Point the two programs at their ports

- **Existing plant software:** should read **COM1**. If it was already set to COM1, **no change** —
  that's the whole point.
- **This display:** already set in `config.json` -> `"serial_port": "COM11"`. Adjust if you used a
  different number.

## Step 7 — Start and test

1. Double-click **`start.bat`**. It launches the copier, the server, `serial-capture`, and the
   display together.
2. With `"serial_debug": true` in `config.json`, open the `serial-capture` window and **scan a
   label**. You should see `received: "..."` then `-> forwarded to display`.
3. Open the **existing plant software** and scan again — it should receive the scan on COM1 as
   normal.
4. Both react to the same scan = success. Set `"serial_debug": false` afterwards to quiet the log.

## Auto-start on boot

`start.bat` already launches the copier (Step via `start-scanner-copier.bat`). To have the whole
system come up automatically when the PC powers on, run **`install.bat` as Administrator** once
(registers `start.bat` with Task Scheduler).

## Troubleshooting

| Symptom | Fix |
|---|---|
| `serial-capture` says `cannot open COM11` | com0com pair for COM11 not created (Step 4), or wrong number in `config.json`. |
| Copier window flashes and closes | Wrong `HUB4COM` path or `REAL_PORT` in `start-scanner-copier.bat`. Run it directly to read the error. |
| Display works but plant software gets nothing | The plant software isn't pointed at COM1, or the COM1 virtual pair wasn't created / named. |
| `raw bytes` look like garbage | Baud mismatch — set `BAUD` (copier) and `serial_mode` (config) to the scanner's real baud. |
| Neither gets scans | Confirm hub4com is running and `REAL_PORT` matches the renamed physical port (Step 3). |
