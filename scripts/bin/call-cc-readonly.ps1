param(
    [string]$Message,

    [string]$MessageFile,

    [string]$WorkDir = ""
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

$rulesDir = Join-Path (Split-Path -Parent (Split-Path -Parent $PSCommandPath)) "rules"
$domainRules = ""
if (Test-Path -LiteralPath $rulesDir) {
    Get-ChildItem -LiteralPath $rulesDir -Filter "*.md" | ForEach-Object {
        $domainRules += "`n`n" + (Get-Content -LiteralPath $_.FullName -Raw -Encoding UTF8)
    }
}

$systemPrompt = @"
You are Claude Code being called as a read-only backend by a lightweight controller.
Do not modify files.
Do not run shell commands.
Do not spawn subagents.
Use only read/search/list style tools if needed.
Return concise, structured output.
For complex scientific work, produce a short plan first and ask for follow-up expansion by candidate/task.
$domainRules
"@

$tmpPrompt = Join-Path $env:TEMP "cc-readonly-prompt-$([guid]::NewGuid().ToString('N')).txt"
$Message | Set-Content -Encoding UTF8 -LiteralPath $tmpPrompt -NoNewline

Push-Location -LiteralPath $WorkDir
try {
    Get-Content -LiteralPath $tmpPrompt -Raw -Encoding UTF8 | & $claudeCmd -p --system-prompt $systemPrompt --allowedTools Read,Glob,Grep,LS --disallowedTools Bash,Edit,Write,MultiEdit,NotebookEdit --permission-mode default --output-format text --no-session-persistence
    exit $LASTEXITCODE
} finally {
    Pop-Location
    Remove-Item -LiteralPath $tmpPrompt -ErrorAction SilentlyContinue
}
