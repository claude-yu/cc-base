<p align="center">
  <h1 align="center">cc-base</h1>
  <p align="center"><strong>移动端远程操控 Claude Code + Codex 的多 Agent 控制器</strong></p>
  <p align="center">
    🤖 微信 / QQ 远程操控 &nbsp;|&nbsp; 🔄 Session 连续对话 &nbsp;|&nbsp; 📋 计划审查 &nbsp;|&nbsp; 🧠 自动学习
  </p>
</p>

---

## 功能概览

| 功能 | 命令 | 说明 |
|------|------|------|
| **Session 对话** | `/cc <消息>` | 连续对话，自动判断模式（建议/只读/执行），保持上下文 |
| **多项目支持** | `/项目` `/切项目` | 切换科研项目，每个项目独立 session 隔离 |
| **计划审查** | `/计划审查 <任务>` | CC 写计划 + Codex 审查，异步执行自动回传 |
| **异步问答** | `/问codex <问题>` | 异步询问 Codex，完成后自动推回聊天 |
| **自动修复** | `/修复 <问题>` | CC 自动修复 controller/cc-connect 基础设施报错 |
| **系统状态** | `/状态` | 系统状态仪表盘（项目、活动任务、卡住检测） |
| **状态监控** | `/md状态检查 [目录]` | 只读扫描 GROMACS MD 工作目录和 log tail |
| **任务执行** | `/执行 <RunId>` | 二次确认后执行任务（完整工具权限） |
| **任务取消** | `/取消任务 [RunId]` | 取消运行中的任务 |
| **计划质询** | `/质询计划` | Grill-Me 模式逐条质询审查结果 |
| **自动回传** | `/自动回传 开/关` | 异步任务完成后自动推回聊天窗口 |

---

## 快速开始

```powershell
# 1. 克隆
git clone https://github.com/claude-yu/cc-base.git
cd cc-base

# 2. 一键安装
powershell -NoProfile -ExecutionPolicy Bypass -File install.ps1 -ProjectDir "E:\ai\myproject"

# 3. 编译 Go 控制器
cd E:\ai\myproject\controller
go build -o cc-controller.exe .\cmd\cc-controller\

# 4. 填写凭据后启动
powershell -NoProfile -ExecutionPolicy Bypass -File "E:\ai\myproject\cc-connect\start.ps1"
```

---

## 系统架构

```
手机（微信 / QQ）
    │
    ▼
cc-connect（Go 多平台聊天网关）
    │
    ├── Project: cc（Claude Code Agent）
    │   └── 命令 → cc-controller.exe（Go 控制器）
    │
    ├── Project: codex（Codex Agent）
    │   └── 命令 → cc-controller.exe（Go 控制器）
    │
    └── [[commands]] 路由
        ├── Go 二进制直调：/cc /问codex /项目 /切项目 /执行
        ├── PowerShell pipeline：/计划审查 /查看审查 /质询计划
        └── PowerShell 单步：/修复controller /md状态检查 /学习状态
```

### 三种异步管道

| 模式 | 入口 | 后台执行 | 回传 |
|------|------|----------|------|
| Session-aware CC | `/cc <msg>` → `exec-cc` | `run-cc --session <id>` | 自动 |
| Codex 问答 | `/问codex <q>` → `ask-codex` | `run-codex <RunId>` | 自动 |
| 计划审查 | `/计划审查 <task>` → `submit-plan-review.ps1` | `plan-review-runner.ps1` | 可选 |

### Go 控制器结构

`cc-controller.exe`（10 文件模块化，`controller/cmd/cc-controller/`）：

| 文件 | 职责 |
|------|------|
| `main.go` | 入口、路由 |
| `common.go` | 共享工具函数 |
| `exec.go` | Session 对话管理 |
| `cc.go` | Claude Code 执行器 + 心跳 |
| `codex.go` | Codex 执行器 |
| `project.go` | 多项目切换 |
| `classify.go` | 模式分类器 |
| `cancel.go` | 任务取消 |
| `status.go` | 状态持久化 + 查询 |
| `ask.go` | 无状态 ask 入口 |

---

## 前置依赖

| 依赖 | 版本 | 安装 |
|------|------|------|
| PowerShell 5.1+ | Windows 内置 | — |
| Node.js | 18+ | [nodejs.org](https://nodejs.org) |
| Go | 1.21+ | [go.dev](https://go.dev) |
| cc-connect | 1.3.2+ | `npm install -g cc-connect` |
| Claude Code CLI | latest | `npm install -g @anthropic-ai/claude-code` |
| Codex CLI | optional | `npm install -g @openai/codex` |
| 微信企业号 bot | 必需 | 企业号后台申请 |
| NapCat QQ | optional | [NapCat](https://napcat.napneko.icu/) |

---

## 命令速查

| 命令 | 别名 | 效果 | 实现 |
|------|------|------|------|
| `/cc <消息>` | 问cc、opus | Session-aware CC 连续对话 | Go |
| `/计划审查 <任务>` | 审查计划、让cc写计划 | CC 写计划 + Codex 审查 | PS |
| `/查看审查 [RunId]` | 审查结果 | 查看计划审查结果 | PS |
| `/问codex <问题>` | 发给codex、gpt | 异步 Codex 问答 | Go |
| `/codex结果 [RunId]` | 查看codex | 查看执行结果 | Go |
| `/cc结果 [RunId]` | 查看cc | 查看 CC 对话结果 | Go |
| `/修复 <问题>` | 修复bug、fix | CC 修复基础设施 | PS |
| `/md状态检查 [目录]` | md进度、查md | 只读扫描 MD 目录 | PS |
| `/项目` | 项目信息、当前项目 | 查看当前项目 | Go |
| `/状态` | — | 查看系统状态（项目、活动任务、最近记录） | Go |
| `/切项目 <名称|路径>` | 切换项目、切换到 | 切换科研项目 | Go |
| `/执行 <RunId>` | — | 执行确认的任务 | Go |
| `/取消任务 [RunId]` | 取消、中止、停止 | 取消运行中任务 | Go |
| `/批准执行 <RunId>` | 执行批准任务 | 执行被批准的 run | PS |
| `/质询计划` | grill、grillme | 逐条质询审查结果 | PS |
| `/学习状态` | — | 查看学习统计 | PS |
| `/进化习惯` | 进化 | 生成习惯进化候选 | PS |
| `/自动回传 开/关` | 回传 | 开关自动回传 | PS |

---

## 特性详解

### Session-aware CC 对话 (`/cc`)

```
➡️ /cc 帮我看看最新状态
⬅️ 正在读取状态...（30秒心跳推送进度）
⬅️ [CC] 当前项目 work-9，最近运行结果：...
```

- **连续对话**：同一 session 记住前文，无需重复交代背景
- **自动分类**：根据内容选择 advice / readonly / execute_request 模式
  - `"修改文件"` → execute_request（二次确认）
  - `"读取状态"` → readonly
  - `"怎么看结果"` → advice（科研问句受保护，不误判为执行）
- **30秒心跳**：长任务不超时等待，每 30 秒推送进度
- **自动回传**：完成后推结果到聊天窗口

### 多项目支持 (`/项目` `/切项目`)

```text
/项目     → 当前项目: work-9, Session: work-9-default
/切项目 work-15 → 已切换项目, Session: work-15-default
```

- 每个项目独立 session 上下文，互不干扰
- 自动解析同层目录，也支持完整路径
- 项目信息持久化到 `active_project.json`

### 计划审查 (`/计划审查`)

```
➡️ /计划审查 帮我设计蛋白结合位点分析方案
⬅️ Run ID: 20260519-123456-plan-review-xxxx
...（后台异步执行）...
⬅️ [审查完成] Codex verdict: APPROVE
    建议执行: /批准执行 20260519-123456-plan-review-xxxx
```

- CC 只读模式生成计划 → Codex 独立审查
- 自动注入领域规则
- 审查结果包含 verdict（APPROVE / REVISE / BLOCK）

### Grill-Me 质询 (`/质询计划`)

当审查结果不理想时，逐个维度深度质询：

- 成功标准是否明确
- 输入验证是否充分
- 回滚方案是否完备
- 是否有遗漏的风险

### Chat-Instinct 学习 (`/学习状态` `/进化习惯`)

- 自动记录命令使用模式
- 分析重复修复模式，生成进化候选
- 用户确认后才写入 instinct，避免噪音污染

---

## 常见问题解决方案

| 问题 | 原因 | 解决办法 |
|------|------|---------|
| 中文乱码 | GBK/UTF-8 编码链 | 脚本顶部设置 `[Console]::OutputEncoding = [System.Text.Encoding]::GetEncoding(936)` |
| 命令卡住无回复 | 后台进程挂起 | 使用 `/取消任务` 终止，或用 `/修复` 诊断 |
| `/修复` 无法修复 | 复杂底层问题 | 检查 cc-connect config 和 logs |
| Codex 长时间无回传 | Codex CLI 断连 | 检查 `CODEX_PROXY` 代理设置 |
| 分类器误判 | 关键词匹配边界 | 用 `/cc` 会二次确认执行型任务，误判不自动执行 |

---

## 相关文档

| 文档 | 内容 |
|------|------|
| [SKILL.md](SKILL.md) | 完整 Skill 参考（文件结构、部署步骤、环境变量） |
| [指导.md](指导.md) | 聊天窗口使用指南 |
| [rules/](rules/) | 编码规则（编码/代理/进程/安全/PowerShell） |
| [docs/](docs/) | 深度文档（Config管理/学习系统/WeChat接入/Codex策略） |

---

## 致谢

cc-base 基于 [cc-connect](https://github.com/chenhg5/cc-connect) 构建，感谢 [chenhg5](https://github.com/chenhg5) 提供的多平台聊天网关基础设施。

---

<p align="center">
  <a href="https://github.com/claude-yu/cc-base">GitHub</a>
</p>
