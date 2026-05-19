param(
    [string]$ControllerDir = ""
)
$ErrorActionPreference = 'Continue'
[Console]::OutputEncoding = [System.Text.Encoding]::GetEncoding(936)

# --- Counters ---
$script:passed   = 0
$script:failed   = 0
$script:skipped  = 0
$script:warnings = 0

function Write-OK   { param([string]$Label, [string]$Detail)
    Write-Host "[OK] " -ForegroundColor Green -NoNewline
    Write-Host "${Label}: $Detail"
    $script:passed++
}
function Write-Fail { param([string]$Label, [string]$Detail)
    Write-Host "[!!] " -ForegroundColor Red -NoNewline
    Write-Host "${Label}: $Detail"
    $script:failed++
}
function Write-Warn { param([string]$Label, [string]$Detail)
    Write-Host "[! ] " -ForegroundColor Yellow -NoNewline
    Write-Host "${Label}: $Detail"
    $script:warnings++
}
function Write-Skip { param([string]$Label, [string]$Detail)
    Write-Host "[--] " -ForegroundColor DarkGray -NoNewline
    Write-Host "${Label}: $Detail"
    $script:skipped++
}

# --- Resolve controller dir ---
function Resolve-ControllerDir {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) {
        return $Explicit
    }
    $fromEnv = $env:CC_CONTROLLER_DIR
    if (-not [string]::IsNullOrWhiteSpace($fromEnv)) {
        return $fromEnv
    }
    # Auto-resolve from script location: scripts/check-install.ps1 -> scripts -> cc-base -> (skill root)
    # But real controller is at the deployed location; try going from selfwork_ytl layout
    if ($PSCommandPath) {
        # scripts/check-install.ps1 -> scripts/ -> cc-base/
        $skillRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
        $candidate = Join-Path $skillRoot "controller"
        if (Test-Path (Join-Path $candidate "cc-controller.exe")) {
            return $candidate
        }
        # Also try: the skill might be in ~/.claude/skills/cc-base but controller is deployed elsewhere
        # Check sibling pattern: go up further to find controller/
        $workspace = Split-Path -Parent $skillRoot
        $candidate2 = Join-Path $workspace "controller"
        if (Test-Path (Join-Path $candidate2 "cc-controller.exe")) {
            return $candidate2
        }
    }
    return ""
}

# --- Helper: run command and capture output ---
function Get-CmdVersion {
    param([string]$Cmd, [string[]]$Args, [int]$TimeoutMs = 8000)
    try {
        $psi = New-Object System.Diagnostics.ProcessStartInfo
        $psi.FileName = $Cmd
        $psi.Arguments = $Args -join " "
        $psi.RedirectStandardOutput = $true
        $psi.RedirectStandardError  = $true
        $psi.UseShellExecute = $false
        $psi.CreateNoWindow = $true
        $proc = [System.Diagnostics.Process]::Start($psi)
        # Read async to avoid deadlock on large output
        $stdoutTask = $proc.StandardOutput.ReadToEndAsync()
        $stderrTask = $proc.StandardError.ReadToEndAsync()
        if (-not $proc.WaitForExit($TimeoutMs)) {
            try { $proc.Kill() } catch {}
            return $null
        }
        $stdout = $stdoutTask.Result
        $stderr = $stderrTask.Result
        $output = if (-not [string]::IsNullOrWhiteSpace($stdout)) { $stdout.Trim() } else { $stderr.Trim() }
        if (-not [string]::IsNullOrWhiteSpace($output)) {
            return $output
        }
        return $null
    } catch {
        return $null
    }
}

# --- Parse config commands ---
function Get-ConfigCommands {
    param([string]$ConfigPath)
    $names = @()
    if (-not (Test-Path -LiteralPath $ConfigPath)) { return $names }
    $content = Get-Content -LiteralPath $ConfigPath -Encoding UTF8 -ErrorAction SilentlyContinue
    foreach ($line in $content) {
        if ($line -match '^\s*name\s*=\s*"([^"]+)"') {
            $names += $Matches[1]
        }
    }
    return $names
}

# ====================================================
Write-Host ""
Write-Host "=== cc-base 安装自检 ===" -ForegroundColor Cyan
Write-Host ""

# 1. claude CLI
$claudeVer = Get-CmdVersion "claude" @("--version")
if ($claudeVer) {
    # Extract first line only
    $firstLine = ($claudeVer -split "`n")[0].Trim()
    Write-OK "claude CLI" $firstLine
} else {
    Write-Fail "claude CLI" "not found (npm install -g @anthropic-ai/claude-code)"
}

# 2. codex CLI (optional)
$codexVer = Get-CmdVersion "codex" @("--version")
if ($codexVer) {
    $firstLine = ($codexVer -split "`n")[0].Trim()
    Write-OK "codex CLI" $firstLine
} else {
    Write-Skip "codex CLI" "not found (optional, npm install -g @openai/codex)"
}

# 3. cc-connect
$ccConnectVer = Get-CmdVersion "cc-connect" @("--version")
if ($ccConnectVer) {
    $firstLine = ($ccConnectVer -split "`n")[0].Trim()
    Write-OK "cc-connect" $firstLine
} else {
    Write-Fail "cc-connect" "not found"
}

# 4. cc-controller.exe
$ctrlDir = Resolve-ControllerDir $ControllerDir
$ctrlExe = ""
if (-not [string]::IsNullOrWhiteSpace($ctrlDir)) {
    $ctrlExe = Join-Path $ctrlDir "cc-controller.exe"
}
if ($ctrlExe -and (Test-Path -LiteralPath $ctrlExe)) {
    Write-OK "cc-controller.exe" $ctrlExe
} else {
    $tried = if ($ctrlExe) { $ctrlExe } else { "(could not resolve path; set CC_CONTROLLER_DIR or pass -ControllerDir)" }
    Write-Fail "cc-controller.exe" "not found at $tried"
}

# 5. Go compiler (optional)
$goVer = Get-CmdVersion "go" @("version")
if ($goVer) {
    $firstLine = ($goVer -split "`n")[0].Trim()
    Write-Skip "Go" "$firstLine (only needed for rebuilding)"
} else {
    Write-Skip "Go" "not found (only needed for rebuilding)"
}

# 6. Docker (optional)
$dockerVer = Get-CmdVersion "docker" @("--version")
if ($dockerVer -and $dockerVer -match 'Docker version') {
    $firstLine = ($dockerVer -split "`n")[0].Trim()
    Write-OK "Docker" $firstLine
} elseif ($dockerVer) {
    # docker exists but daemon not responding / unexpected output
    Write-Warn "Docker" "installed but version check failed"
    $dockerVer = $null  # prevent NapCat check from running docker ps
} else {
    Write-Skip "Docker" "not found (only needed for QQ/NapCat)"
}

# 7. NapCat container (optional, only if docker available)
if ($dockerVer) {
    try {
        $containers = & docker ps --format "{{.Names}}" 2>$null
        $napcat = $containers | Where-Object { $_ -match 'napcat' }
        if ($napcat) {
            Write-OK "NapCat container" "running"
        } else {
            Write-Skip "NapCat container" "not running (optional, for QQ)"
        }
    } catch {
        Write-Skip "NapCat container" "docker ps failed (optional)"
    }
} else {
    Write-Skip "NapCat container" "skipped (no Docker)"
}

# 8. Config exists
$configPath = Join-Path $env:USERPROFILE ".cc-connect\config.toml"
if (Test-Path -LiteralPath $configPath) {
    Write-OK "Config" $configPath
} else {
    Write-Fail "Config" "not found at $configPath"
}

# 9. Config core commands
$coreCommands = @("cc", "问codex", "状态", "取消任务", "项目", "切项目", "执行", "监控")
if (Test-Path -LiteralPath $configPath) {
    $allNames = Get-ConfigCommands $configPath
    $foundCount = 0
    $missingList = @()
    foreach ($cmd in $coreCommands) {
        if ($allNames -contains $cmd) {
            $foundCount++
        } else {
            $missingList += $cmd
        }
    }
    $total = $coreCommands.Count
    if ($foundCount -eq $total) {
        Write-OK "Config commands" "$foundCount/$total core commands found"
    } else {
        Write-Fail "Config commands" "$foundCount/$total found, missing: $($missingList -join ', ')"
    }
} else {
    Write-Fail "Config commands" "skipped (config file missing)"
}

# 10. CC_EXECUTE_WORK_DIR
$execWorkDir = $env:CC_EXECUTE_WORK_DIR
if ([string]::IsNullOrWhiteSpace($execWorkDir)) {
    $execWorkDir = [Environment]::GetEnvironmentVariable("CC_EXECUTE_WORK_DIR", "User")
}
if ([string]::IsNullOrWhiteSpace($execWorkDir)) {
    $execWorkDir = [Environment]::GetEnvironmentVariable("CC_EXECUTE_WORK_DIR", "Machine")
}
if (-not [string]::IsNullOrWhiteSpace($execWorkDir)) {
    if (Test-Path -LiteralPath $execWorkDir) {
        Write-OK "CC_EXECUTE_WORK_DIR" "$execWorkDir (exists)"
    } else {
        Write-Fail "CC_EXECUTE_WORK_DIR" "$execWorkDir (directory NOT found)"
    }
} else {
    Write-Fail "CC_EXECUTE_WORK_DIR" "env var not set"
}

# 11. CC_WORK_DIR or active_project.json
$ccWorkDir = $env:CC_WORK_DIR
if ([string]::IsNullOrWhiteSpace($ccWorkDir)) {
    $ccWorkDir = [Environment]::GetEnvironmentVariable("CC_WORK_DIR", "User")
}
if ([string]::IsNullOrWhiteSpace($ccWorkDir)) {
    $ccWorkDir = [Environment]::GetEnvironmentVariable("CC_WORK_DIR", "Machine")
}
$activeProjectJson = ""
if (-not [string]::IsNullOrWhiteSpace($ctrlDir)) {
    $activeProjectJson = Join-Path $ctrlDir "active_project.json"
}
if (-not [string]::IsNullOrWhiteSpace($ccWorkDir)) {
    Write-OK "CC_WORK_DIR" $ccWorkDir
} elseif ($activeProjectJson -and (Test-Path -LiteralPath $activeProjectJson)) {
    Write-OK "active_project.json" $activeProjectJson
} else {
    Write-Warn "CC_WORK_DIR" "not set and no active_project.json found (project commands may fail)"
}

# 12. CC_MODEL (info only)
$ccModel = $env:CC_MODEL
if ([string]::IsNullOrWhiteSpace($ccModel)) {
    $ccModel = [Environment]::GetEnvironmentVariable("CC_MODEL", "User")
}
if ([string]::IsNullOrWhiteSpace($ccModel)) {
    $ccModel = [Environment]::GetEnvironmentVariable("CC_MODEL", "Machine")
}
if (-not [string]::IsNullOrWhiteSpace($ccModel)) {
    Write-OK "CC_MODEL" $ccModel
} else {
    Write-Skip "CC_MODEL" "not set (will use claude CLI default)"
}

# ====================================================
# Summary
Write-Host ""
$total = $script:passed + $script:failed + $script:skipped + $script:warnings
Write-Host "=== Result: $($script:passed)/$total passed, $($script:failed) failed, $($script:skipped) optional skipped, $($script:warnings) warnings ===" -ForegroundColor Cyan
Write-Host ""
if ($script:failed -eq 0) {
    Write-Host "  系统就绪" -ForegroundColor Green
} else {
    Write-Host "  有 $($script:failed) 个关键问题需要修复" -ForegroundColor Red
}
Write-Host ""
