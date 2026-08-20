Write-Host "==================================" -ForegroundColor Cyan
Write-Host " Starting DocuNest Local Services " -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan

Write-Host "`n[0/2] Cleaning up old processes..." -ForegroundColor Yellow
# Kill process on port 8080 (Go API)
$goProcess = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
if ($goProcess) {
    Write-Host "Stopping old Go Server on port 8080..." -ForegroundColor Cyan
    Stop-Process -Id $goProcess.OwningProcess -Force -ErrorAction SilentlyContinue
}

# Check if Ollama is responding
Write-Host "`n[1/2] Checking Ollama AI Service..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "http://localhost:11434" -UseBasicParsing -ErrorAction Stop
    Write-Host "Ollama is running!" -ForegroundColor Green
} catch {
    Write-Host "WARNING: Ollama does not seem to be running on port 11434." -ForegroundColor Red
    Write-Host "Attempting to start Ollama service automatically..." -ForegroundColor Yellow
    try {
        Start-Process powershell -ArgumentList "-NoExit -Command `"ollama serve`""
        Write-Host "Ollama Service launching in a new window. Please wait a moment for it to initialize." -ForegroundColor Green
        Start-Sleep -Seconds 3
    } catch {
        Write-Host "Failed to start Ollama. Please start the Ollama application from your Windows Start menu." -ForegroundColor Red
    }
}

# Start Go Backend Server in a new window
Write-Host "`n[2/2] Starting Go API Backend..." -ForegroundColor Yellow
Start-Process powershell -ArgumentList "-NoExit -Command `"go run ./cmd/api/main.go`""
Write-Host "Go Backend launching in a new window." -ForegroundColor Green

Write-Host "`n==================================" -ForegroundColor Cyan
Write-Host "DocuNest services have been launched!" -ForegroundColor Green
Write-Host "Access the app at: http://localhost:8080" -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "NOTE: Great Sage (document intelligence) must be started separately." -ForegroundColor Yellow
Write-Host "  cd ..\great-sage && uvicorn app.main:app --host 127.0.0.1 --port 8000" -ForegroundColor DarkGray

