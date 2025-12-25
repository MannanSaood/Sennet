# Phase 4.2: Documentation Site Tests
# Validates MkDocs configuration and required documentation files

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
Write-Host " Phase 4.2: Documentation Site Tests" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

# Test 1: mkdocs.yml exists
$mkdocsPath = Join-Path $ProjectRoot "mkdocs.yml"
if (-not (Test-Path $mkdocsPath)) {
    Write-Skip "mkdocs.yml" "Not created yet"
    Write-Host "`nRun 'pip install mkdocs-material && mkdocs new .' to enable these tests" -ForegroundColor Yellow
    exit 0
}
Write-TestResult "mkdocs.yml exists" $true

# Read mkdocs config
$mkdocsContent = Get-Content $mkdocsPath -Raw

# Test 2: Uses Material theme
$hasMaterial = $mkdocsContent -match "material"
Write-TestResult "Uses Material theme" $hasMaterial

# Test 3: Has site name
$hasSiteName = $mkdocsContent -match "site_name:"
Write-TestResult "Has site_name configured" $hasSiteName

# Test 4: docs directory exists
$docsDir = Join-Path $ProjectRoot "docs"
$docsExists = Test-Path $docsDir
Write-TestResult "docs/ directory exists" $docsExists

if (-not $docsExists) {
    Write-Host "`nCreate docs/ directory with required files" -ForegroundColor Yellow
    exit 0
}

# Test 5: Required documentation files
Write-Host "`n--- Required Documents ---" -ForegroundColor Cyan

$requiredDocs = @(
    @{ Name = "install.md"; Desc = "Installation guide" },
    @{ Name = "config_reference.md"; Desc = "Configuration reference" },
    @{ Name = "index.md"; Desc = "Home page" }
)

foreach ($doc in $requiredDocs) {
    $docPath = Join-Path $docsDir $doc.Name
    $exists = Test-Path $docPath
    if ($exists) {
        Write-TestResult "$($doc.Name) - $($doc.Desc)" $true
        
        # Check content is not empty/placeholder
        $content = Get-Content $docPath -Raw -ErrorAction SilentlyContinue
        if ($content -and $content.Length -gt 50) {
            Write-Host "         Content length: $($content.Length) chars" -ForegroundColor Gray
        } else {
            Write-Host "         [WARN] File may be placeholder (< 50 chars)" -ForegroundColor Yellow
        }
    } else {
        Write-Skip "$($doc.Name)" "Not created yet"
    }
}

# Test 6: install.md contains one-liner command
$installPath = Join-Path $docsDir "install.md"
if (Test-Path $installPath) {
    $installContent = Get-Content $installPath -Raw
    $hasOneLiner = $installContent -match "curl.*\|.*sh" -or $installContent -match "curl.*bash"
    Write-TestResult "install.md has one-liner command" $hasOneLiner
} else {
    Write-Skip "One-liner in install.md" "File not found"
}

# Test 7: config_reference.md documents YAML schema
$configRefPath = Join-Path $docsDir "config_reference.md"
if (Test-Path $configRefPath) {
    $configContent = Get-Content $configRefPath -Raw
    $hasApiKey = $configContent -match "api_key"
    $hasServerUrl = $configContent -match "server_url"
    $hasYamlExample = $configContent -match "```yaml" -or $configContent -match "```yml"
    Write-TestResult "config_reference.md documents api_key" $hasApiKey
    Write-TestResult "config_reference.md documents server_url" $hasServerUrl
    Write-TestResult "config_reference.md has YAML examples" $hasYamlExample
} else {
    Write-Skip "Config documentation" "File not found"
}

# Test 8: MkDocs can build (if installed)
Write-Host "`n--- Build Test ---" -ForegroundColor Cyan
try {
    $mkdocsVersion = mkdocs --version 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-TestResult "MkDocs installed" $true
        
        Push-Location $ProjectRoot
        $buildResult = mkdocs build --strict 2>&1
        $buildPassed = $LASTEXITCODE -eq 0
        Write-TestResult "MkDocs build (strict mode)" $buildPassed
        Pop-Location
    } else {
        Write-Skip "MkDocs build" "MkDocs not installed (pip install mkdocs-material)"
    }
} catch {
    Write-Skip "MkDocs build" "MkDocs not installed"
}

# Test 9: GitHub Actions docs workflow
$docsWorkflow = Join-Path $ProjectRoot ".github/workflows/docs.yml"
if (Test-Path $docsWorkflow) {
    Write-TestResult "Docs deployment workflow exists" $true
    
    $workflowContent = Get-Content $docsWorkflow -Raw
    $hasPages = $workflowContent -match "pages" -or $workflowContent -match "gh-pages"
    Write-TestResult "Workflow deploys to GitHub Pages" $hasPages
} else {
    Write-Skip "Docs workflow" ".github/workflows/docs.yml not found"
}

Write-Host "`n========================================"
Write-Host " Documentation Tests Complete"
Write-Host "========================================`n"
