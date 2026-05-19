# cc-base Mobile Agent Next Plan

## Summary

本计划用于修复当前微信/QQ 移动端 agent 交互的剩余问题。目标不是重写整个系统，而是把已经验证可用的 Go `cc-controller` 路径补齐：命令可发现、回调回到正确渠道、输出无污染、配置可从 GitHub 模板恢复。

当前已验证能力：

- `/cc` 是 session-aware，对话上下文可延续。
- `/状态` 可查看项目、session、等待队列、活动任务和最近 run。
- 执行型任务会先生成确认卡，`/执行 RunId` 或唯一等待任务时回复 `好/ok/可以/确认` 可执行。
- `CC_EXECUTE_WORK_DIR` 沙盒执行已验证，可把文件创建到 `E:\ai\selfwork_ytl\test`。
- `/项目`、`/切项目` 已支持多项目工作目录。

当前剩余问题：

- ~~`/查看`、`/勘察` 不是 cc-connect command~~ → **已修复 2026-05-19**：新增 `[[commands]]`
- ~~Codex 回答里混入 `taskkill` 成功信息~~ → **已修复 2026-05-19**：codex.go 过滤 + 单测
- ~~cc-base skill 模板与本机配置漂移~~ → **已修复 2026-05-19**：反向同步 + 脱敏
- QQ Codex 回调可能回错渠道（`--reply-project` 隔离未实现）
- CC_MODEL 默认值未在 controller 层面传递 → **已修复 2026-05-19**：cc.go + start.ps1

## Scope

只处理移动端交互可靠性，不做以下事情：

- 不重写 cc-connect。
- 不改 Claude/Codex 模型调用策略。
- 不实现完整多平台消息总线。
- 不把普通自然语言 fallback 全部接管到 Go router。

## Step 1: 恢复 `/查看` 和常用状态别名 ✅ 已完成

### 问题

cc-connect v1.3.2 的 `[[aliases]]` 不匹配 `/` 前缀消息。`/查看` 如果只写 alias，会被当成未知命令转给 Agent。

### 修改

在以下配置中新增真实 `[[commands]]`：

- `E:\ai\selfwork_ytl\cc-connect\config.toml`
- `C:\Users\tian\.cc-connect\config.toml`
- `C:\Users\tian\.claude\skills\cc-base\scripts\config.toml.template`

新增命令：

```toml
[[commands]]
name = "查看"
description = "查看当前系统状态、项目、等待队列和最近任务"
exec = "E:\\ai\\selfwork_ytl\\controller\\cc-controller.exe status"

[[commands]]
name = "勘察"
description = "查看当前系统状态、项目、等待队列和最近任务"
exec = "E:\\ai\\selfwork_ytl\\controller\\cc-controller.exe status"

[[commands]]
name = "看看"
description = "查看当前系统状态、项目、等待队列和最近任务"
exec = "E:\\ai\\selfwork_ytl\\controller\\cc-controller.exe status"
```

模板中使用占位路径：

```toml
exec = "YOUR_PROJECT_ROOT\\controller\\cc-controller.exe status"
```

保留 `/状态`，把 `/查看`、`/勘察`、`/看看` 都作为真实 command，不依赖 alias。

### 验证

微信和 QQ 各发送：

```text
/查看
/勘察
/看看
/状态
```

预期：

- 都返回同一类系统状态。
- 不再出现 `不是 cc-connect 命令`。
- 不再触发 `Invalid signature in thinking block`。

## Step 2: 修复 QQ Codex 通道 — 部分完成

### 问题

当前 config 中 Codex/QQ project 仍存在，但用户报告“大号 QQ 给小号 QQ Codex 消息不行”。可能原因有三类：

1. QQ 发送者不匹配 `allow_from`。
2. `发给codex` 已从旧 direct codex project 改成 `/问codex`，但 QQ 平台没有正确加载新 command。
3. `cc-controller` 回调写死发到 `cc` project，导致 QQ 触发的 Codex 最终结果回到微信或其他通道。

### 修改

先不改大架构，按最小路径修复：

1. 确认 QQ project 仍配置：
   - platform/provider 是 QQ/OneBot。
   - `allow_from` 包含大号 QQ。
   - `[[aliases]]` 中 Codex/GPT alias 只映射到 `/问codex`。
2. 确认 `[[commands]] name = "问codex"` 在部署 config 中存在，且 exec 指向：

```text
E:\ai\selfwork_ytl\controller\cc-controller.exe ask-codex {{args}}
```

3. 给 `ask-codex` 增加 source-aware callback 参数，避免硬编码回 `cc`：

```text
cc-controller.exe ask-codex --reply-project codex {{args}}
```

如果 cc-connect 无法传入来源 project，则先在 QQ/codex 项目的 command 里固定 `--reply-project codex`，微信/cc 项目的 command 固定 `--reply-project cc`。

### 验证

从 QQ 大号发给 QQ 小号：

```text
/状态
发给codex 2+2等于几
```

预期：

- `/状态` 在 QQ 返回。
- Codex 启动消息在 QQ 返回。
- Codex 最终答案也在 QQ 返回。
- 微信不收到这次 QQ 触发的 Codex 回调。

## Step 3: 清理 Codex 输出里的 taskkill 噪音和乱码 ✅ 已完成

### 问题

Codex 回答中出现类似内容：

```text
�ɹ�: ����ֹ PID ...
SUCCESS: The process with PID ...
```

这是 cleanup/taskkill 的 stdout/stderr 混入了模型答案。它不是 Codex 内容，也不应该进入用户回调。

### 修改

在 Go controller 中处理两层：

1. 执行 `taskkill` 时丢弃 stdout/stderr：

```go
cmd.Stdout = io.Discard
cmd.Stderr = io.Discard
```

2. 在 Codex 输出清理函数里增加兜底过滤：

- `SUCCESS: The process with PID`
- `成功: 已终止 PID`
- mojibake 前缀 `�ɹ�:`
- 只包含 PID 终止信息的行

不要过滤真实 Codex answer。

### 验证

运行：

```text
发给codex 你是什么模型
```

预期：

- 回答只包含 Codex 正文和建议下一步。
- 不出现 taskkill PID 行。
- 不出现中文 taskkill 乱码。

新增 Go 单测：

- 输入包含英文 taskkill 行 + 正文，输出只保留正文。
- 输入包含中文 mojibake taskkill 行 + 正文，输出只保留正文。

## Step 4: 配置同步和模板固化 ✅ 已完成

### 问题

cc-base 从 GitHub 同步回来后，模板可能缺少本机新增命令，导致重装或同步后功能消失。

### 修改

把本机已验证配置同步回模板，但模板必须脱敏：

- 保留 command 名称和 exec 结构。
- 路径使用 `YOUR_PROJECT_ROOT`。
- QQ/微信 token、account_id、真实 sender ID 不进入模板。
- `scripts/config.toml.template` 必须包含：
  - `/cc`
  - `/状态`
  - `/查看`
  - `/勘察`
  - `/看看`
  - `/问codex`
  - `/codex结果`
  - `/执行`
  - `/取消`
  - `/项目`
  - `/切项目`
  - `ok/好/可以/确认` alias 到 `/执行`
  - Codex/GPT alias 到 `/问codex`
  - CC/Opus alias 到 `/cc`

### 验证

在 cc-base 仓库扫描：

```powershell
Select-String -Path .\**\* -Pattern "ilinkai|wx_token|account_id = `"[真实值]|G:\\proteinwork|E:\\ai\\selfwork_ytl|:7890|:7891"
```

预期：

- 不出现真实 token、真实 account_id、真实本机敏感路径。
- 模板只出现占位符。

## Step 5: 文档更新 — 部分完成

### 修改

更新以下文件：

- `SKILL.md`
- `README.md`
- `docs/qq-setup.md`
- `docs/config-management.md`

必须写清楚：

- `/查看` 是真实 command，不是 alias。
- `/状态` 和 `/查看` 等价。
- QQ Codex 回调必须回到 QQ project，不应硬编码到微信 `cc` project。
- `发给cc/问cc/opus` 走 session-aware `/cc`。
- `发给codex/问codex/gpt` 走 `/问codex`。
- 执行任务的短确认：`ok/好/可以/确认` 只在唯一 waiting 任务时自动执行；多个 waiting 时必须 `/执行 1` 或 `/执行 RunId`。

## Step 6: 最终验收清单

### 微信端

```text
/查看
/cc 我叫李四，记住
/cc 我叫什么？
/cc 创建文件 mobile-ok.txt
好
```

预期：

- `/查看` 正常返回状态。
- `/cc` 能记住同一 session 内上下文。
- 执行确认卡显示真实 `CC_EXECUTE_WORK_DIR`。
- `好` 能执行唯一等待任务。

### QQ Codex 端

```text
/状态
发给codex 你是什么模型
```

预期：

- QQ 能收到启动消息。
- QQ 能收到最终 Codex 答案。
- 答案无 taskkill 噪音。
- 微信不串台收到 QQ 的 Codex 答案。

### 配置恢复测试

从 `scripts/config.toml.template` 重新生成一份测试 config，替换占位符后应具备同等命令集合。

## Step 7: 多模型 QQ 路由（DeepSeek / GLM）

### 背景

config.toml 已配置三个 provider（OpenAI 已填 key，DeepSeek/GLM 留空待填）。用户希望从 QQ 分别问不同模型。

### 方案候选

1. **每个模型一个 cc-connect project**：QQ 里通过 `/问deepseek`、`/问glm` 命令路由到独立 project
2. **走 cc-controller 命令路由**：加 `[[commands]]` 分别调不同模型的 CLI（类似微信 `/问codex`）

### 前置条件

- DeepSeek API key
- GLM (智谱) API key
- 确定方案后实现路由

### 状态

待实施，API key 到位后开始。

## Step 8: CC_MODEL 支持 ✅ 已完成

### 修改（2026-05-19）

- `cc.go`：读取 `CC_MODEL` 环境变量，传 `--model` 给 Claude CLI
- `start.ps1`：默认设置 `CC_MODEL=claude-opus-4-6`，不影响 CLI 全局默认
- `docs/env-vars.md`：已更新文档

## 踩坑记录（2026-05-19 QQ 接入）

| 坑 | 现象 | 根因 | 修复 |
|----|------|------|------|
| NapCat WS 端口 | cc-connect `ws connect failed` | NapCat WebSocket Server 默认端口 6099 跟 WebUI 冲突 | 改为 3001 |
| NapCat 消息格式 | 连上立刻断开 `close 1005` | 消息格式设为 Array，cc-connect 不支持 | 改为 String |
| NapCat 启用开关 | WebSocket 不监听 | WebSocket Server 配置里"启用"未打开 | 打开启用开关 |
| 启动时序 | cc-connect 比 NapCat 先就绪 | `docker start` 后 NapCat 需 1-3 分钟启动 | 重启 cc-connect 或 start.ps1 加 sleep |
| API key 格式 | `${sk-proj-...}` 被解析为环境变量 | 用户把 key 放进了 `${}` 占位符里 | 直接写 key，不用 `${}` |
| Docker 镜像名 | `napneko-docker:latest` 拉取 403 | 本地镜像是 `napcat-docker`，不是 `napneko-docker` | 用本地已有镜像名 |

## 风险和边界

- 如果 QQ OneBot/NapCat 本身断连，本计划只能让配置正确，不能替代 QQ 网关排障。
- 如果 cc-connect 不提供 source project 给 command，短期只能通过不同 project 的 command 写死 `--reply-project`。
- `/查看` 修复必须写进真实 `[[commands]]`，只写 alias 无效。
- taskkill 过滤只能过滤进程清理噪音，不能掩盖真实 Codex 错误。

## 剩余工作

1. **Step 2**：QQ Codex `--reply-project` 回调隔离（防止 QQ 结果串到微信）
2. **Step 5**：SKILL.md、config-management.md 文档更新
3. **Step 6**：微信 + QQ 双端最终验收
4. **Step 7**：多模型路由（DeepSeek/GLM），待 API key 到位

已完成：Step 1 ✅ Step 3 ✅ Step 4 ✅ Step 8 ✅
