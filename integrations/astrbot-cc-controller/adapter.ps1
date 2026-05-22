<#
.SYNOPSIS
    Adapter wrapping cc-controller for AstrBot integration.
    Outputs a single JSON envelope to stdout. No Write-Host, no table.
.PARAMETER Command
    One of: research-monitor, system-status, submit-review, show-review
.PARAMETER Detector
    Optional detector filter (must be in whitelist).
.PARAMETER TaskText
    Task description for submit-review.
.PARAMETER RunId
    Run ID for show-review.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)]
    [string]$Command,

    [Parameter(Mandatory=$false)]
    [string]$Detector = '',

    [Parameter(Mandatory=$false)]
    [string]$TaskText = '',

    [Parameter(Mandatory=$false)]
    [string]$RunId = ''
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$timeoutSeconds = 20
$integrationRoot = $PSScriptRoot
$repoRoot = 'C:\cc-base'
if (-not (Test-Path -LiteralPath $repoRoot)) {
    $repoRoot = Split-Path -Parent (Split-Path -Parent $integrationRoot)
}
if ($env:CC_BASE_ROOT -ne $null -and $env:CC_BASE_ROOT.Trim() -ne '') {
    $repoRoot = $env:CC_BASE_ROOT
}
$controllerRoot = Join-Path $repoRoot 'controller'
if ($env:CC_CONTROLLER_DIR -ne $null -and $env:CC_CONTROLLER_DIR.Trim() -ne '') {
    $controllerRoot = $env:CC_CONTROLLER_DIR
}
$binary = Join-Path $controllerRoot 'cc-controller.exe'
if ($env:CC_CONTROLLER_BIN -ne $null -and $env:CC_CONTROLLER_BIN.Trim() -ne '') {
    $binary = $env:CC_CONTROLLER_BIN
}
$script:projectRoot = $repoRoot
if ($env:CC_PROJECT_ROOT -ne $null -and $env:CC_PROJECT_ROOT.Trim() -ne '') {
    $script:projectRoot = $env:CC_PROJECT_ROOT
}
$script:defaultWorkDir = 'D:\research-work'
if (-not (Test-Path -LiteralPath $script:defaultWorkDir)) {
    $script:defaultWorkDir = $script:projectRoot
}
if ($env:CC_DEFAULT_WORK_DIR -ne $null -and $env:CC_DEFAULT_WORK_DIR.Trim() -ne '') {
    $script:defaultWorkDir = $env:CC_DEFAULT_WORK_DIR
}

$allowedCommands = @('research-monitor', 'system-status', 'submit-review', 'show-review', 'review-stats', 'memory-status', 'memory-record', 'memory-recap', 'memory-archive', 'memory-archive-execute')
$runIdPattern = '^\d{8}-\d{6}-[a-z\-]+$'
$astrbotReviewMarker = 'astrbot-review.json'
$allowedDetectors = @(
    'gromacs', 'schrodinger', 'haddock3', 'rosetta', 'autodock_vina',
    'alphafold', 'amber_openmm', 'gaussian', 'python_pipeline',
    'r_pipeline', 'generic_cli'
)

function Write-Envelope {
    param(
        [bool]$Ok,
        [string]$Cmd,
        [string]$Det,
        $Data,
        [string]$Error
    )
    $detVal = $null; if ($Det -ne '') { $detVal = $Det }
    $errVal = $null; if ($Error -ne '') { $errVal = $Error }
    $envelope = [ordered]@{
        ok       = $Ok
        command  = $Cmd
        detector = $detVal
        data     = $Data
        error    = $errVal
        ts       = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
    }
    [System.Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    $json = $envelope | ConvertTo-Json -Depth 20 -Compress
    [System.Console]::Out.WriteLine($json)
}

function Quote-ProcessArg {
    param([string]$Value)
    if ($Value -notmatch '[\s"]') { return $Value }
    return '"' + ($Value -replace '"', '\"') + '"'
}

function Get-ReviewStage {
    param([string]$RunDir)
    if (Test-Path -LiteralPath (Join-Path $RunDir 'summary.md')) { return 'done' }
    if (Test-Path -LiteralPath (Join-Path $RunDir 'codex-review-prompt.md')) { return 'codex_review' }
    if (Test-Path -LiteralPath (Join-Path $RunDir 'cc-plan-prompt.md')) { return 'cc_plan' }
    if (Test-Path -LiteralPath (Join-Path $RunDir 'request.md')) { return 'cc_plan' }
    return 'prepare'
}

function Redact-ReviewText {
    param([string]$Text)
    if ([string]::IsNullOrWhiteSpace($Text)) { return '' }
    $clean = $Text -replace '\r?\n', ' '
    $clean = $clean -replace '[A-Za-z]:\\[^\s]+', '<path>'
    $clean = $clean -replace '\\\\[^\\\s]+\\[^\s]+', '<path>'
    $clean = $clean -replace '\s+', ' '
    $clean = $clean.Trim()
    if ($clean.Length -gt 180) { $clean = $clean.Substring(0, 180) + '...' }
    return $clean
}

function Get-JsonPropertyValue {
    param(
        $Object,
        [string]$Name
    )
    if ($null -eq $Object) { return $null }
    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) { return $null }
    return $prop.Value
}

function Get-EventFailure {
    param([string]$RunDir)
    $eventsPath = Join-Path $RunDir 'events.jsonl'
    if (-not (Test-Path -LiteralPath $eventsPath)) { return $null }

    $matched = $null
    foreach ($line in [System.IO.File]::ReadLines($eventsPath, [System.Text.Encoding]::UTF8)) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        try {
            $event = $line | ConvertFrom-Json
        } catch {
            continue
        }
        $typeVal = Get-JsonPropertyValue -Object $event -Name 'type'
        $statusVal = Get-JsonPropertyValue -Object $event -Name 'status'
        $type = ''; if ($null -ne $typeVal) { $type = $typeVal.ToString() }
        $status = ''; if ($null -ne $statusVal) { $status = $statusVal.ToString() }
        if ($type -match '(?i)(failed|error)' -or $status -match '(?i)(failed|error)') {
            $matched = $event
        }
    }
    if ($null -eq $matched) { return $null }

    $stage = ''
    $stageVal = Get-JsonPropertyValue -Object $matched -Name 'stage'
    if ($null -ne $stageVal) { $stage = $stageVal.ToString() }
    $reason = ''
    foreach ($field in @('error', 'message', 'detail', 'reason')) {
        $fieldVal = Get-JsonPropertyValue -Object $matched -Name $field
        if ($null -ne $fieldVal -and -not [string]::IsNullOrWhiteSpace($fieldVal.ToString())) {
            $reason = $fieldVal.ToString()
            break
        }
    }
    if ([string]::IsNullOrWhiteSpace($reason)) {
        $reason = 'Review failed'
    }
    return [ordered]@{
        reason = (Redact-ReviewText $reason)
        stage  = $stage
    }
}

function Get-LogFailureReason {
    param([string]$RunDir)
    $errPath = Join-Path $RunDir 'background-err.log'
    if (-not (Test-Path -LiteralPath $errPath)) { return '' }
    $err = [System.IO.File]::ReadAllText($errPath, [System.Text.Encoding]::UTF8)
    if ([string]::IsNullOrWhiteSpace($err)) { return '' }
    if ($err -match 'WorkDir is required') { return 'WorkDir is required; CC_WORK_DIR was missing for the review runner.' }
    if ($err -match '(?i)(timeout|timed out)') { return 'Review runner timed out.' }
    if ($err -match '(?i)(api key|unauthorized|authentication)') { return 'Review backend authentication failed.' }
    return Redact-ReviewText $err
}

function Get-FailureNextStep {
    param(
        [string]$Reason,
        [string]$Stage
    )
    if ($Reason -match 'CC_WORK_DIR|WorkDir') { return 'Retry after adapter passes CC_WORK_DIR from active_project.json.' }
    if ($Reason -match '(?i)timeout') { return 'Retry the same review; if repeated, inspect backend/network availability.' }
    if ($Reason -match '(?i)api key|authentication|unauthorized') { return 'Check review backend credentials, then retry.' }
    if ($Stage -eq 'cc_plan') { return 'Inspect Claude plan-generation stderr, then retry the same task.' }
    if ($Stage -eq 'codex_review') { return 'Inspect Codex review stderr, then retry the same task.' }
    return 'Inspect the run logs, fix the reported blocker, then retry the same task.'
}

function Get-ReviewFailure {
    param(
        [string]$RunDir,
        [string]$Stage,
        [string]$Status
    )
    if ($Status -ne 'failed') {
        return [ordered]@{
            reason    = $null
            stage     = $null
            next_step = $null
        }
    }

    $failure = Get-EventFailure -RunDir $RunDir
    $failureStage = $Stage
    $reason = ''
    if ($null -ne $failure) {
        $reason = $failure.reason
        if (-not [string]::IsNullOrWhiteSpace($failure.stage)) { $failureStage = $failure.stage }
    }
    if ([string]::IsNullOrWhiteSpace($reason)) {
        $reason = Get-LogFailureReason -RunDir $RunDir
    }
    if ([string]::IsNullOrWhiteSpace($reason)) {
        $reason = 'Review failed; no structured error was recorded.'
    }

    return [ordered]@{
        reason    = $reason
        stage     = $failureStage
        next_step = (Get-FailureNextStep -Reason $reason -Stage $failureStage)
    }
}

function Get-TextLineCount {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) { return 0 }
    try {
        $lines = [System.IO.File]::ReadAllLines($Path, [System.Text.Encoding]::UTF8)
        return $lines.Count
    } catch {
        return 0
    }
}

function Get-MemoryFileInfo {
    param(
        [string]$ProjectRoot,
        [string]$Name
    )
    $path = Join-Path $ProjectRoot $Name
    $exists = Test-Path -LiteralPath $path
    $lines = 0
    $lastModified = $null
    if ($exists) {
        try {
            $item = Get-Item -LiteralPath $path
            $lastModified = $item.LastWriteTime.ToString('yyyy-MM-dd HH:mm:ss')
            $lines = Get-TextLineCount -Path $path
        } catch {}
    }
    return [ordered]@{
        name          = $Name
        exists        = $exists
        lines         = $lines
        last_modified = $lastModified
    }
}

function Get-LatestHandoffInfo {
    param([string]$ProjectRoot)
    $files = @(Get-ChildItem -LiteralPath $ProjectRoot -Filter 'handoff-*.md' -File -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending)
    if ($files.Count -eq 0) {
        return [ordered]@{
            exists        = $false
            name          = $null
            lines         = 0
            last_modified = $null
        }
    }
    $latest = $files[0]
    return [ordered]@{
        exists        = $true
        name          = $latest.Name
        lines         = (Get-TextLineCount -Path $latest.FullName)
        last_modified = $latest.LastWriteTime.ToString('yyyy-MM-dd HH:mm:ss')
    }
}

function Get-ProgressArchiveCandidateCount {
    param([string]$ProjectRoot)
    $path = Join-Path $ProjectRoot 'progress.md'
    if (-not (Test-Path -LiteralPath $path)) { return 0 }
    try {
        $lines = [System.IO.File]::ReadAllLines($path, [System.Text.Encoding]::UTF8)
    } catch {
        return 0
    }
    $count = 0
    foreach ($line in $lines) {
        if ($line -match '^\s*-\s+\d{4}-\d{2}-\d{2}:.*(DONE|done)') {
            $count++
        }
    }
    return $count
}

# --- Validate command ---
if ($allowedCommands -notcontains $Command) {
    Write-Envelope -Ok $false -Cmd $Command -Det $Detector -Data $null -Error 'INVALID_COMMAND'
    exit 0
}

# --- Validate detector ---
if ($Detector -ne '') {
    if ($Detector -notmatch '^[a-z_]+$') {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'INJECTION_BLOCKED'
        exit 0
    }
    if ($allowedDetectors -notcontains $Detector) {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'INVALID_DETECTOR'
        exit 0
    }
}

# --- Check binary ---
if (-not (Test-Path $binary)) {
    Write-Envelope -Ok $false -Cmd $Command -Det $Detector -Data $null -Error 'CONTROLLER_ERROR: binary not found'
    exit 0
}

# --- Resolve controller root + work_dir ---
$activeProjectFile = Join-Path $controllerRoot 'active_project.json'
$workDir = $script:defaultWorkDir
if (Test-Path $activeProjectFile) {
    try {
        $proj = [System.IO.File]::ReadAllText($activeProjectFile, [System.Text.Encoding]::UTF8) | ConvertFrom-Json
        if ($proj.work_dir -ne '' -and (Test-Path $proj.work_dir)) {
            $workDir = $proj.work_dir
        }
    } catch {}
}
$env:CC_RESEARCH_MONITOR_ROOT = $workDir
$env:CC_WORK_DIR = $workDir

# --- work_dir whitelist (#88 security hardening) ---
$allowedWorkDirRoots = @($script:projectRoot, $script:defaultWorkDir)
if ($env:CC_ALLOWED_WORK_ROOTS -ne $null -and $env:CC_ALLOWED_WORK_ROOTS.Trim() -ne '') {
    $allowedWorkDirRoots = $env:CC_ALLOWED_WORK_ROOTS -split ';' | Where-Object { $_.Trim() -ne '' }
}
$commandsUsingWorkDir = @('research-monitor', 'submit-review')
if ($commandsUsingWorkDir -contains $Command) {
    $normalizedWorkDir = $workDir.Replace('/', '\').TrimEnd('\')
    $isAllowed = $false
    foreach ($root in $allowedWorkDirRoots) {
        $normalizedRoot = $root.Replace('/', '\').TrimEnd('\')
        if ($normalizedWorkDir -eq $normalizedRoot -or $normalizedWorkDir.StartsWith("$normalizedRoot\", [System.StringComparison]::OrdinalIgnoreCase)) {
            $isAllowed = $true
            break
        }
    }
    if (-not $isAllowed) {
        Write-Envelope -Ok $false -Cmd $Command -Det $Detector -Data $null -Error 'WORK_DIR_BLOCKED'
        exit 0
    }
}

# --- Helper: run cc-controller binary ---
function Invoke-Controller {
    param([string[]]$ArgList)
    try {
        $psi = [System.Diagnostics.ProcessStartInfo]::new()
        $psi.FileName = $binary
        $psi.UseShellExecute = $false
        $psi.RedirectStandardOutput = $true
        $psi.RedirectStandardError = $true
        $psi.CreateNoWindow = $true
        $psi.Arguments = (($ArgList | ForEach-Object { Quote-ProcessArg $_ }) -join ' ')
        $p = [System.Diagnostics.Process]::new()
        $p.StartInfo = $psi
        [void]$p.Start()
    } catch {
        return @{ ok = $false; error = 'CONTROLLER_ERROR: failed to start'; stdout = ''; exitCode = -1 }
    }
    $waited = $p.WaitForExit($timeoutSeconds * 1000)
    if (-not $waited) {
        try { $p.Kill() } catch {}
        try { $p.WaitForExit() } catch {}
        return @{ ok = $false; error = 'TIMEOUT'; stdout = ''; exitCode = -1 }
    }
    $out = $p.StandardOutput.ReadToEnd().Trim()
    $err = $p.StandardError.ReadToEnd().Trim()
    $ec = $p.ExitCode
    if ($ec -ne 0) {
        $detail = "CONTROLLER_ERROR: exit code $ec"
        if ($err -ne '') { $detail = "${detail}: ${err}" }
        return @{ ok = $false; error = $detail; stdout = $out; exitCode = $ec }
    }
    return @{ ok = $true; error = ''; stdout = $out; exitCode = 0 }
}

# --- Handle: research-monitor ---
if ($Command -eq 'research-monitor') {
    $argList = @('research-monitor', '--format', 'json')
    if ($Detector -ne '') { $argList += @('--detector', $Detector) }
    $r = Invoke-Controller $argList
    if (-not $r.ok) {
        Write-Envelope -Ok $false -Cmd $Command -Det $Detector -Data $null -Error $r.error
        exit 0
    }
    try {
        $data = $r.stdout | ConvertFrom-Json
    } catch {
        Write-Envelope -Ok $false -Cmd $Command -Det $Detector -Data $null -Error 'CONTROLLER_ERROR: invalid JSON from cc-controller'
        exit 0
    }
    Write-Envelope -Ok $true -Cmd $Command -Det $Detector -Data $data -Error ''
    exit 0
}

# --- Handle: system-status ---
if ($Command -eq 'system-status') {
    $r = Invoke-Controller @('status')
    if (-not $r.ok) {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error $r.error
        exit 0
    }
    $data = [ordered]@{ text = $r.stdout }
    Write-Envelope -Ok $true -Cmd $Command -Det '' -Data $data -Error ''
    exit 0
}

# --- Handle: memory-status ---
if ($Command -eq 'memory-status') {
    $projectRoot = $script:projectRoot
    $progressInfo = Get-MemoryFileInfo -ProjectRoot $projectRoot -Name 'progress.md'
    $memoryIndexInfo = Get-MemoryFileInfo -ProjectRoot $projectRoot -Name 'memory-index.md'
    $detectorLogInfo = Get-MemoryFileInfo -ProjectRoot $projectRoot -Name 'detector-learning-log.md'
    $skillsAuditInfo = Get-MemoryFileInfo -ProjectRoot $projectRoot -Name 'skills-audit.md'
    $archiveInfo = Get-MemoryFileInfo -ProjectRoot $projectRoot -Name 'progress.archive.md'
    $latestHandoff = Get-LatestHandoffInfo -ProjectRoot $projectRoot
    $archiveCandidates = Get-ProgressArchiveCandidateCount -ProjectRoot $projectRoot

    $progressLines = [int]$progressInfo.lines
    $noise = 'low'
    if ($progressLines -gt 220 -or $archiveCandidates -gt 12) {
        $noise = 'high'
    } elseif ($progressLines -gt 120 -or $archiveCandidates -gt 5) {
        $noise = 'medium'
    }

    $gaps = @()
    if (-not $progressInfo.exists) { $gaps += 'progress.md missing' }
    if (-not $latestHandoff.exists) { $gaps += 'no handoff-YYYY-MM-DD.md found' }
    if (-not $memoryIndexInfo.exists) { $gaps += 'memory-index.md missing' }
    if (-not $detectorLogInfo.exists) { $gaps += 'detector-learning-log.md missing' }
    if (-not $skillsAuditInfo.exists) { $gaps += 'skills-audit.md missing' }

    $recommendations = @()
    if ($noise -eq 'high') {
        $recommendations += 'Run /璁板繂褰掓。 with Codex/progress-recorder before major new work.'
    } elseif ($noise -eq 'medium') {
        $recommendations += 'Archive stale completed items soon.'
    } else {
        $recommendations += 'No immediate archive required.'
    }
    if ($gaps.Count -gt 0) {
        $recommendations += 'Resolve missing memory files or confirm they are intentionally absent.'
    }

    $data = [ordered]@{
        project_root             = $projectRoot
        progress                 = $progressInfo
        latest_handoff           = $latestHandoff
        memory_index             = $memoryIndexInfo
        detector_learning_log    = $detectorLogInfo
        skills_audit             = $skillsAuditInfo
        progress_archive         = $archiveInfo
        archive_candidates_count = $archiveCandidates
        noise_assessment         = $noise
        gaps                     = $gaps
        recommendations          = $recommendations
    }
    Write-Envelope -Ok $true -Cmd $Command -Det '' -Data $data -Error ''
    exit 0
}

# --- Handle: memory-record ---
if ($Command -eq 'memory-record') {
    $projectRoot = $script:projectRoot
    $today = (Get-Date).ToString('yyyy-MM-dd')
    $now = Get-Date

    # Gather file info
    $progressInfo = Get-MemoryFileInfo -ProjectRoot $projectRoot -Name 'progress.md'
    $memoryIndexInfo = Get-MemoryFileInfo -ProjectRoot $projectRoot -Name 'memory-index.md'
    $detectorLogInfo = Get-MemoryFileInfo -ProjectRoot $projectRoot -Name 'detector-learning-log.md'
    $skillsAuditInfo = Get-MemoryFileInfo -ProjectRoot $projectRoot -Name 'skills-audit.md'

    # Check for today's handoff
    $todayHandoffName = "handoff-${today}.md"
    $todayHandoffPath = Join-Path $projectRoot $todayHandoffName
    $handoffTodayInfo = [ordered]@{
        exists        = $false
        name          = $todayHandoffName
        lines         = 0
        last_modified = $null
    }
    if (Test-Path -LiteralPath $todayHandoffPath) {
        $handoffTodayInfo.exists = $true
        try {
            $item = Get-Item -LiteralPath $todayHandoffPath
            $handoffTodayInfo.last_modified = $item.LastWriteTime.ToString('yyyy-MM-dd HH:mm:ss')
            $handoffTodayInfo.lines = Get-TextLineCount -Path $todayHandoffPath
        } catch {}
    } else {
        # Find latest handoff as fallback info
        $latestHandoff = Get-LatestHandoffInfo -ProjectRoot $projectRoot
        if ($latestHandoff.exists) {
            $handoffTodayInfo.name = $latestHandoff.name
            $handoffTodayInfo.last_modified = $latestHandoff.last_modified
            $handoffTodayInfo.lines = $latestHandoff.lines
        }
    }

    # Compute staleness helper
    function Get-DaysStale {
        param([string]$LastModified)
        if ([string]::IsNullOrWhiteSpace($LastModified)) { return $null }
        try {
            $modDate = [datetime]::ParseExact($LastModified, 'yyyy-MM-dd HH:mm:ss', $null)
            return [int][math]::Floor(($now - $modDate).TotalDays)
        } catch {
            return $null
        }
    }

    # Build file entries with staleness
    $progressDays = Get-DaysStale -LastModified $progressInfo.last_modified
    $progressNeedsUpdate = $false
    if (-not $progressInfo.exists -or $null -eq $progressDays -or $progressDays -gt 0) {
        $progressNeedsUpdate = $true
    }

    $memoryIndexDays = Get-DaysStale -LastModified $memoryIndexInfo.last_modified
    $detectorLogDays = Get-DaysStale -LastModified $detectorLogInfo.last_modified
    $skillsAuditDays = Get-DaysStale -LastModified $skillsAuditInfo.last_modified

    $handoffTodayExists = (Test-Path -LiteralPath $todayHandoffPath)

    # Determine actions needed
    $actionsNeeded = @()
    if (-not $handoffTodayExists) {
        $actionsNeeded += 'needs new handoff today'
    }
    if ($progressNeedsUpdate) {
        $actionsNeeded += 'progress.md not updated today'
    }

    # Check stale files (>0 days)
    if ($null -ne $memoryIndexDays -and $memoryIndexDays -gt 0) {
        $actionsNeeded += "memory-index.md ${memoryIndexDays}d stale"
    }
    if ($null -ne $detectorLogDays -and $detectorLogDays -gt 0) {
        $actionsNeeded += "detector-learning-log.md ${detectorLogDays}d stale"
    }
    if ($null -ne $skillsAuditDays -and $skillsAuditDays -gt 0) {
        $actionsNeeded += "skills-audit.md ${skillsAuditDays}d stale"
    }

    # Build recommendation
    if ($actionsNeeded.Count -eq 0) {
        $recommendation = 'all_fresh'
    } elseif ($actionsNeeded.Count -le 2) {
        $recommendation = 'minor_updates'
    } else {
        $recommendation = 'full_refresh'
    }

    $filesData = [ordered]@{
        progress = [ordered]@{
            exists        = $progressInfo.exists
            lines         = $progressInfo.lines
            last_modified = $progressInfo.last_modified
            days_stale    = $progressDays
            needs_update  = $progressNeedsUpdate
        }
        handoff_today = [ordered]@{
            exists        = $handoffTodayExists
            name          = $handoffTodayInfo.name
            lines         = $handoffTodayInfo.lines
            last_modified = $handoffTodayInfo.last_modified
        }
        memory_index = [ordered]@{
            exists        = $memoryIndexInfo.exists
            lines         = $memoryIndexInfo.lines
            last_modified = $memoryIndexInfo.last_modified
            days_stale    = $memoryIndexDays
        }
        detector_log = [ordered]@{
            exists        = $detectorLogInfo.exists
            lines         = $detectorLogInfo.lines
            last_modified = $detectorLogInfo.last_modified
            days_stale    = $detectorLogDays
        }
        skills_audit = [ordered]@{
            exists        = $skillsAuditInfo.exists
            lines         = $skillsAuditInfo.lines
            last_modified = $skillsAuditInfo.last_modified
            days_stale    = $skillsAuditDays
        }
    }

    $data = [ordered]@{
        today          = $today
        files          = $filesData
        actions_needed = $actionsNeeded
        recommendation = $recommendation
    }
    Write-Envelope -Ok $true -Cmd $Command -Det '' -Data $data -Error ''
    exit 0
}

# --- Handle: memory-recap ---
if ($Command -eq 'memory-recap') {
    $projectRoot = $script:projectRoot

    # --- Latest handoff ---
    $handoffName = $null
    $handoffLines = 0
    $handoffContent = $null
    $hasTodayHandoff = $false
    $today = (Get-Date).ToString('yyyy-MM-dd')

    $handoffFiles = @(Get-ChildItem -LiteralPath $projectRoot -Filter 'handoff-*.md' -File -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending)
    if ($handoffFiles.Count -gt 0) {
        $latestHandoff = $handoffFiles[0]
        $handoffName = $latestHandoff.Name
        $handoffPath = $latestHandoff.FullName
        try {
            $rawHandoff = [System.IO.File]::ReadAllText($handoffPath, [System.Text.Encoding]::UTF8)
            $handoffLines = ([System.IO.File]::ReadAllLines($handoffPath, [System.Text.Encoding]::UTF8)).Count
            if ($rawHandoff.Length -gt 3000) {
                $handoffContent = $rawHandoff.Substring(0, 3000) + '...(truncated)'
            } else {
                $handoffContent = $rawHandoff
            }
        } catch {
            $handoffContent = $null
        }
        if ($handoffName -match "handoff-$today") {
            $hasTodayHandoff = $true
        }
    }

    # --- Progress active sections ---
    $progressLines = 0
    $progressActiveContent = $null
    $progressPath = Join-Path $projectRoot 'progress.md'
    if (Test-Path -LiteralPath $progressPath) {
        try {
            $allProgressLines = [System.IO.File]::ReadAllLines($progressPath, [System.Text.Encoding]::UTF8)
            $progressLines = $allProgressLines.Count
            $activeLines = @()
            foreach ($pLine in $allProgressLines) {
                if ($pLine -match '(?i)^#+\s*(archive|completed|done|history|褰掓。|宸插畬鎴恷鍘嗗彶)') {
                    break
                }
                $activeLines += $pLine
            }
            $activeText = $activeLines -join "`n"
            if ($activeText.Length -gt 2000) {
                $progressActiveContent = $activeText.Substring(0, 2000) + '...(truncated)'
            } else {
                $progressActiveContent = $activeText
            }
        } catch {
            $progressActiveContent = $null
        }
    }

    $data = [ordered]@{
        handoff_name            = $handoffName
        handoff_lines           = $handoffLines
        handoff_content         = $handoffContent
        progress_lines          = $progressLines
        progress_active_content = $progressActiveContent
        has_today_handoff       = $hasTodayHandoff
    }
    Write-Envelope -Ok $true -Cmd $Command -Det '' -Data $data -Error ''
    exit 0
}

# --- Handle: memory-archive ---
if ($Command -eq 'memory-archive') {
    $projectRoot = $script:projectRoot
    $progressPath = Join-Path $projectRoot 'progress.md'

    $progressInfo = Get-MemoryFileInfo -ProjectRoot $projectRoot -Name 'progress.md'
    $archiveInfo = Get-MemoryFileInfo -ProjectRoot $projectRoot -Name 'progress.archive.md'

    $candidates = @()
    if (Test-Path -LiteralPath $progressPath) {
        try {
            $allLines = [System.IO.File]::ReadAllLines($progressPath, [System.Text.Encoding]::UTF8)
        } catch {
            $allLines = @()
        }
        $now = Get-Date
        $staleThresholdDays = 14
        for ($i = 0; $i -lt $allLines.Count; $i++) {
            $line = $allLines[$i]
            # Check completed items
            if ($line -match '^\s*-\s+\d{4}-\d{2}-\d{2}:.*(DONE|done|completed|Completed)') {
                $text = $line.Trim()
                if ($text.Length -gt 120) { $text = $text.Substring(0, 120) + '...' }
                $candidates += [ordered]@{
                    line_number = $i + 1
                    text        = $text
                    reason      = 'completed'
                }
                continue
            }
            # Check stale items (date older than 14 days)
            if ($line -match '^\s*-\s+(\d{4}-\d{2}-\d{2}):') {
                $dateStr = $Matches[1]
                try {
                    $entryDate = [datetime]::ParseExact($dateStr, 'yyyy-MM-dd', $null)
                    $ageDays = [int][math]::Floor(($now - $entryDate).TotalDays)
                    if ($ageDays -gt $staleThresholdDays) {
                        $text = $line.Trim()
                        if ($text.Length -gt 120) { $text = $text.Substring(0, 120) + '...' }
                        $candidates += [ordered]@{
                            line_number = $i + 1
                            text        = $text
                            reason      = "stale ($ageDays days)"
                        }
                    }
                } catch {}
            }
        }
    }

    $candidateCount = $candidates.Count
    if ($candidateCount -eq 0) {
        $recommendation = 'No archive candidates found in progress.md'
    } else {
        $recommendation = "$candidateCount items can be archived to progress.archive.md"
    }

    $data = [ordered]@{
        progress_lines         = [int]$progressInfo.lines
        progress_last_modified = $progressInfo.last_modified
        archive_lines          = [int]$archiveInfo.lines
        archive_last_modified  = $archiveInfo.last_modified
        candidates             = @($candidates)
        candidate_count        = $candidateCount
        recommendation         = $recommendation
    }
    Write-Envelope -Ok $true -Cmd $Command -Det '' -Data $data -Error ''
    exit 0
}

# --- Handle: memory-archive-execute ---
if ($Command -eq 'memory-archive-execute') {
    $projectRoot = $script:projectRoot
    $progressPath = Join-Path $projectRoot 'progress.md'
    $archivePath = Join-Path $projectRoot 'progress.archive.md'

    if (-not (Test-Path -LiteralPath $progressPath)) {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'CONTROLLER_ERROR: progress.md not found'
        exit 0
    }

    try {
        $allLines = [System.IO.File]::ReadAllLines($progressPath, [System.Text.Encoding]::UTF8)
    } catch {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'CONTROLLER_ERROR: failed to read progress.md'
        exit 0
    }

    $now = Get-Date
    $staleThresholdDays = 14
    $candidateLineNumbers = @()
    $archivedEntries = @()

    for ($i = 0; $i -lt $allLines.Count; $i++) {
        $line = $allLines[$i]
        $isCandidate = $false
        $reason = ''
        if ($line -match '^\s*-\s+\d{4}-\d{2}-\d{2}:.*(DONE|done|completed|Completed)') {
            $isCandidate = $true
            $reason = 'completed'
        } elseif ($line -match '^\s*-\s+(\d{4}-\d{2}-\d{2}):') {
            $dateStr = $Matches[1]
            try {
                $entryDate = [datetime]::ParseExact($dateStr, 'yyyy-MM-dd', $null)
                $ageDays = [int][math]::Floor(($now - $entryDate).TotalDays)
                if ($ageDays -gt $staleThresholdDays) {
                    $isCandidate = $true
                    $reason = "stale ($ageDays days)"
                }
            } catch {}
        }
        if ($isCandidate) {
            $candidateLineNumbers += $i
            $text = $line.Trim()
            if ($text.Length -gt 120) { $text = $text.Substring(0, 120) + '...' }
            $archivedEntries += [ordered]@{
                line_number = $i + 1
                text        = $text
                reason      = $reason
            }
        }
    }

    if ($candidateLineNumbers.Count -eq 0) {
        Write-Envelope -Ok $true -Cmd $Command -Det '' -Data ([ordered]@{
            archived_count       = 0
            progress_lines_before = $allLines.Count
            progress_lines_after  = $allLines.Count
            archive_lines_before  = (Get-TextLineCount -Path $archivePath)
            archive_lines_after   = (Get-TextLineCount -Path $archivePath)
            entries               = @()
            message               = 'No candidates to archive'
        }) -Error ''
        exit 0
    }

    $archiveBlock = @()
    $archiveBlock += ''
    $archiveBlock += "## Archived $($now.ToString('yyyy-MM-dd HH:mm'))"
    $archiveBlock += ''
    foreach ($idx in $candidateLineNumbers) {
        $archiveBlock += $allLines[$idx]
    }

    $archiveLinesBefore = Get-TextLineCount -Path $archivePath
    try {
        $appendText = [string]::Join("`n", $archiveBlock) + "`n"
        [System.IO.File]::AppendAllText($archivePath, $appendText, [System.Text.Encoding]::UTF8)
    } catch {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'CONTROLLER_ERROR: failed to write progress.archive.md'
        exit 0
    }

    $keepLines = @()
    for ($i = 0; $i -lt $allLines.Count; $i++) {
        if ($candidateLineNumbers -notcontains $i) {
            $keepLines += $allLines[$i]
        }
    }
    try {
        [System.IO.File]::WriteAllLines($progressPath, $keepLines, [System.Text.Encoding]::UTF8)
    } catch {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'CONTROLLER_ERROR: failed to update progress.md'
        exit 0
    }

    $archiveLinesAfter = Get-TextLineCount -Path $archivePath
    $data = [ordered]@{
        archived_count        = $candidateLineNumbers.Count
        progress_lines_before = $allLines.Count
        progress_lines_after  = $keepLines.Count
        archive_lines_before  = $archiveLinesBefore
        archive_lines_after   = $archiveLinesAfter
        entries               = @($archivedEntries)
        message               = "$($candidateLineNumbers.Count) items archived successfully"
    }
    Write-Envelope -Ok $true -Cmd $Command -Det '' -Data $data -Error ''
    exit 0
}

# --- Handle: submit-review ---
if ($Command -eq 'submit-review') {
    if ($TaskText -eq '' -or $TaskText.Length -lt 2) {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'INVALID_TASK: task text required (min 2 chars)'
        exit 0
    }
    if ($TaskText.Length -gt 500) {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'INVALID_TASK: task text too long (max 500 chars)'
        exit 0
    }
    $submitScript = Join-Path $controllerRoot 'bin\submit-plan-review.ps1'
    if (-not (Test-Path $submitScript)) {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'CONTROLLER_ERROR: submit-plan-review.ps1 not found'
        exit 0
    }
    try {
        $psi = [System.Diagnostics.ProcessStartInfo]::new()
        $psi.FileName = 'powershell.exe'
        $psi.UseShellExecute = $false
        $psi.RedirectStandardOutput = $true
        $psi.RedirectStandardError = $true
        $psi.CreateNoWindow = $true
        $psi.WorkingDirectory = $controllerRoot
        $psi.EnvironmentVariables['CC_WORK_DIR'] = $workDir
        $psi.EnvironmentVariables['CC_RESEARCH_MONITOR_ROOT'] = $workDir
        $escapedScript = Quote-ProcessArg $submitScript
        $escapedTask = Quote-ProcessArg $TaskText
        $psi.Arguments = "-NoProfile -ExecutionPolicy Bypass -File $escapedScript $escapedTask"
        $p = [System.Diagnostics.Process]::new()
        $p.StartInfo = $psi
        [void]$p.Start()
        $waited = $p.WaitForExit($timeoutSeconds * 1000)
        if (-not $waited) {
            try { $p.Kill() } catch {}
            Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'TIMEOUT'
            exit 0
        }
        $out = $p.StandardOutput.ReadToEnd().Trim()
        $ec = $p.ExitCode
    } catch {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'CONTROLLER_ERROR: failed to start review'
        exit 0
    }
    if ($ec -ne 0) {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error "CONTROLLER_ERROR: submit exit code $ec"
        exit 0
    }
    $foundRunId = ''
    if ($out -match '(\d{8}-\d{6}-plan-review)') {
        $foundRunId = $Matches[1]
    }
    if ($foundRunId -ne '') {
        $markerPath = Join-Path (Join-Path $controllerRoot "runs\$foundRunId") $astrbotReviewMarker
        $marker = [ordered]@{
            source     = 'astrbot'
            command    = 'submit-review'
            run_id     = $foundRunId
            task       = $TaskText
            created_at = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
        }
        try {
            $marker | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 -LiteralPath $markerPath
        } catch {
            Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'CONTROLLER_ERROR: failed to write review marker'
            exit 0
        }
    }
    $data = [ordered]@{
        run_id  = $foundRunId
        message = 'review submitted'
        task    = $TaskText
    }
    Write-Envelope -Ok $true -Cmd $Command -Det '' -Data $data -Error ''
    exit 0
}

# --- Handle: show-review ---
if ($Command -eq 'show-review') {
    $runsDir = Join-Path $controllerRoot 'runs'
    $targetRunId = $RunId
    if ($targetRunId -eq '') {
        $planDirs = Get-ChildItem -Directory -Path $runsDir -Filter '*-plan-review*' -ErrorAction SilentlyContinue
        $planDirs = $planDirs |
            Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName $astrbotReviewMarker) } |
            Sort-Object Name -Descending |
            Select-Object -First 1
        if ($null -eq $planDirs) {
            Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'NO_REVIEWS: no AstrBot review runs found'
            exit 0
        }
        $targetRunId = $planDirs.Name
    } else {
        if ($targetRunId -notmatch $runIdPattern) {
            Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error 'INVALID_RUN_ID'
            exit 0
        }
    }
    $runDir = Join-Path $runsDir $targetRunId
    if (-not (Test-Path $runDir)) {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error "NOT_FOUND: run $targetRunId not found"
        exit 0
    }
    if (-not (Test-Path -LiteralPath (Join-Path $runDir $astrbotReviewMarker))) {
        Write-Envelope -Ok $false -Cmd $Command -Det '' -Data $null -Error "NOT_FOUND: run $targetRunId was not submitted through AstrBot"
        exit 0
    }
    $rvStatus = 'unknown'
    $rvStage = Get-ReviewStage -RunDir $runDir
    $verdict = 'UNKNOWN'
    $statusFile = Join-Path $runDir 'status.json'
    if (Test-Path $statusFile) {
        try {
            $sj = [System.IO.File]::ReadAllText($statusFile, [System.Text.Encoding]::UTF8) | ConvertFrom-Json
            $rvStatus = $sj.status
            $rvStage = $sj.stage
        } catch {}
    }
    $exitCodeFile = Join-Path $runDir 'runner.exitcode.txt'
    if (Test-Path $exitCodeFile) {
        $ecText = ([System.IO.File]::ReadAllText($exitCodeFile, [System.Text.Encoding]::UTF8)).Trim()
        if ($ecText -eq '0') { $rvStatus = 'completed' } else { $rvStatus = 'failed' }
    } elseif (Test-Path (Join-Path $runDir 'runner.pid')) {
        $rvStatus = 'running'
    }
    $codexReviewFile = Join-Path $runDir 'codex-review.md'
    if (Test-Path $codexReviewFile) {
        $rv = [System.IO.File]::ReadAllText($codexReviewFile, [System.Text.Encoding]::UTF8)
        if ($rv -match '(?i)\bBLOCK\b') { $verdict = 'BLOCK' }
        elseif ($rv -match '(?i)\bREVISE\b') { $verdict = 'REVISE' }
        elseif ($rv -match '(?i)\bAPPROVE\b') { $verdict = 'APPROVE' }
    }
    $taskContent = ''
    $taskFile = Join-Path $runDir 'incoming-task.md'
    if (Test-Path $taskFile) {
        $taskContent = ([System.IO.File]::ReadAllText($taskFile, [System.Text.Encoding]::UTF8)).Trim()
    }
    $summaryContent = ''
    $summaryFile = Join-Path $runDir 'summary.md'
    if (Test-Path $summaryFile) {
        $summaryContent = ([System.IO.File]::ReadAllText($summaryFile, [System.Text.Encoding]::UTF8)).Trim()
        if ($summaryContent.Length -gt 2000) {
            $summaryContent = $summaryContent.Substring(0, 2000) + '...(truncated)'
        }
    }
    $failure = Get-ReviewFailure -RunDir $runDir -Stage $rvStage -Status $rvStatus
    $data = [ordered]@{
        run_id         = $targetRunId
        status         = $rvStatus
        stage          = $rvStage
        verdict        = $verdict
        failure_reason = $failure.reason
        failure_stage  = $failure.stage
        next_step      = $failure.next_step
        task           = $taskContent
        summary        = $summaryContent
    }
    Write-Envelope -Ok $true -Cmd $Command -Det '' -Data $data -Error ''
    exit 0
}

# --- Handle: review-stats ---
if ($Command -eq 'review-stats') {
    $runsDir = Join-Path $controllerRoot 'runs'
    $allDirs = Get-ChildItem -Directory -Path $runsDir -Filter '*-plan-review*' -ErrorAction SilentlyContinue
    $astrDirs = @()
    if ($null -ne $allDirs) {
        $astrDirs = @($allDirs | Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName $astrbotReviewMarker) })
    }

    $total = $astrDirs.Count
    $completedCount = 0
    $failedCount = 0
    $runningCount = 0
    $verdicts = [ordered]@{ APPROVE = 0; REVISE = 0; BLOCK = 0 }
    $failureStages = @{}
    $durations = @()
    $recentRuns = @()

    $sorted = @($astrDirs | Sort-Object Name -Descending)
    foreach ($dir in $sorted) {
        $runDir = $dir.FullName
        $rId = $dir.Name

        $status = 'unknown'
        $ecFile = Join-Path $runDir 'runner.exitcode.txt'
        if (Test-Path $ecFile) {
            $ecVal = ([System.IO.File]::ReadAllText($ecFile, [System.Text.Encoding]::UTF8)).Trim()
            if ($ecVal -eq '0') { $status = 'completed' } else { $status = 'failed' }
        } elseif (Test-Path (Join-Path $runDir 'runner.pid')) {
            $status = 'running'
        }

        if ($status -eq 'completed') { $completedCount++ }
        elseif ($status -eq 'failed') { $failedCount++ }
        elseif ($status -eq 'running') { $runningCount++ }

        $verd = 'UNKNOWN'
        $crFile = Join-Path $runDir 'codex-review.md'
        if (Test-Path $crFile) {
            $crText = [System.IO.File]::ReadAllText($crFile, [System.Text.Encoding]::UTF8)
            if ($crText -match '(?i)\bBLOCK\b') { $verd = 'BLOCK' }
            elseif ($crText -match '(?i)\bREVISE\b') { $verd = 'REVISE' }
            elseif ($crText -match '(?i)\bAPPROVE\b') { $verd = 'APPROVE' }
        }
        if ($verdicts.Contains($verd)) { $verdicts[$verd]++ }

        if ($status -eq 'failed') {
            $fStage = Get-ReviewStage -RunDir $runDir
            $evFail = Get-EventFailure -RunDir $runDir
            if ($null -ne $evFail -and $evFail.stage -ne '') { $fStage = $evFail.stage }
            if ($failureStages.ContainsKey($fStage)) { $failureStages[$fStage]++ }
            else { $failureStages[$fStage] = 1 }
        }

        $createdAt = $null
        $mFile = Join-Path $runDir $astrbotReviewMarker
        try {
            $mJson = [System.IO.File]::ReadAllText($mFile, [System.Text.Encoding]::UTF8) | ConvertFrom-Json
            $createdAt = [datetime]::Parse($mJson.created_at).ToUniversalTime()
        } catch {}

        $durSec = $null
        if ($null -ne $createdAt) {
            if (($status -eq 'completed' -or $status -eq 'failed') -and (Test-Path $ecFile)) {
                $endTime = (Get-Item -LiteralPath $ecFile).LastWriteTimeUtc
                $durSec = [int]($endTime - $createdAt).TotalSeconds
                if ($durSec -lt 0) { $durSec = $null }
            } elseif ($status -eq 'running') {
                $durSec = [int]((Get-Date).ToUniversalTime() - $createdAt).TotalSeconds
            }
        }
        if ($null -ne $durSec -and $status -eq 'completed') {
            $durations += $durSec
        }

        $taskSnippet = ''
        $tFile = Join-Path $runDir 'incoming-task.md'
        if (Test-Path $tFile) {
            $taskSnippet = ([System.IO.File]::ReadAllText($tFile, [System.Text.Encoding]::UTF8)).Trim()
            $taskSnippet = Redact-ReviewText $taskSnippet
            if ($taskSnippet.Length -gt 80) { $taskSnippet = $taskSnippet.Substring(0, 80) + '...' }
        }

        if ($recentRuns.Count -lt 5) {
            $entry = [ordered]@{
                run_id           = $rId
                status           = $status
                verdict          = $verd
                task             = $taskSnippet
                duration_seconds = $durSec
            }
            $recentRuns += $entry
        }
    }

    $avgDuration = $null
    if ($durations.Count -gt 0) {
        $dSum = 0; foreach ($d in $durations) { $dSum += $d }
        $avgDuration = [int]($dSum / $durations.Count)
    }

    $fsOrdered = [ordered]@{}
    foreach ($k in ($failureStages.Keys | Sort-Object)) { $fsOrdered[$k] = $failureStages[$k] }

    $statsData = [ordered]@{
        total                = $total
        completed            = $completedCount
        failed               = $failedCount
        running              = $runningCount
        verdicts             = $verdicts
        failure_stages       = $fsOrdered
        avg_duration_seconds = $avgDuration
        recent_runs          = @($recentRuns)
    }
    Write-Envelope -Ok $true -Cmd $Command -Det '' -Data $statsData -Error ''
    exit 0
}

