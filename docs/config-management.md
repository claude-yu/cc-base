# Config 管理机制

## 双文件架构

| 文件 | 位置 | 用途 |
|------|------|------|
| 源 config | `<project>/cc-connect/config.toml` | 开发修改此文件 |
| 部署 config | `~/.cc-connect/config.toml` | cc-connect 运行时读取 |

## 同步机制

`start.ps1` 启动时自动从源文件复制到部署位置：

```powershell
$SourceConfig = Join-Path (Split-Path -Parent $PSCommandPath) "config.toml"
if (Test-Path -LiteralPath $SourceConfig) {
    Copy-Item -LiteralPath $SourceConfig -Destination $ConfigPath -Force
}
```

## 规则

1. 所有 config 修改只改**源文件**
2. 重启 cc-connect 时自动同步
3. 紧急情况可手动同步：`Copy-Item -LiteralPath "源" -Destination "部署" -Force`
4. 新增命令必须同时加 `[[commands]]` 和 `[[aliases]]`

## Pipeline 架构

```
用户手机（微信/飞书/Telegram）
    │
    ▼
cc-connect（Go 二进制 + Node.js 安装器）
    │
    ├─→ /计划审查 → submit-plan-review.ps1
    │     └─→ 后台: call-cc-readonly.ps1 → call-codex-review.ps1
    │           └─→ 写 verdict.md + summary.md
    │
    ├─→ /修复controller → fix-controller.ps1
    ├─→ /批准执行 → execute-approved.ps1（需 Codex APPROVE）
    ├─→ /人工批准执行 → execute-manual-approved.ps1（需 manual-approval.md）
    ├─→ /md状态检查 → collect-md-status.ps1
    └─→ 发给cc/发给codex → 直接转发到 CC/Codex agent
```

## 审查结论

| Verdict | 含义 | 下一步 |
|---------|------|--------|
| APPROVE | 计划安全可执行 | `/批准执行 <RunId>` |
| REVISE | 有风险需修改 | CC 修改重提 或 `/人工批准执行` |
| BLOCK | 不可执行 | 重大修改后重新提交 |
