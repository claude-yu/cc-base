# 编码规则

## 1. cc-connect 命令输出编码（GBK 问题）

**根因**：cc-connect (Go) 用 `GetACP()=936` (GBK) 解码命令 stdout，但系统 `ConsoleOutputCP=65001` 让 PowerShell 默认输出 UTF-8。

**修复模板** — 所有被 cc-connect 直接调用且输出中文的脚本必须加：

```powershell
# 放在脚本顶部，任何 Write-Output 之前
[Console]::OutputEncoding = [System.Text.Encoding]::GetEncoding(936)
```

**判断标准**：
- 出现在 config.toml `[[commands]] exec` 中 → 需要此行
- 只在后台运行、输出到文件 → 不需要
- 只输出英文/数字 → 不需要

**诊断方法**：
```powershell
Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public class WinApi { [DllImport("kernel32.dll")] public static extern uint GetACP(); [DllImport("kernel32.dll")] public static extern uint GetConsoleOutputCP(); }'
[WinApi]::GetACP()            # cc-connect 用这个（936=GBK）
[WinApi]::GetConsoleOutputCP() # PowerShell 用这个（65001=UTF-8）
```

## 2. Claude CLI 中文输入（stdin 问题）

`claude -p $ChineseText` 在 Windows PowerShell 5.1 上会丢失中文字符。

**修复模板**：
```powershell
$tmpFile = [System.IO.Path]::GetTempFileName()
$prompt | Set-Content -Encoding UTF8 -LiteralPath $tmpFile
Get-Content -Raw -Encoding UTF8 $tmpFile | & claude -p --output-format text
Remove-Item -LiteralPath $tmpFile -Force
```
