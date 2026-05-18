param(
    [Parameter(Position = 0)]
    [string]$WorkDir = "",
    [string]$RunId = "",
    [int]$LogTail = 80
)

$ErrorActionPreference = "Continue"

[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

$ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)

if (-not [string]::IsNullOrWhiteSpace($WorkDir)) {
    $WorkDir = $WorkDir.Trim().Trim('"')
}

if ([string]::IsNullOrWhiteSpace($WorkDir)) {
    $WorkDir = Resolve-RequiredWorkDir -ParamValue $WorkDir -EnvVarName "CC_WORK_DIR"
}
if ([string]::IsNullOrWhiteSpace($RunId)) {
    $RunId = (Get-Date -Format "yyyyMMdd-HHmmss") + "-md-status"
}

$RunDir = Join-Path $ControllerRoot "runs\$RunId"
New-Item -ItemType Directory -Force -Path $RunDir | Out-Null

Write-ChatObservation -EventType "command_start" -CommandName "md-status"

function Write-Utf8File {
    param(
        [string]$Path,
        [string]$Content
    )
    $Content | Set-Content -Encoding UTF8 -LiteralPath $Path
}

function Format-FileTable {
    param([System.IO.FileInfo[]]$Files)

    if ($null -eq $Files -or $Files.Count -eq 0) {
        return "(no matching MD files found)"
    }

    return ($Files |
        Sort-Object DirectoryName, Name |
        Select-Object FullName, Length, LastWriteTime |
        Format-Table -AutoSize |
        Out-String -Width 240).TrimEnd()
}

function Get-SharedTail {
    param(
        [string]$Path,
        [int]$Lines
    )

    $share = [System.IO.FileShare]([int][System.IO.FileShare]::ReadWrite -bor [int][System.IO.FileShare]::Delete)
    $bufferSizes = @(262144, 131072, 65536, 32768, 8192)
    $skipEndValues = @(0, 4096, 8192, 16384, 32768)
    $lastError = ""

    foreach ($bufferSize in $bufferSizes) {
        foreach ($skipEnd in $skipEndValues) {
            $stream = $null
            try {
                $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, $share)
                $size = $stream.Length
                if ($size -le 0) { return "" }

                if ($skipEnd -ge $size) { continue }
                $readSize = [Math]::Min($bufferSize, $size - $skipEnd)
                $offset = $size - $skipEnd - $readSize
                $stream.Seek($offset, [System.IO.SeekOrigin]::Begin) | Out-Null

                $buffer = New-Object byte[] $readSize
                $bytesRead = $stream.Read($buffer, 0, $readSize)
                if ($bytesRead -le 0) { continue }

                $text = [System.Text.Encoding]::UTF8.GetString($buffer, 0, $bytesRead)
                $allLines = $text -split "`r?`n"
                if ($allLines.Count -le $Lines) {
                    return [string]::Join([Environment]::NewLine, $allLines)
                }

                return [string]::Join([Environment]::NewLine, $allLines[($allLines.Count - $Lines)..($allLines.Count - 1)])
            } catch {
                $lastError = $_.Exception.Message
            } finally {
                if ($null -ne $stream) { $stream.Dispose() }
            }
        }
    }

    return "(unable to read log tail via shared byte-tail: $lastError)"
}

if (-not (Test-Path -LiteralPath $WorkDir)) {
    $message = "WorkDir does not exist or is not readable: $WorkDir"
    Write-Utf8File -Path (Join-Path $RunDir "summary.md") -Content $message
    Write-Error $message
    exit 2
}

$resolvedWorkDir = (Resolve-Path -LiteralPath $WorkDir).Path
$extensions = @(".tpr", ".cpt", ".log", ".xtc", ".trr", ".edr", ".gro", ".pdb", ".top", ".itp", ".mdp")

$files = Get-ChildItem -LiteralPath $resolvedWorkDir -File -Recurse -ErrorAction SilentlyContinue |
    Where-Object { $extensions -contains $_.Extension.ToLowerInvariant() }
$logs = $files | Where-Object { $_.Extension -ieq ".log" } | Sort-Object FullName

$reportParts = New-Object System.Collections.Generic.List[string]
$reportParts.Add("# MD Status")
$reportParts.Add("")
$reportParts.Add("Run ID: $RunId")
$reportParts.Add("WorkDir: $resolvedWorkDir")
$reportParts.Add("Log tail lines: $LogTail")
$reportParts.Add("")
$reportParts.Add("No MD command was executed. This script only lists files and reads log tails.")
$reportParts.Add("")
$reportParts.Add("## Matching Files")
$reportParts.Add("")
$reportParts.Add((Format-FileTable -Files $files))
$reportParts.Add("")
$reportParts.Add("## Log Tails")

if ($null -eq $logs -or $logs.Count -eq 0) {
    $reportParts.Add("")
    $reportParts.Add("(no log files found)")
} else {
    foreach ($log in $logs) {
        $reportParts.Add("")
        $reportParts.Add("### $($log.FullName)")
        $reportParts.Add("")
        $reportParts.Add((Get-SharedTail -Path $log.FullName -Lines $LogTail))
    }
}

$reportParts.Add("")
$reportParts.Add("Run directory:")
$reportParts.Add("")
$reportParts.Add($RunDir)

$summary = [string]::Join([Environment]::NewLine, $reportParts)
Write-Utf8File -Path (Join-Path $RunDir "summary.md") -Content $summary
Write-Utf8File -Path (Join-Path $RunDir "md-status.md") -Content $summary

Write-ChatObservation -EventType "command_end" -CommandName "md-status"

Write-Output $summary
exit 0
