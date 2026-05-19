param(
    [switch]$WithQQ,
    [switch]$CleanSessions
)

$ConfigDir = "$env:USERPROFILE\.cc-connect"
$ConfigPath = "$ConfigDir\config.toml"
$SessionDir = "$ConfigDir\sessions"
$SourceConfig = Join-Path (Split-Path -Parent $PSCommandPath) "config.toml"

Write-Host "=== cc-connect Launcher ===" -ForegroundColor Cyan

if (Test-Path -LiteralPath $SourceConfig) {
    if (-not (Test-Path $ConfigDir)) { New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null }
    Copy-Item -LiteralPath $SourceConfig -Destination $ConfigPath -Force
    Write-Host "[OK] Config synced from source" -ForegroundColor Green
} elseif (-not (Test-Path $ConfigPath)) {
    Write-Host "[FAIL] Config not found: $ConfigPath" -ForegroundColor Red; exit 1
}
Write-Host "[OK] Config: $ConfigPath" -ForegroundColor Green

$apiKey = [Environment]::GetEnvironmentVariable("OPENAI_API_KEY", "Process")
if (-not $apiKey) { $apiKey = [Environment]::GetEnvironmentVariable("OPENAI_API_KEY", "User") }
if (-not $apiKey) { $apiKey = [Environment]::GetEnvironmentVariable("OPENAI_API_KEY", "Machine") }
if (-not $apiKey) {
    Write-Host "[WARN] OPENAI_API_KEY env var not set; continuing because providers may be configured in config.toml or via CLI auth." -ForegroundColor Yellow
} else {
    Write-Host "[OK] OPENAI_API_KEY env var present" -ForegroundColor Green
}

$ccBin = Get-Command cc-connect -ErrorAction SilentlyContinue
if (-not $ccBin) { Write-Host "[FAIL] cc-connect not found" -ForegroundColor Red; exit 1 }
Write-Host "[OK] cc-connect" -ForegroundColor Green

if ($CleanSessions -and (Test-Path $SessionDir)) {
    Get-ChildItem "$SessionDir\*.json" | Remove-Item -Force
    Write-Host "[OK] Sessions cleared" -ForegroundColor Yellow
}

if ($WithQQ) {
    try { docker start napcat 2>&1 | Out-Null; Write-Host "[OK] NapCat" -ForegroundColor Green }
    catch { Write-Host "[WARN] NapCat unavailable" -ForegroundColor Yellow }
}

try { Get-Process cc-connect -ErrorAction Stop | Stop-Process -Force; Start-Sleep 1 } catch {}

if (-not $env:CC_MODEL) {
    $env:CC_MODEL = "claude-opus-4-6"
    Write-Host "[OK] CC_MODEL = $env:CC_MODEL" -ForegroundColor Green
} else {
    Write-Host "[OK] CC_MODEL = $env:CC_MODEL (from env)" -ForegroundColor Green
}

Write-Host ""
Write-Host "[START] Config: $ConfigPath" -ForegroundColor Cyan
Write-Host "  WeChat: cc + codex | /new(reset) /bind(relay)" -ForegroundColor Gray
Write-Host ""

& cc-connect -config $ConfigPath
