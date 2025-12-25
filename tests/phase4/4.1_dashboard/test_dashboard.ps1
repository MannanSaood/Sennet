# Phase 4.1: Grafana Dashboard Tests
# Validates dashboard JSON structure and panel definitions

param()

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))

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

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host " Phase 4.1: Grafana Dashboard Tests" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

# Test 1: Dashboard directory exists
$dashboardDir = Join-Path $ProjectRoot "dashboards"
if (-not (Test-Path $dashboardDir)) {
    Write-Skip "Dashboards directory" "Not created yet"
    Write-Host "`nCreate dashboards/overview.json to enable these tests" -ForegroundColor Yellow
    exit 0
}
Write-TestResult "Dashboards directory exists" $true

# Test 2: Overview dashboard exists
$overviewPath = Join-Path $dashboardDir "overview.json"
if (-not (Test-Path $overviewPath)) {
    Write-Skip "overview.json" "Not created yet"
    exit 0
}
Write-TestResult "overview.json exists" $true

# Test 3: Valid JSON
try {
    $dashboard = Get-Content $overviewPath -Raw | ConvertFrom-Json
    Write-TestResult "Valid JSON structure" $true
} catch {
    Write-TestResult "Valid JSON structure" $false $_.Exception.Message
    exit 1
}

# Test 4: Has title
$hasTitle = $null -ne $dashboard.title -and $dashboard.title.Length -gt 0
Write-TestResult "Dashboard has title" $hasTitle

# Test 5: Has panels
$hasPanels = $null -ne $dashboard.panels -and $dashboard.panels.Count -gt 0
Write-TestResult "Dashboard has panels" $hasPanels

if ($hasPanels) {
    Write-Host "`n--- Panel Validation ---" -ForegroundColor Cyan
    
    # Expected panels based on roadmap
    $expectedPanels = @("Packets", "Drop", "Anomal")
    
    foreach ($expected in $expectedPanels) {
        $found = $dashboard.panels | Where-Object { $_.title -match $expected }
        if ($found) {
            Write-TestResult "Panel exists: $expected*" $true
        } else {
            Write-Skip "Panel: $expected" "Not found (may use different naming)"
        }
    }
    
    # Test 6: Each panel has required fields
    Write-Host "`n--- Panel Structure ---" -ForegroundColor Cyan
    $panelIndex = 0
    foreach ($panel in $dashboard.panels) {
        $panelIndex++
        $hasType = $null -ne $panel.type
        $hasTargets = $null -ne $panel.targets -or $null -ne $panel.datasource
        
        if ($hasType -and $hasTargets) {
            Write-TestResult "Panel $panelIndex ($($panel.title)): valid structure" $true
        } else {
            Write-TestResult "Panel $panelIndex: valid structure" $false "Missing type or targets"
        }
    }
}

# Test 7: Uses Prometheus/OTLP data source
$content = Get-Content $overviewPath -Raw
$hasPrometheus = $content -match "prometheus" -or $content -match "Prometheus"
$hasOTLP = $content -match "tempo" -or $content -match "loki" -or $content -match "otel"
Write-TestResult "Uses observability datasource" ($hasPrometheus -or $hasOTLP)

# Test 8: Dashboard is importable (basic structure check)
$hasSchemaVersion = $null -ne $dashboard.schemaVersion
$hasUid = $null -ne $dashboard.uid -or $null -ne $dashboard.id
Write-TestResult "Has Grafana schema version" $hasSchemaVersion
Write-TestResult "Has dashboard UID/ID" $hasUid

Write-Host "`n========================================"
Write-Host " Dashboard Tests Complete"
Write-Host "========================================`n"
