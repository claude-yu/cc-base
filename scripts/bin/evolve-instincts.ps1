$ErrorActionPreference = "Continue"

. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")

[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$InstinctHome = if ($env:CC_INSTINCT_HOME) { $env:CC_INSTINCT_HOME } else { Join-Path $env:USERPROFILE ".cc-base\instincts" }
$ProjectId = Get-ProjectId
$ProjectName = if ($env:CC_PROJECT_NAME) { $env:CC_PROJECT_NAME } else { $ProjectId }

$projectDir = Join-Path $InstinctHome "projects\$ProjectId"
$obsFile = Join-Path $projectDir "observations.jsonl"
$instinctDir = Join-Path $projectDir "instincts\personal"

$output = New-Object System.Collections.Generic.List[string]
$output.Add("# 进化习惯分析")
$output.Add("")
$output.Add("项目: $ProjectName (ID: $ProjectId)")
$output.Add("")

if (-not (Test-Path -LiteralPath $obsFile)) {
    $output.Add("无 observation 记录，暂无可进化内容。")
    Write-Output ([string]::Join("`n", $output))
    exit 0
}

$obsCount = (Get-Content -LiteralPath $obsFile | Measure-Object -Line).Lines
$output.Add("Observations 总数: $obsCount")
$output.Add("")

if ($obsCount -lt 10) {
    $output.Add("Observation 不足 10 条，建议继续使用后再进化。")
    Write-Output ([string]::Join("`n", $output))
    exit 0
}

$output.Add("## 进化建议")
$output.Add("")
$output.Add("observation 数量已达到进化阈值。可以分析重复模式并生成进化候选：")
$output.Add("")
$output.Add("1. 分析 observation 中的重复模式")
$output.Add("2. 生成进化候选列表（evolved-candidates.md）")
$output.Add("3. **用户确认后**才写入 instinct yaml 文件")
$output.Add("")
$output.Add("Observation 文件: $obsFile")
$output.Add("Instinct 目录: $instinctDir")
$output.Add("")
$output.Add("注意：进化只生成候选建议，不会自动创建 skill 或 command。")

Write-Output ([string]::Join("`n", $output))
exit 0
