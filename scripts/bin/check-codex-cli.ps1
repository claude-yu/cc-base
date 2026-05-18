param(
    [string]$RunDir = ""
)

$ErrorActionPreference = "Continue"

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

$ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
if ([string]::IsNullOrWhiteSpace($RunDir)) {
    $RunDir = Join-Path $ControllerRoot "runs\codex-cli-check"
}

New-Item -ItemType Directory -Force -Path $RunDir | Out-Null

$versionOutput = & cmd /c codex --version 2>&1
$versionExit = $LASTEXITCODE
($versionOutput | ForEach-Object { $_.ToString() }) |
    Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "version.txt")

$testMessage = "Reply with exactly: ok. Do not read or modify files."
$testMessagePath = Join-Path $RunDir "exec-test-prompt.txt"
$testMessage | Set-Content -Encoding UTF8 -LiteralPath $testMessagePath
$reviewScript = Join-Path (Split-Path -Parent $PSCommandPath) "call-codex-review.ps1"
$testOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $reviewScript -MessageFile $testMessagePath 2>&1
$testExit = $LASTEXITCODE
($testOutput | ForEach-Object { $_.ToString() }) |
    Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "exec-test.txt")

$summaryLines = New-Object System.Collections.Generic.List[string]
$summaryLines.Add("# Codex CLI Check")
$summaryLines.Add("")
$summaryLines.Add("Version exit code: " + $versionExit)
$summaryLines.Add("Exec test exit code: " + $testExit)
$summaryLines.Add("")
$summaryLines.Add("## Interpretation")
$summaryLines.Add("")
$summaryLines.Add("If exec-test contains TLS or websocket errors, treat the reviewer path as blocked by Codex CLI network connectivity.")
$summaryLines.Add("Known blocking patterns include: tls handshake eof, wss://api.openai.com/v1/responses.")

$summary = [string]::Join([Environment]::NewLine, $summaryLines)
$summary | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $RunDir "summary.md")

Write-Output $summary
exit $testExit
