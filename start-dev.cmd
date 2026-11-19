@echo off
rem Double-clic : demarre l'API Go, l'interface React et le relais Flask.
rem Arguments possibles : -Stop, -Status, -NoBrowser
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0start-dev.ps1" %*
if errorlevel 1 pause
