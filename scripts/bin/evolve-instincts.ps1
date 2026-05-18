$ErrorActionPreference = "Continue"
. (Join-Path (Split-Path -Parent $PSCommandPath) "_common.ps1")
[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$nl = [Environment]::NewLine
$InstinctHome = if ($env:CC_INSTINCT_HOME) { $env:CC_INSTINCT_HOME } else { Join-Path $env:USERPROFILE ".cc-base\instincts" }
$ProjectId = Get-ProjectId
$ProjectName = if ($env:CC_PROJECT_NAME) { $env:CC_PROJECT_NAME } else { $ProjectId }
$projectDir = Join-Path $InstinctHome "projects/$ProjectId"
$obsFile = Join-Path $projectDir "observations.jsonl"
$evolvedDir = Join-Path $projectDir "evolved"
$candidatesFile = Join-Path $evolvedDir "evolved-candidates.md"
$output = New-Object System.Collections.Generic.List[string]

function Out-Lines { [string]::Join($nl, $output) }
function Add-Line { param([string]$Text) $output.Add($Text) }

Add-Line "# Evolution Analysis"
Add-Line ""
Add-Line "Auto-generated English trigger words for / commands."
Add-Line "Type yes/y to write instinct YAML, or no to skip."
Add-Line ""

if (-not (Test-Path $obsFile)) { Add-Line "No observations found."; Write-Output (Out-Lines); exit 0 }
$obsLines = Get-Content $obsFile | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
$obsCount = ($obsLines | Measure-Object).Count
Add-Line ("Total observations: " + $obsCount)
Add-Line ""
if ($obsCount -lt 10) { Add-Line "Less than 10 observations."; Write-Output (Out-Lines); exit 0 }

$commandCount = @{}
$commandSuccess = @{}
$commandFail = @{}
foreach ($line in $obsLines) {
    try {
        $entry = $line | ConvertFrom-Json
        $cmd = $entry.command
        if (-not $commandCount.ContainsKey($cmd)) { $commandCount[$cmd] = 0; $commandSuccess[$cmd] = 0; $commandFail[$cmd] = 0 }
        $commandCount[$cmd]++
        if ($entry.event -eq "command_end") { $commandSuccess[$cmd]++ }
        if ($entry.event -eq "error") { $commandFail[$cmd]++ }
    } catch {}
}

Add-Line "## Command Usage"
Add-Line ""
$sortedKeys = $commandCount.Keys | Sort-Object { $commandCount[$_] } -Descending
foreach ($c in $sortedKeys) {
    Add-Line ("- " + $c + ": " + $commandCount[$c] + " times")
}
Add-Line ""

Add-Line "## Evolution Candidates"
Add-Line ""

$cmdToKey = @{}
$cmdToKey["计划审查"] = "plan-review"
$cmdToKey["修复controller"] = "fix-controller"
$cmdToKey["自动修复"] = "auto-fix"
$cmdToKey["md-status"] = "md-status"
$cmdToKey["md状态检查"] = "md-status"
$cmdToKey["问codex"] = "codex-ask"
$cmdToKey["质询计划"] = "grill-plan"
$cmdToKey["自动回传"] = "auto-callback"
$cmdToKey["批准执行"] = "approve-exec"
$cmdToKey["人工批准执行"] = "manual-approve"
$cmdToKey["学习状态"] = "learning-stats"

$triggers = @{}
$triggers["plan-review"] = [string[]]@("plan","review","plan-review","plancheck","plans","pl")
$triggers["fix-controller"] = [string[]]@("fix","repair","fixc","f","hotfix")
$triggers["auto-fix"] = [string[]]@("fix","repair","autofix","f","hotfix")
$triggers["md-status"] = [string[]]@("md","check-md","mdstatus","mdcheck","status","st")
$triggers["codex-ask"] = [string[]]@("ask","codex","ask-codex","query","q")
$triggers["grill-plan"] = [string[]]@("grill","drill","grill-plan","challenge","interrogate")
$triggers["auto-callback"] = [string[]]@("callback","auto-callback","cb","back","notify")
$triggers["approve-exec"] = [string[]]@("approve","exec","approve-exec","go","deploy")
$triggers["manual-approve"] = [string[]]@("m-approve","manual-exec")
$triggers["learning-stats"] = [string[]]@("learning","stats","learning-status","habit","ls")

$notes = @{}
$notes["plan-review"] = "Plan review: CC plans + Codex reviews, async"
$notes["fix-controller"] = "Controller fix: auto-diagnose + repair controller"
$notes["auto-fix"] = "Auto fix: triggered when plan-review fails"
$notes["md-status"] = "MD status: read-only scan of GROMACS work directory"
$notes["codex-ask"] = "Codex Q&A: async ask Codex, auto-callback"
$notes["grill-plan"] = "Grill plan: one-question-at-a-time challenge"
$notes["auto-callback"] = "Auto-callback toggle: auto-push results to chat"
$notes["approve-exec"] = "Approve execution: run approved plans"
$notes["manual-approve"] = "Manual approval: manually accept then execute"
$notes["learning-stats"] = "Learning stats: view chat-instinct observations"

$idx = 0
foreach ($c in $sortedKeys) {
    $total = $commandCount[$c]
    $idx++
    $k = $cmdToKey[$c]
    if (-not $k) { $k = $c -replace "[^a-z0-9]", "-" }
    $t = @($k)
    if ($triggers.ContainsKey($k)) { $t = $triggers[$k] }
    $ts = ($t | ForEach-Object { "/" + $_ }) -join ", "
    $conf = [math]::Min(0.3 + ($total * 0.15), 0.95)
    $iid = "trigger-" + $k
    $note = ""
    if ($notes.ContainsKey($k)) { $note = $notes[$k] }

    Add-Line "---"
    Add-Line ""
    Add-Line ("### Candidate " + $idx + ": " + $c)
    Add-Line ""
    Add-Line $note
    Add-Line ""
    Add-Line ("**Triggers:** " + $ts)
    Add-Line ""
    Add-Line '```yaml'
    Add-Line ("id: " + $iid)
    Add-Line ("trigger: " + $ts)
    Add-Line ("confidence: " + $conf)
    Add-Line "domain: workflow"
    Add-Line "scope: project"
    Add-Line ("project_id: " + $ProjectId)
    Add-Line '```'
    Add-Line ""
    Add-Line ("Evidence: " + $total + " usages")
    Add-Line ""
}

Add-Line "---"
Add-Line ""
Add-Line "## How to Apply"
Add-Line ""
Add-Line "Add these to cc-connect config.toml under [[aliases]]:"
Add-Line ""
foreach ($c in $sortedKeys) {
    $k = $cmdToKey[$c]
    if (-not $k) { $k = $c -replace "[^a-z0-9]", "-" }
    if (-not $triggers.ContainsKey($k)) { continue }
    $t = $triggers[$k]
    Add-Line ("### " + $c)
    foreach ($trigger in $t) { Add-Line ("- [ ] " + $trigger + " -> " + $c) }
    Add-Line ""
}

$candidateText = Out-Lines
New-Item -ItemType Directory -Force -Path $evolvedDir -ErrorAction Stop | Out-Null
$candidateText | Set-Content -Encoding UTF8 -LiteralPath $candidatesFile
Write-Output $candidateText
Write-Output ""
Write-Output "---"
Write-Output ""
Write-Output ("Candidates written to: " + $candidatesFile)
Write-Output "Type yes or y to write instinct YAML files."
