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
        [System.IO.File]::WriteAllText($callbackMsg, $Message, [System.Text.UTF8Encoding]::new($false))
        $Message | cc-connect send --stdin -p cc 2>&1 | Out-Null
    } catch {
        "cc-connect send failed: $_" | Add-Content -LiteralPath $logStderr -Encoding UTF8
    }
}

function ConvertTo-Text {
    param($Value)
    if ($null -eq $Value) { return "" }
    if ($Value -is [array]) {
        return (($Value | ForEach-Object { $_.ToString() }) -join "`n")
    }
    return $Value.ToString()
}

function Remove-CodexCliNoise {
    param([string]$Text)

    $lines = $Text -split "`r?`n"
    $kept = New-Object System.Collections.Generic.List[string]
    $skipMetadata = $false

    foreach ($line in $lines) {
        if ($line -match "^Reading prompt from stdin") { continue }
        if ($line -match "^OpenAI Codex v") { $skipMetadata = $true; continue }
        if ($line -match "^-{4,}$") { continue }
        if ($line -match "^System\.Management\.Automation\.RemoteException$") { continue }
        if ($line -match "^tokens used") { continue }
        if ($line -match "^\s*\d{1,3}(,\d{3})+\s*$") { continue }
        if ($line -match "^success:|^成功:") { continue }

        if ($skipMetadata) {
            if ($line -match "^codex\s*$") { $skipMetadata = $false; continue }
            if ($line -match "^(workdir|model|provider|approval|sandbox|reasoning|session id):") { continue }
            if ($line -match "^user\s*$") { continue }
            continue
        }

        $kept.Add($line)
    }

    $clean = ([string]::Join("`n", $kept)).Trim()
    if ([string]::IsNullOrWhiteSpace($clean)) { return $Text.Trim() }
    $deduped = Remove-ConsecutiveDuplicateAnswer -Text $clean
    if (-not [string]::IsNullOrWhiteSpace($deduped)) { return $deduped }
    return $clean
}

function Remove-ConsecutiveDuplicateAnswer {
    param([string]$Text)

    $normalized = $Text.Trim()
    if ([string]::IsNullOrWhiteSpace($normalized)) { return $normalized }

    $blocks = @($normalized -split "(?:`r?`n){2,}" | ForEach-Object { $_.Trim() } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if (($blocks.Count % 2) -eq 0 -and $blocks.Count -ge 2) {
        $halfBlocks = [int]($blocks.Count / 2)
        $firstBlocks = [string]::Join("`n`n", $blocks[0..($halfBlocks - 1)]).Trim()
        $secondBlocks = [string]::Join("`n`n", $blocks[$halfBlocks..($blocks.Count - 1)]).Trim()
        if ($firstBlocks -eq $secondBlocks) { return $firstBlocks }
    }

    $lines = $normalized -split "`r?`n"
    if (($lines.Count % 2) -ne 0) { return $normalized }

    $half = [int]($lines.Count / 2)
    if ($half -lt 2) { return $normalized }

    $first = [string]::Join("`n", $lines[0..($half - 1)]).Trim()
    $second = [string]::Join("`n", $lines[$half..($lines.Count - 1)]).Trim()

    if ($first -eq $second) { return $first }
    return $normalized
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
if (-not $env:CODEX_PROXY) { $env:CODEX_PROXY = [Environment]::GetEnvironmentVariable("CODEX_PROXY", "User") }
if (-not $env:CODEX_PROXY) { $env:CODEX_PROXY = [Environment]::GetEnvironmentVariable("CODEX_PROXY", "Machine") }
Set-CodexProxy

$answerPath = Join-Path $RunDir "codex-answer.md"

try {
    $answer = $fullPrompt | & $codexCmd exec --sandbox read-only --skip-git-repo-check --ephemeral 2>&1
    $codexExit = $LASTEXITCODE
} catch {
    $codexExit = 1
    $answer = "Error: $_"
}

$answerText = ConvertTo-Text $answer
$cleanAnswer = Remove-CodexCliNoise -Text $answerText

$answerText | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "codex-answer.raw.md")
$cleanAnswer | Set-Content -Encoding UTF8 -LiteralPath $answerPath
"$codexExit" | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "codex-answer.exitcode.txt")

# ── Stage: summarize + callback ─────────────────────────────────
Write-Heartbeat -Stage "summarize" -Message "Summarizing codex answer"

if ($codexExit -eq 0) {
    $cleanAnswer | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "summary.md")

    if (Test-Path -LiteralPath $chatLogWriter) {
        try {
            & $chatLogWriter -Channel wechat -Direction out -Lifecycle completed -RecordType message -Command "问codex" -RunId $RunId -Text $cleanAnswer | Out-Null
        } catch {}
    }

    $callbackBody = @"
Codex 已回复。
Run ID: $RunId

---
$cleanAnswer
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
