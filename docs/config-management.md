# Config 管理机制

## 三文件架构

| 文件 | 位置 | 用途 |
|------|------|------|
| Skill 模板 | `skills/cc-base/scripts/config.toml.template` | 版本控制参考，含占位符 |
| 源 config | `<project>/cc-connect/config.toml` | 开发修改此文件（填入实际凭据） |
| 部署 config | `~/.cc-connect/config.toml` | cc-connect 运行时读取 |

三者关系：skill 模板 → 首次部署时复制为源 config → `start.ps1` 启动时同步到部署 config。日常只改源 config，skill 模板仅在 skill 升级时参考更新。

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
5. API key 可直接写入 config.toml（如 `api_key = "sk-xxx"`），也可引用环境变量（如 `api_key = "${OPENAI_API_KEY}"`）

## NapCat WebSocket 关键设置

QQ 通过 NapCat Docker 接入时，WebSocket 配置必须满足：

| 配置项 | 必须值 | 错误后果 |
|--------|--------|----------|
| 端口 | `3001` | 不要用 6099（与 WebUI 冲突） |
| 消息格式 | `String` | 用 Array 会连上即断 |
| 启用开关 | 打开 | 关闭则不监听连接 |

完整步骤见 `docs/qq-setup.md`。

## 多 Provider 配置

config.toml 的 `[[projects.agent.providers]]` 支持多个 LLM 提供商：

```toml
# OpenAI（默认）
[[projects.agent.providers]]
name = "openai"
api_key = "${OPENAI_API_KEY}"

# DeepSeek（可选）
[[projects.agent.providers]]
name = "deepseek"
api_key = "YOUR_DEEPSEEK_API_KEY"
base_url = "https://api.deepseek.com/v1"

# GLM / 智谱（可选）
[[projects.agent.providers]]
name = "glm"
api_key = "YOUR_GLM_API_KEY"
base_url = "https://open.bigmodel.cn/api/paas/v4"
```

模板中 DeepSeek/GLM 默认注释，取消注释并填入 key 即可启用。

## Backend 选择

CC 和 Codex 角色的 backend 选择通过环境变量控制（`CC_BACKEND`、`CC_CODEX_BACKEND`），不通过 config.toml。cc-connect 不解析自定义 TOML 块，因此 config.toml 中的 `[cc_base.*]` 注释仅作为概念参考和 future schema，不影响运行时行为。

实际生效的 backend 配置来源只有环境变量，详见 `docs/env-vars.md`。

临时测试 backend 时，PowerShell 的 `$env:CC_CODEX_BACKEND="deepseek"` 只影响当前窗口和从该窗口启动的 `cc-connect`。测试后恢复 native Codex：

```powershell
Remove-Item Env:\CC_CODEX_BACKEND -ErrorAction SilentlyContinue
Remove-Item Env:\CC_CODEX_API_BASE -ErrorAction SilentlyContinue
Remove-Item Env:\CC_CODEX_API_KEY -ErrorAction SilentlyContinue
Remove-Item Env:\CC_CODEX_MODEL -ErrorAction SilentlyContinue
```

如果已经用该窗口重启过 `cc-connect`，清空变量后还需要再重启一次。完整说明见根目录 `指导.md`。

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
    ├─→ /cc → cc-controller.exe exec-cc（session-aware 对话）
    ├─→ /问codex → cc-controller.exe ask-codex（异步问答）
    ├─→ /执行 → cc-controller.exe execute（waiting queue 分发）
    ├─→ /取消任务 → cc-controller.exe cancel（智能取消）
    ├─→ /监控 → cc-controller.exe monitor（stuck/zombie 检测）
    ├─→ /修复controller → fix-controller.ps1
    ├─→ /批准执行 → execute-approved.ps1（需 Codex APPROVE）
    ├─→ /人工批准执行 → execute-manual-approved.ps1（需 manual-approval.md）
    ├─→ /md状态检查 → collect-md-status.ps1
    └─→ /自检 → check-install.ps1（12 项安装检查）
```

## 审查结论

| Verdict | 含义 | 下一步 |
|---------|------|--------|
| APPROVE | 计划安全可执行 | `/批准执行 <RunId>` |
| REVISE | 有风险需修改 | CC 修改重提 或 `/人工批准执行` |
| BLOCK | 不可执行 | 重大修改后重新提交 |
