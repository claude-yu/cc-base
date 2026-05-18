# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

cc-base is a **skill template repository** for mobile-controlled multi-agent workflows. Scripts are authored here, then deployed to real project `controller/bin/` directories. All real config (`config.toml`) and run data stay in the project, never in this repo.

## Architecture

```
mobile (WeChat/QQ/Feishu) → cc-connect → [[commands]] → PowerShell scripts
```

Two async pipeline patterns:

| Pattern | Entry | Runner | Status |
|---------|-------|--------|--------|
| Plan Review | `submit-plan-review.ps1` → `plan-review-runner.ps1` | `show-plan-review.ps1` |
| Codex Ask | `submit-codex-ask.ps1` → `codex-ask-runner.ps1` | `show-codex-ask.ps1` |

Both: generate RunId → create `controller/runs/<RunId>/` → Start-Process runner → heartbeat via chat-log-writer → callback on completion.

## Shared Library

`_common.ps1` — dot-source in every script:
- `Resolve-ClaudeCmd` / `Resolve-CodexCmd` — locate CLI binaries
- `Set-CodexProxy` — sets ALL_PROXY/HTTP_PROXY/HTTPS_PROXY from `$env:CODEX_PROXY` (only when set)
- `Resolve-RequiredWorkDir` — resolve from param, env var, or CC_WORK_DIR
- `Write-ChatObservation` — append to instinct observations JSONL (failures silently swallowed)
- `Get-ProjectId` — SHA256(CC_WORK_DIR) first 12 hex chars

`chat-log-writer.ps1` — standalone CLI, used by wrapper and runners:
```
-ch wechat -dir out -lifecycle completed -record message -cmd "计划审查" -run $RunId -text $msg
```
Supports `running` lifecycle and `heartbeat` record type.

## Critical Constraints

### Encoding (Windows PowerShell 5.1)
1. **Every script** called by cc-connect must have this at the top (after param, before any stdout):
   ```
   [Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
   [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
   $OutputEncoding = [System.Text.UTF8Encoding]::new($false)
   ```
2. **Any `.ps1` with Chinese characters (including comments) must be UTF-8 with BOM** (0xEF 0xBB 0xBF). PS 5.1 decodes BOM-less as GBK → parser error.
3. Claude CLI Chinese input: use temp file + stdin, never `-p $ChineseText`.
4. Never use `GetEncoding(936)` or GBK output encoding.

### Proxy Isolation
- Claude CLI → HTTP proxy (`$env:CLAUDE_PROXY`). SOCKS5h crashes it.
- Codex CLI → SOCKS5h proxy (`$env:CODEX_PROXY`). HTTP may not work for WebSocket.
- `Set-CodexProxy` only activates when `$env:CODEX_PROXY` is set.
- For runner scripts launched via Start-Process: also check registry via `[Environment]::GetEnvironmentVariable("CODEX_PROXY", "User")`.

### Security
- `--dangerously-skip-permissions` only in `execute-approved.ps1` and `execute-manual-approved.ps1`
- `/修复controller` can only modify: `controller/`, `cc-connect/`, `~/.cc-connect/config.toml`
- All runs must write audit files to `controller/runs/<RunId>/`
- `config.toml` (contains tokens) never enters Git — only `config.toml.template`

## Key Patterns

### Async runner pattern
```
submit script:
  1. validate input
  2. generate RunId, create run dir, write incoming-*.md
  3. write chat-log in record
  4. Start-Process runner script (WindowStyle Hidden)
  5. Write-Output user-facing message (immediate return)

runner script:
  1. Write-Heartbeat per stage
  2. execute backend CLI
  3. write output files (answer/exitcode/summary)
  4. write chat-log out record
  5. Send-Callback (if auto-callback enabled)
  6. write runner.exitcode.txt
```

### Invoke wrapper pattern (synchronous commands)
`invoke-controller-command.ps1` wraps commands with chat-log in/out records. Not used for async pipelines (avoids dual RunId).

### Start-Process caveats
- `-RedirectStandardOutput` passes pipe handle to child → blocks cc-connect. Use `-WindowStyle Hidden` instead.
- `Start-Process` does NOT inherit parent process `$env:` changes. Set env vars persistently (`setx`) or add registry fallback in runner.

## Commands

| Script | Command | Purpose |
|--------|---------|---------|
| `submit-plan-review.ps1` | `/计划审查 <task>` | CC plan + Codex review (async) |
| `show-plan-review.ps1` | `/查看审查 [RunId]` | Plan review status/result |
| `submit-codex-ask.ps1` | `/问codex <question>` | Async Codex Q&A |
| `show-codex-ask.ps1` | `/codex结果 [RunId]` | Codex ask status/result |
| `collect-md-status.ps1` | `/md状态检查 [path]` | Read-only MD workspace scan |
| `fix-controller.ps1` | `/修复controller <desc>` | CC auto-fix infrastructure |
| `grill-plan.ps1` | `/质询计划` | Q&A drill-down on latest review |
| `auto-callback-toggle.ps1` | `/自动回传 开/关` | Toggle auto-callback |

## Deployment

Scripts are edited in this repo, then copied to `E:\ai\selfwork_ytl\controller\bin\`. Real config edits go directly to `E:\ai\selfwork_ytl\cc-connect\config.toml` (never committed).

Commit convention: `feat:`, `fix:`, `refactor:`, `chore:` types. Two-commit pattern for feature + related infra changes.
