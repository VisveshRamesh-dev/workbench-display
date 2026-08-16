# Workbench Display — Setup & Running Guide

**For the person setting this up at the plant.** No programming needed. Just
follow the steps in order. If anything doesn't work, jump to the
**Troubleshooting** section near the end, or call the contact listed at the
bottom.

---

## 1. What this system does (in one minute)

When an operator scans the barcode label on a part, the correct assembly
specification **picture** appears full-screen on a **second monitor** at the
workbench.

The picture **stays on the screen until the next part is scanned** — it never
times out or goes blank. Scan the next part and the picture switches to that
part.

The **same scanner keeps working with your existing software too.** Your current
program on the main monitor receives every scan exactly as it does today — this
display simply shows the matching picture alongside it. Nothing about your
existing setup changes.

Everything runs on **one PC**. There is no internet needed and no cloud — it all
works offline, right on that computer.

---

## 2. What you need before starting

Tick these off first:

- [ ] The **PC** (Windows) that will run everything.
- [ ] **Two monitors** connected to that PC: the **main** screen (where your
      existing software runs) and a **second** screen (where the part pictures
      will show).
- [ ] The **barcode scanner**, connected to the PC (Bluetooth or USB).
- [ ] Your **existing scanning software** (the program you already use with the
      scanner), if you have one.
- [ ] **Google Chrome** installed on the PC. (If it isn't, install it first —
      it's a free download from google.com/chrome.)
- [ ] The **`workbench-display` folder** (the one this guide is inside).

---

## 3. One-time setup

You only do Section 3 **once**. After that, starting it each day is just one
double-click (Section 4).

### Step 3.1 — Copy the folder onto the PC

Copy the whole **`workbench-display`** folder to somewhere simple, like:

```
C:\workbench-display
```

Keep the folder together — don't move individual files out of it.

### Step 3.2 — Set up the two monitors

1. Right-click on the desktop → **Display settings**.
2. You'll see two boxes labelled **1** and **2**. Drag them so they match how
   the monitors physically sit on the desk (left/right).
3. Click monitor box **1**, scroll down, and note whether **"Make this my main
   display"** is ticked. That one is the **main** monitor (your existing
   software). The **other** one (usually **2**) is where the part pictures will
   appear.

### Step 3.3 — Tell the system which monitor to use

The system needs to know where the second monitor is. Do this:

1. Click the **Start** menu, type **PowerShell**, and open it.
2. Copy-paste this line and press Enter:

   ```
   Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Screen]::AllScreens | ForEach-Object { '{0}  Primary={1}  X={2}  Y={3}  Size={4}x{5}' -f $_.DeviceName,$_.Primary,$_.Bounds.X,$_.Bounds.Y,$_.Bounds.Width,$_.Bounds.Height }
   ```

3. You'll get two lines. Find the line that says **`Primary=False`** — that's
   the second monitor. Note its **X** and **Y** numbers. Example:

   ```
   \\.\DISPLAY2  Primary=False  X=1920  Y=0  Size=1920x1080
   ```
   Here X = **1920** and Y = **0**.

4. Now open the file **`start.bat`** in the folder: right-click it → **Edit**
   (it opens in Notepad). Near the top you'll see:

   ```
   set MONITOR_X=1920
   set MONITOR_Y=0
   ```

   Change those two numbers to the **X** and **Y** you just noted. Save and
   close (Notepad → File → Save).

   > Tip: the X number can be negative (like `-1920`) if the second monitor is
   > on the **left**. That's fine — type it exactly as shown, minus sign and all.

### Step 3.4 — Connect the barcode scanner

If Bluetooth:

1. Turn the scanner on and put it into pairing mode (see the scanner's own
   quick-start card — usually you scan a "Bluetooth pairing" barcode from it).
2. On the PC: **Start menu → Settings → Bluetooth & devices → Add device →
   Bluetooth**, and pick the scanner from the list.
3. Wait until it shows **Connected**.

If USB: just plug it in.

Either way, the scanner behaves like a **keyboard** — when it reads a label it
"types" the label's text into the PC. That's exactly what we want.

### Step 3.5 — Check the scanner presses "Enter" after each scan

This is important: after typing a code, the scanner must send an **Enter** (a
new line), so both your software and this display know the scan finished. To
check:

1. Open **Notepad** on the PC.
2. Click inside the blank Notepad window so the cursor is blinking there.
3. Scan any part label once.
4. **What you should see:** the code appears, and the cursor **jumps to the
   next line** by itself.

- If the cursor jumps to a new line → 👍 all good.
- If the code appears but the cursor **stays on the same line** → the scanner
  needs an "add Enter / carriage return suffix" setting turned on. This is done
  by scanning a small setup barcode from the scanner's manual. If you're not
  sure how, contact the person listed at the bottom of this guide.

### Step 3.6 — Sharing the scanner (how one scanner feeds two programs)

This is the key idea, so it's worth understanding. A small background helper
(`scanner-capture.exe`) lets **one scanner feed two things at once**:

- Your **existing software** on the main monitor receives every scan **exactly
  as it does today** — the helper does not block, change, or delay anything.
- This **spec display** on the second monitor **also** reacts to the same scan,
  automatically.

Two things worth knowing:

- **It only reacts to real scans, not to typing.** If someone types on the
  keyboard, the display ignores it. Only an actual barcode scan (a fast burst
  ending in Enter, containing a real part number) shows a picture.
- **No window needs to be "in front."** Unlike a normal program, the helper
  catches scans no matter which window you're working in — so your operators can
  stay in your existing software the whole time and the picture still updates.

The helper **starts automatically** with `start.bat` (Section 4) — you don't run
it separately.

> **For your IT team:** because this helper watches the keyboard to catch scans,
> some antivirus / endpoint tools may flag it the first time and ask you to
> allow it. It only forwards barcode scans to the local display on the same PC
> and sends nothing over the internet. If your existing software runs **"as
> administrator,"** start this system as administrator too (right-click
> `start.bat` → **Run as administrator**), otherwise the display may not pick up
> the scans.

### Step 3.7 — (Optional) Make it start automatically when the PC turns on

If you want the system to launch by itself every time the PC boots:

1. Right-click **`install.bat`** → **Run as administrator**.
2. Say **Yes** to the security prompt.

That's it. (To undo this later, run **`uninstall.bat`** the same way.)

---

## 4. Starting it each day

**Double-click `start.bat`** in the folder.

(If you did Step 3.7, it starts on its own when the PC turns on — you can skip
this.)

**What you should see, in order:**

1. One or two small black windows appear and **shrink to the taskbar** (these
   are the engine and the scanner helper — leave them running, don't close
   them).
2. A few seconds later, the **second monitor** goes full-screen and shows a
   **"Scan Part to Begin"** waiting screen.

The system is now ready. Your operators just use your existing software as
normal — the picture on the second monitor will follow along by itself.

---

## 5. How the operator uses it

1. **Scan** the part label the way you normally do → the correct spec **picture**
   appears on the second monitor, and your existing software receives the scan
   as usual.
2. Assemble the part using the picture. **The picture stays on screen the whole
   time** — it does not time out or disappear.
3. When the next part comes, **scan its label** → the picture switches to that
   part. The previous picture stays up until the next scan.

Good to know:

- Scanning the **same** label again just re-shows that same picture (it briefly
  refreshes) — nothing is skipped.
- Scanning a **different** part's label switches to that part's picture.
- Scanning a label the system doesn't recognise shows a red **error** screen
  with a beep for a moment, then it goes back to the picture that was showing.
- **Typing on the keyboard does not affect the display** — only real scans do.

---

## 6. Shutting down / restarting

- **To shut down:** press **Alt + F4** on the part-picture screen to close it,
  then close the small black window(s) in the taskbar. (Or just shut down the PC
  normally — that closes everything.)
- **To restart if something looks stuck:** close everything as above, then
  double-click **`start.bat`** again.

---

## 7. Troubleshooting

| Problem | What to check / do |
|---|---|
| **My existing software gets the scan, but the second-monitor picture doesn't change** | The scanner helper may not be running. Look for its small black window in the taskbar; if it's not there, double-click `start.bat` again. If your antivirus blocked it, allow it (see Step 3.6). If your software runs as administrator, restart `start.bat` as administrator too. |
| **Nothing happens anywhere when I scan** | 1) Is the scanner **Connected** (Bluetooth) or plugged in (USB)? 2) Do the Notepad test in Step 3.5 — if no text appears in Notepad, the scanner itself isn't sending anything; re-connect or re-pair it. |
| **The picture shows on the wrong monitor** | The monitor numbers in `start.bat` need adjusting. Redo Step 3.3 with the correct X/Y for the second monitor. |
| **The picture screen isn't full-screen / is on top of other things** | Close it with **Alt + F4** and double-click `start.bat` again. |
| **"Unknown Part" red screen when I scan a part** | That part number isn't in the system's part list yet, or it's a different label than expected. Note the part number and contact the person below. |
| **The waiting screen never appears after start.bat** | Make sure **Google Chrome** is installed. Also make sure the small black engine window is still running (minimised in the taskbar) — if it closed, double-click `start.bat` again. |
| **Scanner stopped working after a while** | It may have gone to sleep or lost Bluetooth. Re-check **Bluetooth & devices** shows it **Connected**; re-pair if needed (Step 3.4). |
| **The display reacts to normal typing** | It shouldn't — it only reacts to real part numbers scanned quickly. If this happens, note what was typed and contact the person below. |
| **Everything is frozen** | Restart the system: close everything, double-click `start.bat`. If still stuck, restart the PC. |

---

## 8. For the plant's IT person — updating the part pictures

The pictures are ordinary **PNG image files** in the **`assets`** folder, and
the list of parts is in a text file called **`config.json`**.

- **To replace a picture:** put the new PNG in the `assets` folder with the same
  file name as the old one. Restart with `start.bat`. Done.
- **To add or change which part shows which picture:** this needs a small edit
  to `config.json`. Please contact the person below rather than guessing, so
  nothing breaks.

Nothing here needs the internet, and no data leaves the PC. A record of every
scan the display received is saved automatically in the **`logs`** folder (one
file per day) in case it's ever needed.

---

## 9. Who to contact

If you get stuck on anything in this guide:

- **Name:** _______________________
- **Phone / email:** _______________________

Please include a photo of the screen and the part label if something looks
wrong — it makes it much faster to help.
