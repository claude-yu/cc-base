# WeChat 企业号接入引导

通过微信企业号连接 cc-connect，实现手机远程操控 Claude Code。

## 前置条件

- Node.js 18+
- Claude Code CLI：`npm install -g @anthropic-ai/claude-code`
- cc-connect：`npm install -g cc-connect`

## 步骤

### 1. 准备 config.toml

复制模板并编辑：

```bash
cp scripts/config.toml.template config.toml
```

填入基础字段（`work_dir`、`YOUR_PROJECT_ROOT` 等）。WeChat 凭据部分先留 placeholder。

### 2. 首次启动 cc-connect

```powershell
cc-connect serve --config path/to/config.toml
```

终端会显示微信扫码 URL。

### 3. 扫码注册

用微信扫描终端显示的二维码（或打开 URL）。

**扫码完成后，终端底部会输出注册信息。把这段输出完整复制发给 CC（Claude Code）。**

### 4. 提取凭据

从用户发回的终端输出中提取以下 3 个值：

| 字段 | 来源 | config.toml 对应 |
|------|------|-----------------|
| Bearer 令牌 | `token: Bearer xxx` | `token = "xxx"` |
| 网关地址 | `base_url: https://xxx.weixin.qq.com` | `base_url = "https://xxx.weixin.qq.com"` |
| Bot ID | `ilink_bot_id: xxx@im.bot` | `account_id = "xxx@im.bot"` |

将这 3 个值写入 config.toml 的 `[projects.platforms.options]` 段落。

### 5. 设置 allow_from 和 admin_from

- `allow_from`：你的微信 OpenID（首次发消息时 cc-connect 日志会显示）
- `admin_from`：同上，放在 `[[projects]]` 级别（不是只放在 platforms 里）

### 6. 重启 cc-connect

```powershell
# 停止旧进程
# 重新启动
powershell -NoProfile -ExecutionPolicy Bypass -File start.ps1 -CleanSessions
```

发送 `你好` 测试是否正常回复。

## 安全提醒

- **不要把 `config.toml` 提交到 Git 仓库** — 它包含 token 和 API key
- **不要把扫码输出贴到公开仓库或 issue** — 它包含 Bearer 令牌
- **模板文件 `config.toml.template` 只包含 placeholder**，可以安全提交
- `.gitignore` 已配置排除 `config.toml`

## 常见问题

| 问题 | 解决 |
|------|------|
| 发消息无回复 | 检查 `admin_from` 是否在 `[[projects]]` 级别 |
| 乱码 | 脚本顶部加 UTF-8 三行（**不要用 936**），详见 `rules/encoding.md` |
| Token 过期 | 重新扫码注册，更新 config.toml 中的 token |
| 代理问题 | Claude 用 HTTP、Codex 用 SOCKS5h，详见 `rules/proxy.md` |
