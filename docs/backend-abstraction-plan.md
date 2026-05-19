# cc-base Backend Abstraction Plan

## Summary

cc-base 要作为可上传 GitHub 的通用 skill，不能把“Codex”“Claude”“微信”“QQ”写死成本机专用路径。

本计划的核心定义：

- **平台 platform**：消息从哪里来、回到哪里去，例如 WeChat、QQ、Feishu。
- **角色 role**：用户想调用哪类 agent，例如 CC 主执行者、Codex 第二意见/审查者。
- **后端 backend**：这个角色实际由什么实现，例如 native Claude、native Codex、OpenAI API、DeepSeek API、GLM API。

重要原则：

> CC 和 Codex 是 cc-base 里的“角色”，不是固定模型。native CLI 和第三方 API 都只是角色的 backend。

## Goals

1. 微信、QQ、未来飞书使用同一套角色命令。
2. 用户可以选择是否使用原生 Claude/Codex。
3. 没有 Codex 原生环境的用户，可以用 OpenAI/DeepSeek/GLM API 替代 Codex 角色。
4. 没有 Claude 原生环境的用户，未来可以用 API backend 替代 CC 角色。
5. 本机用户仍可保持最快路径：native Claude + native Codex。
6. 配置模板不泄露本机路径、token、QQ 号、微信 account_id。

## Non-Goals

本阶段不做：

- 不重写 cc-connect。
- 不实现完整飞书 provider。
- 不实现所有第三方 API provider。
- 不改变用户现有可用的微信/QQ native 路径。
- 不把普通聊天全部强制走 Go router。

## Target Architecture

```text
User
  |
  v
Platform Provider
  - wechat
  - qq
  - feishu (future)
  |
  v
cc-connect
  |
  v
Role Command
  - cc
  - codex
  |
  v
Backend Selector
  - native
  - openai
  - deepseek
  - glm
  |
  v
Actual Runner
  - claudecode / claude CLI
  - codex agent / codex CLI
  - OpenAI-compatible API
  - DeepSeek API
  - GLM API
```

## Role Definitions

### CC Role

用途：

- 主对话
- 项目上下文
- 文件读写
- 执行任务
- 长 session 记忆
- controller/cc-connect 修复

入口不变：

```text
/cc <消息>
发给cc <消息>
问cc <消息>
opus <消息>
```

可选 backend：

```text
native_claude      # 当前本机默认，Claude Code / claudecode
anthropic_api      # future
openai             # future, OpenAI-compatible
deepseek           # future, OpenAI-compatible
glm                # future, OpenAI-compatible
```

### Codex Role

用途：

- 第二意见
- 计划审查
- 技术建议
- 代码审查
- 与 CC 交叉验证

入口不变：

```text
发给codex <消息>
问codex <消息>
gpt <消息>
/问codex <消息>
```

可选 backend：

```text
native_codex   # 当前本机默认，cc-connect 原生 codex agent 或 codex CLI
openai         # API 替代 Codex 角色
deepseek       # API 替代 Codex 角色
glm            # API 替代 Codex 角色
```

## Configuration Model

在模板中增加概念性配置块。第一版可以只写文档和模板注释，不要求 cc-connect 原生解析这些块。

```toml
[cc_base.platform]
provider = "wechat" # wechat | qq | feishu

[cc_base.cc_backend]
type = "native_claude" # native_claude | anthropic_api | openai | deepseek | glm

[cc_base.codex_backend]
type = "native_codex" # native_codex | openai | deepseek | glm
```

### Backend Presets

模板文档提供以下 preset：

```text
Preset A: WeChat + native Claude + native Codex
Preset B: QQ + native Claude + native Codex
Preset C: WeChat + native Claude + DeepSeek as Codex role
Preset D: QQ + native Claude + DeepSeek as Codex role
Preset E: WeChat/QQ + API-only role backends (future)
Preset F: Feishu output callback + native/API role backends (future)
```

## Environment Variable Model

CC role 和 Codex role 使用对称环境变量。

CC role：

```text
CC_CC_BACKEND=native_claude
CC_CC_BACKEND=anthropic_api
CC_CC_BACKEND=openai
CC_CC_BACKEND=deepseek
CC_CC_BACKEND=glm

CC_CC_API_BASE=...
CC_CC_API_KEY=...
CC_CC_MODEL=...
```

Codex role：

```text
CC_CODEX_BACKEND=native_codex
CC_CODEX_BACKEND=openai
CC_CODEX_BACKEND=deepseek
CC_CODEX_BACKEND=glm

CC_CODEX_API_BASE=...
CC_CODEX_API_KEY=...
CC_CODEX_MODEL=...
```

第一轮只要求 `CC_CC_BACKEND=native_claude` 真正可执行。其它 CC role API backend 只允许配置，并在运行时明确提示未实现。

## Feishu Integration Plan

飞书集成分两阶段做。不要在第一轮把飞书当成完整 cc-connect 替代品。

可复用项目：

```text
https://github.com/riba2534/feishu-cli.git
```

定位：

- `feishu-cli` 是飞书开放平台 CLI。
- 它擅长飞书文档、知识库、表格、消息、日历、任务等 API 操作。
- 它可以帮助 cc-base 少写大量飞书 API 封装。
- 它不是直接替代 cc-connect 的实时聊天桥接。

### Phase 1: Feishu Output Provider

第一阶段只做输出能力，不做实时接收飞书消息。

目标：

```text
cc-controller run 完成
  -> callback provider = feishu
  -> 发送结果到飞书 P2P/群
  -> 可选：把 plan/review/summary 导入飞书文档或知识库
```

新增配置：

```text
CC_CALLBACK_PROVIDER=feishu
CC_FEISHU_CLI=feishu-cli
CC_FEISHU_CHAT_ID=...
CC_FEISHU_DOC_FOLDER=...
```

建议新增：

```text
internal/callback/feishu.go
```

职责：

- 调用 `feishu-cli` 发送文本消息。
- 后续可调用 `feishu-cli` 导入 Markdown 到飞书文档。
- callback 失败时写入 run 目录的 `callback-error.md`。

第一阶段不做：

- 不做飞书 webhook receiver。
- 不处理飞书事件订阅。
- 不实现消息去重。
- 不把飞书作为主交互入口。

验收：

```text
CC_CALLBACK_PROVIDER=feishu
```

运行任意已完成任务后：

- 飞书指定 chat 能收到结果。
- 本地 run 目录保留 callback 状态。
- callback 失败时错误清晰，不影响 run 本身完成状态。

### Phase 2: Feishu Input Provider

第二阶段才做完整飞书聊天入口。

目标：

```text
Feishu user message
  -> feishu receiver/webhook
  -> cc-controller route
  -> CC/Codex role
  -> feishu callback
```

需要设计：

- 飞书事件订阅。
- webhook 验签。
- 消息去重。
- 用户 allowlist/adminlist。
- P2P 和群聊 chat_id 映射。
- 与微信/QQ 的 `reply-project` 等价隔离。

此阶段可以参考 `feishu-cli` 的消息读取、发送和 auth 逻辑，但不要把它直接当 receiver。

验收：

- 飞书发 `/查看` 能返回状态。
- 飞书发 `发给codex 2+2等于几` 能回到飞书。
- 微信/QQ/飞书三平台互不串台。

### Setup Integration

`setup.ps1` 增加飞书相关选项，但第一轮只标为 future。

平台选择未来扩展为：

```text
1. 微信
2. QQ (NapCat/OneBot)
3. 微信 + QQ
4. 飞书输出 callback
5. 飞书完整聊天入口 (future)
```

当用户选择飞书输出 callback 时，提示：

```text
请先安装并登录 feishu-cli。
然后填写 CC_FEISHU_CHAT_ID。
```

不要在 setup 第一轮中自动安装或登录 `feishu-cli`。

## Command Mapping Rules

### Rule 1: User-facing commands should not expose backend

用户继续使用：

```text
发给cc
问cc
opus
发给codex
问codex
gpt
```

不要要求普通用户理解：

```text
ask-openai
ask-deepseek
ask-native-codex
```

### Rule 2: Backend is chosen by config

例如：

```toml
[cc_base.codex_backend]
type = "deepseek"
```

此时：

```text
发给codex 这个计划怎么样
```

实际应调用 DeepSeek backend，但用户仍然理解为“让 Codex 角色审查”。

### Rule 3: Native remains fastest default

如果用户有原生 Codex/Claude 环境，默认使用 native backend。

第三方 API backend 是替代能力，不应该让本机 native 用户变慢。

### Rule 4: Platform should not change role semantics

微信和 QQ 的命令语义一致：

```text
微信: 发给codex xxx
QQ:   发给codex xxx
```

两者只差 platform provider，不差 agent role。

## Implementation Plan

### Step 0: Interactive First-Time Setup

cc-base 第一次安装时需要一个交互式配置向导，避免新用户手工理解整份 `config.toml.template`。

新增：

```text
scripts/setup.ps1
```

目标：

- 让用户通过选择数字和粘贴凭据完成配置。
- 根据用户选择生成可运行的 `config.toml`。
- 同时写出必要的环境变量提示。
- 不把真实 token/API key 写回 Git 模板。

交互流程：

```text
=== cc-base 首次配置 ===

1. 选择平台
   [1] 微信
   [2] QQ (NapCat/OneBot)
   [3] 微信 + QQ

2. CC 角色后端
   [1] native Claude Code (推荐，需已安装 claude CLI)
   [2] Anthropic API (future，不在第一轮实现)
   [3] OpenAI API (future，不在第一轮实现)
   [4] DeepSeek API (future，不在第一轮实现)
   [5] GLM API (future，不在第一轮实现)

3. Codex 角色后端
   [1] native Codex CLI (推荐，需已安装 codex CLI)
   [2] OpenAI API
   [3] DeepSeek API
   [4] GLM API

4. 填入凭据
   - 微信：提示用户先运行 cc-connect 扫码，并粘贴终端输出中的 token/base_url/account_id。
   - QQ：输入 bot QQ、允许访问的用户 QQ、NapCat WebSocket URL、token。
   - API：输入 API base_url、model、API key。

5. 生成配置
   - 输出 config.toml。
   - 输出需要设置的环境变量。
   - 提醒用户真实 config 不要提交 Git。
```

第一轮 `setup.ps1` 只需要支持：

- 平台：微信、QQ、微信+QQ。
- CC backend：`native_claude`，并允许选择 `anthropic_api/openai/deepseek/glm` 作为 future 配置。
- Codex backend：`native_codex`、`openai`、`deepseek`、`glm`。
- API backend 只写配置和环境变量，不要求所有 provider 真正可调用。

`setup.ps1` 不做：

- 不自动安装 claude/codex/cc-connect/NapCat。
- 不自动启动或登录微信/QQ。
- 不把 API key 写进 Git 跟踪文件。
- 不修改 cc-connect 源码。

输出文件建议：

```text
config.toml                  # 真实配置，gitignore
.env.cc-base.local           # 本机环境变量提示，gitignore
```

验收：

- 新用户只看向导，也能生成一份基本可用配置。
- `native_codex` 用户不需要填写 API key。
- API backend 用户能看到明确的 CC role 与 Codex role 环境变量设置。
- 生成文件中真实 token/key 不进入 `scripts/config.toml.template`。

### Step 1: Documentation First

修改：

- `README.md`
- `SKILL.md`
- `docs/config-management.md`
- `docs/qq-setup.md`
- `docs/wechat-setup.md`

写清楚：

- platform / role / backend 三层定义。
- CC/Codex 是角色，不是固定模型。
- native/API 是 backend。
- 微信/QQ 是 platform provider。
- 第三方 API 用来替代 Codex role，不是给本机 native 用户强制绕远路。

验收：

- 新用户读 README 能理解自己应该选 native 还是 API。
- 文档不再把 Codex 等同于 OpenAI API。
- 文档不再把微信和 QQ 配置写成两套互相独立的系统。

### Step 2: Template Presets

修改：

- `scripts/config.toml.template`

增加注释区：

```toml
# === cc-base role/backend selection ===
# Codex role can be backed by native Codex, OpenAI, DeepSeek, or GLM.
# Choose one preset below and keep command names stable.
```

提供 preset 注释，不要求用户一次理解所有字段。

验收：

- 模板中没有真实 token、QQ 号、微信 account_id、本机路径。
- 模板能表达 native Codex 和 API Codex 两种模式。
- `发给codex/问codex/gpt` 命令入口保持一致。

### Step 3: Minimal Backend Selector for Codex Role

在 Go controller 中增加最小 backend selector。

建议新增：

```text
cmd/cc-controller/backend.go
```

职责：

- 读取环境变量或配置文件。
- 返回 Codex role 当前 backend。
- 第一版只支持：
  - `native_codex`
  - `openai`
  - `deepseek`
  - `glm`

建议环境变量：

```text
CC_CODEX_BACKEND=native_codex
CC_CODEX_BACKEND=openai
CC_CODEX_BACKEND=deepseek
CC_CODEX_BACKEND=glm
```

第一版行为：

- `native_codex`：保持当前 native 路径，不绕 API。
- `openai/deepseek/glm`：走现有 API runner 或占位返回明确错误。

如果某 backend 尚未实现，不要静默失败，返回：

```text
Codex backend 'deepseek' is configured but not implemented or missing API key.
```

验收：

- 未设置 `CC_CODEX_BACKEND` 时默认 `native_codex`。
- 设置未知值时明确报错。
- native 路径不被破坏。

### Step 4: Configurable API Backend

API backend 使用统一 OpenAI-compatible 配置：

```text
CC_CODEX_BACKEND=openai
CC_CODEX_API_BASE=https://api.openai.com/v1
CC_CODEX_API_KEY=...
CC_CODEX_MODEL=...
```

DeepSeek 示例：

```text
CC_CODEX_BACKEND=deepseek
CC_CODEX_API_BASE=https://api.deepseek.com/v1
CC_CODEX_API_KEY=...
CC_CODEX_MODEL=deepseek-chat
```

GLM 示例：

```text
CC_CODEX_BACKEND=glm
CC_CODEX_API_BASE=https://open.bigmodel.cn/api/paas/v4
CC_CODEX_API_KEY=...
CC_CODEX_MODEL=glm-4
```

验收：

- 缺 API key 时明确提示。
- API backend 的输出格式和 native Codex role 一致。
- 用户命令不变。

### Step 5: Platform Callback Consistency

继续使用已经完成的 `reply-project` 隔离思路。

要求：

- 微信发起，回微信。
- QQ 发起，回 QQ。
- Codex backend 是 native/API 不影响回调目标。

验收：

微信：

```text
发给codex 2+2等于几
```

QQ：

```text
发给codex 2+2等于几
```

预期：

- 两个平台都收到各自结果。
- 不串台。
- backend 切换后仍不串台。

## Migration Rules

### Existing Local User

你的本机配置应保持：

```text
CC backend: native_claude
Codex backend: native_codex
Platform: wechat + qq
```

目标：

- 不牺牲速度。
- 不强制改用 API。
- 只把已跑通的配置固化到模板。

### New User Without Native Codex

新用户可以选择：

```text
CC backend: native_claude
Codex backend: deepseek/openai/glm
```

目标：

- 仍然可以使用 `发给codex` 做第二意见。
- 不需要安装 Codex 原生 agent。

### API-only User

未来支持：

```text
CC backend: anthropic_api/openai/deepseek/glm
Codex backend: deepseek/openai/glm
```

本阶段只写为 future，不要求实现。

## Verification Checklist

### Native Mode

```text
CC_CODEX_BACKEND=native_codex
```

测试：

```text
发给codex 2+2等于几
发给codex 你是什么模型
```

预期：

- 走 native Codex。
- 速度不比当前明显变慢。
- 输出不混入 taskkill 噪音。

### API Mode

```text
CC_CODEX_BACKEND=deepseek
```

测试：

```text
发给codex 2+2等于几
```

预期：

- 如果 key 已配置：返回 DeepSeek 结果，但以 Codex role 格式展示。
- 如果 key 未配置：明确提示缺少 `CC_CODEX_API_KEY`。

### Platform Mode

微信和 QQ 分别测试：

```text
/查看
发给codex 2+2等于几
```

预期：

- `/查看` 两端都能用。
- 回调回到发起平台。
- backend 选择不影响平台回调。

## Risks

### Risk 1: 名称误导

如果 `发给codex` 背后是 DeepSeek，可能让用户误解为真正 Codex。

处理：

在回调中显示 backend：

```text
[Codex role: deepseek] 已回复
```

native 时：

```text
[Codex role: native] 已回复
```

### Risk 2: 过早抽象

不要一次性实现所有 provider。先实现 selector + native 默认 + API 缺 key 明确报错。

### Risk 3: 配置漂移

每次改本机 config 后，必须同步：

- source config
- deployed config
- `scripts/config.toml.template`
- docs

## Recommended First PR

第一轮只做：

1. 写文档：platform / role / backend 三层模型。
2. 增加 `scripts/setup.ps1` 的第一版交互向导。
3. 更新 `config.toml.template`，加入 backend preset 注释。
4. 增加 `CC_CODEX_BACKEND` 读取逻辑，默认 `native_codex`。
5. 未实现 backend 明确报错。
6. 不改变现有 native 行为。

第一轮不要做：

- 不接 DeepSeek 真 API。
- 不接 GLM 真 API。
- 不改 cc-connect 内部。
- 不改变用户已跑通的微信/QQ native 路径。

这样风险最小，也能把 cc-base 从“本机脚本集合”推进成“可配置通用模板”。

## Implementation Constraints

执行本计划时必须遵守以下约束。

### Constraint 1: Backend selector 插入点

backend selector 只插在 `codex.go` 的 `runCodex()` 入口处。

目标结构：

```text
runCodex()
  -> resolveCodexBackend()
       -> native_codex -> existing codex CLI path
       -> openai/deepseek/glm -> runAPIBackend()
```

不要在 `ask.go` 或 `common.go` 里做 backend 分支。

理由：

- `ask.go` 负责提交 run。
- `common.go` 负责通用工具。
- `codex.go` 才是 Codex role 的唯一执行入口。

### Constraint 2: `[cc_base.*]` 第一轮只写注释

cc-connect 不解析自定义 TOML 块，因此第一轮不要让用户以为以下配置已经被 cc-connect 原生读取：

```toml
# [cc_base.codex_backend]
# type = "native_codex"
```

真实生效来源只使用环境变量：

```text
CC_CODEX_BACKEND
CC_CODEX_API_BASE
CC_CODEX_API_KEY
CC_CODEX_MODEL
```

模板中的 `[cc_base.*]` 只作为说明和 future schema。

### Constraint 3: API backend 统一 OpenAI-compatible

OpenAI、DeepSeek、GLM 第一轮统一走 OpenAI-compatible chat completions client。

不要写三套 provider client。

差异只来自：

```text
base_url
api_key
model
```

### Constraint 4: Native Codex zero regression

默认情况必须保持现有 native Codex 行为。

要求：

- `CC_CODEX_BACKEND` 未设置时等价于 `native_codex`。
- `CC_CODEX_BACKEND=native_codex` 时走现有 codex CLI 路径。
- 现有微信/QQ native Codex 路径不变慢、不改输出协议、不破坏 callback 隔离。
- 加测试或手动验收记录说明 native path 没有回归。

### Constraint 5: Setup 不负责真实 provider 全量实现

`setup.ps1` 可以生成 API backend 配置，但不代表所有 API provider 第一轮都已可用。

如果用户选择了尚未实现或缺 key 的 backend，运行时必须明确提示：

```text
Codex backend 'deepseek' is configured but missing CC_CODEX_API_KEY or not implemented yet.
```

不要静默 fallback 到 native，也不要假装执行成功。

CC role 同理：

```text
CC backend 'deepseek' is configured but not implemented yet. Please use native_claude or implement runCCAPIBackend().
```
