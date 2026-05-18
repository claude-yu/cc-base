# 自动 Bug 诊断经验总结

## `/修复controller` 工作流

用户报告基础设施错误时，CC 直接诊断修复，不走 Codex：

```
用户报错（微信截图/错误信息）
    │
    ├─→ 1. 识别错误类型
    │     ├── config 问题（命令未注册/参数错误）
    │     ├── 编码问题（乱码/mojibake）
    │     ├── 进程问题（卡住/无回复）
    │     ├── 权限问题（admin_from/command not found）
    │     └── 脚本 bug（null-guard/路径/参数解析）
    │
    ├─→ 2. 定位根因
    │     ├── 读 config.toml 检查命令注册
    │     ├── 检查 Windows API 编码状态
    │     ├── 查进程列表和 run 目录状态
    │     └── 读脚本代码定位 bug
    │
    ├─→ 3. 修复（限制范围）
    │     ├── 只改 controller/、cc-connect/、~/.cc-connect/config.toml
    │     ├── 禁止碰项目数据
    │     └── 修完同步 config
    │
    └─→ 4. 报告
          ├── 改了什么、为什么、怎么验证
          └── 是否需要重启 cc-connect
```

## 常见 Bug 速查表

| 症状 | 根因 | 修复 |
|------|------|------|
| 命令"不是 cc-connect 命令" | 源 config 有但部署 config 没有 | `Copy-Item` 同步 + 重启 |
| 中文回复乱码 | `GetACP()=936` vs `ConsoleOutputCP=65001` | 脚本加 `OutputEncoding=936` |
| 命令无回复/卡住 | `Start-Process -RedirectStandardOutput` handle 继承 | 用 runner 脚本模式 |
| "administrator permission required" | `admin_from` 不在 `[[projects]]` 级别 | 移到 projects 级 |
| Claude CLI 丢中文 | `-p $ChineseText` 编码丢失 | 用 stdin 管道传入 |
| Codex CLI 连接失败 | 代理协议不匹配 | 分别设 HTTP/SOCKS5h |
| `$ArgsRest` null crash | `{{args}}` 为空时 `.ToString()` 崩溃 | null-guard 检查 |
| PowerShell 解析错误 | 任务文本含 `{}();$\|><"` | 替换为安全文本 |

## 踩坑实录

### GBK 编码链路（2026-05-18）

调试过程：先试了 `[Console]::OpenStandardOutput()` 写原始 UTF-8 字节 → 还是乱码。再试了去掉所有 UTF-8 编码覆盖 → 默认也是 UTF-8，还是乱码。最终发现 cc-connect Go 二进制内含 `gbk`、`GetACP`、`936` 字符串 → 确认是 Go 端用 GBK 解码。

**关键发现**：Windows 系统上三个不同的编码值：
- `GetACP()=936` (ANSI) — cc-connect 用这个
- `GetOEMCP()=936` (OEM)
- `GetConsoleOutputCP()=65001` (Console) — PowerShell 用这个

### Handle 继承问题（2026-05-18）

`Start-Process -RedirectStandardOutput` 创建 pipe 后，子进程继承了 parent 的 stdout handle。cc-connect 等待 stdout pipe EOF 来收集回复，但子进程（后台 pipeline）持有 handle 不释放，导致 cc-connect 一直等到后台任务完成（几分钟）才收到回复。

### Token 浪费事件（2026-05-18）

调试编码问题时，4+ 次 `/计划审查` 在后台启动了 CC + Codex pipeline。每次提交都是秒级返回（乱码），但后台 pipeline 已经在消耗 token。教训：乱码出现时第一时间杀后台进程，不要先排查编码。
