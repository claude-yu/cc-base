param(
    [string]$RunId = ""
)

$ErrorActionPreference = "Continue"

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

# cc-connect uses GetACP()=936(GBK) to decode command stdout
[Console]::OutputEncoding = [System.Text.Encoding]::GetEncoding(936)

$ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$RunsRoot = Join-Path $ControllerRoot "runs"

if ([string]::IsNullOrWhiteSpace($RunId)) {
    $latest = Get-ChildItem -LiteralPath $RunsRoot -Directory |
        Where-Object { $_.Name -like "*plan-review*" } |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if ($null -eq $latest) {
        Write-Output "No plan-review run found."
        exit 1
    }
    $RunId = $latest.Name
}

$RunDir = Join-Path $RunsRoot $RunId
if (-not (Test-Path -LiteralPath $RunDir)) {
    Write-Output "Run not found: $RunId"
    exit 1
}

$summaryPath = Join-Path $RunDir "summary.md"
$ccExitPath = Join-Path $RunDir "cc-plan.exitcode.txt"
$reviewExitPath = Join-Path $RunDir "codex-review.exitcode.txt"
$reviewPath = Join-Path $RunDir "codex-review.md"

if (-not (Test-Path -LiteralPath $summaryPath)) {
    Write-Output "Run ID: $RunId"
    Write-Output "Status: RUNNING"
    $files = Get-ChildItem -LiteralPath $RunDir | Select-Object -ExpandProperty Name
    Write-Output ("Files: " + ($files -join ", "))
    Write-Output "Check later with: /review-status $RunId"
    exit 0
}

$ccExit = if (Test-Path -LiteralPath $ccExitPath) { (Get-Content -LiteralPath $ccExitPath -Raw).Trim() } else { "missing" }
$reviewExit = if (Test-Path -LiteralPath $reviewExitPath) { (Get-Content -LiteralPath $reviewExitPath -Raw).Trim() } else { "missing" }
$review = if (Test-Path -LiteralPath $reviewPath) { Get-Content -LiteralPath $reviewPath -Raw } else { "" }

$verdict = "UNKNOWN"
if ($review -match "(?i)\bAPPROVE\b") { $verdict = "APPROVE" }
if ($review -match "(?i)\bREVISE\b") { $verdict = "REVISE" }
if ($review -match "(?i)\bBLOCK\b") { $verdict = "BLOCK" }

Write-ChatObservation -EventType "command_start" -CommandName "查看审查" -Detail "RunId=$RunId"

Write-Output "Run ID: $RunId"
Write-Output "Status: DONE"
Write-Output "CC exit: $ccExit"
Write-Output "Codex exit: $reviewExit"
Write-Output "Codex verdict: $verdict"
Write-Output "Directory: $RunDir"
Write-Output ""
Write-Output "Codex review preview:"
Write-Output (($review -split "`r?`n" | Select-Object -First 30) -join "`n")
