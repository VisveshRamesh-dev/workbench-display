@echo off
REM =====================================================================
REM  Workbench Display Controller - one-time install
REM  Run once, as Administrator, on the industrial PC.
REM =====================================================================

setlocal
cd /d "%~dp0"

echo.
echo =====================================================
echo   Workbench Display Controller - Installation
echo =====================================================
echo.

if not exist "workbench-display.exe" (
    echo ERROR: workbench-display.exe not found. Build it first with:
    echo   go build -o workbench-display.exe
    pause
    exit /b 1
)
if not exist "start.bat" (
    echo ERROR: start.bat not found.
    pause
    exit /b 1
)

echo Registering Workbench Display Controller for autostart on user logon...
schtasks /Create /F /SC ONLOGON /RL HIGHEST /TN "WorkbenchDisplayController" /TR "\"%~dp0start.bat\"" >nul
if errorlevel 1 (
    echo WARNING: Task Scheduler registration failed. Falling back to Startup folder.
    powershell -NoProfile -Command "$s = (New-Object -ComObject WScript.Shell).CreateShortcut([Environment]::GetFolderPath('Startup') + '\WorkbenchDisplayController.lnk'); $s.TargetPath = '%~dp0start.bat'; $s.WorkingDirectory = '%~dp0'; $s.Save()"
)

echo.
echo Installation complete.
echo.
echo Next steps:
echo   1. Open start.bat and set MONITOR_X / MONITOR_Y to the secondary
echo      monitor's top-left corner (see Windows Display Settings).
echo   2. Run start.bat to launch immediately, or reboot to test autostart.
echo   3. Press Alt+F4 in a kiosk window to exit for maintenance.
echo.
pause
endlocal
