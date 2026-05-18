# 环境变量配置

cc-base 通过环境变量配置，所有脚本通用。

## 必需变量

| 变量 | 说明 |
|------|------|
| `CC_WORK_DIR` | CC agent 工作目录。也用于生成 project-id hash（SHA256 前 12 hex） |

## 可选变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CC_CODEX_WORK_DIR` | `$CC_WORK_DIR` | Codex agent 工作目录 |
| `CODEX_PROXY` | （不设置） | Codex CLI 代理（`socks5h://host:port`） |
| `CLAUDE_PROXY` | （不设置） | Claude CLI 代理（`http://host:port`） |
| `OPENAI_API_KEY` | — | Codex CLI 所需的 OpenAI API key |
| `CC_INSTINCT_HOME` | `~/.cc-base/instincts` | Chat-instinct 存储根目录 |
| `CC_PROJECT_NAME` | project-id hash | 项目显示名（只做 metadata，目录用 hash） |
| `CC_CHAT_PLATFORM` | `unknown` | 聊天平台标识（weixin/feishu/qq/telegram） |

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
