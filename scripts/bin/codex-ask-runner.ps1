param(
    [Parameter(Mandatory = $true)]
    [string]$RunId,

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
$chatLogWriter = Join-Path (Split-Path -Parent $PSCommandPath) "chat-log-writer.ps1"
$autoCallbackDisabled = Join-Path $ControllerRoot "auto-callback.disabled"
$callbackMsg = Join-Path $RunDir "callback-msg.md"

function Write-Heartbeat {
    param([string]$Stage, [string]$Message)
    if (-not (Test-Path -LiteralPath $chatLogWriter)) { return }
    try {
        $meta = @{ stage = $Stage }
        & $chatLogWriter -Channel wechat -Direction out -Lifecycle running -RecordType heartbeat -Command "问codex" -RunId $RunId -Text $Message -Meta $meta | Out-Null
    } catch {
        "heartbeat failed: $_" | Add-Content -LiteralPath $logStderr -Encoding UTF8
    }
}

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

# ── Stage: prepare ──────────────────────────────────────────────
Write-Heartbeat -Stage "prepare" -Message "Preparing codex ask"

$questionPath = Join-Path $RunDir "incoming-question.md"
$question = Get-Content -LiteralPath $questionPath -Raw -Encoding UTF8

$systemPrompt = @"
Reply in the same language as the user's question. If the question contains Chinese, use Simplified Chinese.

You are Codex acting as an independent technical advisor.
Do not read files. Do not run commands. Do not modify anything.
Answer the user's question directly based on your knowledge.

At the end of your answer, include a "建议下一步" section:

## 建议下一步
- P1 ...
- P2 ...
- P3 ...

If no action is needed, write:
- 无需后续操作。
"@

$fullPrompt = "$systemPrompt`n`n---`n$question"
$fullPrompt | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "question.md")

# ── Stage: codex_running ────────────────────────────────────────
Write-Heartbeat -Stage "codex_running" -Message "Codex is processing"

$codexCmd = Resolve-CodexCmd
Set-CodexProxy

$answerPath = Join-Path $RunDir "codex-answer.md"

try {
    $answer = $fullPrompt | & $codexCmd exec --sandbox none --skip-git-repo-check --ephemeral 2>&1
    $codexExit = $LASTEXITCODE
} catch {
    $codexExit = 1
    $answer = "Error: $_"
}

$answer | Set-Content -Encoding UTF8 -LiteralPath $answerPath
"$codexExit" | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "codex-answer.exitcode.txt")

# ── Stage: summarize + callback ─────────────────────────────────
Write-Heartbeat -Stage "summarize" -Message "Summarizing codex answer"

if ($codexExit -eq 0) {
    $answer | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "summary.md")

    if (Test-Path -LiteralPath $chatLogWriter) {
        try {
            & $chatLogWriter -Channel wechat -Direction out -Lifecycle completed -RecordType message -Command "问codex" -RunId $RunId -Text $answer | Out-Null
        } catch {}
    }

    $callbackBody = @"
Codex 已回复。
Run ID: $RunId

---
$answer
"@
    Send-Callback -Message $callbackBody
} else {
    $errText = if ($answer) { $answer } else { "(no output)" }
    $errText | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "summary.md")

    if (Test-Path -LiteralPath $chatLogWriter) {
        try {
            & $chatLogWriter -Channel wechat -Direction out -Lifecycle failed -RecordType message -Command "问codex" -RunId $RunId -Text "exit=$codexExit" | Out-Null
        } catch {}
    }

    $callbackBody = @"
❌ Codex 调用失败

RunId:
$RunId

建议：
1. /codex结果 $RunId
2. 检查 CODEX_PROXY / OPENAI_API_KEY
3. /修复controller <错误>
"@
    Send-Callback -Message $callbackBody
}

"0" | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "runner.exitcode.txt")
exit 0
