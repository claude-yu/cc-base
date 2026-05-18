# cc-base: Mobile-Controlled Multi-Agent Controller

> 基于 cc-connect 创建的一个微信/飞书自用工具，搭载对话学习 learning 和 grill-me 能力。
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

# Edit credentials then start:
powershell -NoProfile -ExecutionPolicy Bypass -File "E:\ai\myproject\cc-connect\start.ps1" -CleanSessions
```

`install.ps1` handles: prerequisite check (PowerShell, Node.js, cc-connect npm package), directory structure, script deployment, config template setup, and next-steps guidance.

## Quick Start

1. **Install**: `install.ps1 -ProjectDir <your-project>`
2. **Configure**: edit `<project>/cc-connect/config.toml` with your WeChat token/base_url/account_id
3. **Set env vars**: `CC_WORK_DIR` (required), `CODEX_PROXY` (optional, for Codex CLI)
4. **Start**: `start.ps1 -CleanSessions`
5. **Use from phone**: send `/计划审查 帮我设计xxx方案` in WeChat

## Prerequisites

| Dependency | Version | Install |
|------------|---------|---------|
| PowerShell | 5.1+ | Windows built-in |
| Node.js | 18+ | [nodejs.org](https://nodejs.org) |
| cc-connect | 1.3.2+ | `npm install -g cc-connect` |
| Claude Code CLI | latest | `npm install -g @anthropic-ai/claude-code` |
| Codex CLI | latest | `npm install -g @openai/codex` (optional) |

## System Architecture

```
手机（微信/飞书/QQ/Telegram）
    │
    ▼
cc-connect v1.3.2（Go 二进制，Node.js 分发）
    │
    ├── WeChat 平台 ←→ 微信企业号 API
    ├── QQ 平台 ←→ NapCat WebSocket
    │
    ├── Project: cc（Claude Code Agent）
    │   └── work_dir: 用户工作目录
    │
    ├── Project: codex（Codex Agent）
    │   └── work_dir: codex-review 目录
    │
    └── Commands（PowerShell 脚本 pipeline）
        ├── /计划审查    → CC 写计划 + Codex 审查（异步）
        ├── /修复controller → CC 直接修复 controller/cc-connect 报错
        ├── /md状态检查   → 只读检查任意 MD/GROMACS 工作目录
        ├── /批准执行     → 执行 Codex APPROVE 的 run
        ├── /人工批准执行  → 手动接受后执行
        ├── /质询计划     → Grill-Me 逐条质询计划
        ├── /学习状态     → 查看 chat-instinct 学习统计
        ├── /进化习惯     → 分析 observation 生成进化候选
        ├── /自动回传     → 开关计划审查完成后自动推回聊天窗口
        ├── /问codex      → 异步询问 Codex，完成后自动回传
        └── /codex结果    → 查看 Codex ask 状态或结果
```

## Feature List

### 1. Plan Review Pipeline

**Command**: `/计划审查 <task description>`

- Sub-second Run ID return, async background execution
- CC read-only plan generation → Codex independent review
- Auto-injects `rules/*.md` domain rules
- Outputs verdict.md (APPROVE / REVISE / BLOCK) + summary.md

### 2. Controller Auto-Repair

**Command**: `/修复controller <error description>`
**Aliases**: `/修复`, `/fix`, `/修复bug`

CC directly diagnoses and fixes: config sync, encoding corruption, script bugs, permissions, stuck processes.
Security: only `controller/`, `cc-connect/`, `~/.cc-connect/config.toml` are writable.

### 3. Status Monitoring

**Command**: `/md状态检查 [MD工作目录]`

Read-only scan of any MD/GROMACS work directory, lists `.tpr/.cpt/.log/.xtc/.trr/.edr/.gro/.pdb/.top/.itp/.mdp`, reads `.log` tail. Defaults to `CC_WORK_DIR`.

### 4. Execution Approval

| Command | Condition |
|---------|-----------|
| `/批准执行 <RunId>` | Codex verdict = APPROVE |
| `/人工批准执行 <RunId>` | manual-approval.md + not BLOCK |

### 5. Grill-Me Plan Challenge

**Command**: `/质询计划` or `grill` / `grillme`

When Codex returns REVISE and you're unsure how to improve: one-question-at-a-time interrogation across 8 dimensions (success criteria, input validation, rollback plan, etc.), each with recommended answer.

### 6. Chat-Instinct Learning

**View**: `/学习状态`
**Evolve**: `/进化习惯`

cc-base built-in chat entry learning:
- Controller commands auto-record user operation patterns
- `/进化习惯` analyzes patterns and generates candidates (user confirms before writing instinct)
- Project-level isolation (hash-based project-id)

### 7. Auto-Callback

**Command**: `/自动回传 开` or `/自动回传 关`

Controls whether plan-review results auto-push back to chat:
- On: background runner sends results on completion
- Off: results stay local, manual `/查看审查 <RunId>` to view

### 8. Codex Async Q&A

**Command**: `/问codex <question>`
**View**: `/codex结果 [RunId]`

Sends question to Codex, returns immediately with RunId, background processes, auto-callbacks on completion.

### 9. Free Chat

| Send | Effect |
|------|--------|
| `发给cc <message>` | Forward to Claude Code agent |
| `发给codex <message>` | Async Codex Q&A via `/问codex` |
| `发给gpt <message>` | Async Codex Q&A via `/问codex` |

## Command Cheatsheet

| Command | Aliases | Effect |
|---------|---------|--------|
| `/计划审查 <task>` | 审查计划、让cc写计划 | CC plan + Codex review (async) |
| `/修复controller <error>` | 修复bug、修复、fix、修controller | CC auto-fix infrastructure |
| `/md状态检查 [path]` | md进度、md检查、查md | Read-only MD workspace scan |
| `/批准执行 <RunId>` | 执行批准任务 | Execute approved run |
| `/人工批准执行 <RunId>` | 手动批准执行 | Manual accept + execute |
| `/质询计划` | grill、grillme、拷问计划 | Grill-Me challenge |
| `/学习状态` | 学习状态 | View learning stats |
| `/进化习惯` | 进化 | Analyze + generate candidates |
| `/自动回传 开/关` | 回传 | Toggle auto-callback |
| `/问codex <question>` | 发给codex、gpt、问codex | Async Codex Q&A |
| `/codex结果 [RunId]` | 查看codex | View Codex ask result |
| `发给cc <message>` | 发给claude、opus、问cc | Forward to Claude Code |

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

## Safety Rules

- CC read-only plan mode: no execution commands
- Execution requires Codex APPROVE or manual confirm + not BLOCK
- `--dangerously-skip-permissions` only in execute-approved.ps1 and execute-manual-approved.ps1
- All runs write audit files to controller/runs/
- `/修复controller` must not touch project data

## Environment Variables

See `docs/env-vars.md`. Required: `CC_WORK_DIR`.

## Startup

```powershell
# Standard (WeChat + QQ)
powershell -NoProfile -ExecutionPolicy Bypass -File <project>/cc-connect/start.ps1 -WithQQ -CleanSessions

# WeChat only
powershell -NoProfile -ExecutionPolicy Bypass -File <project>/cc-connect/start.ps1 -CleanSessions
```

## Dependencies

- [cc-connect](https://github.com/chenhg5/cc-connect) v1.3.2+ — multi-platform chat gateway, the foundational infrastructure
- Claude Code CLI (`claude`)
- Codex CLI (`codex`, optional)
- PowerShell 5.1 (Windows)
- WeChat Work bot token / Feishu / Telegram bot token
- NapCat (QQ, optional)

## Attribution

cc-base is a skill template and controller pipeline **built on [cc-connect](https://github.com/chenhg5/cc-connect)** by [chenhg5](https://github.com/chenhg5). cc-connect provides the multi-platform chat gateway (WeChat Work / QQ / Telegram / Feishu) that enables mobile-to-CLI communication. All credit for the underlying infrastructure goes to the cc-connect project.

---

*cc-base repository: https://github.com/claude-yu/cc-base*
