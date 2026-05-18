param(
    [string]$RunId = ""
)

$ErrorActionPreference = "Continue"

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

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

function Get-RunningStage {
    if (Test-Path -LiteralPath (Join-Path $RunDir "summary.md")) { return "summarize" }
    if (Test-Path -LiteralPath (Join-Path $RunDir "codex-review-prompt.md")) { return "codex_review" }
    if (Test-Path -LiteralPath (Join-Path $RunDir "cc-plan-prompt.md")) { return "cc_plan" }
    if (Test-Path -LiteralPath (Join-Path $RunDir "request.md")) { return "cc_plan" }
    return "prepare"
}

function Get-StageText {
    param([string]$Stage)
    switch ($Stage) {
        "prepare" { return "准备计划审查任务" }
        "cc_plan" { return "Claude Code 正在生成计划" }
        "codex_review" { return "Codex 正在审查计划" }
        "summarize" { return "正在汇总审查结果" }
        default { return $Stage }
    }
}

if (-not (Test-Path -LiteralPath $summaryPath)) {
    $stage = Get-RunningStage
    Write-Output "Run ID: $RunId"
    Write-Output "Status: RUNNING"
    Write-Output "Stage: $stage"
    Write-Output "Now: $(Get-StageText -Stage $stage)"
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
