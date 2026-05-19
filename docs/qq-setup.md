# QQ (NapCat) 接入引导

通过 NapCat + Docker 连接 QQ，实现手机远程操控 CC + Codex agent。

## 前置条件

- Docker Desktop（Windows）已安装并运行
- cc-connect 1.3.2+：`npm install -g cc-connect`
- 一个 QQ 号用于 bot 登录

## 步骤

### 1. 启动 NapCat Docker 容器

```powershell
docker run -d `
  --name napcat `
  --restart always `
  -e ACCOUNT=YOUR_QQ_NUMBER `
  -e TZ=Asia/Shanghai `
  -p 3001:3001 `
  -p 6099:6099 `
  mlikiowa/napneko-docker:latest
```

| 参数 | 说明 |
|------|------|
| `ACCOUNT` | bot 使用的 QQ 号码 |
| `3001` | OneBot WebSocket 服务端口（cc-connect 连接用） |
| `6099` | NapCat WebUI 管理面板端口 |

### 2. 获取 WebUI 登录 Token

NapCat 首次启动后会生成一个 WebUI Token，从容器日志中获取：

```powershell
docker logs napcat 2>&1 | Select-String "WebUi Token"
```

输出示例：
```
[NapCat] [WebUi] WebUi Token: xxxxxxxxxxxx
[NapCat] [WebUi] WebUi User Panel Url: http://127.0.0.1:6099/webui?token=xxxxxxxxxxxx
```

记下这个 token。

### 3. 登录 NapCat 管理后台

1. 浏览器打开 `http://127.0.0.1:6099`
2. 输入上一步获取的 WebUI Token，点击"登录"
3. 进入管理面板后，如果状态显示 **Offline**，需要登录 QQ

### 4. 登录 QQ 账号

在 NapCat 管理面板中：

1. 找到 QQ 登录入口（扫码 / 账号密码）
2. 用手机 QQ 扫码登录
3. 登录成功后状态变为 **Online**
4. 此时 WebSocket 服务（端口 3001）开始监听

### 5. 配置 NapCat WebSocket

在 NapCat 管理面板的"网络配置"中，新建或编辑 **WebSocket 服务器**：

| 配置项 | 值 | 注意 |
|--------|-----|------|
| **启用** | **打开** | 必须打开，否则不监听 |
| 名称 | 随意（如 `cc`） | — |
| Host | `0.0.0.0` | — |
| **端口** | **`3001`** | 不要用 6099，会跟 WebUI 冲突 |
| **消息格式** | **`String`** | 不要用 Array，否则连上立刻断开 |
| Access Token | 自定义（需与 cc-connect config.toml 中的 `token` 一致） | — |
| 强制推送事件 | 打开 | — |

### 6. 配置 cc-connect

在 `config.toml` 中添加 QQ 平台的 project：

```toml
[[projects]]
admin_from = "YOUR_QQ_ADMIN_ID"
name = "codex"
reset_on_idle_mins = 0

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "YOUR_CODEX_WORK_DIR"
mode = "suggest"

[[projects.agent.providers]]
name = "openai"
api_key = "${OPENAI_API_KEY}"

[[projects.platforms]]
type = "qq"

[projects.platforms.options]
ws_url = "ws://127.0.0.1:3001"
token = "YOUR_NAPCAT_WS_TOKEN"
allow_from = "YOUR_QQ_NUMBER"
admin_from = "YOUR_QQ_NUMBER"
```

关键字段：

| 字段 | 说明 |
|------|------|
| `ws_url` | NapCat WebSocket 地址，默认 `ws://127.0.0.1:3001` |
| `token` | NapCat WebSocket 的 Access Token（步骤 5 中设置的） |
| `allow_from` | 允许发消息的 QQ 号 |
| `admin_from` | 管理员 QQ 号（必须在 `[[projects]]` 级别也设置） |

### 7. 启动 cc-connect

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File start.ps1 -WithQQ
```

`-WithQQ` 会自动 `docker start napcat`。确认日志中 QQ platform 显示 `platform ready`。

## 启动顺序

NapCat 容器启动到 WebSocket 就绪需要 1-3 分钟。如果 cc-connect 启动时 QQ 连接失败：

```
platform start failed: qq: ws connect failed (ws://127.0.0.1:3001)
```

等 NapCat 完全就绪后重启 cc-connect 即可。也可以在 `start.ps1` 中加 sleep：

```powershell
docker start napcat 2>&1 | Out-Null
Start-Sleep -Seconds 10  # 等待 NapCat WebSocket 就绪
```

## 常见问题

| 问题 | 原因 | 解决 |
|------|------|------|
| WebUI 显示 Offline | QQ 未登录 | 在 NapCat 管理面板扫码登录 |
| ws connect failed (port 3001) | NapCat 未就绪 / WebSocket 未配置 | 检查 NapCat 管理面板网络配置，确认 WS Server 已启用 |
| 忘记 WebUI Token | — | `docker logs napcat \| Select-String "WebUi Token"` |
| QQ 掉线 | 长时间未活动 / QQ 风控 | 重新扫码登录，考虑使用密码登录方式 |
| OPENAI_API_KEY not set | 原生 Codex agent 需要 API key | 设环境变量，或改用 cc-controller 路径（见下方说明） |
| NapCat 容器重启后需重新登录 | QQ session 过期 | 重新扫码，或配置 NapCat 的持久化 session 目录 |

## 推荐架构：QQ 并入 cc project

v2.5.0 起，QQ 平台已从独立 `codex` project 移入 `cc` project，与微信共享全部 `[[commands]]` 路由。这意味着 QQ 可直接使用 `/cc`、`/问codex`、`/状态`、`/查看`、`/监控`、`/自检` 等全部命令。

如仍需 QQ 独立 codex agent（不走 cc-controller），可将 QQ 平台配到单独 `codex` project，但需额外设置 `OPENAI_API_KEY`。

## 两种 Codex 接入路径

| 路径 | 认证方式 | 适用 |
|------|----------|------|
| `/问codex` → cc-controller → codex CLI | ChatGPT OAuth（`~/.codex/auth.json`） | cc project 命令路由（推荐） |
| cc-connect 原生 codex agent | OpenAI API key（`${OPENAI_API_KEY}`） | 独立 codex project |
