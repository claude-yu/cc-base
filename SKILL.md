---
name: cc-base
description: Claude Code 基础工作流 + 移动端远程操控 pipeline + Chat-Instinct 学习。涵盖 cc-connect 命令编码、后台进程管理、config 同步、PowerShell 安全、controller 修复、聊天入口学习、计划质询、Codex 异步问答和自动回传。所有通过微信/飞书等移动 app 远程操控 CC+Codex 的场景必须先加载此 skill。完全自包含：内置 22 个 PowerShell 脚本模板 + config 模板。
version: 2.3.0
---

# cc-base — 远程操控 Skill v2.3.0

所有任务的前置流程，优先级高于具体领域 skill。
适用于通过微信/飞书/Telegram 等移动 app 远程操控 CC+Codex 的场景。

**v2.3.0 新增**：Research-Memory bridge、通用 MD 状态检查、controller 命令 chat-log wrapper、计划审查自动回传、Codex 异步问答。

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
│       ├── submit-codex-ask.ps1      # /问codex 入口（异步）
│       ├── codex-ask-runner.ps1      # Codex ask 后台 runner + 自动回传
│       ├── show-codex-ask.ps1        # /codex结果
│       ├── chat-log-writer.ps1       # research-memory chat JSONL writer
│       ├── auto-callback-toggle.ps1  # /自动回传 开/关
│       ├── grill-plan.ps1            # /质询计划（Grill-Me）
│       ├── instinct-status.ps1       # /学习状态
│       └── evolve-instincts.ps1      # /进化习惯
├── rules/                            # 操作规则
│   ├── encoding.md                   # GBK/UTF-8 编码规则
│   ├── proxy.md                      # 代理隔离规则
│   ├── process.md                    # 后台进程管理规则
│   ├── safety.md                     # 安全约束/路径限制
│   ├── powershell.md                 # 特殊字符/null-guard
│   └── new-script-checklist.md       # 新脚本检查清单
└── docs/                             # 经验总结
    ├── bug-diagnosis.md              # Bug 诊断速查 + 踩坑实录
    ├── codex-convergence.md          # Codex 审查收敛策略 + Grill-Me
    ├── config-management.md          # Config 双文件 + Pipeline 架构
    ├── env-vars.md                   # ★ 环境变量配置参考
    ├── instinct-learning.md          # ★ Chat-Instinct 学习系统文档
    ├── research-memory-plan.md       # ★ Research-Memory bridge 冻结计划
    └── wechat-setup.md               # ★ WeChat 企业号接入引导
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
| Chat-Instinct 学习系统 | `docs/instinct-learning.md` |
| Research-Memory bridge | `docs/research-memory-plan.md` |
| WeChat 接入 | `docs/wechat-setup.md` |
| 所有脚本源码 | `scripts/bin/*.ps1` |
| Config 模板 | `scripts/config.toml.template` |
| 启动脚本 | `scripts/start.ps1` |

## 移动端命令速查

| 命令 | 用途 |
|------|------|
| `/计划审查 <任务>` | CC 写计划 + Codex 审查，异步执行，完成后可自动回传 |
| `/查看审查 [RunId]` | 查看计划审查状态或结果；不传 RunId 时看最新 |
| `/修复 <问题描述>` | CC 直接修复 controller/cc-connect 报错（别名到 `/修复controller`） |
| `/md状态检查 [MD工作目录]` | 只读扫描任意 MD/GROMACS 工作目录和 log tail |
| `/问codex <问题>` | 异步询问 Codex，完成后自动回传 |
| `/codex结果 [RunId]` | 查看 Codex ask 状态或结果 |
| `/质询计划` | Grill-Me 模式质询最近计划 |
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

计划审查 runner 支持完成后主动推回微信/飞书聊天窗口，避免用户反复发送“情况如何”。

**用户操作**：
- `/自动回传 开` — 开启计划审查完成后自动推回
- `/自动回传 关` — 关闭自动推回，只保留 run 结果
- `/自动回传` — 查看当前状态

底层脚本：`scripts/bin/auto-callback-toggle.ps1`。关闭状态通过 `<controller>/auto-callback.disabled` 标记。

## 内置：Codex 异步问答

`/问codex <问题>` 是 advice-only 异步问答入口，不走 `invoke-controller-command` wrapper，避免 wrapper RunId 和 AskRunId 双 ID。提交脚本自己负责生成 AskRunId、写 chat-log、启动 runner、自动回传。

**查看结果**：`/codex结果 [RunId]`；不传 RunId 时显示最新 Codex ask run。

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
3. 复制 `scripts/config.toml.template` → `<project>/cc-connect/config.toml`，填入实际凭据
4. 复制 `scripts/start.ps1` → `<project>/cc-connect/start.ps1`
5. 创建 `<project>/controller/runs/` 目录
6. 设置环境变量：`CC_WORK_DIR`（必需），其他按需
7. 启动：`powershell -NoProfile -ExecutionPolicy Bypass -File <project>/cc-connect/start.ps1 -CleanSessions`

## 依赖

- [cc-connect](https://github.com/chenhg5/cc-connect) v1.3.2+
- Claude Code CLI (`claude`)
- Codex CLI (`codex`，可选）
- PowerShell 5.1 (Windows)
- 微信企业号 bot token / 飞书 / Telegram bot token
