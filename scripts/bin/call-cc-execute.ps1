param(
    [string]$Message,

    [string]$MessageFile,

    [string]$WorkDir = "",

    [switch]$DangerouslySkipPermissions
)

$ErrorActionPreference = "Stop"

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$WorkDir = Resolve-RequiredWorkDir -ParamValue $WorkDir -EnvVarName "CC_WORK_DIR"

if (-not [string]::IsNullOrWhiteSpace($MessageFile)) {
    if (-not (Test-Path -LiteralPath $MessageFile)) {
        throw "MessageFile does not exist: $MessageFile"
    }
    $Message = Get-Content -LiteralPath $MessageFile -Raw
}

if ([string]::IsNullOrWhiteSpace($Message)) {
    throw "Message or MessageFile is required."
}

$claudeCmd = Resolve-ClaudeCmd

$systemPrompt = @"
You are Claude Code being called as an execution backend by a lightweight controller.
Only execute the user-approved task.
Before any destructive operation, stop and ask for explicit confirmation.
Return a concise report with commands run, files changed, output paths, and remaining risks.
"@

$tmpPrompt = Join-Path $env:TEMP "cc-execute-prompt-$([guid]::NewGuid().ToString('N')).txt"
$Message | Set-Content -Encoding UTF8 -LiteralPath $tmpPrompt -NoNewline

Push-Location -LiteralPath $WorkDir
try {
    if ($DangerouslySkipPermissions) {
        Get-Content -LiteralPath $tmpPrompt -Raw -Encoding UTF8 | & $claudeCmd -p --system-prompt $systemPrompt --dangerously-skip-permissions --output-format text
    } else {
        Get-Content -LiteralPath $tmpPrompt -Raw -Encoding UTF8 | & $claudeCmd -p --system-prompt $systemPrompt --permission-mode default --output-format text
    }
    exit $LASTEXITCODE
} finally {
    Pop-Location
    Remove-Item -LiteralPath $tmpPrompt -ErrorAction SilentlyContinue
}
