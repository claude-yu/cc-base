param(
    [Parameter(Mandatory = $true)]
    [string]$RunId,

    [string]$WorkDir = "",

    [switch]$DryRun
)

$ErrorActionPreference = "Continue"

[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

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

# --- Gate 1: Run directory exists ---
if (-not (Test-Path -LiteralPath $RunDir)) {
    Stop-WithAudit -Message "Run directory does not exist: $RunDir" -Code 2
}

# --- Gate 2: Required audit files ---
$planPath = Join-Path $RunDir "cc-plan.md"
$reviewPath = Join-Path $RunDir "codex-review.md"
$approvalPath = Join-Path $RunDir "manual-approval.md"
$ccExitPath = Join-Path $RunDir "cc-plan.exitcode.txt"
$reviewExitPath = Join-Path $RunDir "codex-review.exitcode.txt"

foreach ($required in @($planPath, $reviewPath, $approvalPath)) {
    if (-not (Test-Path -LiteralPath $required)) {
        Stop-WithAudit -Message "Required audit file is missing: $required"
    }
}

# --- Gate 3: Exit codes (allow missing exitcode files for manual runs, but if present must be 0) ---
if (Test-Path -LiteralPath $ccExitPath) {
    $ccExit = (Get-Content -LiteralPath $ccExitPath -Raw).Trim()
    if ($ccExit -ne "0") {
        Stop-WithAudit -Message "CC planning failed (exit code: $ccExit). Manual approval cannot override a failed plan."
    }
}
if (Test-Path -LiteralPath $reviewExitPath) {
    $reviewExit = (Get-Content -LiteralPath $reviewExitPath -Raw).Trim()
    if ($reviewExit -ne "0") {
        Stop-WithAudit -Message "Codex review script failed (exit code: $reviewExit). Manual approval cannot override a crashed review."
    }
}

# --- Gate 4: manual-approval.md structured validation ---
$approvalText = Get-Content -LiteralPath $approvalPath -Raw

if ($approvalText -notmatch "(?i)Status:\s*USER_ACCEPTED_WITH_REVISE") {
    Stop-WithAudit -Message "manual-approval.md must contain 'Status: USER_ACCEPTED_WITH_REVISE'."
}

if ($approvalText -notmatch "(?i)RunId:\s*$([regex]::Escape($RunId))") {
    Stop-WithAudit -Message "manual-approval.md RunId does not match '$RunId'."
}

if ($approvalText -notmatch "(?i)Accepted\s+risks:") {
    Stop-WithAudit -Message "manual-approval.md must contain 'Accepted risks:' section."
}

# --- Gate 5: Codex review must NOT be BLOCK ---
$reviewText = Get-Content -LiteralPath $reviewPath -Raw
if ($reviewText -match "(?i)Verdict:\s*BLOCK") {
    Stop-WithAudit -Message "Refusing execution because Codex verdict is BLOCK. Manual approval cannot override BLOCK."
}

# --- Gate 6: CC plan must be non-empty ---
$planText = Get-Content -LiteralPath $planPath -Raw
if ([string]::IsNullOrWhiteSpace($planText)) {
    Stop-WithAudit -Message "CC plan is empty."
}

# --- All gates passed, build execution prompt ---
$executionPrompt = @"
Execute the user-approved task below.

IMPORTANT: This plan was USER_ACCEPTED_WITH_REVISE (not Codex APPROVE).
The user has manually reviewed all remaining Codex suggestions and accepted the plan.

Hard requirements:
1. Execute only what is covered by the approved plan.
2. Do not broaden scope.
3. Before destructive actions such as delete, overwrite, reset, or irreversible cleanup, stop and ask for explicit confirmation.
4. All GROMACS commands that depend on custom index groups MUST include -n index.ndx (except gmx energy).
5. All WSL paths MUST use /mnt/g/... format with single-quote shell escaping. Never pass Join-Path output to wsl gmx.
6. PID detection must be system-specific (match target system path, not just "gmx").
7. Return commands run, files changed, output paths, and remaining risks.

Approved plan:

$planText
"@

$executionPromptPath = Join-Path $RunDir "manual-execution-prompt.md"
Write-Utf8File -Path $executionPromptPath -Content $executionPrompt

Write-ChatObservation -EventType "command_start" -CommandName "人工批准执行" -Detail "RunId=$RunId"

# --- DryRun: stop here, do not call CC execute ---
if ($DryRun) {
    $summary = @"
# Manual-Approved DRY RUN Summary

Run ID: $RunId
WorkDir: $WorkDir
Mode: DRY RUN (no CC execution, no --dangerously-skip-permissions)

Safety gates passed:
- [PASS] Gate 1: Run directory exists
- [PASS] Gate 2: cc-plan.md / codex-review.md / manual-approval.md present
- [PASS] Gate 3: Exit codes (if present) are 0
- [PASS] Gate 4: Status=USER_ACCEPTED_WITH_REVISE, RunId matches, Accepted risks present
- [PASS] Gate 5: Codex verdict is not BLOCK
- [PASS] Gate 6: CC plan is non-empty

Generated files:
- manual-execution-prompt.md (review this to verify the prompt CC would receive)

Plan size: $($planText.Length) chars, $($planText.Split("`n").Count) lines

NOT executed:
- call-cc-execute.ps1 was NOT called
- --dangerously-skip-permissions was NOT used
- No GROMACS commands were run
"@

    Write-Utf8File -Path (Join-Path $RunDir "dryrun-summary.md") -Content $summary
    Write-Output $summary
    exit 0
}

# --- Real execution via CC with dangerously-skip-permissions ---
$output = & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $ControllerRoot "bin\call-cc-execute.ps1") -MessageFile $executionPromptPath -WorkDir $WorkDir -DangerouslySkipPermissions 2>&1
$exitCode = $LASTEXITCODE
if ($null -eq $exitCode) { $exitCode = 0 }

$outputText = ($output | ForEach-Object { $_.ToString() }) -join "`n"
Write-Utf8File -Path (Join-Path $RunDir "cc-execution.md") -Content $outputText
Write-Utf8File -Path (Join-Path $RunDir "cc-execution.exitcode.txt") -Content "$exitCode"

$summary = @"
# Manual-Approved Execution Summary

Run ID: $RunId
WorkDir: $WorkDir
CC execution exit code: $exitCode
Approval type: USER_ACCEPTED_WITH_REVISE

Safety gates passed:
- [PASS] Gate 1: Run directory exists
- [PASS] Gate 2: cc-plan.md / codex-review.md / manual-approval.md present
- [PASS] Gate 3: Exit codes (if present) are 0
- [PASS] Gate 4: Status=USER_ACCEPTED_WITH_REVISE, RunId matches, Accepted risks present
- [PASS] Gate 5: Codex verdict is not BLOCK
- [PASS] Gate 6: CC plan is non-empty

Output:

$outputText
"@

Write-Utf8File -Path (Join-Path $RunDir "execution-summary.md") -Content $summary

Write-ChatObservation -EventType "command_end" -CommandName "人工批准执行" -Detail "RunId=$RunId exit=$exitCode"

Write-Output $summary
exit $exitCode
