---
name: cc-base
description: Claude Code 基础工作流 + 移动端远程操控 pipeline + Chat-Instinct 学习。涵盖 cc-connect 命令编码、后台进程管理、config 同步、PowerShell 安全、controller 修复、聊天入口学习、计划质询、Codex 异步问答、Session-aware CC 对话、多项目切换和自动回传。所有通过微信/QQ 等移动 app 远程操控 CC+Codex 的场景必须先加载此 skill。完全自包含：内置 22 个 PowerShell 脚本模板 + Go 二进制控制器 + config 模板。
version: 2.5.0
---

# cc-base — 远程操控 Skill v2.5.0

所有任务的前置流程，优先级高于具体领域 skill。
适用于通过微信/QQ 等移动 app 远程操控 CC+Codex 的场景。

**v2.5.0 新增**：QQ NapCat 接入（`docs/qq-setup.md`）、`CC_MODEL` 环境变量（controller 传 `--model` 到 Claude CLI）、多 provider 支持（OpenAI/DeepSeek/GLM）、`/查看` `/勘察` `/看看` 提升为独立 `[[commands]]`、执行确认快捷词（`ok/好/可以/确认`）单任务自动匹配。

## Skill 内置结构

```
skills/cc-base/
├── SKILL.md                          # ★ 入口索引（本文件）
├── README.md                         # 功能介绍文档
├── install.ps1                       # ★ 一键安装脚本（推荐方式）
├── 指导.md                            # 使用指导（聊天窗口速查）
├── .gitignore                        # 排除 config.toml、runs/
├── scripts/
│   ├── config.toml.template          # cc-connect 配置模板（安全，可提交 Git）
│   ├── start.ps1                     # cc-connect 启动器（自动同步 config）
│   └── bin/                          # Pipeline 脚本（部署到 controller/bin/）
│       ├── _common.ps1               # ★ 共享工具函数（CLI 检测、WorkDir、代理、观察记录）
│       ├── invoke-controller-command.ps1 # 通用命令 wrapper（chat-log in/out）
│       ├── submit-plan-review.ps1    # /计划审查 入口（异步）
│       ├── plan-review-confirm.ps1   # 后台：CC 计划 + Codex 审查
│       ├── plan-review-runner.ps1    # 后台 runner + heartbeat + 自动回传
│       ├── call-cc-readonly.ps1      # Claude CLI 只读调用
│       ├── call-cc-execute.ps1       # Claude CLI 执行调用
│       ├── call-codex-review.ps1     # Codex CLI 审查调用
│       ├── show-plan-review.ps1      # /查看审查
│       ├── collect-md-status.ps1     # /md状态检查（任意 MD/GROMACS 工作目录）
│       ├── fix-controller.ps1        # /修复controller
│       ├── execute-approved.ps1      # /批准执行（需 APPROVE）
│       ├── execute-manual-approved.ps1 # /人工批准执行
│       ├── check-codex-cli.ps1       # Codex CLI 诊断工具
│       ├── submit-codex-ask.ps1      # ⚠️SUPERSEDED → Go cc-controller.exe
│       ├── codex-ask-runner.ps1      # ⚠️SUPERSEDED → Go cc-controller.exe
│       ├── show-codex-ask.ps1        # ⚠️SUPERSEDED → Go cc-controller.exe
│       ├── chat-log-writer.ps1       # research-memory chat JSONL writer
│       ├── auto-callback-toggle.ps1  # /自动回传 开/关
│       ├── grill-plan.ps1            # /质询计划（Grill-Me）
│       ├── instinct-status.ps1       # /学习状态
│       └── evolve-instincts.ps1      # /进化习惯
├── controller/                       # ★ Go 控制器（session-aware cc/codex 权威实现）
│   ├── go.mod
│   └── cmd/cc-controller/            # 18 文件（14 源码 + 4 测试）
│       ├── main.go                   # 入口、类型、main() switch、usage()
│       ├── common.go                 # 共享辅助函数
│       ├── ask.go                    # genRunID、stateless ask
│       ├── exec.go                   # session-aware exec、mode classifier、session 管理
│       ├── cc.go                     # Claude Code runner + heartbeat goroutine
│       ├── codex.go                  # Codex runner + 输出清理 + heartbeat
│       ├── cancel.go                 # 任务取消（PID 扫描）
│       ├── project.go                # 多项目切换（active_project.json、session ID）
│       ├── status.go                 # 状态/事件/transcript + show
│       ├── classify.go               # 模式分类器（advice/readonly/execute_request）
│       ├── backend.go                # Backend 选择器（native/openai/deepseek/glm）+ API client
│       ├── queue.go                  # Waiting queue（CRUD + prune + 智能分发）
│       ├── monitor.go                # Stuck/zombie task 监控 + 自动清理
│       ├── detector.go               # 科研任务 detector 框架（GROMACS/Python/R/GenericCLI）
│       ├── detector_docker.go        # Docker 容器 detector（prosettac/haddock3/colabfold/rosetta 等）
│       ├── research_monitor.go       # /科研监控 命令处理 + 报告生成 + 手机端摘要
│       ├── main_test.go              # 单元测试（classifier、readInput、findLatestRun）
│       ├── classify_test.go          # 分类器测试
│       └── detector_test.go          # detector 单元测试（37 tests, 含误判防护）
├── rules/                            # 操作规则
│   ├── encoding.md                   # GBK/UTF-8 编码规则
│   ├── proxy.md                      # 代理隔离规则
│   ├── process.md                    # 后台进程管理规则
│   ├── safety.md                     # 安全约束/路径限制
│   ├── powershell.md                 # 特殊字符/null-guard
│   └── new-script-checklist.md       # 新脚本检查清单
└── docs/                             # 经验总结
    ├── backend-abstraction-plan.md    # ★ Backend 三层抽象架构设计
    ├── bug-diagnosis.md              # Bug 诊断速查 + 踩坑实录
    ├── codex-convergence.md          # Codex 审查收敛策略 + Grill-Me
    ├── config-management.md          # Config 双文件 + Pipeline 架构
    ├── env-vars.md                   # ★ 环境变量配置参考
    ├── instinct-learning.md          # ★ Chat-Instinct 学习系统文档
    ├── mobile-agent-next-plan.md     # 移动端 Agent 改进计划 + 踩坑实录
    ├── research-memory-plan.md       # Research-Memory bridge（冻结规格，未实现）
    ├── research-job-monitor-plan.md  # ★ 科研任务监控框架设计 + detector 规格
    ├── wechat-setup.md               # ★ WeChat 企业号接入引导
    └── qq-setup.md                   # ★ QQ NapCat Docker 接入引导
```

## 快速查找

| 要找什么 | 看哪个文件 |
|---------|-----------|
| 乱码/编码问题 | `rules/encoding.md` |
| 代理/网络问题 | `rules/proxy.md` |
| 命令卡住/无回复 | `rules/process.md` |
| 权限/路径问题 | `rules/safety.md` |
| 参数解析错误 | `rules/powershell.md` |
| 新建脚本要注意什么 | `rules/new-script-checklist.md` |
| 常见 Bug 怎么修 | `docs/bug-diagnosis.md` |
| Codex 审查策略 | `docs/codex-convergence.md` |
| Config 管理/Pipeline 架构 | `docs/config-management.md` |
| 环境变量配置 | `docs/env-vars.md` |
| Backend 抽象架构 | `docs/backend-abstraction-plan.md` |
| Chat-Instinct 学习系统 | `docs/instinct-learning.md` |
| Research-Memory bridge | `docs/research-memory-plan.md`（冻结规格，未实现） |
| 科研任务监控框架 | `docs/research-job-monitor-plan.md` |
| WeChat 接入 | `docs/wechat-setup.md` |
| QQ NapCat 接入 | `docs/qq-setup.md` |
| 所有脚本源码 | `scripts/bin/*.ps1` |
| Config 模板 | `scripts/config.toml.template` |
| 启动脚本 | `scripts/start.ps1` |

## 移动端命令速查

| 命令 | 用途 |
|------|------|
| `/cc <消息>` | **Session-aware CC 对话（推荐）** — 保持上下文连续对话，自动判断模式 |
| `/计划审查 <任务>` | CC 写计划 + Codex 审查，异步执行，完成后可自动回传 |
| `/查看审查 [RunId]` | 查看计划审查状态或结果；不传 RunId 时看最新 |
| `/修复 <问题描述>` | CC 直接修复 controller/cc-connect 报错（别名到 `/修复controller`） |
| `/md状态检查 [MD工作目录]` | 只读扫描任意 MD/GROMACS 工作目录和 log tail |
| `/项目` | 查看当前工作项目信息（Name、WorkDir、ProjectID） |
| `/状态` `/查看` `/勘察` `/看看` | 查看系统状态（项目、活动任务、最近记录）。四者均为独立 `[[commands]]`，等价 |
| `/切项目 <name\|path>` | 切换到指定项目，项目级 session 自动隔离 |
| `/问codex <问题>` | 异步询问 Codex，完成后自动回传 |
| `/codex结果 [RunId]` | 查看 Codex/CC ask 状态或结果 |
| `/cc结果 [RunId]` | 查看 CC session 状态或结果 |
| `/质询计划` | Grill-Me 模式质询最近计划 |
| `/执行 <RunId>` | 执行已确认的任务（完整工具权限） |
| `/取消任务 [RunId]` | 取消正在运行的任务 |
| `/监控` | 检查 stuck/zombie 任务，自动清理并通知 |
| `/科研监控` | 扫描科研项目目录，检测 GROMACS/Python/R/Docker 任务状态（只读） |
| `/自检` | 12 项安装自检（Claude/Codex/cc-connect/Go/Docker/config 等） |
| `/学习状态` | 查看 chat-instinct 观察记录和习惯统计 |
| `/进化习惯` | 分析观察记录，生成习惯进化候选 |
| `/自动回传 开/关` | 开关计划审查完成后是否自动推回聊天窗口 |

## 核心工作流

### Step 1：查 Skill 库（必做）

收到任何任务后，第一件事：

```
Glob ~/.claude/skills/**/SKILL.md
```

找到相关 skill → 读取模板代码 → 在其基础上改。

| Skill | 匹配关键词 |
|-------|-----------|
| `cc-base/` | 远程操控/cc-connect/pipeline/基础设施 |
| `gromacs-md/` | GROMACS/MD/分子动力学/PBC/RMSD |
| `bioinfo/` | 生信/转录组/蛋白组/DEG/WGCNA |

### Step 2：模板优先，改而不写

有 skill 脚本模板时 → 读取 `scripts/` 中的对应脚本 → 在其基础上适配。

### Step 3：Codex 按需调用（不自动）

默认不调用 Codex。只在用户明确要求审查/分析时启动。
详见 `docs/codex-convergence.md`。

## 内置：Session-Aware CC 对话

**入口命令**：`/cc <消息>`（推荐）或 `/问cc <消息>`（别名）

**核心能力**：
- **连续对话**：同一 session 内 `/cc` 自动注入最近 10 轮（≤20KB）对话上下文
- **模式分类**：auto 模式根据消息内容自动选择 advice/readonly/execute_request
- **30s 心跳**：后台 runner 每 30 秒推微信进度，长任务不超时等待
- **自动回传**：完成后自动推结果到聊天窗口

**模式分类规则**（`classify.go`）：

| 优先级 | 触发词 | 模式 |
|--------|--------|------|
| 1 | 修改/修复/删除/创建/提交/部署/commit/push 等 | execute_request（需确认） |
| 2 | 生成 + 文件/报告/脚本（非问句） | execute_request |
| 3 | 运行/执行 + 命令/脚本/测试/代码（非科研句） | execute_request |
| 4 | 查看/读取/grep/list/show 等 | readonly |
| 5 | 怎么看/在哪/是什么/表达上调等科研问句 | readonly（被 question guard 保护） |
| 6 | 其他 | advice |

**安全保护**：
- 科研场景词（修饰蛋白/位点/上调/下调/信号通路等）阻断误判为 execute
- 执行型任务生成确认卡片，需要 `/执行 RunId` 二次确认后才执行
- 执行模式使用独立沙盒目录（`CC_EXECUTE_WORK_DIR`），不污染科研项目

**查看结果**：`/cc结果 [RunId]`，不传 RunId 显示最新 run。

## 内置：多项目切换

**入口命令**：`/项目` `/切项目 <name|path>`

**核心能力**：
- **多项目支持**：同一 controller 可配置多个科研项目，互不干扰
- **项目级 session**：每个项目有独立 session 上下文（如 `work-9-default`、`work-15-default`）
- **同层解析**：`/切项目 work-15` 自动解析到 `CC_WORK_DIR` 的同层目录
- **完整路径**：也可用 `/切项目 E:\projects\work-15` 切换任意目录
- **目录不存在**时提示创建，不自动创建（安全）

**切换输出示例**：
```
已切换项目
Name: work-15
WorkDir: E:\projects\work-15
Session: work-15-default
```

**存储位置**：`controller/active_project.json`，切换后自动同步 `CC_WORK_DIR` env var。
**session 存储**：`controller/sessions/<project_id>-<name>/`，独立 transcript.jsonl。

## 内置：Chat-Instinct 学习

cc-base 内置聊天入口学习能力，自动记录用户命令使用模式。

**工作方式**：
- controller 脚本（计划审查/修复/状态检查/执行/质询）自动记录 observation
- 记录内容：command_start/end、错误、exit code、平台来源
- 存储位置：`$CC_INSTINCT_HOME/projects/<project-id>/observations.jsonl`
- project-id 由 `CC_WORK_DIR` SHA256 hash 生成，确保项目隔离

**用户操作**：
- `/学习状态` — 查看 observations 总数和已有 instincts
- `/进化习惯` — 分析重复模式，生成进化候选（用户确认后才写 instinct）

详见 `docs/instinct-learning.md`。

## 内置：自动回传

计划审查 runner 支持完成后主动推回微信聊天窗口，避免用户反复发送"情况如何"。

**用户操作**：
- `/自动回传 开` — 开启计划审查完成后自动推回
- `/自动回传 关` — 关闭自动推回，只保留 run 结果
- `/自动回传` — 查看当前状态

底层脚本：`scripts/bin/auto-callback-toggle.ps1`。关闭状态通过 `<controller>/auto-callback.disabled` 标记。

## 内置：Codex 异步问答

`/问codex <问题>` 是 advice-only 异步问答入口。**权威实现为 Go 二进制 `controller/cc-controller.exe`**，config.toml 调 `cc-controller.exe ask-codex {{args}}`；不走 `invoke-controller-command` wrapper，`ask-codex` 一次 `genRunID` 生成单一 RunId 贯穿后台 `run-codex` runner，写 run 目录 + callback 自动回传。Codex 调用用 `--sandbox read-only`。

> PS 脚本 `submit-codex-ask.ps1`/`codex-ask-runner.ps1`/`show-codex-ask.ps1` 已 **SUPERSEDED-BY-GO**，保留作历史，不再调用。`rules/powershell.md` 的「文本去特殊字符 + 压成单行」约定**仍适用**（cc-connect→argv 行为不变）。

**查看结果**：`/codex结果 [RunId]`；不传 RunId 时显示最新 run（Go `show` 经 `runIDPattern` 过滤 sidecar 目录，返回最新时间戳 run）。

## 可选：计划质询（Grill-Me）

当 `/计划审查` 结果不理想时，可启动 grill-me 模式逐条质询。

**触发**：`/质询计划` 或别名 `grill` / `grillme` / `拷问计划`

**行为**：自动读取最近一次 plan-review 的结果，one-question-at-a-time 质询。
详见 `docs/codex-convergence.md` 的"计划质询"段落。

## 可选增强：continuous-learning-v2

cc-base 的 chat-instinct 是独立内置能力，不依赖 continuous-learning-v2。

continuous-learning-v2 可作为增强：
- 提供 session 级 PreToolUse/PostToolUse hooks（100% 工具调用观察）
- 更完整的进化管线（instinct → cluster → skill/command/agent）
- 需要用户自行在 `~/.claude/settings.json` 配置 hooks

两者共享 instinct 理念和 yaml 格式，可共存不冲突。

## 新用户：WeChat 接入

首次接入微信企业号的完整流程见 `docs/wechat-setup.md`。

关键步骤：扫码后把终端底部输出发回 → 提取 token/base_url/account_id → 写入 config.toml。

## 新用户：QQ NapCat 接入

通过 NapCat Docker 连接 QQ，完整流程见 `docs/qq-setup.md`。

关键步骤：`docker run` NapCat → WebUI 扫码登录 → 配置 WebSocket（端口 3001、消息格式 String） → config.toml 添加 QQ platform。

## 别名路由

| 用户输入 | 路由到 | 说明 |
|---------|--------|------|
| `发给cc` / `问cc` / `opus` | `/cc`（session-aware） | 保持上下文连续对话 |
| `发给codex` / `问codex` / `gpt` | `/问codex` | 异步 Codex 问答 |
| `ok` / `好` / `可以` / `确认` | `/执行` | 仅当恰好有 1 个 waiting 任务时自动执行；多个 waiting 任务时需 `/执行 RunId` 指定 |

## CC_MODEL 与多 Provider 支持

**CC_MODEL**：`start.ps1` 默认设置 `CC_MODEL=claude-opus-4-6`，controller（`cc.go`）读取后传 `--model` 给 Claude CLI。可通过环境变量覆盖。

**多 Provider**：config.toml 的 `[[projects.agent.providers]]` 支持配置多个 LLM 提供商（用于 Codex agent）：
- OpenAI（默认）：`api_key` 可用 `${OPENAI_API_KEY}` 或直接写入
- DeepSeek：`base_url = "https://api.deepseek.com/v1"`
- GLM（智谱）：`base_url = "https://open.bigmodel.cn/api/paas/v4"`

模板见 `scripts/config.toml.template` 中的注释段。

## Backend 配置

CC 和 Codex 是 cc-base 中的"角色"（role），不是固定模型。每个角色的实际执行引擎（backend）通过环境变量选择。

### CC_CODEX_BACKEND

Codex 角色的 backend 选择。

| 值 | 说明 |
|----|------|
| `native_codex` | Codex CLI 原生执行（默认，推荐） |
| `openai` | OpenAI API |
| `deepseek` | DeepSeek API |
| `glm` | GLM（智谱）API |

未设置时默认 `native_codex`。API backend 需同时设置 `CC_CODEX_API_BASE`、`CC_CODEX_API_KEY`、`CC_CODEX_MODEL`。

### CC_BACKEND

CC 角色的 backend 选择（未来扩展，当前只支持 `native_claude`）。

| 值 | 说明 |
|----|------|
| `native_claude` | Claude Code CLI 原生执行（默认，推荐） |
| `anthropic_api` | Anthropic API（未来） |
| `openai` / `deepseek` / `glm` | OpenAI-compatible API（未来） |

### 使用说明

- 用户命令不变：`发给codex`/`问codex`/`gpt` 始终调用 Codex 角色，backend 对用户透明
- Native 用户无需配置任何 backend 变量，默认走最快的本机路径
- API backend 用户只需设置对应环境变量，命令入口保持一致
- 缺少 API key 时运行时会明确报错，不会静默 fallback

## 部署步骤

### 方式 A：一键安装（推荐）

```powershell
git clone https://github.com/claude-yu/cc-base.git
cd cc-base
powershell -NoProfile -ExecutionPolicy Bypass -File install.ps1 -ProjectDir "E:\ai\myproject"
```

### 方式 B：手动部署

1. 安装 cc-connect：`npm install -g cc-connect`
2. 复制 `scripts/bin/*.ps1` → `<project>/controller/bin/`
3. 编译 `controller/cmd/cc-controller/` → `<project>/controller/cc-controller.exe`
4. 复制 `scripts/config.toml.template` → `<project>/cc-connect/config.toml`，填入实际凭据
5. 复制 `scripts/start.ps1` → `<project>/cc-connect/start.ps1`
6. 创建 `<project>/controller/runs/` + `<project>/controller/sessions/` 目录
7. 设置环境变量：`CC_WORK_DIR`（必需）、`CC_CONTROLLER_DIR`（可选）、`CC_EXECUTE_WORK_DIR`（可选）
8. 启动：`powershell -NoProfile -ExecutionPolicy Bypass -File <project>/cc-connect/start.ps1 -CleanSessions`

## 环境变量参考

| 变量 | 必需 | 默认 | 说明 |
|------|------|------|------|
| `CC_WORK_DIR` | 是 | — | 科研项目工作目录 |
| `CC_MODEL` | 否 | `claude-opus-4-6` | Claude CLI 模型（controller 传 `--model`，`start.ps1` 默认设置） |
| `CC_CONTROLLER_DIR` | 否 | 自动检测 | controller 根目录（覆盖 auto-detect） |
| `CC_EXECUTE_WORK_DIR` | 否 | 同 `CC_WORK_DIR` | 执行模式沙盒目录 |
| `CC_CHAT_LOG_DIR` | 否 | — | 聊天日志输出目录 |
| `CC_INSTINCT_HOME` | 否 | `~/.cc-controller` | 学习系统存储目录 |
| `CLAUDE_CMD` | 否 | auto-detect | Claude CLI 路径覆盖 |
| `CLAUDE_PROXY` | 否 | — | Claude CLI HTTP 代理 |
| `CODEX_PROXY` | 否 | — | Codex CLI SOCKS5h 代理 |

## 依赖

- [cc-connect](https://github.com/chenhg5/cc-connect) v1.3.2+
- Claude Code CLI (`claude`)
- Codex CLI (`codex`，可选）
- Go 1.21+（编译 cc-controller 需要）
- PowerShell 5.1 (Windows)
- 微信企业号 bot token / QQ NapCat
