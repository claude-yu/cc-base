param(
    [string]$Message,

    [string]$MessageFile,

    [string]$WorkDir = ""
)

$ErrorActionPreference = "Continue"

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

Set-CodexProxy

$WorkDir = Resolve-RequiredWorkDir -ParamValue $WorkDir -EnvVarName "CC_CODEX_WORK_DIR"

if (-not [string]::IsNullOrWhiteSpace($MessageFile)) {
    if (-not (Test-Path -LiteralPath $MessageFile)) {
        throw "MessageFile does not exist: $MessageFile"
    }
    $Message = Get-Content -LiteralPath $MessageFile -Raw
}

if ([string]::IsNullOrWhiteSpace($Message)) {
    throw "Message or MessageFile is required."
}

$codexCmd = Resolve-CodexCmd

$reviewPrompt = @"
You are Codex acting only as an independent reviewer.
You must not modify files.
You must not execute shell commands unless needed for read-only inspection.

Review the provided plan or task for:
- scientific correctness;
- missing inputs and outputs;
- path and parameter risks;
- validation gaps;
- destructive or irreversible actions;
- whether execution should wait for explicit user confirmation.

Return concise Simplified Chinese output with exactly these sections:
1. Verdict: APPROVE / REVISE / BLOCK
2. 主要风险
3. 必须修改
4. 可选建议

Do not include CLI metadata, session information, or token counts in the answer.

Plan or task to review:

$Message
"@

Push-Location -LiteralPath $WorkDir
try {
    $reviewPrompt | & $codexCmd exec --sandbox read-only --skip-git-repo-check --ephemeral 2>&1
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
