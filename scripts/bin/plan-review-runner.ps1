param(
    [Parameter(Mandatory = $true)]
    [string]$RunId,

    [Parameter(Mandatory = $true)]
    [string]$TaskFile,

    [string]$ControllerRoot = ""
)

$ErrorActionPreference = "Continue"

if ([string]::IsNullOrWhiteSpace($ControllerRoot)) {
    $ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
}

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

$RunDir = Join-Path $ControllerRoot "runs\$RunId"
$logStdout = Join-Path $RunDir "background.log"
$logStderr = Join-Path $RunDir "background-err.log"
$callbackMsg = Join-Path $RunDir "callback-msg.md"
$autoCallbackFlag = Join-Path $ControllerRoot "auto-callback.flag"

function Send-Callback {
    param([string]$Message)
    if (-not (Test-Path -LiteralPath $autoCallbackFlag)) { return }
    try {
        $Message | Set-Content -Encoding UTF8 -LiteralPath $callbackMsg
        Get-Content -LiteralPath $callbackMsg -Raw -Encoding UTF8 | cc-connect send --stdin -p cc 2>&1 | Out-Null
    } catch {
        "cc-connect send failed: $_" | Add-Content -LiteralPath $logStderr -Encoding UTF8
    }
}

$scriptPath = Join-Path $ControllerRoot "bin\plan-review-confirm.ps1"

& powershell -NoProfile -ExecutionPolicy Bypass -File $scriptPath -TaskFile $TaskFile -RunId $RunId > $logStdout 2> $logStderr
$exitCode = $LASTEXITCODE
if ($null -eq $exitCode) { $exitCode = 0 }

"$exitCode" | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "runner.exitcode.txt")

$summaryPath = Join-Path $RunDir "summary.md"

if ($exitCode -eq 0 -and (Test-Path -LiteralPath $summaryPath)) {
    Write-ChatObservation -EventType "command_end" -CommandName "计划审查" -Detail "RunId=$RunId exit=0"
    $summary = Get-Content -LiteralPath $summaryPath -Raw -Encoding UTF8
    Send-Callback -Message $summary
    exit 0
}

$errDetail = ""
if (Test-Path -LiteralPath $logStderr) {
    $errDetail = Get-Content -LiteralPath $logStderr -Raw -Encoding UTF8
}
if ([string]::IsNullOrWhiteSpace($errDetail) -and (Test-Path -LiteralPath $logStdout)) {
    $errDetail = Get-Content -LiteralPath $logStdout -Raw -Encoding UTF8 | Select-Object -Last 30
}

Write-ChatObservation -EventType "error" -CommandName "计划审查" -Detail "RunId=$RunId exit=$exitCode"

if ($env:CC_AUTO_FIX_IN_PROGRESS -eq "1") {
    Send-Callback -Message "计划审查失败 (exit=$exitCode)，自动修复也失败。请手动排查。`nRun ID: $RunId"
    exit $exitCode
}

$env:CC_AUTO_FIX_IN_PROGRESS = "1"
$fixScriptPath = Join-Path $ControllerRoot "bin\fix-controller.ps1"
$fixInput = "计划审查后台任务失败 (exit=$exitCode)。RunId=$RunId`n错误输出:`n$errDetail"

Write-ChatObservation -EventType "command_start" -CommandName "自动修复" -Detail "triggered by 计划审查 $RunId"

$env:CONTROLLER_ERROR = $fixInput
$fixOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $fixScriptPath 2>&1
$fixExit = $LASTEXITCODE
if ($null -eq $fixExit) { $fixExit = 0 }
$fixText = ($fixOutput | ForEach-Object { $_.ToString() }) -join "`n"

$fixText | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "auto-fix-result.md")
"$fixExit" | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "auto-fix.exitcode.txt")

Write-ChatObservation -EventType "command_end" -CommandName "自动修复" -Detail "exit=$fixExit"

$fixMsg = @"
计划审查失败 (exit=$exitCode)，已自动尝试修复 (exit=$fixExit)。

修复结果:
$fixText

请重新发送原命令，或用 /查看审查 $RunId 查看详情。
"@

Send-Callback -Message $fixMsg
exit $exitCode
