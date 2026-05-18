# 代理隔离规则

## Claude CLI vs Codex CLI 代理

| 工具 | 需要的代理 | 环境变量 |
|------|-----------|---------|
| Claude CLI | HTTP proxy | `$env:CLAUDE_PROXY`（如 `http://your-proxy:port`） |
| Codex CLI | SOCKS5h proxy | `$env:CODEX_PROXY`（如 `socks5h://your-proxy:port`） |

## 规则

- Claude 收到 `socks5h://` → 崩溃（UnsupportedProxyProtocol）
- SOCKS5h 代理只在 `call-codex-review.ps1` 内部设置，绝不设为全局
- 两种代理绝不混用

## Codex CLI 代理设置模板

```powershell
# 仅在 call-codex-review.ps1 内部设置（由 Set-CodexProxy 函数处理）
# 只在 $env:CODEX_PROXY 存在时覆盖，不清空用户已有 proxy
if ($env:CODEX_PROXY) {
    $env:ALL_PROXY   = $env:CODEX_PROXY
    $env:HTTPS_PROXY = $env:CODEX_PROXY
    $env:HTTP_PROXY  = $env:CODEX_PROXY
}
```
