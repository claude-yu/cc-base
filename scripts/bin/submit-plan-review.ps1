param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ArgsRest
)

$ErrorActionPreference = "Stop"

[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

$ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$Task = ""
if ($null -ne $ArgsRest -and $ArgsRest.Count -gt 0) {
    $Task = ($ArgsRest | ForEach-Object { $_.ToString() }) -join " "
}

if ([string]::IsNullOrWhiteSpace($Task) -and -not [string]::IsNullOrWhiteSpace($env:CONTROLLER_TASK)) {
    $Task = $env:CONTROLLER_TASK
}

if ([string]::IsNullOrWhiteSpace($Task)) {
    Write-Output "用法: /计划审查 <任务描述>"
    Write-Output "示例: /计划审查 分析蛋白结合位点的RMSD变化"
    exit 1
}

$RunId = (Get-Date -Format "yyyyMMdd-HHmmss") + "-plan-review"
$RunDir = Join-Path $ControllerRoot "runs\$RunId"
New-Item -ItemType Directory -Force -Path $RunDir | Out-Null

$TaskFile = Join-Path $RunDir "incoming-task.md"
$Task | Set-Content -Encoding UTF8 -LiteralPath $TaskFile

Write-ChatObservation -EventType "command_start" -CommandName "计划审查" -Detail $Task

$runnerPath = Join-Path $ControllerRoot "bin\plan-review-runner.ps1"

Start-Process -FilePath "powershell" -ArgumentList @(
    "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $runnerPath,
    "-RunId", $RunId, "-TaskFile", $TaskFile, "-ControllerRoot", $ControllerRoot
) -WorkingDirectory $ControllerRoot -WindowStyle Hidden

Write-Output "已开始计划审查，预计需要 1-3 分钟。"
Write-Output "完成后会自动发送结果。"
Write-Output "也可手动查看: /查看审查 $RunId"
