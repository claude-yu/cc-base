param(
    [string]$RunId = ""
)

$ErrorActionPreference = "Continue"

[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$RunsRoot = Join-Path $ControllerRoot "runs"

if ([string]::IsNullOrWhiteSpace($RunId)) {
    $latest = Get-ChildItem -LiteralPath $RunsRoot -Directory |
        Where-Object { $_.Name -like "*-cc-ask" } |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if ($null -eq $latest) {
        Write-Output "No cc-ask run found."
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
$answerPath = Join-Path $RunDir "cc-answer.md"
$exitCodePath = Join-Path $RunDir "cc-answer.exitcode.txt"
$runnerExitPath = Join-Path $RunDir "runner.exitcode.txt"

function Get-RunningStage {
    if (Test-Path -LiteralPath $summaryPath) { return "summarize" }
    if (Test-Path -LiteralPath $answerPath) { return "cc_running" }
    return "prepare"
}

function Get-StageText {
    param([string]$Stage)
    switch ($Stage) {
        "prepare" { return "准备问题" }
        "cc_running" { return "CC 思考中" }
        "summarize" { return "正在汇总结果" }
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
    exit 0
}

$exitCode = if (Test-Path -LiteralPath $exitCodePath) { (Get-Content -LiteralPath $exitCodePath -Raw).Trim() } else { "missing" }
$runnerExit = if (Test-Path -LiteralPath $runnerExitPath) { (Get-Content -LiteralPath $runnerExitPath -Raw).Trim() } else { "missing" }
$answer = if (Test-Path -LiteralPath $answerPath) { Get-Content -LiteralPath $answerPath -Raw -Encoding UTF8 } else { "" }

if ($exitCode -eq "0") {
    Write-Output "Run ID: $RunId"
    Write-Output "Status: DONE"
    Write-Output "CC exit: $exitCode"
    Write-Output "Directory: $RunDir"
    Write-Output ""
    Write-Output "CC answer:"
    Write-Output (($answer -split "`r?`n" | Select-Object -First 50) -join "`n")
} else {
    Write-Output "Run ID: $RunId"
    Write-Output "Status: FAILED"
    Write-Output "CC exit: $exitCode"
    Write-Output "Runner exit: $runnerExit"
    Write-Output "Directory: $RunDir"
    $bgLog = Join-Path $RunDir "background-err.log"
    if (Test-Path -LiteralPath $bgLog) {
        Write-Output ""
        Write-Output "Error log (last 15 lines):"
        Get-Content -LiteralPath $bgLog -Encoding UTF8 | Select-Object -Last 15
    }
}
