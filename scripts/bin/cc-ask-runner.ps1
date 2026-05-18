param(
    [Parameter(Mandatory = $true)]
    [string]$RunId,

    [string]$ControllerRoot = ""
)

$ErrorActionPreference = "Continue"

[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

if ([string]::IsNullOrWhiteSpace($ControllerRoot)) {
    $ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
}

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

# ── Proxy fallback for Start-Process context ────────────────────
# Start-Process does NOT inherit parent shell env. Read registry if missing.
if ([string]::IsNullOrWhiteSpace($env:HTTP_PROXY)) {
    $v = [Environment]::GetEnvironmentVariable("HTTP_PROXY", "User")
    if ($v) { $env:HTTP_PROXY = $v }
}
if ([string]::IsNullOrWhiteSpace($env:HTTPS_PROXY)) {
    $v = [Environment]::GetEnvironmentVariable("HTTPS_PROXY", "User")
    if ($v) { $env:HTTPS_PROXY = $v }
}

$RunDir = Join-Path $ControllerRoot "runs\$RunId"
$logStderr = Join-Path $RunDir "background-err.log"
$chatLogWriter = Join-Path (Split-Path -Parent $PSCommandPath) "chat-log-writer.ps1"
$autoCallbackDisabled = Join-Path $ControllerRoot "auto-callback.disabled"
$callbackMsg = Join-Path $RunDir "callback-msg.md"

function Write-Heartbeat {
    param([string]$Stage, [string]$Message)
    if (-not (Test-Path -LiteralPath $chatLogWriter)) { return }
    try {
        $meta = @{ stage = $Stage }
        & $chatLogWriter -Channel wechat -Direction out -Lifecycle running -RecordType heartbeat -Command "问cc" -RunId $RunId -Text $Message -Meta $meta | Out-Null
    } catch {
        "heartbeat failed: $_" | Add-Content -LiteralPath $logStderr -Encoding UTF8
    }
}

function Send-Callback {
    param([string]$Message)
    if (Test-Path -LiteralPath $autoCallbackDisabled) { return }
    try {
        [System.IO.File]::WriteAllText($callbackMsg, $Message, [System.Text.UTF8Encoding]::new($false))
        $Message | cc-connect send --stdin -p cc 2>&1 | Out-Null
    } catch {
        "cc-connect send failed: $_" | Add-Content -LiteralPath $logStderr -Encoding UTF8
    }
}

function Write-Diag {
    param([string]$Path, [string]$Content)
    try { $Content | Set-Content -Encoding UTF8 -LiteralPath $Path } catch {}
}

# ── Stage: prepare ──────────────────────────────────────────────
Write-Heartbeat -Stage "prepare" -Message "Preparing CC ask"

$questionPath = Join-Path $RunDir "incoming-question.md"
$question = Get-Content -LiteralPath $questionPath -Raw -Encoding UTF8

# env diagnostics (proxies redacted for chat-log but full for file)
$diag = @(
    "HTTP_PROXY=$($env:HTTP_PROXY -replace '.', '*')",
    "HTTPS_PROXY=$($env:HTTPS_PROXY -replace '.', '*')",
    "CLAUDE_PROXY=$($env:CLAUDE_PROXY -replace '.', '*')"
) -join "`n"
Write-Diag -Path (Join-Path $RunDir "env-diag.txt") -Content $diag

# ── Stage: call Claude CLI directly ────────────────────────────
Write-Heartbeat -Stage "cc_running" -Message "CC is processing"

$claudeCmd = Resolve-ClaudeCmd
$workDir = $env:CC_WORK_DIR
if (-not $workDir) { $workDir = [Environment]::GetEnvironmentVariable("CC_WORK_DIR", "User") }
if (-not $workDir) { $workDir = [Environment]::GetEnvironmentVariable("CC_WORK_DIR", "Machine") }

$systemPrompt = "You are Claude Code acting as an advice-only assistant. Do not modify files. Do not run shell commands. Do not spawn subagents. Read files if needed, but return concise, structured output. Answer the user's question directly."

$tmpFile = Join-Path $env:TEMP "cc-ask-$([guid]::NewGuid().ToString('N')).txt"
$question | Set-Content -Encoding UTF8 -LiteralPath $tmpFile -NoNewline

Push-Location $workDir
try {
    $answer = Get-Content $tmpFile -Raw -Encoding UTF8 | & $claudeCmd -p --system-prompt $systemPrompt --output-format text --no-session-persistence 2>&1
    $ccExit = $LASTEXITCODE
} catch {
    $ccExit = 1
    $answer = "Error: $_"
} finally {
    Pop-Location
    Remove-Item $tmpFile -ErrorAction SilentlyContinue
}

$answerText = if ($answer -is [array]) { ($answer | ForEach-Object { $_.ToString() }) -join "`n" } else { $answer.ToString() }
$cleanAnswer = $answerText.Trim()

# ── Write output files ──────────────────────────────────────────
$cleanAnswer | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "cc-answer.raw.md")
$cleanAnswer | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "cc-answer.md")
"$ccExit" | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "cc-answer.exitcode.txt")

# ── Stage: summarize ───────────────────────────────────────────
Write-Heartbeat -Stage "summarize" -Message "Summarizing CC answer"

if ($ccExit -eq 0) {
    $cleanAnswer | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "summary.md")

    if (Test-Path $chatLogWriter) {
        try { & $chatLogWriter -Channel wechat -Direction out -Lifecycle completed -RecordType message -Command "问cc" -RunId $RunId -Text $cleanAnswer | Out-Null } catch {}
    }

    Send-Callback -Message @"
[CC] 已回复 (RunId: $RunId)
$cleanAnswer
"@
} else {
    $errText = if ($answerText) { $answerText } else { "(no output)" }
    $errText | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "summary.md")

    if (Test-Path $chatLogWriter) {
        try { & $chatLogWriter -Channel wechat -Direction out -Lifecycle failed -RecordType message -Command "问cc" -RunId $RunId -Text "exit=$ccExit" | Out-Null } catch {}
    }

    Send-Callback -Message "[CC] 调用失败 (RunId: $RunId). 检查 Claude CLI / CLAUDE_PROXY / /修复controller"
}

"0" | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "runner.exitcode.txt")
exit 0
