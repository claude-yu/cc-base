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
New-Item -ItemType Directory -Force -Path $RunDir | Out-Null
$logStdout = Join-Path $RunDir "background.log"
$logStderr = Join-Path $RunDir "background-err.log"
$callbackMsg = Join-Path $RunDir "callback-msg.md"
$autoCallbackFlag = Join-Path $ControllerRoot "auto-callback.flag"
$autoCallbackDisabled = Join-Path $ControllerRoot "auto-callback.disabled"
$chatLogWriter = Join-Path (Split-Path -Parent $PSCommandPath) "chat-log-writer.ps1"

function Send-Callback {
    param([string]$Message)
    if (Test-Path -LiteralPath $autoCallbackDisabled) { return }
    try {
        $Message | Set-Content -Encoding UTF8 -LiteralPath $callbackMsg
        Get-Content -LiteralPath $callbackMsg -Raw -Encoding UTF8 | cc-connect send --stdin -p cc 2>&1 | Out-Null
    } catch {
        "cc-connect send failed: $_" | Add-Content -LiteralPath $logStderr -Encoding UTF8
    }
}

function Get-PlanReviewVerdict {
    $reviewPath = Join-Path $RunDir "codex-review.md"
    if (-not (Test-Path -LiteralPath $reviewPath)) { return "UNKNOWN" }
    $review = Get-Content -LiteralPath $reviewPath -Raw -Encoding UTF8
    if ($review -match "(?i)\bBLOCK\b") { return "BLOCK" }
    if ($review -match "(?i)\bREVISE\b") { return "REVISE" }
    if ($review -match "(?i)\bAPPROVE\b") { return "APPROVE" }
    return "UNKNOWN"
}

function Get-CompletionCallbackMessage {
    param([string]$Summary)
    $verdict = Get-PlanReviewVerdict
    $nextAction = switch ($verdict) {
        "APPROVE" {
@"
建议下一步（按优先级）：
P1 需要执行就发送：/批准执行 $RunId
P2 想先看细节就发送：/查看审查 $RunId
P3 如果计划仍不放心，发送：/质询计划 $RunId
"@
        }
        "REVISE" {
@"
建议下一步（按优先级）：
P1 发送：/质询计划 $RunId，逐条补齐风险、输入、输出和确认点
P2 修改任务描述后重新发送：/计划审查 <修改后的任务>
P3 只有你明确接受剩余风险时，再考虑人工批准执行
"@
        }
        "BLOCK" {
@"
建议下一步（按优先级）：
P1 不要执行当前计划
P2 发送：/查看审查 $RunId，先定位 BLOCK 原因
P3 修正阻塞项后重新发送：/计划审查 <修正后的任务>
"@
        }
        default {
@"
建议下一步（按优先级）：
P1 发送：/查看审查 $RunId，先确认审查细节
P2 根据结果决定是 /批准执行、/质询计划，还是重新 /计划审查
P3 如果不确定，直接说“现在该做什么”，我会按当前 run 状态建议
"@
        }
    }

    @"
计划审查已完成。
Run ID: $RunId
Codex verdict: $verdict

$nextAction

---

$Summary
"@
}

function Write-PlanReviewHeartbeat {
    param(
        [string]$Stage,
        [string]$Message
    )
    if (-not (Test-Path -LiteralPath $chatLogWriter)) { return }
    try {
        $meta = @{
            stage = $Stage
        }
        & $chatLogWriter -Channel wechat -Direction out -Lifecycle running -RecordType heartbeat -Command "计划审查" -RunId $RunId -Text $Message -Meta $meta | Out-Null
    } catch {
        "chat heartbeat failed: $_" | Add-Content -LiteralPath $logStderr -Encoding UTF8
    }
}

function Get-PlanReviewStageText {
    param([string]$Stage)
    switch ($Stage) {
        "prepare" { return "准备计划审查任务" }
        "cc_plan" { return "Claude Code 正在生成计划" }
        "codex_review" { return "Codex 正在审查计划" }
        "summarize" { return "正在汇总审查结果" }
        default { return $Stage }
    }
}

function Get-PlanReviewStage {
    if (Test-Path -LiteralPath (Join-Path $RunDir "codex-review-prompt.md")) { return "codex_review" }
    if (Test-Path -LiteralPath (Join-Path $RunDir "cc-plan-prompt.md")) { return "cc_plan" }
    if (Test-Path -LiteralPath (Join-Path $RunDir "request.md")) { return "cc_plan" }
    return "prepare"
}

function Format-ProcessArgument {
    param([string]$Value)
    if ($Value -match '[\s"]') {
        return '"' + ($Value -replace '"', '\"') + '"'
    }
    return $Value
}

$scriptPath = Join-Path $ControllerRoot "bin\plan-review-confirm.ps1"

$processArgs = @(
    "-NoProfile",
    "-ExecutionPolicy",
    "Bypass",
    "-File",
    $scriptPath,
    "-TaskFile",
    $TaskFile,
    "-RunId",
    $RunId
) | ForEach-Object { Format-ProcessArgument $_ }

$commandLine = "powershell " + ($processArgs -join " ") + " > " + (Format-ProcessArgument $logStdout) + " 2> " + (Format-ProcessArgument $logStderr)
$processInfo = [System.Diagnostics.ProcessStartInfo]::new()
$processInfo.FileName = "cmd.exe"
$processInfo.Arguments = "/d /c " + (Format-ProcessArgument $commandLine)
$processInfo.WorkingDirectory = $ControllerRoot
$processInfo.UseShellExecute = $false
$processInfo.CreateNoWindow = $true

$process = [System.Diagnostics.Process]::new()
$process.StartInfo = $processInfo
$started = $process.Start()
if (-not $started) {
    throw "Failed to start plan-review process."
}

$lastStage = ""
$lastHeartbeat = [DateTime]::MinValue
while (-not $process.HasExited) {
    $stage = Get-PlanReviewStage
    $now = Get-Date
    $stageChanged = $stage -ne $lastStage
    if ($stageChanged -or (($now - $lastHeartbeat).TotalSeconds -ge 30)) {
        Write-PlanReviewHeartbeat -Stage $stage -Message "plan-review stage=$stage still running"
        if ($stageChanged -and $stage -ne "prepare") {
            Send-Callback -Message "计划审查仍在进行：$(Get-PlanReviewStageText -Stage $stage)。`nRun ID: $RunId"
        }
        $lastStage = $stage
        $lastHeartbeat = $now
    }
    Start-Sleep -Seconds 2
    $process.Refresh()
}

$process.WaitForExit()
$exitCode = $process.ExitCode
if ($null -eq $exitCode) { $exitCode = 0 }

"$exitCode" | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "runner.exitcode.txt")

$summaryPath = Join-Path $RunDir "summary.md"

if ($exitCode -eq 0 -and (Test-Path -LiteralPath $summaryPath)) {
    Write-ChatObservation -EventType "command_end" -CommandName "计划审查" -Detail "RunId=$RunId exit=0"
    $summary = Get-Content -LiteralPath $summaryPath -Raw -Encoding UTF8
    Send-Callback -Message (Get-CompletionCallbackMessage -Summary $summary)
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
