---
name: cc-base
description: Claude Code 基础工作流 + 移动端远程操控 pipeline + Chat-Instinct 学习。涵盖 cc-connect 命令编码、后台进程管理、config 同步、PowerShell 安全、自动 bug 诊断修复、聊天入口学习、计划质询。所有通过微信/飞书等移动 app 远程操控 CC+Codex 的场景必须先加载此 skill。完全自包含：内置 18 个 PowerShell 脚本模板 + config 模板。
version: 2.2.0
---

# cc-base — 远程操控 Skill v2.2.0

所有任务的前置流程，优先级高于具体领域 skill。
适用于通过微信/飞书/Telegram 等移动 app 远程操控 CC+Codex 的场景。

**v2.2.0 新增**：Chat-Instinct 聊天学习（内置）、Grill-Me 计划质询、WeChat 接入引导、hash-based 项目隔离。

## Skill 内置结构

```
skills/cc-base/
├── SKILL.md                          # ★ 入口索引（本文件）
├── README.md                         # 功能介绍文档
├── .gitignore                        # 排除 config.toml、runs/
├── scripts/
│   ├── config.toml.template          # cc-connect 配置模板（安全，可提交 Git）
│   ├── start.ps1                     # cc-connect 启动器（自动同步 config）
│   └── bin/                          # Pipeline 脚本（部署到 controller/bin/）
│       ├── _common.ps1               # ★ 共享工具函数（CLI 检测、WorkDir、代理、观察记录）
│       ├── submit-plan-review.ps1    # /计划审查 入口（异步）
│       ├── plan-review-confirm.ps1   # 后台：CC 计划 + Codex 审查
│       ├── call-cc-readonly.ps1      # Claude CLI 只读调用
│       ├── call-cc-execute.ps1       # Claude CLI 执行调用
│       ├── call-codex-review.ps1     # Codex CLI 审查调用
│       ├── show-plan-review.ps1      # /查看审查
│       ├── collect-md-status.ps1     # /md状态检查
│       ├── fix-controller.ps1        # /修复controller
│       ├── execute-approved.ps1      # /批准执行（需 APPROVE）
│       ├── execute-manual-approved.ps1 # /人工批准执行
│       ├── check-codex-cli.ps1       # Codex CLI 诊断工具
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
| WeChat 接入 | `docs/wechat-setup.md` |
| 所有脚本源码 | `scripts/bin/*.ps1` |
| Config 模板 | `scripts/config.toml.template` |
| 启动脚本 | `scripts/start.ps1` |

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
