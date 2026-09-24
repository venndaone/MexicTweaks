@echo off
REM Baut MexxicTweak100.exe (benoetigt Go 1.22+ : https://go.dev/dl/)
cd /d "%~dp0"
go mod tidy || goto :err
go build -trimpath -ldflags "-H windowsgui -s -w" -o ..\MexxicTweak100.exe . || goto :err
echo.
echo Fertig: MexxicTweak100.exe liegt im Hauptordner.
pause
exit /b 0
:err
echo Build fehlgeschlagen.
pause
exit /b 1
