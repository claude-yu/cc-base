<#
.SYNOPSIS
    One-click install for cc-base — mobile-controlled multi-agent controller.
.DESCRIPTION
    Deploys controller scripts, creates config from template, installs dependencies.
    Based on cc-connect (https://github.com/chenhg5/cc-connect).
.PARAMETER ProjectDir
    Target project root (where controller/ and cc-connect/ will be created).
    Default: current directory.
.PARAMETER WithCodex
    Also configure Codex CLI project (requires CODEX_WORK_DIR).
.PARAMETER WithQQ
    Also configure QQ platform via NapCat.
.EXAMPLE
    .\install.ps1 -ProjectDir "E:\ai\myproject"
    .\install.ps1 -ProjectDir "E:\ai\myproject" -WithCodex -WithQQ
#>

param(
    [string]$ProjectDir = (Get-Location).Path,
    [switch]$WithCodex,
    [switch]$WithQQ
)

$ErrorActionPreference = "Stop"
$nl = [Environment]::NewLine
$host.UI.RawUI.WindowTitle = "cc-base Installer"

# ── Banner ────────────────────────────────────────────────────
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  cc-base v2.3.0 — One-Click Install" -ForegroundColor Cyan
Write-Host "  Based on cc-connect" -ForegroundColor Cyan
Write-Host "  https://github.com/chenhg5/cc-connect" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# ── Step 1: Prerequisites ────────────────────────────────────
Write-Host "[1/5] Checking prerequisites..." -ForegroundColor Yellow

# PowerShell version
if ($PSVersionTable.PSVersion.Major -lt 5) {
    Write-Error "PowerShell 5.1+ required. Current: $($PSVersionTable.PSVersion)"
    exit 1
}
Write-Host "  [OK] PowerShell $($PSVersionTable.PSVersion)"

# Node.js
$nodeVer = node --version 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Error "Node.js not found. Install from https://nodejs.org"
    exit 1
}
Write-Host "  [OK] Node.js $nodeVer"

# cc-connect
$ccConnectVer = cc-connect --version 2>$null
if ($LASTEXITCODE -eq 0) {
    Write-Host "  [OK] cc-connect $ccConnectVer"
} else {
    Write-Host "  [..] Installing cc-connect..."
    npm install -g cc-connect
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Failed to install cc-connect"
        exit 1
    }
    $ccConnectVer = cc-connect --version
    Write-Host "  [OK] cc-connect $ccConnectVer installed"
}

# Codex CLI (optional)
if ($WithCodex) {
    $codexVer = codex --version 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  [OK] Codex CLI $codexVer"
    } else {
        Write-Host "  [..] Codex CLI not found. Install with: npm install -g @openai/codex"
    }
}

Write-Host ""

# ── Step 2: Create directory structure ────────────────────────
Write-Host "[2/5] Creating directory structure..." -ForegroundColor Yellow
$dirs = @(
    (Join-Path $ProjectDir "controller"),
    (Join-Path $ProjectDir "controller\bin"),
    (Join-Path $ProjectDir "controller\runs"),
    (Join-Path $ProjectDir "cc-connect"),
    (Join-Path $ProjectDir "docs")
)
foreach ($d in $dirs) {
    New-Item -ItemType Directory -Force -Path $d | Out-Null
}
Write-Host "  [OK] Directories created under: $ProjectDir"

# ── Step 3: Deploy scripts ─────────────────────────────────────
Write-Host "[3/5] Deploying controller scripts..." -ForegroundColor Yellow

$skillRoot = Split-Path -Parent $PSCommandPath
$binSrc = Join-Path $skillRoot "scripts\bin"
$binDst = Join-Path $ProjectDir "controller\bin"

if (Test-Path $binSrc) {
    Get-ChildItem -Path $binSrc -Filter "*.ps1" | Copy-Item -Destination $binDst -Force
    Write-Host "  [OK] Scripts copied to: $binDst"
} else {
    Write-Warning "  [!!] Scripts source not found: $binSrc"
}

# Rules and docs (optional, for reference)
$rulesSrc = Join-Path $skillRoot "rules"
$rulesDst = Join-Path $ProjectDir "controller\rules"
if (Test-Path $rulesSrc) {
    New-Item -ItemType Directory -Force -Path $rulesDst | Out-Null
    Get-ChildItem -Path $rulesSrc -Filter "*.md" | Copy-Item -Destination $rulesDst -Force
    Write-Host "  [OK] Rules copied to: $rulesDst"
}

$docsSrc = Join-Path $skillRoot "docs"
$docsDst = Join-Path $ProjectDir "controller\docs"
if (Test-Path $docsSrc) {
    New-Item -ItemType Directory -Force -Path $docsDst | Out-Null
    Get-ChildItem -Path $docsSrc -Filter "*.md" | Copy-Item -Destination $docsDst -Force
    Write-Host "  [OK] Docs copied to: $docsDst"
}

# ── Step 4: Create config.toml from template ──────────────────
Write-Host "[4/5] Creating config.toml..." -ForegroundColor Yellow

$templatePath = Join-Path $skillRoot "scripts\config.toml.template"
$configPath = Join-Path $ProjectDir "cc-connect\config.toml"
$startScript = Join-Path $skillRoot "scripts\start.ps1"

if (Test-Path $templatePath) {
    $template = Get-Content $templatePath -Raw -Encoding UTF8
    # Replace placeholders
    $template = $template -replace "YOUR_PROJECT_ROOT", ($ProjectDir -replace "\\", "\\")
    $template = $template -replace "YOUR_ADMIN_ID", "<your-admin-id>"
    $template = $template -replace "YOUR_WECHAT_OPENID", "<your-wechat-openid>"
    $template = $template -replace "YOUR_WECHAT_TOKEN", "<your-wechat-token>"
    $template = $template -replace "YOUR_CORP_SUBDOMAIN", "<your-corp-subdomain>"
    $template = $template -replace "YOUR_WECHAT_ACCOUNT_ID", "<your-wechat-account-id>"
    $template = $template -replace "YOUR_WORK_DIR", (Join-Path $ProjectDir "work")
    $template = $template -replace "YOUR_CODEX_WORK_DIR", (Join-Path $ProjectDir "codex-work")
    $template = $template -replace "YOUR_QQ_ID", "<your-qq-id>"
    $template = $template -replace "YOUR_QQ_TOKEN", "<your-qq-token>"
    # Remove optional sections based on flags
    if (-not $WithCodex) {
        # Remove codex project section (between ## Codex project and next ##)
        $template = $template -replace "(?s)# ═══════════════════════════════════════[\r\n]+# Codex project.*?(?=# ═══════════════════════════════════════|$)", ""
    }
    if (-not $WithQQ) {
        # Uncomment QQ platform section
        $template = $template -replace "# \[\[projects\.platforms\]\]`r?`n# type = ", "[[projects.platforms]]`ntype = "
        $template = $template -replace "# type = ""qq""", 'type = "qq"'
        $template = $template -replace "# \[projects\.platforms\.options\]", "[projects.platforms.options]"
        $template = $template -replace "# ws_url = ", "ws_url = "
        $template = $template -replace "# token = ""YOUR_QQ_TOKEN""", 'token = "<your-qq-token>"'
        $template = $template -replace "# allow_from = ""YOUR_QQ_ID""", 'allow_from = "<your-qq-id>"'
        $template = $template -replace "# admin_from = ""YOUR_QQ_ID""", 'admin_from = "<your-qq-id>"'

        # If WithQQ, remove the commented section markers
        if ($WithQQ) {
            $template = $template -replace "# Optional: QQ platform via NapCat[\r\n]*", ""
            $template = $template -replace "# \[\[projects\.platforms\]\]\s*", "[[projects.platforms]]`n"
        }
    }

    # Write config
    $utf8bom = New-Object System.Text.UTF8Encoding $true
    [System.IO.File]::WriteAllText($configPath, $template, $utf8bom)
    Write-Host "  [OK] Config created: $configPath"
    Write-Host "  [!!] EDIT THIS FILE with your real credentials!" -ForegroundColor Red
} else {
    Write-Warning "  [!!] Template not found: $templatePath"
}

# Copy start.ps1
if (Test-Path $startScript) {
    $startDst = Join-Path $ProjectDir "cc-connect\start.ps1"
    Copy-Item $startScript $startDst -Force
    Write-Host "  [OK] Start script copied to: $startDst"
}

# ── Step 5: Summary ───────────────────────────────────────────
Write-Host ""
Write-Host "[5/5] Install complete!" -ForegroundColor Green
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Summary" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Project root:  $ProjectDir"
Write-Host "  Scripts:       $binDst"
Write-Host "  Config:        $configPath"
Write-Host "  Start script:  $startDst"
Write-Host "  Runs:          $(Join-Path $ProjectDir 'controller\runs')"
Write-Host ""

if (-not ($template -match "<your-")) {
    Write-Host "  [WARN] Config still has placeholder values!" -ForegroundColor Red
    Write-Host "  Edit $configPath and fill in:" -ForegroundColor Red
    Write-Host "    - WeChat token / base_url / account_id" -ForegroundColor Red
    Write-Host "    - WorkDir path" -ForegroundColor Red
    Write-Host "    - Admin/User IDs" -ForegroundColor Red
    Write-Host ""
}

Write-Host "  Next steps:" -ForegroundColor Yellow
Write-Host "  1. Edit config:   $configPath" -ForegroundColor Yellow
Write-Host "  2. Set env vars:  CC_WORK_DIR (required), CODEX_PROXY (optional)" -ForegroundColor Yellow
Write-Host "  3. Start:         powershell -NoProfile -ExecutionPolicy Bypass -File $startDst -CleanSessions" -ForegroundColor Yellow
Write-Host ""
Write-Host "  Attribution:" -ForegroundColor Gray
Write-Host "  cc-base is a skill for cc-connect (https://github.com/chenhg5/cc-connect)" -ForegroundColor Gray
Write-Host "  Repository: https://github.com/claude-yu/cc-base" -ForegroundColor Gray
Write-Host "========================================" -ForegroundColor Cyan
