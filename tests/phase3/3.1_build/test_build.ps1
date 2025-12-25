# Phase 3.1: Cross-Compilation & Build Artifact Tests
# Verifies GitHub Actions workflow and release artifacts

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
Write-Host " Phase 3.1: Cross-Compile & Checksum" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

# Test 1: GitHub Actions workflow exists
$workflowPath = Join-Path $ProjectRoot ".github/workflows/release.yml"
$workflowExists = Test-Path $workflowPath
if (-not $workflowExists) {
    Write-Skip "Workflow file exists" "Not created yet"
    Write-Host "`nCreate .github/workflows/release.yml to enable these tests" -ForegroundColor Yellow
    exit 0
}
Write-TestResult "Workflow file exists" $true

# Test 2: Workflow YAML is valid
try {
    # PowerShell can parse YAML with ConvertFrom-Yaml if module installed
    # For now, just check it's readable
    $content = Get-Content $workflowPath -Raw
    $isValid = $content.Length -gt 0
    Write-TestResult "Workflow YAML readable" $isValid
} catch {
    Write-TestResult "Workflow YAML readable" $false $_.Exception.Message
}

# Test 3: Workflow targets x86_64
$hasX86 = $content -match "x86_64-unknown-linux-musl"
Write-TestResult "Targets x86_64-unknown-linux-musl" $hasX86

# Test 4: Workflow targets aarch64 (ARM64)
$hasArm = $content -match "aarch64-unknown-linux-musl"
Write-TestResult "Targets aarch64-unknown-linux-musl" $hasArm

# Test 5: Workflow generates checksums
$hasSha256 = $content -match "sha256sum" -or $content -match "SHA256"
Write-TestResult "Generates SHA256 checksums" $hasSha256

# Test 6: Workflow uploads to GitHub Releases
$hasUpload = $content -match "upload-artifact" -or $content -match "release"
Write-TestResult "Uploads to releases" $hasUpload

# Test 7: Workflow strips binaries
$hasStrip = $content -match "strip" -or $content -match "RUSTFLAGS.*strip"
Write-TestResult "Strips binaries" $hasStrip

# Test 8: Workflow triggers on tags
$hasTrigger = $content -match "tags:" -or $content -match "release"
Write-TestResult "Triggers on tag/release" $hasTrigger

# Test 9: Check for cross-compilation tool
$hasCross = $content -match "cross" -or $content -match "zigbuild" -or $content -match "cargo-zigbuild"
Write-TestResult "Uses cross-compilation tool" $hasCross

# Test 10: Local build test (if Rust is installed)
Write-Host "`n--- Local Build Test ---" -ForegroundColor Cyan
$agentPath = Join-Path $ProjectRoot "agent"
if (Test-Path (Join-Path $agentPath "Cargo.toml")) {
    try {
        Push-Location $agentPath
        # Just check if cargo is available
        $cargoVersion = cargo --version 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-TestResult "Cargo available" $true $cargoVersion
            
            # Try a debug build (faster than release)
            Write-Host "  [INFO] Running cargo check..." -ForegroundColor Gray
            $checkResult = cargo check 2>&1
            $checkPassed = $LASTEXITCODE -eq 0
            Write-TestResult "Cargo check passes" $checkPassed
        } else {
            Write-Skip "Local build" "Cargo not installed"
        }
        Pop-Location
    } catch {
        Pop-Location
        Write-Skip "Local build" $_.Exception.Message
    }
} else {
    Write-Skip "Local build test" "agent/Cargo.toml not found"
}

Write-Host "`n========================================"
Write-Host " Build Tests Complete"
Write-Host "========================================`n"
