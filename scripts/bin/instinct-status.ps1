$ErrorActionPreference = "Continue"

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$InstinctHome = if ($env:CC_INSTINCT_HOME) { $env:CC_INSTINCT_HOME } else { Join-Path $env:USERPROFILE ".cc-base\instincts" }
$ProjectId = Get-ProjectId
$ProjectName = if ($env:CC_PROJECT_NAME) { $env:CC_PROJECT_NAME } else { $ProjectId }

$projectDir = Join-Path $InstinctHome "projects\$ProjectId"

$output = New-Object System.Collections.Generic.List[string]
$output.Add("# Chat-Instinct 学习状态")
$output.Add("")
$output.Add("项目: $ProjectName (ID: $ProjectId)")
$output.Add("存储: $InstinctHome")
$output.Add("")

if (-not (Test-Path -LiteralPath $projectDir)) {
    $output.Add("尚无学习记录。使用系统后会自动积累 observation。")
    Write-Output ([string]::Join("`n", $output))
    exit 0
}

$obsFile = Join-Path $projectDir "observations.jsonl"
$obsCount = 0
if (Test-Path -LiteralPath $obsFile) {
    $obsCount = (Get-Content -LiteralPath $obsFile | Measure-Object -Line).Lines
}

$instinctDir = Join-Path $projectDir "instincts\personal"
$instincts = @()
if (Test-Path -LiteralPath $instinctDir) {
    $instincts = Get-ChildItem -LiteralPath $instinctDir -Filter "*.yaml" -ErrorAction SilentlyContinue
}

$output.Add("## 统计")
$output.Add("")
$output.Add("- Observations: $obsCount 条")
$output.Add("- Instincts: $($instincts.Count) 个")
$output.Add("")

if ($instincts.Count -gt 0) {
    $output.Add("## Instinct 列表")
    $output.Add("")
    $output.Add("| ID | 置信度 | 域 | 触发条件 |")
    $output.Add("|----|----|----|----|")
    foreach ($f in $instincts) {
        $content = Get-Content -LiteralPath $f.FullName -Raw -ErrorAction SilentlyContinue
        $id = if ($content -match "(?m)^id:\s*(.+)") { $Matches[1].Trim() } else { $f.BaseName }
        $conf = if ($content -match "(?m)^confidence:\s*([\d.]+)") { $Matches[1] } else { "?" }
        $domain = if ($content -match "(?m)^domain:\s*(.+)") { $Matches[1].Trim() } else { "-" }
        $trigger = if ($content -match "(?m)^trigger:\s*""?(.+?)""?\s*$") { $Matches[1] } else { "-" }
        $output.Add("| $id | $conf | $domain | $trigger |")
    }
}

$evolvedDir = Join-Path $projectDir "evolved"
$evolvedSkills = @()
$evolvedCmds = @()
if (Test-Path -LiteralPath (Join-Path $evolvedDir "skills")) {
    $evolvedSkills = Get-ChildItem -LiteralPath (Join-Path $evolvedDir "skills") -ErrorAction SilentlyContinue
}
if (Test-Path -LiteralPath (Join-Path $evolvedDir "commands")) {
    $evolvedCmds = Get-ChildItem -LiteralPath (Join-Path $evolvedDir "commands") -ErrorAction SilentlyContinue
}

if ($evolvedSkills.Count -gt 0 -or $evolvedCmds.Count -gt 0) {
    $output.Add("")
    $output.Add("## 已进化")
    $output.Add("")
    $output.Add("- Skills: $($evolvedSkills.Count)")
    $output.Add("- Commands: $($evolvedCmds.Count)")
}

Write-Output ([string]::Join("`n", $output))
exit 0
