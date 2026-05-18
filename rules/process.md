# 后台进程管理规则

## 1. Start-Process Handle 继承修复

**问题**：`Start-Process -RedirectStandardOutput` 导致子进程继承父进程的 stdout pipe handle，cc-connect 等不到 pipe 关闭，回复被卡住。

**正确模板**（异步后台启动）：
```powershell
Write-Output "回复文本"  # 先输出回复

$runnerPath = Join-Path $RunDir "_bg-runner.ps1"
@"
`$ErrorActionPreference = 'Stop'
& '$scriptPath' -TaskFile '$TaskFile' -RunId '$RunId' > '$logStdout' 2> '$logStderr'
"@ | Set-Content -LiteralPath $runnerPath -Encoding UTF8

Start-Process -FilePath "powershell" -ArgumentList @(
    "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $runnerPath
) -WorkingDirectory $ControllerRoot -WindowStyle Hidden
```

**关键点**：
- `Write-Output` 必须在 `Start-Process` **之前**
- 不使用 `-RedirectStandardOutput/-Error`（改用 runner 脚本内部 `>` 重定向）
- runner 脚本用 `-WindowStyle Hidden` 启动，不继承父进程 handle

**错误示范**：
```powershell
# ❌ -RedirectStandardOutput 导致 handle 继承
Start-Process ... -RedirectStandardOutput $logFile -RedirectStandardError $errFile
Write-Output "这行永远不会被 cc-connect 收到"
```

## 2. 乱码输出 → 立即止血

**当 cc-connect 命令输出乱码时，后台 pipeline 已经在跑，正在消耗 token！**

处理流程：
1. **第一步：告诉用户立即杀后台进程**（不要先排查编码）
2. 第二步：杀进程

```powershell
Get-WmiObject Win32_Process | Where-Object {
    $_.Name -match 'claude|codex' -and
    $_.CreationDate -gt (Get-Date).AddHours(-1).ToString('yyyyMMddHHmmss')
} | Select-Object ProcessId, Name, CreationDate, CommandLine

@(PID1, PID2, PID3) | ForEach-Object { Stop-Process -Id $_ -Force }
```

3. 第三步：修复编码问题（见 encoding.md）
4. 第四步：重新提交
