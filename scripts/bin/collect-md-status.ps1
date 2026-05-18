param(
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

if ([string]::IsNullOrWhiteSpace($WorkDir)) {
    $WorkDir = Resolve-RequiredWorkDir -ParamValue $WorkDir -EnvVarName "CC_WORK_DIR"
}
if ([string]::IsNullOrWhiteSpace($RunId)) {
    $RunId = (Get-Date -Format "yyyyMMdd-HHmmss") + "-md-status"
}

$RunDir = Join-Path $ControllerRoot "runs\$RunId"
New-Item -ItemType Directory -Force -Path $RunDir | Out-Null

Write-ChatObservation -EventType "command_start" -CommandName "md状态检查"

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

function Resolve-CandidatePath {
    param(
        [string]$Label,
        [string[]]$KnownPaths,
        [string[]]$NamePatterns
    )

    foreach ($path in $KnownPaths) {
        if (Test-Path -LiteralPath $path) {
            return (Resolve-Path -LiteralPath $path).Path
        }
    }

    $roots = @($WorkDir, (Join-Path $WorkDir "md_ternary"), (Join-Path $WorkDir "MD"), (Join-Path $WorkDir "md"))
    foreach ($root in $roots) {
        if (-not (Test-Path -LiteralPath $root)) { continue }

        $dirs = Get-ChildItem -LiteralPath $root -Directory -Recurse -ErrorAction SilentlyContinue
        foreach ($dir in $dirs) {
            $name = $dir.FullName.ToLowerInvariant()
            $matched = $true
            foreach ($pattern in $NamePatterns) {
                if (-not $name.Contains($pattern.ToLowerInvariant())) {
                    $matched = $false
                    break
                }
            }
            if ($matched) { return $dir.FullName }
        }
    }

    return ""
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

$candidates = @(
    [PSCustomObject]@{
        Label = "PEG5_Lena"
        KnownPaths = @(
            (Join-Path $WorkDir "PEG5_Lena"),
            (Join-Path $WorkDir "md_ternary\peg5_lena")
        )
        Patterns = @("peg5", "lena")
    },
    [PSCustomObject]@{
        Label = "3289_PEG3_Poma"
        KnownPaths = @(
            (Join-Path $WorkDir "3289_PEG3_Poma"),
            (Join-Path $WorkDir "md_ternary\3289_peg3_poma")
        )
        Patterns = @("3289")
    },
    [PSCustomObject]@{
        Label = "8011_PEG4_Lena"
        KnownPaths = @(
            (Join-Path $WorkDir "8011_PEG4_Lena"),
            (Join-Path $WorkDir "md_ternary\8011_peg4_lena")
        )
        Patterns = @("8011")
    }
)

$extensions = @(".tpr", ".cpt", ".log", ".xtc", ".trr", ".edr", ".gro", ".pdb", ".top", ".itp", ".mdp")
$summaryParts = New-Object System.Collections.Generic.List[string]
$summaryParts.Add("# MD Status Collection")
$summaryParts.Add("")
$summaryParts.Add("Run ID: $RunId")
$summaryParts.Add("WorkDir: $WorkDir")
$summaryParts.Add("Log tail lines: $LogTail")
$summaryParts.Add("")
$summaryParts.Add("No MD command was executed. This script only lists files and reads log tails.")

foreach ($candidate in $candidates) {
    $label = $candidate.Label
    $path = Resolve-CandidatePath -Label $label -KnownPaths $candidate.KnownPaths -NamePatterns $candidate.Patterns
    $candidateReport = New-Object System.Collections.Generic.List[string]
    $candidateReport.Add("# $label")
    $candidateReport.Add("")

    if ([string]::IsNullOrWhiteSpace($path)) {
        $candidateReport.Add("Status: directory not found")
        $candidateReport.Add("Known paths checked:")
        foreach ($knownPath in $candidate.KnownPaths) {
            $candidateReport.Add("- $knownPath")
        }
        $content = [string]::Join([Environment]::NewLine, $candidateReport)
        Write-Utf8File -Path (Join-Path $RunDir "$label.md") -Content $content
        $summaryParts.Add("")
        $summaryParts.Add("## $label")
        $summaryParts.Add("")
        $summaryParts.Add("Directory not found.")
        continue
    }

    $candidateReport.Add("Directory: $path")
    $candidateReport.Add("")
    $candidateReport.Add("## Matching Files")
    $candidateReport.Add("")

    $files = Get-ChildItem -LiteralPath $path -File -Recurse -ErrorAction SilentlyContinue |
        Where-Object { $extensions -contains $_.Extension.ToLowerInvariant() }
    $candidateReport.Add((Format-FileTable -Files $files))

    $logs = $files | Where-Object { $_.Extension -ieq ".log" } | Sort-Object FullName
    $candidateReport.Add("")
    $candidateReport.Add("## Log Tails")
    if ($null -eq $logs -or $logs.Count -eq 0) {
        $candidateReport.Add("")
        $candidateReport.Add("(no log files found)")
    } else {
        foreach ($log in $logs) {
            $candidateReport.Add("")
            $candidateReport.Add("### $($log.FullName)")
            $candidateReport.Add("")
            $candidateReport.Add((Get-SharedTail -Path $log.FullName -Lines $LogTail))
        }
    }

    $content = [string]::Join([Environment]::NewLine, $candidateReport)
    Write-Utf8File -Path (Join-Path $RunDir "$label.md") -Content $content

    $summaryParts.Add("")
    $summaryParts.Add("## $label")
    $summaryParts.Add("")
    $summaryParts.Add("Directory: $path")
    $summaryParts.Add("Matching files: " + (($files | Measure-Object).Count))
    $summaryParts.Add("Log files: " + (($logs | Measure-Object).Count))
}

$summaryParts.Add("")
$summaryParts.Add("Run directory:")
$summaryParts.Add("")
$summaryParts.Add($RunDir)

$summary = [string]::Join([Environment]::NewLine, $summaryParts)
Write-Utf8File -Path (Join-Path $RunDir "summary.md") -Content $summary

Write-ChatObservation -EventType "command_end" -CommandName "md状态检查"

Write-Output $summary
exit 0
