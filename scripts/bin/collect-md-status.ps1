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

# 单行过长保护：GROMACS 进度刷屏行（如 "Reading energy frame ..."）可达数千字符，
# 会撑爆聊天输出。超长行截断；短行（Temperature/Energy/"complete" 等状态行）原样保留，
# 不影响读取 MD 状态信息。
function Limit-LineLength {
    param([string]$Text, [int]$Max)

    if ([string]::IsNullOrEmpty($Text)) { return $Text }
    return (($Text -split "`r?`n" | ForEach-Object {
        if ($_.Length -gt $Max) {
            $_.Substring(0, $Max) + " ...[行过长已截断 +" + ($_.Length - $Max) + " 字符]"
        } else {
            $_
        }
    }) -join [Environment]::NewLine)
}

if (-not (Test-Path -LiteralPath $WorkDir)) {
    $message = "WorkDir does not exist or is not readable: $WorkDir"
    Write-Utf8File -Path (Join-Path $RunDir "summary.md") -Content $message
    Write-Error $message
    exit 2
}

$resolvedWorkDir = (Resolve-Path -LiteralPath $WorkDir).Path

# .pdb 在对接工作区会有成千上万个产物文件（如 clipdrug docking 结果），与 MD 状态无关；
# MD 状态由 .log/.edr/.tpr/.cpt 决定。.pdb 单独计数、不进文件表，避免输出膨胀
# （曾实测 work-9 → 约 395 KB，微信端无法显示）。
$tableExts = @(".tpr", ".cpt", ".log", ".xtc", ".trr", ".edr", ".gro", ".top", ".itp", ".mdp")
$MaxTableRows = 200

$allMatched = Get-ChildItem -LiteralPath $resolvedWorkDir -File -Recurse -ErrorAction SilentlyContinue |
    Where-Object { ($tableExts -contains $_.Extension.ToLowerInvariant()) -or ($_.Extension -ieq ".pdb") }
$pdbFiles = @($allMatched | Where-Object { $_.Extension -ieq ".pdb" })
$files = @($allMatched |
    Where-Object { $tableExts -contains $_.Extension.ToLowerInvariant() } |
    Sort-Object DirectoryName, Name)
$logs = $files | Where-Object { $_.Extension -ieq ".log" } | Sort-Object FullName

# 完整清单（含每个 .pdb）始终落盘，信息不丢失；聊天/摘要仅做有界摘要。
$fullListPath = Join-Path $RunDir "files-full.txt"
Write-Utf8File -Path $fullListPath -Content (
    ($allMatched | Sort-Object DirectoryName, Name | ForEach-Object {
        "{0}`t{1}`t{2}" -f $_.LastWriteTime, $_.Length, $_.FullName
    }) -join [Environment]::NewLine)

$tableFiles = $files
$truncatedNote = $null
if ($files.Count -gt $MaxTableRows) {
    $tableFiles = $files | Select-Object -First $MaxTableRows
    $truncatedNote = "（MD 相关文件过多：仅列前 $MaxTableRows / 共 $($files.Count) 个；完整清单见 files-full.txt）"
}

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
if ($pdbFiles.Count -gt 0) {
    $reportParts.Add(".pdb 文件：$($pdbFiles.Count) 个（对接产物，与 MD 状态无关，未列出；完整清单见 files-full.txt）")
    $reportParts.Add("")
}
if ($truncatedNote) {
    $reportParts.Add($truncatedNote)
    $reportParts.Add("")
}
$reportParts.Add((Format-FileTable -Files $tableFiles))
$reportParts.Add("")
$reportParts.Add("## Log Tails")

if ($null -eq $logs -or $logs.Count -eq 0) {
    $reportParts.Add("")
    $reportParts.Add("(no log files found)")
} else {
    # MD 状态只需看最新日志；旧日志（如建模阶段 em_cg.log）只列出、不展开 tail，
    # 避免在多 .log 工作区输出膨胀。日志文件本身在磁盘未改动，需要某个可单独索取。
    $MaxLogTails = 6
    $logsByRecent = @($logs | Sort-Object LastWriteTime -Descending)
    $tailLogs = @($logsByRecent | Select-Object -First $MaxLogTails)
    $restLogs = @($logsByRecent | Select-Object -Skip $MaxLogTails)

    foreach ($log in $tailLogs) {
        $reportParts.Add("")
        $reportParts.Add("### $($log.FullName)  (修改 $($log.LastWriteTime))")
        $reportParts.Add("")
        $reportParts.Add((Limit-LineLength -Text (Get-SharedTail -Path $log.FullName -Lines $LogTail) -Max 1000))
    }

    if ($restLogs.Count -gt 0) {
        $reportParts.Add("")
        $reportParts.Add("### 其余 $($restLogs.Count) 个较旧 .log（仅列出，未展开 tail；需要某个内容请单独索取）")
        $reportParts.Add("")
        foreach ($log in $restLogs) {
            $reportParts.Add("- $($log.LastWriteTime.ToString('yyyy-MM-dd HH:mm'))  $($log.FullName)")
        }
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
