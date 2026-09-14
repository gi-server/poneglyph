Write-Host "========================================" -ForegroundColor Cyan
Write-Host " Building Poneglyph Production Executable " -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$ErrorActionPreference = "Stop"

# 1. Build Vite + Tailwind v4 Frontend
Write-Host "`n[1/2] Compiling Vite + React + Tailwind v4 into ./public..." -ForegroundColor Yellow
Push-Location frontend
try {
    npm run build
} finally {
    Pop-Location
}

if (!(Test-Path "./public/index.html")) {
    Write-Error "Frontend build failed: ./public/index.html not found."
    exit 1
}
Write-Host "Frontend successfully compiled into ./public!" -ForegroundColor Green

# 2. Compile Go backend into single binary (directly embedding ./public)
Write-Host "`n[2/2] Compiling standalone Go binary (poneglyph.exe)..." -ForegroundColor Yellow
go build -ldflags="-s -w" -o poneglyph.exe .

if (!(Test-Path "./poneglyph.exe")) {
    Write-Error "Go compilation failed: ./poneglyph.exe was not created."
    exit 1
}

$exeSize = (Get-Item ./poneglyph.exe).Length / 1MB
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "SUCCESS: Production build complete!" -ForegroundColor Green
Write-Host "Standalone executable: poneglyph.exe ($([math]::Round($exeSize, 2)) MB)" -ForegroundColor Cyan
Write-Host "To run the production app, execute:" -ForegroundColor Yellow
Write-Host "  .\poneglyph.exe" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
