param(
    [string]$Task,

    [string]$TaskFile,

    [string]$WorkDir = "",

    [string]$RunId = ""
)

$ErrorActionPreference = "Continue"

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

if ([string]::IsNullOrWhiteSpace($WorkDir)) {
    $WorkDir = Resolve-RequiredWorkDir -ParamValue $WorkDir -EnvVarName "CC_WORK_DIR"
}

if (-not [string]::IsNullOrWhiteSpace($TaskFile)) {
    if (-not (Test-Path -LiteralPath $TaskFile)) {
        throw "TaskFile does not exist: $TaskFile"
    }
    $Task = Get-Content -LiteralPath $TaskFile -Raw
}

if ([string]::IsNullOrWhiteSpace($Task) -and -not [string]::IsNullOrWhiteSpace($env:CONTROLLER_TASK)) {
    $Task = $env:CONTROLLER_TASK
}

if ([string]::IsNullOrWhiteSpace($Task)) {
    throw "Task, TaskFile, or CONTROLLER_TASK is required."
}

$ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
if ([string]::IsNullOrWhiteSpace($RunId)) {
    $RunId = (Get-Date -Format "yyyyMMdd-HHmmss") + "-plan-review"
}

$RunDir = Join-Path $ControllerRoot "runs\$RunId"
New-Item -ItemType Directory -Force -Path $RunDir | Out-Null

function Write-Utf8File {
    param(
        [string]$Path,
        [string]$Content
    )
    $Content | Set-Content -Encoding UTF8 -LiteralPath $Path
}

function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Script
    )

    $output = & $Script 2>&1
    $exitCode = $LASTEXITCODE
    if ($null -eq $exitCode) { $exitCode = 0 }
    $text = ($output | ForEach-Object { $_.ToString() }) -join "`n"

    Write-Utf8File -Path (Join-Path $RunDir "$Name.md") -Content $text
    Write-Utf8File -Path (Join-Path $RunDir "$Name.exitcode.txt") -Content "$exitCode"

    return [PSCustomObject]@{
        Name = $Name
        ExitCode = $exitCode
        Text = $text
    }
}

function Remove-CodexCliNoise {
    param([string]$Text)

    $lines = $Text -split "`r?`n"
    $kept = New-Object System.Collections.Generic.List[string]
    $skipMetadata = $false

    foreach ($line in $lines) {
        if ($line -match "^\s*codex\.cmd\s*:") { continue }
        if ($line -match "^\s*At .*call-codex-review\.ps1:") { continue }
        if ($line -match "^\s*\+\s*.*") { continue }
        if ($line -match "^\s*\+ CategoryInfo\s*:") { continue }
        if ($line -match "^\s*\+ FullyQualifiedErrorId\s*:") { continue }
        if ($line -match "^OpenAI Codex v") { $skipMetadata = $true; continue }
        if ($line -match "^-{4,}$") { continue }
        if ($skipMetadata) {
            if ($line -match "^codex\s*$") { $skipMetadata = $false; continue }
            if ($line -match "^(workdir|model|provider|approval|sandbox|reasoning|session id):") { continue }
            if ($line -match "^user\s*$") { continue }
            continue
        }
        if ($line -match "^tokens used") { continue }
        $kept.Add($line)
    }

    $clean = ([string]::Join("`n", $kept)).Trim()
    if ([string]::IsNullOrWhiteSpace($clean)) { return $Text }
    return $clean
}

Write-Utf8File -Path (Join-Path $RunDir "request.md") -Content $Task

$planPrompt = @"
Create a compact plan for the task below.
Hard requirements:
1. Planning only. Do not execute.
2. Do not modify files.
3. Do not read large trajectory or simulation data files.
4. Keep the output within 30 lines.
5. For each step, state objective, inputs, outputs, risks, and validation.
6. Clearly mark steps that require explicit user confirmation.

Task:
$Task
"@

$planPromptPath = Join-Path $RunDir "cc-plan-prompt.md"
Write-Utf8File -Path $planPromptPath -Content $planPrompt

$ccStep = Invoke-Step -Name "cc-plan" -Script {
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $ControllerRoot "bin\call-cc-readonly.ps1") -MessageFile $planPromptPath -WorkDir $WorkDir
}

$reviewPrompt = @"
Review the Claude Code plan below.
Do not execute anything.
Decide whether it is ready for user confirmation.

Original task:
$Task

Claude Code plan:
$($ccStep.Text)
"@

$reviewPromptPath = Join-Path $RunDir "codex-review-prompt.md"
Write-Utf8File -Path $reviewPromptPath -Content $reviewPrompt

$codexStep = Invoke-Step -Name "codex-review" -Script {
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $ControllerRoot "bin\call-codex-review.ps1") -MessageFile $reviewPromptPath
}

$codexReviewRawPath = Join-Path $RunDir "codex-review.raw.md"
Write-Utf8File -Path $codexReviewRawPath -Content $codexStep.Text
$codexStep.Text = Remove-CodexCliNoise -Text $codexStep.Text
Write-Utf8File -Path (Join-Path $RunDir "codex-review.md") -Content $codexStep.Text

$reviewStatus = if ($codexStep.ExitCode -eq 0) { "review-completed" } else { "review-blocked" }
$recommendation = if (($ccStep.ExitCode -eq 0) -and ($codexStep.ExitCode -eq 0)) { "WAIT_FOR_USER_APPROVAL" } else { "FIX_BLOCKERS_BEFORE_EXECUTION" }

$summary = @"
# Plan Review Summary

Run ID: $RunId
WorkDir: $WorkDir

## Status

- CC plan exit code: $($ccStep.ExitCode)
- Codex review exit code: $($codexStep.ExitCode)
- Review status: $reviewStatus
- Recommendation: $recommendation

## CC Plan

$($ccStep.Text)

## Codex Review

$($codexStep.Text)

## Next Action

No real task execution was performed.
Read summary.md, cc-plan.md, and codex-review.md.
Execution must wait for explicit user approval.

Run directory:

$RunDir
"@

Write-Utf8File -Path (Join-Path $RunDir "summary.md") -Content $summary

$verdict = @"
# Verdict

Status: $reviewStatus
Recommendation: $recommendation

No execution was performed.

Files:
- request.md
- cc-plan.md
- codex-review.md
- summary.md
"@

Write-Utf8File -Path (Join-Path $RunDir "verdict.md") -Content $verdict

Write-Output $summary
exit 0
