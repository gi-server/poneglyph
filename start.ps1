Write-Host "==================================" -ForegroundColor Cyan
Write-Host " Starting Poneglyph Local Services " -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan

Write-Host "`n[0/3] Cleaning up old processes..." -ForegroundColor Yellow
# Kill process on port 8080 (Go API)
$goProcess = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
if ($goProcess) {
    Write-Host "Stopping old Go Server on port 8080..." -ForegroundColor Cyan
    Stop-Process -Id $goProcess.OwningProcess -Force -ErrorAction SilentlyContinue
}

# Kill process on port 5173 (Vite Dev Server)
$viteProcess = Get-NetTCPConnection -LocalPort 5173 -ErrorAction SilentlyContinue
if ($viteProcess) {
    Write-Host "Stopping old Vite Dev Server on port 5173..." -ForegroundColor Cyan
    Stop-Process -Id $viteProcess.OwningProcess -Force -ErrorAction SilentlyContinue
}

# Start Go Backend Server in a new window
Write-Host "`n[1/2] Starting Go API Backend (port 8080)..." -ForegroundColor Yellow
Start-Process powershell -ArgumentList "-NoExit -Command `"go run main.go`""
Write-Host "Go Backend launching on :8080 in a new window." -ForegroundColor Green

# Start Vite Frontend Dev Server with HMR in a new window
Write-Host "`n[2/2] Starting Vite + React + Tailwind v4 Dev Server (port 5173)..." -ForegroundColor Yellow
Start-Process powershell -ArgumentList "-NoExit -Command `"cd frontend; npm run dev`""
Write-Host "Vite Dev Server launching on :5173 in a new window." -ForegroundColor Green

Write-Host "`n==================================" -ForegroundColor Cyan
Write-Host "Poneglyph services are launching!" -ForegroundColor Green
Write-Host "Vite Dev UI (with Hot Reload):  http://localhost:5173" -ForegroundColor Cyan
Write-Host "Go Backend & Production UI:     http://localhost:8080" -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "To produce a standalone production .exe file, run:" -ForegroundColor Yellow
Write-Host "  .\build.ps1" -ForegroundColor Green
