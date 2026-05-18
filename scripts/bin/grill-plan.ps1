param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ArgsRest
)

$ErrorActionPreference = "Stop"

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

[Console]::OutputEncoding = [System.Text.Encoding]::GetEncoding(936)

$ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)

$Context = ""
if ($null -ne $ArgsRest -and $ArgsRest.Count -gt 0) {
    $Context = ($ArgsRest | ForEach-Object { $_.ToString() }) -join " "
}

$RunId = (Get-Date -Format "yyyyMMdd-HHmmss") + "-grill"
$RunDir = Join-Path $ControllerRoot "runs\$RunId"
New-Item -ItemType Directory -Force -Path $RunDir | Out-Null

$latestPlan = ""
$latestRunDir = Get-ChildItem -LiteralPath (Join-Path $ControllerRoot "runs") -Directory |
    Where-Object { $_.Name -like "*plan-review*" } |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 1

if ($null -ne $latestRunDir) {
    $planPath = Join-Path $latestRunDir.FullName "cc-plan.md"
    $reviewPath = Join-Path $latestRunDir.FullName "codex-review.md"
    if (Test-Path -LiteralPath $planPath) {
        $latestPlan = Get-Content -LiteralPath $planPath -Raw
    }
    if (Test-Path -LiteralPath $reviewPath) {
        $latestPlan += "`n`n## Codex Review`n`n" + (Get-Content -LiteralPath $reviewPath -Raw)
    }
}

$claudeCmd = Resolve-ClaudeCmd

$systemPrompt = @"
You are in grill-me mode. Your job is to stress-test a plan by asking tough questions.

Rules:
1. Ask ONE question at a time. Wait for the answer before asking the next.
2. For each question, provide a recommended answer the user can accept or modify.
3. If the question can be answered by reading code/files, do it yourself first.
4. Don't move to the next branch until the current question is resolved.

Question tree:
1. What is the success criteria for this plan?
2. Are all required inputs available and verified?
3. Are outputs verifiable?
4. Which steps are irreversible?
5. Which steps need explicit user confirmation?
6. What is the fallback if the primary approach fails?
7. Which parameters should be saved as instincts for future reuse?
8. Which workflows should evolve into skills?

Be direct. Be skeptical. Challenge assumptions.
"@

$prompt = "质询以下计划"
if (-not [string]::IsNullOrWhiteSpace($Context)) {
    $prompt += "，用户补充：$Context"
}
if (-not [string]::IsNullOrWhiteSpace($latestPlan)) {
    $prompt += "`n`n最近的计划：`n$latestPlan"
} else {
    $prompt += "`n`n（未找到最近的计划审查记录，请用户描述要质询的计划）"
}

$promptFile = Join-Path $RunDir "grill-prompt.md"
$prompt | Set-Content -Encoding UTF8 -LiteralPath $promptFile -NoNewline

Write-ChatObservation -EventType "command_start" -CommandName "质询计划" -Detail $Context

$WorkDir = Resolve-RequiredWorkDir -ParamValue "" -EnvVarName "CC_WORK_DIR"

Push-Location -LiteralPath $WorkDir
try {
    Get-Content -LiteralPath $promptFile -Raw -Encoding UTF8 | & $claudeCmd -p --system-prompt $systemPrompt --allowedTools Read,Glob,Grep,LS --disallowedTools Bash,Edit,Write --permission-mode default --output-format text --no-session-persistence
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
