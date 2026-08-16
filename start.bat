@echo off
REM =====================================================================
REM  Workbench Display Controller - single-station launcher
REM
REM  Starts the Go server, then opens ONE Chrome kiosk window on the
REM  SECONDARY monitor. The Bluetooth scanner types into this window, so
REM  it must stay focused. Set MONITOR_X / MONITOR_Y to the top-left
REM  corner of the secondary monitor (from Windows Display Settings).
REM =====================================================================

setlocal
cd /d "%~dp0"

REM ---- Secondary monitor top-left corner ----
REM Settings > System > Display shows each monitor's position. If the
REM secondary sits to the RIGHT of a 1920-wide primary, use X=1920, Y=0.
set MONITOR_X=1920
set MONITOR_Y=0

REM ---- Find Chrome ----
set CHROME=""
if exist "%ProgramFiles%\Google\Chrome\Application\chrome.exe" set CHROME="%ProgramFiles%\Google\Chrome\Application\chrome.exe"
if exist "%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe" set CHROME="%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe"
if exist "%LocalAppData%\Google\Chrome\Application\chrome.exe" set CHROME="%LocalAppData%\Google\Chrome\Application\chrome.exe"
if %CHROME%=="" (
    echo ERROR: Chrome not found. Install Google Chrome first.
    pause
    exit /b 1
)

REM ---- Start the Go server ----
echo Starting Workbench Display Controller server...
start "workbench-server" /MIN cmd /c "workbench-display.exe"

REM Wait for server to bind the port
timeout /t 2 /nobreak >nul

REM ---- Start the scan capture helper ----
REM This scanner is on a SERIAL/COM port, so we read it with serial-capture
REM (set serial_port / serial_mode in config.json to OUR port from the COM
REM splitter). For a keyboard-mode scanner instead, launch scanner-capture.exe.
if exist "serial-capture.exe" (
    echo Starting serial capture helper...
    start "serial-capture" /MIN cmd /c "serial-capture.exe"
) else if exist "scanner-capture.exe" (
    echo Starting keyboard capture helper...
    start "scanner-capture" /MIN cmd /c "scanner-capture.exe"
)

set BASE=http://localhost:8080

REM ---- Open the single kiosk display on the secondary monitor ----
REM --kiosk gives full-screen with no browser chrome. --window-position
REM places it on the monitor whose top-left corner matches MONITOR_X/Y.
set FLAGS=--kiosk --disable-pinch --overscroll-history-navigation=0 --disable-features=TranslateUI --no-first-run --disable-session-crashed-bubble

echo Launching display on secondary monitor at %MONITOR_X%,%MONITOR_Y% ...
start "" %CHROME% %FLAGS% --user-data-dir="%~dp0chrome-profile-1" --window-position=%MONITOR_X%,%MONITOR_Y% %BASE%/station/WB-01

echo.
echo Display launched. The server console is minimized.
echo Keep the kiosk window focused so scanner input lands on it.
echo   Supervisor view: %BASE%/supervisor
echo   Simulator:       %BASE%/simulator

endlocal
