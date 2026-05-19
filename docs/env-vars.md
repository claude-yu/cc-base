# 环境变量配置

cc-base 通过环境变量配置，所有脚本通用。

## 必需变量

| 变量 | 说明 |
|------|------|
| `CC_WORK_DIR` | CC agent 工作目录。也用于生成 project-id hash（SHA256 前 12 hex） |

## 可选变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CC_MODEL` | `claude-opus-4-6` | Controller 调 Claude CLI 的模型（`start.ps1` 默认设置，不影响 CLI 全局默认） |
| `CC_CONTROLLER_DIR` | 自动检测 | controller 根目录（覆盖 auto-detect） |
| `CC_EXECUTE_WORK_DIR` | `$CC_WORK_DIR` | 执行模式沙盒目录（`execute_request` 在此目录运行，隔离科研数据） |
| `CC_CODEX_WORK_DIR` | `$CC_WORK_DIR` | Codex agent 工作目录 |
| `CODEX_PROXY` | （不设置） | Codex CLI 代理（`socks5h://host:port`） |
| `CLAUDE_PROXY` | （不设置） | Claude CLI 代理（`http://host:port`） |
| `OPENAI_API_KEY` | — | Codex CLI 所需的 OpenAI API key |
| `CC_INSTINCT_HOME` | `~/.cc-base/instincts` | Chat-instinct 存储根目录 |
| `CC_PROJECT_NAME` | project-id hash | 项目显示名（只做 metadata，目录用 hash） |
| `CC_CHAT_PLATFORM` | `unknown` | 聊天平台标识（weixin/feishu/qq/telegram） |

## Backend 选择变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CC_BACKEND` | `native_claude` | CC 角色 backend（当前只支持 native_claude，其他值为 future） |
| `CC_CODEX_BACKEND` | `native_codex` | Codex 角色 backend：`native_codex` / `openai` / `deepseek` / `glm` |
| `CC_CODEX_API_BASE` | — | Codex API backend 的 base URL（仅 API backend 时需要） |
| `CC_CODEX_API_KEY` | — | Codex API backend 的 API key（仅 API backend 时需要） |
| `CC_CODEX_MODEL` | — | Codex API backend 的模型名（仅 API backend 时需要） |

示例：使用 DeepSeek 作为 Codex 角色 backend：

```
CC_CODEX_BACKEND=deepseek
CC_CODEX_API_BASE=https://api.deepseek.com/v1
CC_CODEX_API_KEY=sk-xxx
CC_CODEX_MODEL=deepseek-chat
```

使用 native Codex 时无需设置任何 backend 变量（默认行为）。

## 代理注意事项

- Claude CLI 只接受 HTTP 代理，收到 `socks5h://` 会崩溃
- Codex CLI 需要 SOCKS5h 代理
- 代理只在 `$env:CODEX_PROXY` 存在时设置，不清空用户已有代理
- 详见 `rules/proxy.md`

## Project ID 生成规则

```
CC_WORK_DIR 存在 → SHA256(CC_WORK_DIR.ToLower().TrimEnd('/\')) 前 12 hex
CC_WORK_DIR 不存在 → git remote get-url origin → SHA256(remote) 前 12 hex
都不存在 → "default"
```
