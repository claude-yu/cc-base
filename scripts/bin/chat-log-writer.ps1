[CmdletBinding()]
param(
  [Parameter(Mandatory=$true)][ValidateSet('wechat','feishu')][string]$Channel,
  [Parameter(Mandatory=$true)][ValidateSet('in','out')][string]$Direction,
  [Parameter(Mandatory=$true)][ValidateSet('started','completed','failed','replied','cancelled')][string]$Lifecycle,
  [string]$Command,
  [string]$Alias,
  [string]$RunId,
  [string]$Text,
  [ValidateSet('corrected','confirmed','ignored')][string]$Signal,
  [ValidateSet('message','signal_patch')][string]$RecordType = 'message',
  [hashtable]$Meta
)

$ErrorActionPreference = 'Stop'

$mode = if ($env:CC_CHAT_LOG_MODE) { $env:CC_CHAT_LOG_MODE } else { 'full' }
if ($mode -eq 'off') { return }
if ($mode -notin @('full','metadata')) { throw "Invalid CC_CHAT_LOG_MODE: $mode (expected: full|metadata|off)" }

$workDir = if ($env:CC_WORK_DIR) { $env:CC_WORK_DIR } else { (Get-Location).Path }
$logDir  = if ($env:CC_CHAT_LOG_DIR) { $env:CC_CHAT_LOG_DIR } else { Join-Path $workDir 'memory/chat' }

if (-not (Test-Path $logDir)) {
  New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}

function Get-Sha256Hex {
  param([string]$Value)
  $sha = [System.Security.Cryptography.SHA256]::Create()
  try {
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($Value)
    $hash  = $sha.ComputeHash($bytes)
    return -join ($hash | ForEach-Object { $_.ToString('x2') })
  } finally { $sha.Dispose() }
}

function Invoke-Redact {
  param([string]$Value)
  if ([string]::IsNullOrEmpty($Value)) { return $Value }
  $r = $Value
  $r = [regex]::Replace($r, '(?i)\b(wx_token|token)\s*[:=]\s*\S+',  '$1=***REDACTED***')
  $r = [regex]::Replace($r, '(?i)\bBearer\s+\S+',                   'Bearer ***REDACTED***')
  $r = [regex]::Replace($r, '[A-Za-z]:\\[^\s''"`,;]+',              '***PATH***')
  return $r
}

$projectId = (Get-Sha256Hex $workDir).Substring(0,12)
$ts        = [DateTimeOffset]::Now.ToString('yyyy-MM-ddTHH:mm:sszzz')

$record = [ordered]@{
  ts          = $ts
  channel     = $Channel
  direction   = $Direction
  record_type = $RecordType
  lifecycle   = $Lifecycle
  signal      = if ($PSBoundParameters.ContainsKey('Signal')) { $Signal } else { $null }
  command     = if ($PSBoundParameters.ContainsKey('Command')) { $Command } else { $null }
  alias       = if ($PSBoundParameters.ContainsKey('Alias')) { $Alias } else { $null }
  run_id      = if ($PSBoundParameters.ContainsKey('RunId')) { $RunId } else { $null }
  work_dir    = $workDir
  project_id  = $projectId
}

if ($PSBoundParameters.ContainsKey('Text') -and -not [string]::IsNullOrEmpty($Text)) {
  $redacted = Invoke-Redact $Text
  if ($mode -eq 'full') { $record['text'] = $redacted }
  $record['text_hash'] = 'sha256:' + (Get-Sha256Hex $redacted)
}

$record['meta'] = if ($PSBoundParameters.ContainsKey('Meta') -and $Meta) { $Meta } else { @{} }

$line = ($record | ConvertTo-Json -Compress -Depth 10)
$date = [DateTime]::Now.ToString('yyyy-MM-dd')
$file = Join-Path $logDir "$date.jsonl"

[System.IO.File]::AppendAllText($file, $line + "`n", [System.Text.UTF8Encoding]::new($false))

Write-Output $file
