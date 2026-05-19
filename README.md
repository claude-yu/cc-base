# cc-base: Mobile-Controlled Multi-Agent Controller

> 基于 cc-connect 创建的一个微信/QQ 自用工具，搭载 session-aware CC 对话、多项目切换、模式分类、对话学习、grill-me 和自动回传能力。
> 通过移动 App 远程操控 Claude Code + Codex，支持计划审批、状态监控、自动修复、聊天学习。
>
> **基于 [cc-connect](https://github.com/chenhg5/cc-connect)** — Go 编写的多平台聊天机器人网关，支持微信企业号 / QQ / 飞书 / Telegram。

## One-Click Install

```powershell
# Clone
git clone https://github.com/claude-yu/cc-base.git
cd cc-base

# Install (interactive, guides you through setup)
powershell -NoProfile -ExecutionPolicy Bypass -File install.ps1 -ProjectDir "E:\ai\myproject"

# Or with Codex + QQ support:
powershell -NoProfile -ExecutionPolicy Bypass -File install.ps1 -ProjectDir "E:\ai\myproject" -WithCodex -WithQQ

# Compile Go controller:
cd E:\ai\myproject\controller
go build -o cc-controller.exe .\cmd\cc-controller\

# Edit credentials then start:
powershell -NoProfile -ExecutionPolicy Bypass -File "E:\ai\myproject\cc-connect\start.ps1" -CleanSessions
```

`install.ps1` handles: prerequisite check (PowerShell, Node.js, cc-connect npm package), directory structure, script deployment, config template setup, and next-steps guidance.

## Quick Start

1. **Install**: `install.ps1 -ProjectDir <your-project>`
2. **Compile controller**: `go build -o cc-controller.exe ./cmd/cc-controller/`
3. **Configure**: edit `<project>/cc-connect/config.toml` with your WeChat token/base_url/account_id
4. **Set env vars**: `CC_WORK_DIR` (required), `CODEX_PROXY` (optional)
5. **Start**: `start.ps1 -CleanSessions`
6. **Use from phone**: send `/cc 帮我查看最新状态` or `/计划审查 帮我设计方案` in WeChat

## Prerequisites

| Dependency | Version | Install |
|------------|---------|---------|
| PowerShell | 5.1+ | Windows built-in |
| Node.js | 18+ | [nodejs.org](https://nodejs.org) |
| Go | 1.21+ | [go.dev](https://go.dev) (for cc-controller.exe) |
| cc-connect | 1.3.2+ | `npm install -g cc-connect` |
| Claude Code CLI | latest | `npm install -g @anthropic-ai/claude-code` |
| Codex CLI | latest | `npm install -g @openai/codex` (optional) |

## System Architecture

```
手机（微信/QQ）
    │
    ▼
cc-connect v1.3.2（Go 二进制，Node.js 分发）
    │
    ├── WeChat 平台 ←→ 微信企业号 API
    ├── QQ 平台 ←→ NapCat WebSocket
    │
    ├── Project: cc（Claude Code Agent）
    │   └── 命令 → cc-controller.exe（Go 控制器）
    │
    ├── Project: codex（Codex Agent）
    │   └── 命令 → cc-controller.exe（Go 控制器）
    │
    └── [[commands]] 路由
        ├── Go 二进制直调（推荐）：/cc /问codex /项目 /切项目 /执行 /取消任务
        ├── PowerShell pipeline（遗留）：/计划审查 /查看审查 /质询计划
        └── PowerShell 单步（遗留）：/修复controller /md状态检查 /学习状态
```

### Three Async Pipeline Patterns

| Pattern | Entry | Runner |
|---------|-------|--------|
| Session-aware CC (Go) | `cc-controller.exe exec-cc --session <id>` | `cc-controller.exe run-cc --session <id>` |
| CC Ask (Go) | `cc-controller.exe ask-cc <text>` | `cc-controller.exe run-cc <RunId>` |
| Codex Ask (Go) | `cc-controller.exe ask-codex <text>` | `cc-controller.exe run-codex <RunId>` |
| Plan Review (PS) | `submit-plan-review.ps1` | `plan-review-runner.ps1` |

### Go Binary Architecture

`cc-controller.exe` (8 files, `controller/cmd/cc-controller/`):

| File | Responsibility |
|------|---------------|
| `main.go` | Entry, types, main() switch |
| `common.go` | Shared helpers (CLI resolve, sendCallback, readInput) |
| `ask.go` | genRunID, stateless ask entry |
| `exec.go` | Session-aware exec, session management, mode dispatch |
| `cc.go` | Claude Code runner + heartbeat goroutine |
| `codex.go` | Codex runner + output cleanup |
| `project.go` | Multi-project switching, session ID isolation |
| `classify.go` | Mode classifier (advice/readonly/execute_request) |
| `cancel.go` | Task cancellation by PID |
| `status.go` | Status/event/transcript persistence + show |

All subcommands:
- `/cc <msg>` — Session-aware CC with context persistence
- `/问cc <msg>` — Stateless CC Q&A (legacy alias)
- `/问codex <msg>` — Async Codex Q&A
- `/项目` — Show current project info
- `/切项目 <name\|path>` — Switch research project
- `/执行 <RunId>` — Execute confirmed task (full tools)
- `/取消任务 [RunId]` — Cancel running task
- `/cc结果` / `/codex结果 [RunId]` — View run status/result

## Feature List

### 1. Session-Aware CC Dialogue

**Command**: `/cc <message>` (recommended primary entry)
**Aliases**: `/问cc`, `问cc`, `opus`

- Continuous conversation with context injection (last 10 turns, ≤20KB)
- Auto mode classification: advice / readonly / execute_request
- 30-second heartbeat pushes WeChat progress for long tasks
- Background execution with auto-callback on completion

### 2. Multi-Project Switching

**Command**: `/项目` (info) | `/切项目 <name|path>` (switch)

- Each project has isolated session context (`work-9-default`, `work-15-default`)
- Sibling directory auto-resolution: `/切项目 work-15` → `G:\proteinwork\work-15`
- Full path support: `/切项目 G:\proteinwork\work-15`
- Session context stored in `controller/sessions/<project_id>-<name>/transcript.jsonl`

### 3. Plan Review Pipeline

**Command**: `/计划审查 <task description>`

- Sub-second Run ID return, async background execution
- CC read-only plan generation → Codex independent review
- Auto-injects `rules/*.md` domain rules
- Outputs verdict.md (APPROVE / REVISE / BLOCK) + summary.md

### 4. Controller Auto-Repair

**Command**: `/修复controller <error description>`
**Aliases**: `/修复`, `/fix`, `/修复bug`, `修controller`, `fixcontroller`

CC directly diagnoses and fixes: config sync, encoding corruption, script bugs, permissions, stuck processes.
Security: only `controller/`, `cc-connect/`, `~/.cc-connect/config.toml` are writable.

### 5. Status Monitoring

**Command**: `/md状态检查 [MD工作目录]`

Read-only scan of any MD/GROMACS work directory, lists `.tpr/.cpt/.log/.xtc/.trr/.edr/.gro/.pdb/.top/.itp/.mdp`, reads `.log` tail. Defaults to `CC_WORK_DIR`.

### 6. Execution Approval

| Command | Condition |
|---------|-----------|
| `/批准执行 <RunId>` | Codex verdict = APPROVE |
| `/人工批准执行 <RunId>` | manual-approval.md + not BLOCK |

### 7. Grill-Me Plan Challenge

**Command**: `/质询计划` or `grill` / `grillme`

When Codex returns REVISE and you're unsure how to improve: one-question-at-a-time interrogation across 8 dimensions (success criteria, input validation, rollback plan, etc.), each with recommended answer.

### 8. Chat-Instinct Learning

**View**: `/学习状态`
**Evolve**: `/进化习惯`

cc-base built-in chat entry learning:
- Controller commands auto-record user operation patterns
- `/进化习惯` analyzes patterns and generates candidates (user confirms before writing instinct)
- Project-level isolation (hash-based project-id)

### 9. Auto-Callback

**Command**: `/自动回传 开` or `/自动回传 关`

Controls whether plan-review results auto-push back to chat:
- On: background runner sends results on completion
- Off: results stay local, manual `/查看审查 <RunId>` to view

### 10. Codex Async Q&A

**Command**: `/问codex <question>`
**View**: `/codex结果 [RunId]`

Sends question to Codex, returns immediately with RunId, background processes, auto-callbacks on completion.

### 11. Task Cancellation

**Command**: `/取消任务 [RunId]`
**Aliases**: `/取消`, `/中止`, `/停止`, `cancel`, `abort`, `stop`

Cancels running task by PID kill tree. Omitting RunId auto-finds the latest running task.

## Command Cheatsheet

| Command | Aliases | Effect | Implementation |
|---------|---------|--------|---------------|
| `/cc <msg>` | 问cc、opus | Session-aware CC dialogue | Go exec-cc |
| `/问codex <q>` | 发给codex、gpt | Async Codex Q&A | Go ask-codex |
| `/计划审查 <task>` | 审查计划、让cc写计划 | CC plan + Codex review (async) | PS pipeline |
| `/修复controller <err>` | 修复bug、修复、fix | CC auto-fix infrastructure | PS |
| `/md状态检查 [path]` | md进度、md检查、查md | Read-only MD workspace scan | PS |
| `/项目` | 项目信息、当前项目 | Show current project info | Go |
| `/切项目 <name\|path>` | 切换项目、切换到 | Switch research project | Go |
| `/执行 <RunId>` | 执行 | Execute confirmed task | Go |
| `/取消任务 [RunId]` | 取消、中止、停止 | Cancel running task | Go |
| `/批准执行 <RunId>` | 执行批准任务 | Execute approved run | PS |
| `/人工批准执行 <RunId>` | 手动批准执行 | Manual accept + execute | PS |
| `/质询计划` | grill、grillme、拷问计划 | Grill-Me challenge | PS |
| `/学习状态` | 学习状态 | View learning stats | PS |
| `/进化习惯` | 进化 | Analyze + generate candidates | PS |
| `/自动回传 开/关` | 回传 | Toggle auto-callback | PS |
| `/codex结果 [RunId]` | 查看codex | View Codex/CC ask result | Go show |
| `/cc结果 [RunId]` | 查看cc | View CC session result | Go show |
| `/查看审查 [RunId]` | 审查结果 | View plan-review result | PS |

## Resolved Technical Issues

### GBK/UTF-8 Encoding Chain
cc-connect (Go) decodes command stdout with `GetACP()=936` (GBK), but PowerShell outputs UTF-8.
Fix: `[Console]::OutputEncoding = GetEncoding(936)` at script top.

### Start-Process Handle Inheritance
`-RedirectStandardOutput` passes pipe handle to child, blocking cc-connect.
Fix: runner script pattern - Write-Output first, then Start-Process -WindowStyle Hidden.

### Config Dual-File Sync
Source config (dev edits) → start.ps1 auto-copies → deployed config (runtime reads).

### Claude CLI Chinese Loss
`claude -p $ChineseText` drops non-ASCII characters.
Fix: UTF-8 temp file piped via stdin.

### Proxy Isolation
- Claude Code → HTTP proxy (`$env:CLAUDE_PROXY`)
- Codex CLI → SOCKS5h proxy (`$env:CODEX_PROXY`)
- Never mix. See `rules/proxy.md`.

### PowerShell `<>` Redirection Conflict
`<RunId>` in config descriptions caused PowerShell to attempt file redirection.
Fix: remove `<>` from config descriptions.

### Scientific False Positive Classification
Mode classifier initially misclassified research questions as execute_request.
Fix: 3-tier keyword matching + question pattern guard protecting research terms.

## Safety Rules

- CC read-only/advice mode: no execution commands
- Execution requires Codex APPROVE or manual confirm + not BLOCK
- `--dangerously-skip-permissions` only in execute-approved.ps1 and execute-manual-approved.ps1
- All runs write audit files to controller/runs/
- `/修复controller` must not touch project data
- Mode classifier guards: execute_request generates confirmation card (no auto-execute)

## Environment Variables

See `docs/env-vars.md` and `SKILL.md`. Key variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CC_WORK_DIR` | Yes | — | Research project working directory |
| `CC_CONTROLLER_DIR` | No | Auto-detect | Controller root (overrides auto-detect) |
| `CC_EXECUTE_WORK_DIR` | No | Same as CC_WORK_DIR | Execute mode sandbox directory |

## Startup

```powershell
# Standard (WeChat + QQ)
powershell -NoProfile -ExecutionPolicy Bypass -File <project>/cc-connect/start.ps1 -WithQQ

# WeChat only
powershell -NoProfile -ExecutionPolicy Bypass -File <project>/cc-connect/start.ps1
```

## Dependencies

- [cc-connect](https://github.com/chenhg5/cc-connect) v1.3.2+ — multi-platform chat gateway, the foundational infrastructure
- Claude Code CLI (`claude`)
- Codex CLI (`codex`, optional)
- Go 1.21+ (for cc-controller.exe compilation)
- PowerShell 5.1 (Windows)
- WeChat Work bot token / QQ NapCat

## Attribution

cc-base is a skill template and controller pipeline **built on [cc-connect](https://github.com/chenhg5/cc-connect)** by [chenhg5](https://github.com/chenhg5). cc-connect provides the multi-platform chat gateway (WeChat Work / QQ / Telegram / Feishu) that enables mobile-to-CLI communication. All credit for the underlying infrastructure goes to the cc-connect project.

---

*cc-base repository: https://github.com/claude-yu/cc-base*
