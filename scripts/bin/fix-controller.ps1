param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ArgsRest
)

$ErrorActionPreference = "Stop"

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

# UTF-8 for capturing Claude CLI output; switched to GBK before final Write-Output
[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)

$ErrorText = ""
if ($null -ne $ArgsRest -and $ArgsRest.Count -gt 0) {
    $ErrorText = ($ArgsRest | ForEach-Object { $_.ToString() }) -join " "
}

if ([string]::IsNullOrWhiteSpace($ErrorText) -and -not [string]::IsNullOrWhiteSpace($env:CONTROLLER_ERROR)) {
    $ErrorText = $env:CONTROLLER_ERROR
}

if ([string]::IsNullOrWhiteSpace($ErrorText)) {
    throw "Error text is required. Pass as arguments or set CONTROLLER_ERROR env var."
}

$RunId = (Get-Date -Format "yyyyMMdd-HHmmss") + "-fix-controller"
$RunDir = Join-Path $ControllerRoot "runs\$RunId"
New-Item -ItemType Directory -Force -Path $RunDir | Out-Null

$ErrorFile = Join-Path $RunDir "error-report.md"
$ErrorText | Set-Content -Encoding UTF8 -LiteralPath $ErrorFile

Write-ChatObservation -EventType "command_start" -CommandName "修复controller" -Detail $ErrorText

$claudeCmd = Resolve-ClaudeCmd

$rulesDir = Join-Path $ControllerRoot "rules"
$domainRules = ""
if (Test-Path -LiteralPath $rulesDir) {
    Get-ChildItem -LiteralPath $rulesDir -Filter "*.md" | ForEach-Object {
        $domainRules += "`n`n" + (Get-Content -LiteralPath $_.FullName -Raw -Encoding UTF8)
    }
}

$ccConnectDir = Split-Path -Parent $ControllerRoot | Join-Path -ChildPath "cc-connect"
$ccConnectConfig = Join-Path $env:USERPROFILE ".cc-connect\config.toml"

$systemPrompt = @"
You are Claude Code fixing a controller/cc-connect infrastructure error.
You may read and modify files ONLY in these directories:
- $ControllerRoot
- $ccConnectDir
- $ccConnectConfig

You MUST NOT touch:
- The project work directory or any data/trajectories/results
- Any file outside the allowed directories

After fixing, report:
1. What files were changed
2. Why each change was made
3. How to verify the fix
4. Whether cc-connect needs restart

Do not call Codex. Do not review plans. Fix only the reported infrastructure error.
$domainRules
"@

$prompt = @"
Fix this controller/cc-connect error:

$ErrorText

Diagnose the root cause, fix it, and report what you changed.
"@

$promptFile = Join-Path $RunDir "fix-prompt.md"
$prompt | Set-Content -Encoding UTF8 -LiteralPath $promptFile -NoNewline

Push-Location -LiteralPath $ControllerRoot
try {
    $result = Get-Content -LiteralPath $promptFile -Raw -Encoding UTF8 | & $claudeCmd -p --system-prompt $systemPrompt --allowedTools Read,Glob,Grep,Edit,Write,Bash,LS --permission-mode default --output-format text --no-session-persistence
    $exitCode = $LASTEXITCODE
    if ($null -eq $exitCode) { $exitCode = 0 }

    $result | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "fix-result.md")
    "$exitCode" | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "fix-result.exitcode.txt")

    Write-ChatObservation -EventType "command_end" -CommandName "修复controller" -Detail "exit=$exitCode"

    Write-Output $result
    exit $exitCode
} finally {
    Pop-Location
}
