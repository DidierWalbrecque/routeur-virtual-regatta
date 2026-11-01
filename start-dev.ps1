<#
.SYNOPSIS
Démarre le routeur en local : API Go, interface React et relais Flask de l'extension.

.DESCRIPTION
Ouvre trois fenêtres (une par service), attend que chacune réponde, puis ouvre
l'interface dans le navigateur.

  API Go                   http://localhost:8080   (calculs, météo, état du jeu)
  Interface React (Vite)   http://localhost:5173   (à ouvrir dans le navigateur)
  Relais Flask             http://localhost:5000   (reçoit l'état envoyé par l'extension)

.PARAMETER Stop
Arrête les services démarrés par ce script.

.PARAMETER Status
Indique quels services répondent, sans rien démarrer.

.PARAMETER NoBrowser
N'ouvre pas le navigateur.

.EXAMPLE
.\start-dev.ps1

.EXAMPLE
.\start-dev.ps1 -Stop
#>
param(
    [switch]$Stop,
    [switch]$Status,
    [switch]$NoBrowser
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$PidFile = Join-Path $Root '.dev-pids.json'
$VenvPython = Join-Path $Root '.venv-flask\Scripts\python.exe'

$Services = @(
    @{
        Name = 'API Go'; Port = 8080; Url = 'http://localhost:8080/health'
        Directory = Join-Path $Root 'backend'
        Command = 'go run ./cmd/api'
    },
    @{
        Name = 'Interface React'; Port = 5173; Url = 'http://localhost:5173/'
        Directory = Join-Path $Root 'frontend'
        Command = 'npm run dev -- --port 5173 --strictPort'
    },
    @{
        Name = 'Relais Flask'; Port = 5000; Url = 'http://localhost:5000/'
        Directory = $Root
        Command = '.venv-flask\Scripts\python.exe -m flask --app flaskapp.app run --host 127.0.0.1 --port 5000'
    }
)

function Write-Step($message) { Write-Host "  $message" }
function Write-Ok($message) { Write-Host "  [ok] $message" -ForegroundColor Green }
function Write-Warn($message) { Write-Host "  [!]  $message" -ForegroundColor Yellow }
function Write-Fail($message) { Write-Host "  [x]  $message" -ForegroundColor Red }

function Get-PortOwner([int]$Port) {
    $connection = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if (-not $connection) { return $null }
    $process = Get-Process -Id $connection.OwningProcess -ErrorAction SilentlyContinue
    return [pscustomobject]@{
        Id   = $connection.OwningProcess
        Name = if ($process) { $process.ProcessName } else { 'inconnu' }
    }
}

# Vrai dès que le service répond en HTTP, même par une erreur (la page d'accueil
# du relais répond 503 tant que l'interface n'est pas construite).
function Test-ServiceUp([string]$Url) {
    try {
        Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 2 | Out-Null
        return $true
    } catch [System.Net.WebException] {
        return $null -ne $_.Exception.Response
    } catch {
        return $false
    }
}

function Stop-StartedServices {
    if (-not (Test-Path $PidFile)) {
        Write-Step 'Aucun service démarré par ce script.'
        return
    }
    $entries = Get-Content $PidFile -Raw | ConvertFrom-Json
    foreach ($entry in $entries) {
        # /T arrête aussi les processus fils (go run, node, flask).
        & taskkill.exe /PID $entry.Pid /T /F 2>&1 | Out-Null
        if ($LASTEXITCODE -eq 0) { Write-Ok "$($entry.Name) arrêté" } else { Write-Step "$($entry.Name) : déjà arrêté" }
    }
    Remove-Item $PidFile -ErrorAction SilentlyContinue
}

function Show-Status {
    foreach ($service in $Services) {
        $owner = Get-PortOwner $service.Port
        if ($owner -and (Test-ServiceUp $service.Url)) {
            Write-Ok "$($service.Name) répond sur le port $($service.Port) (PID $($owner.Id))"
        } elseif ($owner) {
            Write-Warn "$($service.Name) : le port $($service.Port) est occupé par $($owner.Name) (PID $($owner.Id)) mais ne répond pas"
        } else {
            Write-Step "$($service.Name) : arrêté (port $($service.Port) libre)"
        }
    }
}

Write-Host ''
Write-Host 'Routeur Virtual Regatta' -ForegroundColor Cyan

if ($Stop) { Stop-StartedServices; exit 0 }
if ($Status) { Show-Status; exit 0 }

# --- Prérequis ---------------------------------------------------------------
foreach ($tool in 'go', 'node', 'npm') {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        Write-Fail "'$tool' est introuvable. Installez-le puis relancez ce script."
        exit 1
    }
}

if (-not (Test-Path $VenvPython)) {
    Write-Warn 'Environnement Python du relais absent : création de .venv-flask'
    $system = Get-Command py, python -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $system) {
        Write-Fail "Python est introuvable. Installez-le puis relancez ce script."
        exit 1
    }
    Push-Location $Root
    try {
        & $system.Source -m venv .venv-flask
        & $VenvPython -m pip install --quiet -r flaskapp\requirements.txt
    } finally { Pop-Location }
    Write-Ok 'Environnement Python prêt'
}

if (-not (Test-Path (Join-Path $Root 'frontend\node_modules'))) {
    Write-Warn 'Dépendances de l''interface absentes : npm install'
    Push-Location (Join-Path $Root 'frontend')
    try { & npm install } finally { Pop-Location }
}

# --- Ports -------------------------------------------------------------------
# Un ancien processus encore actif répondrait avec l'ancienne version du code :
# on refuse de démarrer par-dessus.
$known = @()
if (Test-Path $PidFile) { $known = @(Get-Content $PidFile -Raw | ConvertFrom-Json) }
$blocked = $false
foreach ($service in $Services) {
    $owner = Get-PortOwner $service.Port
    if ($owner) {
        $blocked = $true
        Write-Fail "Le port $($service.Port) ($($service.Name)) est déjà utilisé par $($owner.Name) (PID $($owner.Id))."
    }
}
if ($blocked) {
    if ($known.Count -gt 0) { Write-Step 'Arrêtez les services lancés par ce script : .\start-dev.ps1 -Stop' }
    Write-Step 'Sinon, fermez le processus indiqué (Stop-Process -Id <PID>) puis relancez.'
    exit 1
}

# --- Démarrage ---------------------------------------------------------------
$started = @()
foreach ($service in $Services) {
    Write-Step "Démarrage : $($service.Name)"
    $process = Start-Process -FilePath 'cmd.exe' `
        -ArgumentList '/k', "title $($service.Name) && $($service.Command)" `
        -WorkingDirectory $service.Directory -PassThru
    $started += [pscustomobject]@{ Name = $service.Name; Pid = $process.Id }
}
$started | ConvertTo-Json | Set-Content -Path $PidFile -Encoding UTF8

# --- Attente -----------------------------------------------------------------
$failed = $false
foreach ($service in $Services) {
    $deadline = (Get-Date).AddSeconds(120)
    $up = $false
    while ((Get-Date) -lt $deadline) {
        if (Test-ServiceUp $service.Url) { $up = $true; break }
        Start-Sleep -Milliseconds 700
    }
    if ($up) { Write-Ok "$($service.Name) : $($service.Url)" }
    else { Write-Fail "$($service.Name) ne répond pas : regardez sa fenêtre."; $failed = $true }
}

Write-Host ''
Write-Host '  Interface : http://localhost:5173' -ForegroundColor Cyan
Write-Host '  Extension : état envoyé à http://localhost:5000 (relais) puis à l''API Go'
Write-Host '  Arrêt     : .\start-dev.ps1 -Stop   (ou fermez les trois fenêtres)'
Write-Host ''

if ($failed) { exit 1 }
if (-not $NoBrowser) { Start-Process 'http://localhost:5173' }
