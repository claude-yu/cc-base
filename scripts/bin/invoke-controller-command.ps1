[CmdletBinding(PositionalBinding=$false)]
param(
  [Parameter(Mandatory=$true)][string]$CommandName,
  [Parameter(Mandatory=$true)][string]$Real,
  [string]$Channel = 'wechat',
  [string]$Alias,
  [string]$RunId,
  [Parameter(Position=0, ValueFromRemainingArguments=$true)][string[]]$RealArgs
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path -LiteralPath $Real)) {
  throw "Real script not found: $Real"
}

if ([string]::IsNullOrWhiteSpace($RunId)) {
  $RunId = (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + ([guid]::NewGuid().ToString('N').Substring(0,6))
}

$writer   = Join-Path $PSScriptRoot 'chat-log-writer.ps1'
$argsText = if ($RealArgs) { ($RealArgs -join ' ') } else { '' }

$inParams = @{
  Channel   = $Channel
  Direction = 'in'
  Lifecycle = 'started'
  Command   = $CommandName
  RunId     = $RunId
}
if ($Alias)    { $inParams['Alias'] = $Alias }
if ($argsText) { $inParams['Text']  = $argsText }
& $writer @inParams | Out-Null

$global:LASTEXITCODE = 0
$exitCode = 0
try {
  & $Real @RealArgs
  $exitCode = if ($null -ne $LASTEXITCODE) { $LASTEXITCODE } else { 0 }
} catch {
  $exitCode = 1
}

$outParams = @{
  Channel   = $Channel
  Direction = 'out'
  Lifecycle = if ($exitCode -eq 0) { 'completed' } else { 'failed' }
  Command   = $CommandName
  RunId     = $RunId
}
if ($Alias) { $outParams['Alias'] = $Alias }
& $writer @outParams | Out-Null

exit $exitCode
