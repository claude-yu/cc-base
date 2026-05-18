param(
    [Parameter(Mandatory = $true)]
    [string]$RunId,

    [string]$WorkDir = ""
)

$ErrorActionPreference = "Continue"

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

$ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$WorkDir = Resolve-RequiredWorkDir -ParamValue $WorkDir -EnvVarName "CC_WORK_DIR"
$RunDir = Join-Path $ControllerRoot "runs\$RunId"

function Write-Utf8File {
    param(
        [string]$Path,
        [string]$Content
    )
    $Content | Set-Content -Encoding UTF8 -LiteralPath $Path
}

function Stop-WithAudit {
    param(
        [string]$Message,
        [int]$Code = 1
    )
    New-Item -ItemType Directory -Force -Path $RunDir | Out-Null
    Write-Utf8File -Path (Join-Path $RunDir "execution-blocked.md") -Content $Message
    Write-Error $Message
    exit $Code
}

if (-not (Test-Path -LiteralPath $RunDir)) {
    Stop-WithAudit -Message "Run directory does not exist: $RunDir" -Code 2
}

$requestPath = Join-Path $RunDir "request.md"
$planPath = Join-Path $RunDir "cc-plan.md"
$reviewPath = Join-Path $RunDir "codex-review.md"
$ccExitPath = Join-Path $RunDir "cc-plan.exitcode.txt"
$reviewExitPath = Join-Path $RunDir "codex-review.exitcode.txt"

foreach ($required in @($requestPath, $planPath, $reviewPath, $ccExitPath, $reviewExitPath)) {
    if (-not (Test-Path -LiteralPath $required)) {
        Stop-WithAudit -Message "Required audit file is missing: $required"
    }
}

$ccExit = (Get-Content -LiteralPath $ccExitPath -Raw).Trim()
$reviewExit = (Get-Content -LiteralPath $reviewExitPath -Raw).Trim()
$reviewText = Get-Content -LiteralPath $reviewPath -Raw

if ($ccExit -ne "0") {
    Stop-WithAudit -Message "Refusing execution because CC planning failed. cc-plan exit code: $ccExit"
}

if ($reviewExit -ne "0") {
    Stop-WithAudit -Message "Refusing execution because Codex review failed. codex-review exit code: $reviewExit"
}

if ($reviewText -notmatch "(?i)\bAPPROVE\b") {
    Stop-WithAudit -Message "Refusing execution because Codex review does not contain APPROVE."
}

if ($reviewText -match "(?i)\b(BLOCK|REVISE)\b") {
    Stop-WithAudit -Message "Refusing execution because Codex review contains BLOCK or REVISE."
}

$task = Get-Content -LiteralPath $requestPath -Raw
$plan = Get-Content -LiteralPath $planPath -Raw

$executionPrompt = @"
Execute the user-approved task below.

Hard requirements:
1. Execute only what is covered by the approved plan.
2. Do not broaden scope.
3. Before destructive actions such as delete, overwrite, reset, or irreversible cleanup, stop and ask for explicit confirmation.
4. Return commands run, files changed, output paths, and remaining risks.

Original task:
$task

Approved plan:
$plan
"@

$executionPromptPath = Join-Path $RunDir "approved-execution-prompt.md"
Write-Utf8File -Path $executionPromptPath -Content $executionPrompt

Write-ChatObservation -EventType "command_start" -CommandName "批准执行" -Detail "RunId=$RunId"

$output = & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $ControllerRoot "bin\call-cc-execute.ps1") -MessageFile $executionPromptPath -WorkDir $WorkDir -DangerouslySkipPermissions 2>&1
$exitCode = $LASTEXITCODE
if ($null -eq $exitCode) { $exitCode = 0 }

$outputText = ($output | ForEach-Object { $_.ToString() }) -join "`n"
Write-Utf8File -Path (Join-Path $RunDir "cc-execution.md") -Content $outputText
Write-Utf8File -Path (Join-Path $RunDir "cc-execution.exitcode.txt") -Content "$exitCode"

$summary = @"
# Approved Execution Summary

Run ID: $RunId
WorkDir: $WorkDir
CC execution exit code: $exitCode

Dangerous permission bypass was used only after:
- CC plan exit code was 0
- Codex review exit code was 0
- Codex review contained APPROVE
- Codex review did not contain BLOCK or REVISE

Output:

$outputText
"@

Write-Utf8File -Path (Join-Path $RunDir "execution-summary.md") -Content $summary

Write-ChatObservation -EventType "command_end" -CommandName "批准执行" -Detail "RunId=$RunId exit=$exitCode"

Write-Output $summary
exit $exitCode
