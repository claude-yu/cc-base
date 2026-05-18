# 新脚本 Checklist

新建 cc-connect 命令脚本时，逐项检查：

- [ ] stdout 输出中文时：param() 后加 UTF-8 三行块（见 rules/encoding.md），**禁用 `GetEncoding(936)`**
- [ ] 脚本含中文（含注释）时：文件存为 UTF-8 **with BOM**，验证前 3 字节为 `EF BB BF`
- [ ] `$ArgsRest` null-guard（接收 `{{args}}` 时）
- [ ] `Write-Output` 在 `Start-Process` 之前（有后台启动时）
- [ ] 不用 `-RedirectStandardOutput/-Error`（用 runner 脚本替代）
- [ ] 中文传给 Claude CLI 用 stdin 管道，不用 `-p` 参数
- [ ] Codex CLI 代理用 `socks5h://`，Claude CLI 用 HTTP proxy
- [ ] config.toml 修改在**源文件**，不改部署文件
- [ ] 新增命令同时加 `[[commands]]` 和 `[[aliases]]`
- [ ] 修完后同步 config：`Copy-Item` 或重启 cc-connect

## 新增命令 config 模板

```toml
[[commands]]
name = "命令名"
description = "描述"
exec = "powershell -NoProfile -ExecutionPolicy Bypass -File YOUR_PROJECT_ROOT\\controller\\bin\\脚本.ps1 {{args}}"
work_dir = "YOUR_PROJECT_ROOT\\controller"

[[aliases]]
name = "别名"
command = "/命令名"
```
