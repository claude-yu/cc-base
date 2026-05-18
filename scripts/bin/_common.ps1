function Resolve-ClaudeCmd {
    $cmd = (Get-Command claude.cmd -ErrorAction SilentlyContinue).Source
    if (-not $cmd) { $cmd = Join-Path $env:APPDATA "npm\claude.cmd" }
    if (-not (Test-Path -LiteralPath $cmd)) {
        throw "Claude Code CLI not found. Install: npm install -g @anthropic-ai/claude-code"
    }
    return $cmd
}

function Resolve-CodexCmd {
    $cmd = (Get-Command codex.cmd -ErrorAction SilentlyContinue).Source
    if (-not $cmd) { $cmd = Join-Path $env:APPDATA "npm\codex.cmd" }
    if (-not (Test-Path -LiteralPath $cmd)) {
        throw "Codex CLI not found. Install: npm install -g @openai/codex"
    }
    return $cmd
}

function Resolve-RequiredWorkDir {
    param(
        [string]$ParamValue,
        [string]$EnvVarName = "CC_WORK_DIR"
    )
    if (-not [string]::IsNullOrWhiteSpace($ParamValue)) {
        if (-not (Test-Path -LiteralPath $ParamValue)) {
            throw "WorkDir does not exist: $ParamValue"
        }
        return $ParamValue
    }
    $envVal = [Environment]::GetEnvironmentVariable($EnvVarName)
    if (-not [string]::IsNullOrWhiteSpace($envVal)) {
        if (-not (Test-Path -LiteralPath $envVal)) {
            throw "WorkDir from $EnvVarName does not exist: $envVal"
        }
        return $envVal
    }
    $fallback = [Environment]::GetEnvironmentVariable("CC_WORK_DIR")
    if (-not [string]::IsNullOrWhiteSpace($fallback)) {
        if (-not (Test-Path -LiteralPath $fallback)) {
            throw "WorkDir from CC_WORK_DIR does not exist: $fallback"
        }
        return $fallback
    }
    throw "WorkDir is required. Set $EnvVarName environment variable, or pass -WorkDir explicitly."
}

function Set-CodexProxy {
    if ($env:CODEX_PROXY) {
        $env:ALL_PROXY   = $env:CODEX_PROXY
        $env:HTTPS_PROXY = $env:CODEX_PROXY
        $env:HTTP_PROXY  = $env:CODEX_PROXY
    }
}

function Get-ProjectId {
    if ($env:CC_WORK_DIR) {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($env:CC_WORK_DIR.ToLower().TrimEnd('\','/'))
        $sha = [System.Security.Cryptography.SHA256]::Create()
        $hash = $sha.ComputeHash($bytes)
        return ($hash | Select-Object -First 6 | ForEach-Object { $_.ToString("x2") }) -join ""
    }
    try {
        $remote = & git remote get-url origin 2>$null
        if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($remote)) {
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($remote.Trim())
            $sha = [System.Security.Cryptography.SHA256]::Create()
            $hash = $sha.ComputeHash($bytes)
            return ($hash | Select-Object -First 6 | ForEach-Object { $_.ToString("x2") }) -join ""
        }
    } catch {}
    return "default"
}

function Get-ObservationDir {
    $instinctHome = if ($env:CC_INSTINCT_HOME) { $env:CC_INSTINCT_HOME } else { Join-Path $env:USERPROFILE ".cc-base\instincts" }
    $projectId = Get-ProjectId
    $projectDir = Join-Path $instinctHome "projects\$projectId"
    New-Item -ItemType Directory -Force -Path $projectDir -ErrorAction SilentlyContinue | Out-Null
    return $projectDir
}

function Write-ChatObservation {
    param(
        [string]$EventType,
        [string]$CommandName,
        [string]$Detail = ""
    )
    try {
        $projectDir = Get-ObservationDir
        $obsFile = Join-Path $projectDir "observations.jsonl"
        $obs = @{
            timestamp = (Get-Date -Format "o")
            event     = $EventType
            command   = $CommandName
            detail    = $Detail
            platform  = if ($env:CC_CHAT_PLATFORM) { $env:CC_CHAT_PLATFORM } else { "unknown" }
        } | ConvertTo-Json -Compress
        $obs | Add-Content -LiteralPath $obsFile -Encoding UTF8
    } catch {
        # observation 记录失败不影响主流程
    }
}
