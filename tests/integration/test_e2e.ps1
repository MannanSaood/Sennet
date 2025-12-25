# Sennet End-to-End Integration Tests
# Tests the full agent-to-server flow

param(
    [switch]$LocalOnly = $false
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)

function Write-TestResult($name, $passed, $message = "") {
    if ($passed) {
        Write-Host "  [PASS] $name" -ForegroundColor Green
    } else {
        Write-Host "  [FAIL] $name" -ForegroundColor Red
        if ($message) { Write-Host "         $message" -ForegroundColor Yellow }
    }
}

function Write-Skip($name, $reason) {
    Write-Host "  [SKIP] $name - $reason" -ForegroundColor Yellow
}

Write-Host "`n================================================" -ForegroundColor Magenta
Write-Host " Sennet Integration Tests" -ForegroundColor Magenta
Write-Host "================================================`n" -ForegroundColor Magenta

# Prerequisites check
Write-Host "--- Prerequisites ---" -ForegroundColor Cyan

# Check Go
try {
    $goVersion = go version 2>&1
    Write-TestResult "Go installed" $true $goVersion
} catch {
    Write-Skip "Go installed" "Go not found in PATH"
}

# Check Rust
try {
    $rustVersion = rustc --version 2>&1
    Write-TestResult "Rust installed" $true $rustVersion
} catch {
    Write-Skip "Rust installed" "Rust not found in PATH"
}

# Check for built components
$backendBinary = Join-Path $ProjectRoot "backend/sennet-server.exe"
$agentBinary = Join-Path $ProjectRoot "agent/target/release/sennet.exe"

$hasBackend = Test-Path $backendBinary -or (Test-Path (Join-Path $ProjectRoot "backend/main.go"))
$hasAgent = Test-Path $agentBinary -or (Test-Path (Join-Path $ProjectRoot "agent/Cargo.toml"))

if (-not ($hasBackend -and $hasAgent)) {
    Write-Host "`n[INFO] Integration tests require both backend and agent to be built." -ForegroundColor Yellow
    Write-Host "       Run these first:" -ForegroundColor Yellow
    Write-Host "         cd backend && go build -o sennet-server.exe" -ForegroundColor Gray
    Write-Host "         cd agent && cargo build --release" -ForegroundColor Gray
    
    if (-not $hasBackend) { Write-Skip "Backend binary" "Not built yet" }
    if (-not $hasAgent) { Write-Skip "Agent binary" "Not built yet" }
    exit 0
}

Write-Host "`n--- Integration Scenarios ---" -ForegroundColor Cyan

# Scenario 1: Server starts and responds to health check
Write-Host "`n[Scenario 1] Server Health Check" -ForegroundColor White
Write-Skip "Server health check" "Implementation pending"

<#
# Start server in background
$serverProcess = Start-Process -FilePath $backendBinary -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 2

# Check health endpoint
try {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/health" -Method Get
    Write-TestResult "Server responds to /health" $true
} catch {
    Write-TestResult "Server responds to /health" $false $_.Exception.Message
}

# Cleanup
$serverProcess | Stop-Process -Force
#>

# Scenario 2: Heartbeat without auth returns 401
Write-Host "`n[Scenario 2] Auth Required" -ForegroundColor White
Write-Skip "Unauthenticated request returns 401" "Implementation pending"

<#
$serverProcess = Start-Process -FilePath $backendBinary -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 2

try {
    $response = Invoke-WebRequest -Uri "http://localhost:8080/sentinel.v1.SentinelService/Heartbeat" `
        -Method Post -ContentType "application/json" -Body "{}"
    Write-TestResult "Returns 401 without auth" $false "Expected 401, got $($response.StatusCode)"
} catch {
    if ($_.Exception.Response.StatusCode -eq 401) {
        Write-TestResult "Returns 401 without auth" $true
    } else {
        Write-TestResult "Returns 401 without auth" $false $_.Exception.Message
    }
}

$serverProcess | Stop-Process -Force
#>

# Scenario 3: Heartbeat with valid key succeeds
Write-Host "`n[Scenario 3] Authenticated Heartbeat" -ForegroundColor White
Write-Skip "Authenticated heartbeat succeeds" "Implementation pending"

# Scenario 4: Agent connects and sends heartbeat
Write-Host "`n[Scenario 4] Full Agent-Server Flow" -ForegroundColor White
Write-Skip "Agent connects to server" "Implementation pending"

# Scenario 5: Version mismatch triggers UPGRADE command
Write-Host "`n[Scenario 5] Upgrade Command Flow" -ForegroundColor White
Write-Skip "Server issues UPGRADE for old agent" "Implementation pending"

Write-Host "`n================================================"
Write-Host " Integration Tests Complete"
Write-Host " (Most tests pending implementation)"
Write-Host "================================================`n"
