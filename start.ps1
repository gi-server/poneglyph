$candidateDirs = @()
if ($PSScriptRoot) { $candidateDirs += $PSScriptRoot }
if ($PSCommandPath) { $candidateDirs += (Split-Path -Parent $PSCommandPath) }
if ($MyInvocation.MyCommand.Path) { $candidateDirs += (Split-Path -Parent $MyInvocation.MyCommand.Path) }
$candidateDirs += $PWD.Path
$candidateDirs += "c:\Users\varun\poneglyph"

$scriptDir = $null
foreach ($dir in $candidateDirs) {
    if ($dir -and (Test-Path (Join-Path $dir "main.go"))) {
        $scriptDir = (Resolve-Path $dir).Path
        break
    }
}

if (-not $scriptDir) {
    Write-Error "Could not locate Poneglyph project directory containing main.go."
    exit 1
}

Set-Location $scriptDir

Write-Host "==================================" -ForegroundColor Cyan
Write-Host " Starting Poneglyph Local Services " -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan

Write-Host "`n[0/3] Checking MongoDB..." -ForegroundColor Yellow
$mongoConn = Get-NetTCPConnection -LocalPort 27017 -ErrorAction SilentlyContinue
if (-not $mongoConn) {
    Write-Host "WARNING: MongoDB does not seem to be running on port 27017." -ForegroundColor Red
    Write-Host "Ensure MongoDB service or Docker container is running before proceeding." -ForegroundColor DarkYellow
} else {
    Write-Host "MongoDB is running on port 27017." -ForegroundColor Green
}

Write-Host "`n[1/3] Cleaning up old processes on ports 8080 & 5173..." -ForegroundColor Yellow
# Safely kill process on port 8080 (Go API)
$goPids = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique | Where-Object { $_ -gt 4 }
if ($goPids) {
    Write-Host "Stopping old Go Server on port 8080 (PID: $($goPids -join ', '))..." -ForegroundColor Cyan
    $goPids | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }
}

# Safely kill process on port 5173 (Vite Dev Server)
$vitePids = Get-NetTCPConnection -LocalPort 5173 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique | Where-Object { $_ -gt 4 }
if ($vitePids) {
    Write-Host "Stopping old Vite Dev Server on port 5173 (PID: $($vitePids -join ', '))..." -ForegroundColor Cyan
    $vitePids | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }
}

if ($goPids -or $vitePids) {
    Start-Sleep -Milliseconds 500
}

Write-Host "`n[2/3] Launching Go Backend & Vite Dev Server in this terminal..." -ForegroundColor Yellow

$goJob = Start-Job -Name "GoBackend" -ScriptBlock {
    param($root)
    Set-Location $root
    $env:BIND_ADDR = "127.0.0.1:8080"
    & go run (Join-Path $root "main.go") 2>&1
} -ArgumentList $scriptDir

$viteJob = Start-Job -Name "ViteDev" -ScriptBlock {
    param($root)
    $frontDir = Join-Path $root "frontend"
    Set-Location $frontDir
    npm run dev 2>&1
} -ArgumentList $scriptDir

Write-Host "`n==================================" -ForegroundColor Cyan
Write-Host "Poneglyph services are active!" -ForegroundColor Green
Write-Host "Frontend (Vite Dev UI):     http://localhost:5173" -ForegroundColor Cyan
Write-Host "Backend (Go REST API):      http://localhost:8080/api" -ForegroundColor Cyan
Write-Host "Press Ctrl+C at any time to stop all services." -ForegroundColor Magenta
Write-Host "==================================`n" -ForegroundColor Cyan

try {
    while ($true) {
        $goOutput = Receive-Job -Job $goJob -ErrorAction SilentlyContinue
        if ($goOutput) {
            $goOutput | ForEach-Object { Write-Host "[Go]   $_" -ForegroundColor Cyan }
        }

        $viteOutput = Receive-Job -Job $viteJob -ErrorAction SilentlyContinue
        if ($viteOutput) {
            $viteOutput | ForEach-Object { Write-Host "[Vite] $_" -ForegroundColor Green }
        }

        # If any job completed/failed unexpectedly, notify and break
        if ($goJob.State -notin @("Running", "NotStarted")) {
            Write-Host "`n[Go backend stopped (State: $($goJob.State))]" -ForegroundColor Red
            break
        }
        if ($viteJob.State -notin @("Running", "NotStarted")) {
            Write-Host "`n[Vite server stopped (State: $($viteJob.State))]" -ForegroundColor Red
            break
        }

        Start-Sleep -Milliseconds 250
    }
}
finally {
    Write-Host "`n`nShutting down Poneglyph services..." -ForegroundColor Yellow

    # Stop and remove background jobs
    Stop-Job $goJob, $viteJob -ErrorAction SilentlyContinue
    Remove-Job $goJob, $viteJob -ErrorAction SilentlyContinue

    # Ensure child processes on ports are stopped
    $goPids = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique | Where-Object { $_ -gt 4 }
    if ($goPids) {
        $goPids | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }
    }
    $vitePids = Get-NetTCPConnection -LocalPort 5173 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique | Where-Object { $_ -gt 4 }
    if ($vitePids) {
        $vitePids | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }
    }

    Write-Host "All services stopped successfully." -ForegroundColor Green
}
