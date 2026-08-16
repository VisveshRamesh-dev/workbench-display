@echo off
REM =====================================================================
REM  Scanner copier (hub4com)
REM
REM  Reads the REAL scanner serial port and mirrors every scan to two
REM  virtual ports, so the existing plant software AND this display can
REM  both read the same scanner. See COM-SHARING-SETUP.md for the full
REM  one-time setup (installing com0com, renaming ports, etc.).
REM
REM  EDIT the values below to match your machine, then this launches
REM  automatically from start.bat.
REM =====================================================================

setlocal

REM --- Full path to hub4com.exe (wherever you unzipped it) ---
set HUB4COM="C:\hub4com\hub4com.exe"

REM --- The REAL scanner port AFTER you renamed it in Device Manager ---
set REAL_PORT=COM20

REM --- The two com0com "A-side" ports the copies are written to.       ---
REM --- Their B-side twins are what the two programs read:              ---
REM ---   CNCA0 -> CNCB0  (renamed COM1  -> the existing plant software) ---
REM ---   CNCA1 -> CNCB1  (renamed COM11 -> this display, serial-capture)---
set VIRT_A=CNCA0
set VIRT_B=CNCA1

REM --- Scanner baud rate (must match the real scanner) ---
set BAUD=9600

if not exist %HUB4COM% (
    echo ERROR: hub4com not found at %HUB4COM%
    echo Edit start-scanner-copier.bat and set HUB4COM to the correct path.
    pause
    exit /b 1
)

echo Starting scanner copier: %REAL_PORT% -> %VIRT_A% + %VIRT_B%
%HUB4COM% --baud=%BAUD% --octs=off --route=0:1,2 \\.\%REAL_PORT% \\.\%VIRT_A% \\.\%VIRT_B%

endlocal
