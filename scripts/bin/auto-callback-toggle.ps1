param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ArgsRest
)

$ErrorActionPreference = "Stop"

[Console]::OutputEncoding = [System.Text.Encoding]::GetEncoding(936)

$ControllerRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$flagFile = Join-Path $ControllerRoot "auto-callback.flag"

$action = ""
if ($null -ne $ArgsRest -and $ArgsRest.Count -gt 0) {
    $action = ($ArgsRest -join " ").Trim()
}

switch -Regex ($action) {
    "^(开|on|1|enable)$" {
        "1" | Set-Content -LiteralPath $flagFile -Encoding UTF8
        Write-Output "自动回传: 已开启"
        Write-Output "计划审查完成后会自动发送结果到聊天窗口。"
    }
    "^(关|off|0|disable)$" {
        if (Test-Path -LiteralPath $flagFile) { Remove-Item -LiteralPath $flagFile -Force }
        Write-Output "自动回传: 已关闭"
        Write-Output "需要手动 /查看审查 查看结果。"
    }
    default {
        $status = if (Test-Path -LiteralPath $flagFile) { "开启" } else { "关闭" }
        Write-Output "自动回传: $status"
        Write-Output "用法: /自动回传 开  或  /自动回传 关"
    }
}
