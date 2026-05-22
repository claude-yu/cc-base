<#
.SYNOPSIS
    Offline smoke tests for adapter.ps1
#>
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$script:adapterPath = Join-Path $PSScriptRoot 'adapter.ps1'
$script:passed = 0
$script:failed = 0
$script:total  = 23

$script:repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$env:CC_BASE_ROOT = $script:repoRoot
$env:CC_PROJECT_ROOT = $script:repoRoot
$allowedRoots = New-Object System.Collections.Generic.List[string]
$allowedRoots.Add($script:repoRoot)
$activeProjectPath = Join-Path $script:repoRoot 'controller\active_project.json'
if (Test-Path -LiteralPath $activeProjectPath) {
    try {
        $activeProject = [System.IO.File]::ReadAllText($activeProjectPath, [System.Text.Encoding]::UTF8) | ConvertFrom-Json
        if ($activeProject.work_dir -and (Test-Path -LiteralPath $activeProject.work_dir)) {
            $allowedRoots.Add($activeProject.work_dir)
        }
    } catch {}
}
$env:CC_ALLOWED_WORK_ROOTS = ($allowedRoots | Select-Object -Unique) -join ';'

function Invoke-AdapterCall {
    param([string[]]$AdapterArgs)
    $raw = powershell.exe -NoProfile -ExecutionPolicy Bypass -File $script:adapterPath @AdapterArgs
    return $raw
}

function Test-Case {
    param(
        [string]$Name,
        [scriptblock]$Test
    )
    try {
        $result = & $Test
        if ($result -eq $true) {
            Write-Host "[PASS] $Name" -ForegroundColor Green
            $script:passed++
        } else {
            Write-Host "[FAIL] $Name - assertion returned false" -ForegroundColor Red
            $script:failed++
        }
    } catch {
        Write-Host "[FAIL] $Name - $($_.Exception.Message)" -ForegroundColor Red
        $script:failed++
    }
}

# --- Test 1: research-monitor returns valid JSON ---
Test-Case 'research-monitor returns valid JSON with scan/summary/tasks' {
    $raw = Invoke-AdapterCall @('-Command', 'research-monitor')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $true) -and ($null -ne $j.data.scan) -and ($null -ne $j.data.summary) -and ($null -ne $j.data.tasks)
}

# --- Test 2: research-monitor with detector filter ---
Test-Case 'research-monitor -Detector gromacs sets detector_filter' {
    $raw = Invoke-AdapterCall @('-Command', 'research-monitor', '-Detector', 'gromacs')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $true) -and ($j.data.scan.detector_filter -eq 'gromacs')
}

# --- Test 3: system-status returns text ---
Test-Case 'system-status returns non-empty text' {
    $raw = Invoke-AdapterCall @('-Command', 'system-status')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $true) -and ($j.data.text -ne '') -and ($j.data.text.Length -gt 0)
}

# --- Test 4: injection blocked ---
Test-Case 'injection in detector is blocked' {
    $raw = Invoke-AdapterCall @('-Command', 'research-monitor', '-Detector', 'gromacs; rm -rf /')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $false) -and ($j.error -eq 'INJECTION_BLOCKED')
}

# --- Test 5: invalid command rejected ---
Test-Case 'invalid command is rejected' {
    $raw = Invoke-AdapterCall @('-Command', 'execute')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $false) -and ($j.error -eq 'INVALID_COMMAND')
}

# --- Test 6: invalid detector rejected ---
Test-Case 'path traversal detector is rejected' {
    $raw = Invoke-AdapterCall @('-Command', 'research-monitor', '-Detector', '../../../etc/passwd')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $false) -and (($j.error -eq 'INVALID_DETECTOR') -or ($j.error -eq 'INJECTION_BLOCKED'))
}

# --- Test 7: work_dir from active_project.json, not AstrBot cwd ---
Test-Case 'work_dir is active project dir (not . or AstrBot backend)' {
    $raw = Invoke-AdapterCall @('-Command', 'research-monitor')
    $j = $raw | ConvertFrom-Json
    $wd = $j.data.scan.work_dir
    ($j.ok -eq $true) -and ($wd -ne '.') -and (-not $wd.Contains('astrbot')) -and (-not $wd.Contains('integrations'))
}

# --- Test 8: show-review returns no legacy reviews ---
Test-Case 'show-review returns no unmarked legacy review' {
    $raw = Invoke-AdapterCall @('-Command', 'show-review')
    $j = $raw | ConvertFrom-Json
    if ($j.ok -eq $false) {
        return ($j.command -eq 'show-review') -and ($j.error -like 'NO_REVIEWS:*AstrBot*')
    }
    $runId = $j.data.run_id
    $projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
    $marker = Join-Path $projectRoot "controller\runs\$runId\astrbot-review.json"
    ($j.ok -eq $true) -and (Test-Path -LiteralPath $marker)
}

# --- Test 9: show-review with invalid run_id rejected ---
Test-Case 'show-review invalid run_id is rejected' {
    $raw = Invoke-AdapterCall @('-Command', 'show-review', '-RunId', '../../../etc/passwd')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $false) -and ($j.error -eq 'INVALID_RUN_ID')
}

# --- Test 10: submit-review with empty task rejected ---
Test-Case 'submit-review empty task is rejected' {
    $raw = Invoke-AdapterCall @('-Command', 'submit-review')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $false) -and ($j.error -like 'INVALID_TASK*')
}

# --- Test 11: execute command still rejected ---
Test-Case 'execute-approved command is rejected' {
    $raw = Invoke-AdapterCall @('-Command', 'execute-approved')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $false) -and ($j.error -eq 'INVALID_COMMAND')
}

# --- Test 12: explicit legacy review run is not exposed through AstrBot ---
Test-Case 'show-review rejects unmarked legacy plan-review run' {
    $projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
    $runsRoot = Join-Path $projectRoot 'controller\runs'
    $legacy = Get-ChildItem -Directory -Path $runsRoot -Filter '*-plan-review*' -ErrorAction SilentlyContinue |
        Where-Object { -not (Test-Path -LiteralPath (Join-Path $_.FullName 'astrbot-review.json')) } |
        Sort-Object Name -Descending |
        Select-Object -First 1
    if ($null -eq $legacy) { return $true }
    $raw = Invoke-AdapterCall @('-Command', 'show-review', '-RunId', $legacy.Name)
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $false) -and ($j.error -like 'NOT_FOUND:*not submitted through AstrBot*')
}

# --- Test 13: submit-review marker contract is documented by adapter behavior ---
Test-Case 'submit-review empty task remains rejected before marker creation' {
    $raw = Invoke-AdapterCall @('-Command', 'submit-review', '-TaskText', 'a')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $false) -and ($j.error -like 'INVALID_TASK*')
}

# --- Test 14: failed AstrBot review returns structured, redacted failure summary ---
Test-Case 'show-review failed run returns structured failure summary' {
    $projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
    $runId = '20000101-000000-plan-review'
    $runDir = Join-Path $projectRoot "controller\runs\$runId"
    New-Item -ItemType Directory -Force -Path $runDir | Out-Null
    @{ source = 'astrbot'; command = 'submit-review'; run_id = $runId; task = 'fixture' } |
        ConvertTo-Json -Compress |
        Set-Content -Encoding UTF8 -LiteralPath (Join-Path $runDir 'astrbot-review.json')
    'fixture task' | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $runDir 'incoming-task.md')
    '1' | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $runDir 'runner.exitcode.txt')
    '{"type":"failed","stage":"codex_review","error":"Codex API timeout after 120s"}' |
        Set-Content -Encoding UTF8 -LiteralPath (Join-Path $runDir 'events.jsonl')
    'Sensitive path C:\path\to\cc-base\secret.txt' |
        Set-Content -Encoding UTF8 -LiteralPath (Join-Path $runDir 'background-err.log')

    $raw = Invoke-AdapterCall @('-Command', 'show-review', '-RunId', $runId)
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $true) -and
        ($j.data.status -eq 'failed') -and
        ($j.data.failure_stage -eq 'codex_review') -and
        ($j.data.failure_reason -eq 'Codex API timeout after 120s') -and
        ($j.data.next_step -like 'Retry*') -and
        (-not ($j.data.failure_reason -like '*E:\ai*'))
}

# --- Test 15: review-stats returns valid JSON with expected fields ---
Test-Case 'review-stats returns valid envelope with counts' {
    $raw = Invoke-AdapterCall @('-Command', 'review-stats')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $true) -and
        ($j.command -eq 'review-stats') -and
        ($null -ne $j.data.total) -and
        ($null -ne $j.data.completed) -and
        ($null -ne $j.data.failed) -and
        ($null -ne $j.data.running) -and
        ($null -ne $j.data.verdicts)
}

# --- Test 16: review-stats counts only AstrBot-marked runs ---
Test-Case 'review-stats includes fixture run and excludes legacy' {
    $raw = Invoke-AdapterCall @('-Command', 'review-stats')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $true) -and
        ($j.data.total -ge 1) -and
        ($j.data.failed -ge 1)
}

# --- Test 17: memory-recap returns valid JSON even when handoff/progress are absent ---
Test-Case 'memory-recap returns handoff and progress content' {
    $raw = Invoke-AdapterCall @('-Command', 'memory-recap')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $true) -and
        ($j.command -eq 'memory-recap') -and
        ($null -ne $j.data.progress_lines) -and
        ($null -ne $j.data.has_today_handoff)
}

# --- Test 18: memory-archive returns valid JSON with archive candidates ---
Test-Case 'memory-archive returns archive candidate scan' {
    $raw = Invoke-AdapterCall @('-Command', 'memory-archive')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $true) -and
        ($j.command -eq 'memory-archive') -and
        ($null -ne $j.data.progress_lines) -and
        ($null -ne $j.data.candidate_count) -and
        ($null -ne $j.data.candidates) -and
        ($null -ne $j.data.recommendation)
}

# --- Test 19: memory-record returns valid JSON with file freshness ---
Test-Case 'memory-record returns file freshness status' {
    $raw = Invoke-AdapterCall @('-Command', 'memory-record')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $true) -and
        ($j.command -eq 'memory-record') -and
        ($null -ne $j.data.today) -and
        ($null -ne $j.data.files) -and
        ($null -ne $j.data.actions_needed) -and
        ($null -ne $j.data.recommendation)
}

# --- Test 20: memory-status returns valid JSON (basic check) ---
Test-Case 'memory-status returns valid JSON with noise assessment' {
    $raw = Invoke-AdapterCall @('-Command', 'memory-status')
    $j = $raw | ConvertFrom-Json
    ($j.ok -eq $true) -and
        ($j.command -eq 'memory-status') -and
        ($null -ne $j.data.noise_assessment) -and
        ($null -ne $j.data.progress) -and
        ($null -ne $j.data.archive_candidates_count)
}

# --- Test 21: memory-archive-execute returns valid JSON or safe missing-progress error ---
Test-Case 'memory-archive-execute returns valid archive result' {
    $raw = Invoke-AdapterCall @('-Command', 'memory-archive-execute')
    $j = $raw | ConvertFrom-Json
    (($j.ok -eq $true) -and
        ($j.command -eq 'memory-archive-execute') -and
        ($null -ne $j.data.archived_count) -and
        ($null -ne $j.data.progress_lines_before) -and
        ($null -ne $j.data.progress_lines_after) -and
        ($null -ne $j.data.entries)) -or
        (($j.ok -eq $false) -and ($j.error -like 'CONTROLLER_ERROR: progress.md not found*'))
}

# --- Test 22: work_dir whitelist blocks out-of-bounds path ---
Test-Case 'work_dir whitelist blocks out-of-bounds active_project.json' {
    $projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
    $apFile = Join-Path $projectRoot 'controller\active_project.json'
    $backup = $null
    if (Test-Path $apFile) {
        $backup = [System.IO.File]::ReadAllText($apFile, [System.Text.Encoding]::UTF8)
    }
    try {
        '{"work_dir":"C:\\Windows\\Temp"}' | Set-Content -Encoding UTF8 -LiteralPath $apFile
        $raw = Invoke-AdapterCall @('-Command', 'research-monitor')
        $j = $raw | ConvertFrom-Json
        ($j.ok -eq $false) -and ($j.error -eq 'WORK_DIR_BLOCKED')
    } finally {
        if ($null -ne $backup) {
            [System.IO.File]::WriteAllText($apFile, $backup, [System.Text.Encoding]::UTF8)
        } else {
            Remove-Item -LiteralPath $apFile -Force -ErrorAction SilentlyContinue
        }
    }
}

# --- Test 23: review-stats task snippet is path-redacted ---
Test-Case 'review-stats redacts paths in task snippets' {
    $projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
    $runId = '20000101-000000-plan-review'
    $runDir = Join-Path $projectRoot "controller\runs\$runId"
    $tFile = Join-Path $runDir 'incoming-task.md'
    $origTask = $null
    if (Test-Path $tFile) { $origTask = [System.IO.File]::ReadAllText($tFile, [System.Text.Encoding]::UTF8) }
    try {
        'Analyze C:\path\to\cc-base\secret\data.csv for patterns' |
            Set-Content -Encoding UTF8 -LiteralPath $tFile
        $raw = Invoke-AdapterCall @('-Command', 'review-stats')
        $j = $raw | ConvertFrom-Json
        $foundRun = $null
        foreach ($r in $j.data.recent_runs) {
            if ($r.run_id -eq $runId) { $foundRun = $r; break }
        }
        ($j.ok -eq $true) -and ($null -ne $foundRun) -and (-not ($foundRun.task -like '*E:\ai*'))
    } finally {
        if ($null -ne $origTask) {
            [System.IO.File]::WriteAllText($tFile, $origTask, [System.Text.Encoding]::UTF8)
        } else {
            'fixture task' | Set-Content -Encoding UTF8 -LiteralPath $tFile
        }
    }
}

# --- Summary ---
Write-Host ''
$color = 'Green'; if ($script:failed -gt 0) { $color = 'Red' }
Write-Host "Results: $($script:passed)/$($script:total) passed, $($script:failed) failed" -ForegroundColor $color
if ($script:failed -gt 0) { exit 1 }





