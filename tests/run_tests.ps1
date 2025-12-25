# Sennet Test Runner
# Usage: .\run_tests.ps1 -Phase 1.1 | -Phase 1 | -All

param(
    [string]$Phase = "",
    [switch]$All = $false
)

$ErrorActionPreference = "Stop"
$script:TestsPassed = 0
$script:TestsFailed = 0
$script:TestsSkipped = 0

function Write-TestHeader($text) {
    Write-Host "`n$('=' * 60)" -ForegroundColor Cyan
    Write-Host " $text" -ForegroundColor Cyan
    Write-Host "$('=' * 60)" -ForegroundColor Cyan
}

function Write-TestResult($name, $passed, $message = "") {
    if ($passed) {
        Write-Host "  [PASS] $name" -ForegroundColor Green
        $script:TestsPassed++
    } else {
        Write-Host "  [FAIL] $name" -ForegroundColor Red
        if ($message) { Write-Host "         $message" -ForegroundColor Yellow }
        $script:TestsFailed++
    }
}

function Write-TestSkipped($name, $reason) {
    Write-Host "  [SKIP] $name - $reason" -ForegroundColor Yellow
    $script:TestsSkipped++
}

# ============================================================
# PHASE 1.1: Proto Generation Tests
# ============================================================
function Test-Phase1_1 {
    Write-TestHeader "Phase 1.1: Buf & ConnectRPC Definition"
    
    # Test 1: buf.yaml exists
    $bufYaml = Test-Path "buf.yaml"
    Write-TestResult "buf.yaml exists" $bufYaml
    
    # Test 2: buf.gen.yaml exists
    $bufGenYaml = Test-Path "buf.gen.yaml"
    Write-TestResult "buf.gen.yaml exists" $bufGenYaml
    
    # Test 3: Proto file exists
    $protoExists = Test-Path "proto/sentinel/v1/sentinel.proto"
    Write-TestResult "sentinel.proto exists" $protoExists
    
    # Test 4: Proto lints successfully
    try {
        $lintResult = npx buf lint 2>&1
        $lintPassed = $LASTEXITCODE -eq 0
        Write-TestResult "Proto lint passes" $lintPassed $lintResult
    } catch {
        Write-TestResult "Proto lint passes" $false $_.Exception.Message
    }
    
    # Test 5: Proto builds successfully
    try {
        $buildResult = npx buf build 2>&1
        $buildPassed = $LASTEXITCODE -eq 0
        Write-TestResult "Proto build passes" $buildPassed $buildResult
    } catch {
        Write-TestResult "Proto build passes" $false $_.Exception.Message
    }
    
    # Test 6: Go generated files exist
    $goFiles = @(
        "gen/go/sentinel/v1/sentinel.pb.go",
        "gen/go/sentinel/v1/sentinelv1connect/sentinel.connect.go"
    )
    foreach ($file in $goFiles) {
        $exists = Test-Path $file
        Write-TestResult "Generated: $file" $exists
    }
    
    # Test 7: Rust generated files exist
    $rustFile = "gen/rust/sentinel/v1/sentinel.v1.rs"
    $rustExists = Test-Path $rustFile
    Write-TestResult "Generated: $rustFile" $rustExists
    
    # Test 8: Proto contains required messages
    if ($protoExists) {
        $protoContent = Get-Content "proto/sentinel/v1/sentinel.proto" -Raw
        $hasHeartbeat = $protoContent -match "rpc Heartbeat"
        $hasCommand = $protoContent -match "enum Command"
        $hasMetrics = $protoContent -match "message MetricsSummary"
        Write-TestResult "Proto defines Heartbeat RPC" $hasHeartbeat
        Write-TestResult "Proto defines Command enum" $hasCommand
        Write-TestResult "Proto defines MetricsSummary" $hasMetrics
    }
}

# ============================================================
# PHASE 1.2: Go Backend Tests
# ============================================================
function Test-Phase1_2 {
    Write-TestHeader "Phase 1.2: Go Backend & Auth Middleware"
    
    # Test 1: Go module exists
    $goMod = Test-Path "backend/go.mod"
    if (-not $goMod) {
        Write-TestSkipped "Go module exists" "backend/go.mod not created yet"
        Write-TestSkipped "Go dependencies" "Requires go.mod"
        Write-TestSkipped "Go tests pass" "Requires implementation"
        return
    }
    Write-TestResult "Go module exists" $goMod
    
    # Test 2: Required dependencies in go.mod
    $goModContent = Get-Content "backend/go.mod" -Raw
    $hasConnect = $goModContent -match "connectrpc.com/connect"
    $hasSqlite = $goModContent -match "modernc.org/sqlite"
    Write-TestResult "Has ConnectRPC dependency" $hasConnect
    Write-TestResult "Has SQLite dependency" $hasSqlite
    
    # Test 3: Run Go tests
    try {
        Push-Location "backend"
        $testResult = go test ./... 2>&1
        $testPassed = $LASTEXITCODE -eq 0
        Write-TestResult "Go tests pass" $testPassed ($testResult | Out-String)
        Pop-Location
    } catch {
        Pop-Location
        Write-TestResult "Go tests pass" $false $_.Exception.Message
    }
}

# ============================================================
# PHASE 1.3: Rust Agent Tests
# ============================================================
function Test-Phase1_3 {
    Write-TestHeader "Phase 1.3: Rust Agent Skeleton"
    
    # Test 1: Cargo.toml exists
    $cargoToml = Test-Path "agent/Cargo.toml"
    if (-not $cargoToml) {
        Write-TestSkipped "Cargo.toml exists" "agent/Cargo.toml not created yet"
        Write-TestSkipped "Rust dependencies" "Requires Cargo.toml"
        Write-TestSkipped "Rust tests pass" "Requires implementation"
        return
    }
    Write-TestResult "Cargo.toml exists" $cargoToml
    
    # Test 2: Required dependencies
    $cargoContent = Get-Content "agent/Cargo.toml" -Raw
    $hasTokio = $cargoContent -match "tokio"
    $hasSerde = $cargoContent -match "serde"
    $hasBackoff = $cargoContent -match "backoff"
    Write-TestResult "Has tokio dependency" $hasTokio
    Write-TestResult "Has serde dependency" $hasSerde
    Write-TestResult "Has backoff dependency" $hasBackoff
    
    # Test 3: Run Rust tests
    try {
        Push-Location "agent"
        $testResult = cargo test 2>&1
        $testPassed = $LASTEXITCODE -eq 0
        Write-TestResult "Rust tests pass" $testPassed ($testResult | Out-String)
        Pop-Location
    } catch {
        Pop-Location
        Write-TestResult "Rust tests pass" $false $_.Exception.Message
    }
}

# ============================================================
# PHASE 2.1: eBPF TC Classifier Tests
# ============================================================
function Test-Phase2_1 {
    Write-TestHeader "Phase 2.1: eBPF TC Classifier"
    
    # Check if eBPF crate exists
    $ebpfCrate = Test-Path "agent/sennet-ebpf/Cargo.toml"
    if (-not $ebpfCrate) {
        Write-TestSkipped "eBPF crate exists" "sennet-ebpf not scaffolded yet"
        Write-TestSkipped "eBPF compiles" "Requires eBPF crate"
        return
    }
    Write-TestResult "eBPF crate exists" $ebpfCrate
    
    # Note: eBPF compilation requires Linux with bpf-linker
    if ($IsWindows -or $env:OS -match "Windows") {
        Write-TestSkipped "eBPF compiles" "eBPF compilation requires Linux"
        Write-TestSkipped "eBPF loads" "eBPF loading requires Linux kernel 5.15+"
    }
}

# ============================================================
# PHASE 2.2: Interface Discovery Tests
# ============================================================
function Test-Phase2_2 {
    Write-TestHeader "Phase 2.2: Interface Auto-Discovery"
    
    # This will be tested as part of the Rust agent
    $hasInterfaceCode = Test-Path "agent/src/interface.rs"
    if (-not $hasInterfaceCode) {
        Write-TestSkipped "Interface module exists" "agent/src/interface.rs not created yet"
        return
    }
    Write-TestResult "Interface module exists" $hasInterfaceCode
}

# ============================================================
# PHASE 3.1: Build Artifacts Tests
# ============================================================
function Test-Phase3_1 {
    Write-TestHeader "Phase 3.1: Cross-Compile & Checksum"
    
    # Check for GitHub Actions workflow
    $workflow = Test-Path ".github/workflows/release.yml"
    if (-not $workflow) {
        Write-TestSkipped "Release workflow exists" ".github/workflows/release.yml not created yet"
        return
    }
    Write-TestResult "Release workflow exists" $workflow
    
    # Validate workflow YAML
    $workflowContent = Get-Content ".github/workflows/release.yml" -Raw
    $hasX86 = $workflowContent -match "x86_64-unknown-linux-musl"
    $hasArm = $workflowContent -match "aarch64-unknown-linux-musl"
    $hasSha256 = $workflowContent -match "sha256sum"
    Write-TestResult "Workflow targets x86_64" $hasX86
    Write-TestResult "Workflow targets aarch64" $hasArm
    Write-TestResult "Workflow generates checksums" $hasSha256
}

# ============================================================
# PHASE 3.2: Installer Tests
# ============================================================
function Test-Phase3_2 {
    Write-TestHeader "Phase 3.2: Self-Updater & Installer"
    
    # Check for install script
    $installScript = Test-Path "install.sh"
    if (-not $installScript) {
        Write-TestSkipped "Install script exists" "install.sh not created yet"
        return
    }
    Write-TestResult "Install script exists" $installScript
    
    # Validate install script content
    $scriptContent = Get-Content "install.sh" -Raw
    $hasArchDetect = $scriptContent -match "uname -m" -or $scriptContent -match "arch"
    $hasSha256Check = $scriptContent -match "sha256" -or $scriptContent -match "checksum"
    $hasSystemd = $scriptContent -match "systemctl" -or $scriptContent -match "systemd"
    Write-TestResult "Script detects architecture" $hasArchDetect
    Write-TestResult "Script verifies checksum" $hasSha256Check
    Write-TestResult "Script creates systemd service" $hasSystemd
}

# ============================================================
# PHASE 4.1: Grafana Dashboard Tests
# ============================================================
function Test-Phase4_1 {
    Write-TestHeader "Phase 4.1: Grafana Dashboard as Code"
    
    $dashboard = Test-Path "dashboards/overview.json"
    if (-not $dashboard) {
        Write-TestSkipped "Dashboard JSON exists" "dashboards/overview.json not created yet"
        return
    }
    Write-TestResult "Dashboard JSON exists" $dashboard
    
    # Validate JSON structure
    try {
        $json = Get-Content "dashboards/overview.json" -Raw | ConvertFrom-Json
        $hasPanels = $null -ne $json.panels
        Write-TestResult "Dashboard has panels" $hasPanels
    } catch {
        Write-TestResult "Dashboard JSON valid" $false $_.Exception.Message
    }
}

# ============================================================
# PHASE 4.2: Documentation Tests
# ============================================================
function Test-Phase4_2 {
    Write-TestHeader "Phase 4.2: Documentation Site"
    
    $mkdocs = Test-Path "mkdocs.yml"
    if (-not $mkdocs) {
        Write-TestSkipped "MkDocs config exists" "mkdocs.yml not created yet"
        return
    }
    Write-TestResult "MkDocs config exists" $mkdocs
    
    # Check for required docs
    $requiredDocs = @("docs/install.md", "docs/config_reference.md")
    foreach ($doc in $requiredDocs) {
        $exists = Test-Path $doc
        Write-TestResult "Doc exists: $doc" $exists
    }
}

# ============================================================
# MAIN EXECUTION
# ============================================================

# Change to project root
$projectRoot = Split-Path -Parent $PSScriptRoot
if (Test-Path $projectRoot) {
    Push-Location $projectRoot
}

Write-Host "`nSennet Test Runner" -ForegroundColor Magenta
Write-Host "==================`n" -ForegroundColor Magenta

# Determine which tests to run
$testsToRun = @()

if ($All) {
    $testsToRun = @("1.1", "1.2", "1.3", "2.1", "2.2", "3.1", "3.2", "4.1", "4.2")
} elseif ($Phase -eq "1") {
    $testsToRun = @("1.1", "1.2", "1.3")
} elseif ($Phase -eq "2") {
    $testsToRun = @("2.1", "2.2")
} elseif ($Phase -eq "3") {
    $testsToRun = @("3.1", "3.2")
} elseif ($Phase -eq "4") {
    $testsToRun = @("4.1", "4.2")
} elseif ($Phase) {
    $testsToRun = @($Phase)
} else {
    Write-Host "Usage: .\run_tests.ps1 -Phase <1.1|1.2|1.3|2.1|2.2|3.1|3.2|4.1|4.2|1|2|3|4> [-All]" -ForegroundColor Yellow
    exit 1
}

# Run tests
foreach ($test in $testsToRun) {
    switch ($test) {
        "1.1" { Test-Phase1_1 }
        "1.2" { Test-Phase1_2 }
        "1.3" { Test-Phase1_3 }
        "2.1" { Test-Phase2_1 }
        "2.2" { Test-Phase2_2 }
        "3.1" { Test-Phase3_1 }
        "3.2" { Test-Phase3_2 }
        "4.1" { Test-Phase4_1 }
        "4.2" { Test-Phase4_2 }
    }
}

# Summary
Write-Host "`n$('=' * 60)" -ForegroundColor Cyan
Write-Host " TEST SUMMARY" -ForegroundColor Cyan
Write-Host "$('=' * 60)" -ForegroundColor Cyan
Write-Host "  Passed:  $script:TestsPassed" -ForegroundColor Green
Write-Host "  Failed:  $script:TestsFailed" -ForegroundColor $(if ($script:TestsFailed -gt 0) { "Red" } else { "Green" })
Write-Host "  Skipped: $script:TestsSkipped" -ForegroundColor Yellow
Write-Host ""

if (Test-Path $projectRoot) {
    Pop-Location
}

# Exit with appropriate code
if ($script:TestsFailed -gt 0) {
    exit 1
} else {
    exit 0
}
