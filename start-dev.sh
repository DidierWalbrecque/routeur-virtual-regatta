#!/usr/bin/env bash
# Lance start-dev.ps1 depuis Git Bash : demarre l'API Go, l'interface React et le
# relais Flask de l'extension. Arguments possibles : -Stop, -Status, -NoBrowser
directory="$(cd "$(dirname "$0")" && pwd -W 2>/dev/null || pwd)"
exec powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$directory/start-dev.ps1" "$@"
