@echo off
setlocal
echo Removing autostart entry...
schtasks /Delete /F /TN "WorkbenchDisplayController" 2>nul
del "%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\WorkbenchDisplayController.lnk" 2>nul
echo Done. The app folder itself has not been deleted.
pause
endlocal
