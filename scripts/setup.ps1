# setup.ps1 - cc-base first-time configuration wizard
# Usage: powershell -NoProfile -ExecutionPolicy Bypass -File setup.ps1
# Generates config.toml from template with user-selected platform/backend/credentials.

param(
    [string]$TemplateDir = $PSScriptRoot
)

$ErrorActionPreference = 'Stop'

# ── Helpers ──────────────────────────────────────────────────

function Write-Banner  { param([string]$Text) Write-Host "`n=== $Text ===" -ForegroundColor Cyan }
function Write-Ok      { param([string]$Text) Write-Host "  [OK] $Text"   -ForegroundColor Green }
function Write-Warn    { param([string]$Text) Write-Host "  [!]  $Text"   -ForegroundColor Yellow }
function Write-Err     { param([string]$Text) Write-Host "  [X]  $Text"   -ForegroundColor Red }
function Write-Info    { param([string]$Text) Write-Host "  $Text"        -ForegroundColor Gray }

function Read-Choice {
    param([string]$Prompt, [int]$Min = 1, [int]$Max)
    while ($true) {
        $raw = Read-Host $Prompt
        $val = 0
        if ([int]::TryParse($raw, [ref]$val) -and $val -ge $Min -and $val -le $Max) {
            return $val
        }
        Write-Err "Please enter a number between $Min and $Max"
    }
}

function Read-NonEmpty {
    param([string]$Prompt, [string]$Default = '')
    while ($true) {
        $suffix = if ($Default) { " [$Default]" } else { '' }
        $raw = Read-Host "$Prompt$suffix"
        if ([string]::IsNullOrWhiteSpace($raw) -and $Default) { return $Default }
        if (-not [string]::IsNullOrWhiteSpace($raw)) { return $raw.Trim() }
        Write-Err "Input cannot be empty"
    }
}

function Read-Optional {
    param([string]$Prompt, [string]$Default = '')
    $suffix = if ($Default) { " [$Default]" } else { " (Enter to skip)" }
    $raw = Read-Host "$Prompt$suffix"
    if ([string]::IsNullOrWhiteSpace($raw)) { return $Default }
    return $raw.Trim()
}

function Test-Url([string]$u) {
    return ($u -match '^https?://')
}

function Test-NumericOnly([string]$s) {
    return ($s -match '^\d+$')
}

function Detect-Tool {
    param([string]$Name, [string]$Cmd)
    try {
        $out = & $Name --version 2>$null
        if ($LASTEXITCODE -eq 0 -or $out) {
            return $out
        }
    } catch {}
    return $null
}

# ── Step 0: Environment detection ────────────────────────────

Write-Banner "cc-base 首次配置向导"
Write-Host ""
Write-Host "Detecting environment..." -ForegroundColor White

$detected = @{
    claude  = $false
    codex   = $false
    connect = $false
    napcat  = $false
}

$claudeVer = Detect-Tool 'claude'
if ($claudeVer) { $detected.claude = $true; Write-Ok "claude CLI: $claudeVer" }
else { Write-Warn "claude CLI not found" }

$codexVer = Detect-Tool 'codex'
if ($codexVer) { $detected.codex = $true; Write-Ok "codex CLI: $codexVer" }
else { Write-Warn "codex CLI not found" }

$connectVer = Detect-Tool 'cc-connect'
if ($connectVer) { $detected.connect = $true; Write-Ok "cc-connect: $connectVer" }
else { Write-Warn "cc-connect not found" }

try {
    $containers = docker ps --format '{{.Names}}' 2>$null
    if ($containers -match 'napcat') {
        $detected.napcat = $true
        Write-Ok "NapCat container detected (QQ available)"
    } else {
        Write-Warn "NapCat container not running"
    }
} catch {
    Write-Warn "Docker not available / NapCat not detected"
}

Write-Host ""

# ── Migration detection ──────────────────────────────────────

$existingConfig = Join-Path $env:USERPROFILE '.cc-connect\config.toml'
if (Test-Path $existingConfig) {
    Write-Warn "Existing config found: $existingConfig"
    Write-Host "  [1] Overwrite" -ForegroundColor White
    Write-Host "  [2] Keep existing and exit" -ForegroundColor White
    Write-Host "  [3] Backup and overwrite" -ForegroundColor White
    $migration = Read-Choice "Choose" -Max 3
    if ($migration -eq 2) {
        Write-Host "Keeping existing config. Exiting." -ForegroundColor Green
        exit 0
    }
    if ($migration -eq 3) {
        $backupPath = "$existingConfig.bak.$(Get-Date -Format 'yyyyMMdd-HHmmss')"
        Copy-Item $existingConfig $backupPath
        Write-Ok "Backup saved to $backupPath"
    }
}

# ── Step 1: Platform selection ───────────────────────────────

Write-Banner "Step 1/5 - Platform selection"
Write-Host "  [1] WeChat (微信)" -ForegroundColor White
Write-Host "  [2] QQ (NapCat/OneBot)" -ForegroundColor White
Write-Host "  [3] WeChat + QQ" -ForegroundColor White
$platformChoice = Read-Choice "Select platform" -Max 3

$useWeChat = ($platformChoice -eq 1 -or $platformChoice -eq 3)
$useQQ     = ($platformChoice -eq 2 -or $platformChoice -eq 3)

# ── Step 2: CC role backend ──────────────────────────────────

Write-Banner "Step 2/5 - CC role backend"
$ccLabel = if ($detected.claude) { " (推荐, detected)" } else { "" }
Write-Host "  [1] native Claude Code$ccLabel" -ForegroundColor White
Write-Host "  [2] Anthropic API (future)" -ForegroundColor White
Write-Host "  [3] OpenAI API (future)" -ForegroundColor White
Write-Host "  [4] DeepSeek API (future)" -ForegroundColor White
Write-Host "  [5] GLM API (future)" -ForegroundColor White
$ccChoice = Read-Choice "Select CC backend" -Max 5

$ccBackendMap = @{ 1='native_claude'; 2='anthropic_api'; 3='openai'; 4='deepseek'; 5='glm' }
$ccBackend = $ccBackendMap[$ccChoice]

if ($ccChoice -ge 2) {
    Write-Warn "Backend '$ccBackend' for CC role will be configured but is NOT yet implemented."
    Write-Info "You can set it now and switch to a working backend later."
}

# ── Step 3: Codex role backend ───────────────────────────────

Write-Banner "Step 3/5 - Codex role backend"
$codexLabel = if ($detected.codex) { " (推荐, detected)" } else { "" }
Write-Host "  [1] native Codex CLI$codexLabel" -ForegroundColor White
Write-Host "  [2] OpenAI API" -ForegroundColor White
Write-Host "  [3] DeepSeek API" -ForegroundColor White
Write-Host "  [4] GLM (智谱) API" -ForegroundColor White
$codexChoice = Read-Choice "Select Codex backend" -Max 4

$codexBackendMap = @{ 1='native_codex'; 2='openai'; 3='deepseek'; 4='glm' }
$codexBackend = $codexBackendMap[$codexChoice]

# ── Step 4: Collect credentials ──────────────────────────────

Write-Banner "Step 4/5 - Credentials"

# -- Project root & work_dir --
$defaultRoot = (Get-Item $TemplateDir).Parent.Parent.FullName
$projectRoot = Read-NonEmpty "Project root path (where controller/ lives)" -Default $defaultRoot
$workDir = Read-NonEmpty "CC agent work_dir (default project execution directory)" -Default $projectRoot
$executeWorkDir = Read-Optional "CC_EXECUTE_WORK_DIR (sandbox for execute mode, Enter to use work_dir)" -Default $workDir

Write-Host "`n  --- Model ---" -ForegroundColor Cyan
$ccModel = Read-Optional "CC_MODEL (Claude model for CC role)" -Default "claude-opus-4-6"

# -- WeChat credentials --
$wx = @{}
if ($useWeChat) {
    Write-Host "`n  --- WeChat ---" -ForegroundColor Cyan
    $wx.token      = Read-NonEmpty   "WeChat token"
    while ($true) {
        $wx.base_url = Read-NonEmpty "WeChat base_url (https://XXX.weixin.qq.com)"
        if (Test-Url $wx.base_url) { break }
        Write-Err "URL must start with http:// or https://"
    }
    $wx.account_id = Read-NonEmpty   "WeChat account_id"
    $wx.admin_id   = Read-NonEmpty   "Admin WeChat OpenID"
}

# -- QQ credentials --
$qq = @{}
if ($useQQ) {
    Write-Host "`n  --- QQ (NapCat) ---" -ForegroundColor Cyan
    while ($true) {
        $qq.bot_id = Read-NonEmpty "Bot QQ number"
        if (Test-NumericOnly $qq.bot_id) { break }
        Write-Err "QQ number must be numeric"
    }
    while ($true) {
        $qq.admin_id = Read-NonEmpty "Admin QQ number"
        if (Test-NumericOnly $qq.admin_id) { break }
        Write-Err "QQ number must be numeric"
    }
    $qq.ws_url = Read-Optional "NapCat WS URL" -Default "ws://127.0.0.1:3001"
    $qq.token  = Read-Optional "NapCat WS token" -Default ""
}

# -- CC API credentials (if non-native) --
$ccApi = @{}
if ($ccChoice -ge 2) {
    Write-Host "`n  --- CC role API ---" -ForegroundColor Cyan
    $ccApiDefaults = @{
        2 = @{ base = 'https://api.anthropic.com/v1'; model = 'claude-opus-4-6'; prefix = '' }
        3 = @{ base = 'https://api.openai.com/v1';    model = 'gpt-4o';          prefix = 'sk-proj-' }
        4 = @{ base = 'https://api.deepseek.com/v1';  model = 'deepseek-chat';   prefix = '' }
        5 = @{ base = 'https://open.bigmodel.cn/api/paas/v4'; model = 'glm-4'; prefix = '' }
    }
    $defaults = $ccApiDefaults[$ccChoice]
    $ccApi.base  = Read-Optional "CC API base URL" -Default $defaults.base
    $ccApi.model = Read-Optional "CC model name"   -Default $defaults.model
    $ccApi.key   = Read-Optional "CC API key"
    if (-not $ccApi.key) {
        Write-Warn "No API key provided. Set CC_CC_API_KEY env var before running."
    } elseif ($ccChoice -eq 3 -and $ccApi.key -and -not $ccApi.key.StartsWith('sk-proj-')) {
        Write-Warn "OpenAI keys usually start with sk-proj-*. Proceeding anyway."
    }
}

# -- Codex API credentials (if non-native) --
$codexApi = @{}
if ($codexChoice -ge 2) {
    Write-Host "`n  --- Codex role API ---" -ForegroundColor Cyan
    $codexApiDefaults = @{
        2 = @{ base = 'https://api.openai.com/v1';           model = 'gpt-4o';        prefix = 'sk-proj-' }
        3 = @{ base = 'https://api.deepseek.com/v1';         model = 'deepseek-chat';  prefix = '' }
        4 = @{ base = 'https://open.bigmodel.cn/api/paas/v4'; model = 'glm-4';         prefix = '' }
    }
    $defaults = $codexApiDefaults[$codexChoice]
    $codexApi.base  = Read-Optional "Codex API base URL" -Default $defaults.base
    $codexApi.model = Read-Optional "Codex model name"   -Default $defaults.model
    $codexApi.key   = Read-Optional "Codex API key"
    if (-not $codexApi.key) {
        Write-Warn "No API key provided. Set CC_CODEX_API_KEY env var before running."
    } elseif ($codexChoice -eq 2 -and $codexApi.key -and -not $codexApi.key.StartsWith('sk-proj-')) {
        Write-Warn "OpenAI keys usually start with sk-proj-*. Proceeding anyway."
    }
}

# ── Step 5: Generate config.toml ─────────────────────────────

Write-Banner "Step 5/5 - Generating config.toml"

$templatePath = Join-Path $TemplateDir 'config.toml.template'
if (-not (Test-Path $templatePath)) {
    Write-Err "Template not found: $templatePath"
    exit 1
}

$template = [System.IO.File]::ReadAllText($templatePath, [System.Text.UTF8Encoding]::new($false))

# Normalize project root for TOML (backslash-escaped)
$rootEscaped = $projectRoot -replace '\\', '\\\\'
$workDirEscaped = $workDir -replace '\\', '\\\\'

# Determine admin_from (WeChat OpenID takes priority, then QQ)
$adminFrom = if ($useWeChat) { $wx.admin_id } elseif ($useQQ) { $qq.admin_id } else { 'YOUR_ADMIN_ID' }

# Basic placeholder replacement
$config = $template
$config = $config -replace 'YOUR_PROJECT_ROOT', $rootEscaped
$config = $config -replace 'YOUR_WORK_DIR', $workDirEscaped
$config = $config -replace 'YOUR_ADMIN_ID', $adminFrom

# WeChat placeholders
if ($useWeChat) {
    $config = $config -replace 'YOUR_WECHAT_TOKEN',      $wx.token
    $config = $config -replace 'YOUR_CORP_SUBDOMAIN\.weixin\.qq\.com', ($wx.base_url -replace '^https://', '')
    $config = $config -replace 'YOUR_WECHAT_ACCOUNT_ID', $wx.account_id
    $config = $config -replace 'YOUR_WECHAT_OPENID',     $wx.admin_id
} else {
    # Comment out entire WeChat platform block if not selected
    $config = $config -replace '(?m)^(\[\[projects\.platforms\]\]\s*\ntype = "weixin")', '# $1'
}

# QQ platform: uncomment if selected
if ($useQQ) {
    # Uncomment the QQ platform block under cc project
    $qqBlock = @"

[[projects.platforms]]
type = "qq"
[projects.platforms.options]
ws_url = "$($qq.ws_url)"
token = "$($qq.token)"
allow_from = "$($qq.admin_id)"
admin_from = "$($qq.admin_id)"
"@
    # Replace the commented QQ block with an active one
    $config = $config -replace '(?ms)# Optional: QQ platform via NapCat \(see docs/qq-setup\.md\)\s*\n(#\s*\[\[projects\.platforms\]\].*?\n#\s*admin_from = "YOUR_QQ_ID")', $qqBlock.TrimStart()
}

# Codex native project: uncomment if QQ is selected and user wants it
# (Keep commented for now per template — Codex role goes through cc-controller)

# Append cc_base metadata block as comments
$metaBlock = @"

# === cc-base role/backend selection (metadata, not parsed by cc-connect) ===
# [cc_base.platform]
# provider = "$(if ($platformChoice -eq 3) {'wechat+qq'} elseif ($useQQ) {'qq'} else {'wechat'})"
#
# [cc_base.cc_backend]
# type = "$ccBackend"
#
# [cc_base.codex_backend]
# type = "$codexBackend"
"@
$config = $config + "`n" + $metaBlock + "`n"

# Write output
$outputPath = Join-Path (Get-Location) 'config.toml'
[System.IO.File]::WriteAllText($outputPath, $config, [System.Text.UTF8Encoding]::new($false))
Write-Ok "Config written to: $outputPath"

# ── Generate .env.cc-base.local ─────────────────────────────

$envFilePath = Join-Path (Get-Location) '.env.cc-base.local'
$envContent = @()
$envContent += "# cc-base environment variables"
$envContent += "# Source this file or add to your shell profile"
$envContent += "# Generated by setup.ps1 on $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
$envContent += ""
$envContent += "CC_CONTROLLER_DIR=$projectRoot\controller"
$envContent += "CC_WORK_DIR=$workDir"
$envContent += "CC_EXECUTE_WORK_DIR=$executeWorkDir"
$envContent += "CC_MODEL=$ccModel"
$envContent += "CC_BACKEND=$ccBackend"
$envContent += "CC_CODEX_BACKEND=$codexBackend"

if ($ccChoice -ge 2) {
    $envContent += ""
    $envContent += "# CC role API"
    if ($ccApi.base)  { $envContent += "CC_CC_API_BASE=$($ccApi.base)" }
    if ($ccApi.key)   { $envContent += "CC_CC_API_KEY=$($ccApi.key)" }
    else              { $envContent += "CC_CC_API_KEY=<set your key>" }
    if ($ccApi.model) { $envContent += "CC_CC_MODEL=$($ccApi.model)" }
}

if ($codexChoice -ge 2) {
    $envContent += ""
    $envContent += "# Codex role API"
    if ($codexApi.base)  { $envContent += "CC_CODEX_API_BASE=$($codexApi.base)" }
    if ($codexApi.key)   { $envContent += "CC_CODEX_API_KEY=$($codexApi.key)" }
    else                 { $envContent += "CC_CODEX_API_KEY=<set your key>" }
    if ($codexApi.model) { $envContent += "CC_CODEX_MODEL=$($codexApi.model)" }
}

$envContent += ""
$envContent += "# PowerShell quick-set:"
$envContent += "# `$env:CC_CONTROLLER_DIR = `"$projectRoot\controller`""
$envContent += "# `$env:CC_WORK_DIR = `"$workDir`""
$envContent += "# `$env:CC_EXECUTE_WORK_DIR = `"$executeWorkDir`""
$envContent += "# `$env:CC_MODEL = `"$ccModel`""
$envContent += "# `$env:CC_BACKEND = `"$ccBackend`""
$envContent += "# `$env:CC_CODEX_BACKEND = `"$codexBackend`""

[System.IO.File]::WriteAllText($envFilePath, ($envContent -join "`n"), [System.Text.UTF8Encoding]::new($false))
Write-Ok "Env file written to: $envFilePath"

# ── Print env vars ───────────────────────────────────────────

Write-Host ""
Write-Banner "Environment Variables"
Write-Info "Set these in your shell profile or system environment:"
Write-Host ""

$envLines = @()
$envLines += "CC_CONTROLLER_DIR=$projectRoot\controller"
$envLines += "CC_WORK_DIR=$workDir"
$envLines += "CC_EXECUTE_WORK_DIR=$executeWorkDir"
$envLines += "CC_MODEL=$ccModel"
$envLines += "CC_CC_BACKEND=$ccBackend"
if ($ccChoice -ge 2) {
    if ($ccApi.base)  { $envLines += "CC_CC_API_BASE=$($ccApi.base)" }
    if ($ccApi.model) { $envLines += "CC_CC_MODEL=$($ccApi.model)" }
    if ($ccApi.key)   { $envLines += "CC_CC_API_KEY=$($ccApi.key)" }
    else              { $envLines += "CC_CC_API_KEY=<set your key>" }
}

$envLines += "CC_CODEX_BACKEND=$codexBackend"
if ($codexChoice -ge 2) {
    if ($codexApi.base)  { $envLines += "CC_CODEX_API_BASE=$($codexApi.base)" }
    if ($codexApi.model) { $envLines += "CC_CODEX_MODEL=$($codexApi.model)" }
    if ($codexApi.key)   { $envLines += "CC_CODEX_API_KEY=$($codexApi.key)" }
    else                 { $envLines += "CC_CODEX_API_KEY=<set your key>" }
}

foreach ($line in $envLines) {
    # Mask API keys in display
    if ($line -match 'API_KEY=(.{4})') {
        $masked = $line -replace '(API_KEY=.{4}).*', '$1****'
        Write-Host "  $masked" -ForegroundColor Yellow
    } else {
        Write-Host "  $line" -ForegroundColor Yellow
    }
}

# PowerShell quick-set
Write-Host ""
Write-Info "PowerShell quick-set (paste into profile):"
foreach ($line in $envLines) {
    $parts = $line -split '=', 2
    Write-Host "  `$env:$($parts[0]) = `"$($parts[1])`"" -ForegroundColor DarkCyan
}

# ── Security warning ─────────────────────────────────────────

Write-Host ""
Write-Warn "config.toml contains secrets (tokens, API keys). Do NOT commit to git."
Write-Info "Add to .gitignore: config.toml"
Write-Host ""

# ── Deploy option ────────────────────────────────────────────

Write-Host ""
$doDeploy = Read-Optional "Deploy config.toml to ~/.cc-connect/? (y/n)" -Default "y"
if ($doDeploy -eq 'y' -or $doDeploy -eq 'Y') {
    $deployDir = Join-Path $env:USERPROFILE '.cc-connect'
    New-Item -ItemType Directory -Path $deployDir -Force | Out-Null
    Copy-Item $outputPath (Join-Path $deployDir 'config.toml') -Force
    Write-Ok "Config deployed to $deployDir\config.toml"
} else {
    $deployTarget = Join-Path $env:USERPROFILE '.cc-connect\config.toml'
    Write-Info "To deploy later: Copy config.toml to $deployTarget"
    Write-Info "Or run start.ps1 which auto-syncs from project source."
}
Write-Host ""
Write-Ok "Setup complete!"
