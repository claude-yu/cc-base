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
$question = ""
if ($null -ne $ArgsRest -and $ArgsRest.Count -gt 0) {
    $question = ($ArgsRest | ForEach-Object { $_.ToString() }) -join " "
}

if ([string]::IsNullOrWhiteSpace($question)) {
    Write-Output "用法: /问codex <你的问题>"
    Write-Output "示例: /问codex 你觉得上述方案如何"
    exit 1
}

$chatLogWriter = Join-Path (Split-Path -Parent $PSCommandPath) "chat-log-writer.ps1"

$RunId = (Get-Date -Format "yyyyMMdd-HHmmss") + "-codex-ask"
$RunDir = Join-Path $ControllerRoot "runs\$RunId"
New-Item -ItemType Directory -Force -Path $RunDir | Out-Null

$question | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "incoming-question.md")

if (Test-Path -LiteralPath $chatLogWriter) {
    try {
        & $chatLogWriter -Channel wechat -Direction in -Lifecycle started -RecordType message -Command "问codex" -RunId $RunId -Text $question | Out-Null
    } catch {}
}

$runnerPath = Join-Path $ControllerRoot "bin\codex-ask-runner.ps1"
Start-Process -FilePath "powershell" -ArgumentList @(
    "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $runnerPath,
    "-RunId", $RunId, "-ControllerRoot", $ControllerRoot
) -WorkingDirectory $ControllerRoot -WindowStyle Hidden

Write-Output "已开始询问 Codex，预计需要 1-3 分钟。"
Write-Output "Run ID: $RunId"
Write-Output "完成后会自动发送结果。"
Write-Output "可手动查看: /查看codex $RunId"
