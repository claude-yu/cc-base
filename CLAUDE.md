# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

cc-base is a **skill template repository** for mobile-controlled multi-agent workflows. Scripts are authored here, then deployed to real project `controller/bin/` directories. All real config (`config.toml`) and run data stay in the project, never in this repo.

## Architecture

```
mobile (WeChat/QQ/Feishu) → cc-connect → [[commands]] → controller pipeline
```

Three async pipeline patterns:

| Pattern | Entry | Runner | Status |
|---------|-------|--------|--------|
| Plan Review (PS) | `submit-plan-review.ps1` → `plan-review-runner.ps1` | `show-plan-review.ps1` |
| CC Ask (Go) | `cc-controller.exe ask-cc` → `cc-controller.exe run-cc` | `cc-controller.exe show` |
| Codex Ask (Go) | `cc-controller.exe ask-codex` → `cc-controller.exe run-codex` | `cc-controller.exe show` |
| Session-aware CC (Go) | `cc-controller.exe exec-cc --session <id>` | `cc-controller.exe run-cc --session <id>` | `cc-controller.exe show [RunId\|kind]` |

Go ask pipelines: generate RunId → create `controller/runs/<RunId>/` → exec.Command background runner → callback on completion. Session-aware variant additionally manages `controller/sessions/<SessionId>/` with transcript persistence and context injection.

## Go Binary: cc-controller.exe

Replaces PowerShell `submit-cc-ask.ps1` / `cc-ask-runner.ps1` and `submit-codex-ask.ps1` / `codex-ask-runner.ps1` hot paths.

Source: `controller/cmd/cc-controller/` (13 `.go` files + 2 test files), built with:
```powershell
go build -C "YOUR_PROJECT_ROOT\controller" -o "cc-controller.exe" "./cmd/cc-controller/"
```

File organization (all `package main` in `controller/cmd/cc-controller/`):
| File | Responsibility |
|------|---------------|
| `main.go` | Package, imports, types, main() switch, usage() |
| `common.go` | Shared helpers (mustHaveArgs, resolveControllerRoot, sendCallback, writeFile, writeError, trimToken, readInput) |
| `ask.go` | genRunID, cmdAsk (stateless ask entry point) |
| `exec.go` | cmdExecCC (session-aware exec entry), parseExecFlags, session management, execute confirmation + sandbox validation |
| `cc.go` | runCC, resolveClaudeCmd, resolveWorkDir (with heartbeat goroutine) |
| `codex.go` | runCodex, cleanCodexOutput, resolveCodexCmd, setCodexProxy (with heartbeat goroutine) |
| `cancel.go` | cancelTask, cancelLatest, isProcessRunning + smart cancel (running→waiting priority) |
| `project.go` | cmdProject, cmdSwitchProject, active_project.json management |
| `status.go` | Status/event/transcript helpers (writeStatusJSON, updateStatusJSON, setExitCode, appendEvent, appendTranscript, writeJSON) + showRun, findLatestRunByKind, findLatestRun, runStatus |
| `classify.go` | Mode classifier (advice/readonly/execute_request) with science-term guard |
| `backend.go` | Backend selector (CC_CODEX_BACKEND env var), runAPICodex (OpenAI-compatible HTTP client), loud-fail on missing key |
| `queue.go` | Waiting queue CRUD (waiting_queue.json), queuePrune, smart dispatch (0→reject/1→execute/N→ask) |
| `monitor.go` | Stuck/zombie task watchdog, heartbeat threshold escalation (3/5/10min), auto-cleanup + callback |
| `detector.go` | Research job detector framework (GROMACS/Python/R/GenericCLI), scoring, context.json parsing, false-positive filtering |
| `detector_docker.go` | Docker container detector (prosettac/haddock3/colabfold/rosetta/rfdiffusion/gromacs/alphafold) |
| `research_monitor.go` | /科研监控 command handler, directory scanning, mobile summary, detail report |
| `main_test.go` | Unit tests (classifier, readInput, findLatestRun, sidecar filtering) |
| `classify_test.go` | Classifier test cases |
| `detector_test.go` | Detector unit tests (37 tests: scoring, false-positive prevention, state detection, Docker parsing) |

Subcommands:
- `ask-cc <text>` — stateless: generate RunId, write incoming question, detach `run-cc` background process, print RunId
- `ask-codex <text>` — same pattern for Codex CLI
- `exec-cc --text <msg> [--session <id>]` — session-aware: create/manage session dir + transcript.jsonl, spawn `run-cc --session`. Default session: "default"
- `run-cc <RunId> [--session <SessionId>]` — call `claude -p` via os/exec, write results, send callback. With --session: injects recent transcript context into prompt
- `run-codex <RunId>` — call `codex exec`, clean output noise, write results, send callback
- `show [RunId|kind]` — display run results (omit RunId = latest; pass non-RunId string = filter by kind, e.g. "cc-session")
- `cancel [RunId]` — cancel running task (omit = cancel latest active task by PID scan)
- `execute [N|RunId]` — execute confirmed task from waiting queue (omit = smart dispatch)
- `monitor` — scan for stuck/zombie tasks, auto-cleanup, send callback notifications
- `research-monitor [--detector <name>]` — scan research project for job status (GROMACS/Python/R/Docker), read-only
- `status` — system status dashboard (project info, active tasks, recent runs)

Key advantages over PowerShell:
- No BOM/encoding issues (Go writes UTF-8 natively)
- `exec.Command` inherits parent env (no registry fallback needed)
- Detached process via `HideWindow` SysProcAttr
- Consistent error handling (proper exit codes)

## Shared Library (PowerShell)

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

### Security
- `--dangerously-skip-permissions` only in `execute-approved.ps1` and `execute-manual-approved.ps1`
- `/修复controller` can only modify: `controller/`, `cc-connect/`, `~/.cc-connect/config.toml`
- All runs must write audit files to `controller/runs/<RunId>/`
- `config.toml` (contains tokens) never enters Git — only `config.toml.template`

## Key Patterns

### Go ask-cc pattern (replaces PowerShell async runner)
```
cc-controller ask-cc "<text>":
  1. generate RunId, create run dir, write incoming-question.md
  2. exec.Command("cc-controller", "run-cc", RunId) detached (HideWindow)
  3. print RunId to stdout (cc-connect returns it to user)

cc-controller run-cc <RunId> (background):
  1. read incoming-question.md
  2. exec.Command("claude", "-p", ...) with stdin = question
  3. write cc-answer.md, summary.md
  4. exec.Command("cc-connect", "send", "--stdin", "-p", "cc") with callback message
  5. write runner.exitcode.txt
```

### Go exec-cc pattern (session-aware, with heartbeat + callback)
```
cc-controller exec-cc --text "<msg>" --session default:
  1. generate RunId with "cc-session" suffix
  2. create run dir + session dir (sessions/<SessionId>/)
  3. create/update session.json (turn_count, last_run_id)
  4. write status.json (status: accepted, session_id, session_scope)
  5. exec.Command("cc-controller", "run-cc", RunId, "--session", SessionId) detached (HideWindow)
  6. print RunId to stdout

cc-controller run-cc <RunId> --session <SessionId> (background):
  1. read incoming-question.md
  2. build context from transcript.jsonl (last 10 turns, max 20KB)
  3. spawn heartbeat goroutine: 30s ticker → appendEvent + sendCallback
  4. exec.Command("claude", "-p", ...) with full input = system prompt + session context + question
  5. close(done) → stops heartbeat goroutine (no leak)
  6. write cc-answer.md, summary.md
  7. append transcript entries (user + assistant) to transcript.jsonl
  8. update session.json (turn_count++, last_run_id)
  9. send callback with RunId + SessionId
  10. write runner.exitcode.txt (0 = success, else = failed)
```

### PowerShell async runner pattern (plan review only)
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

### Task cancellation pattern
```
Runner startup: write PID to runs/<RunId>/runner.pid
cancel command: read PID file → taskkill /F /T /PID <pid>
  → kills entire process tree (runner + child claude/codex)
  → writes summary.md + sends callback "[Cancelled]"

cancel <RunId> — cancel specific task
cancel (no args) — find latest running task by runner.pid + tasklist check
```

### Heartbeat goroutine (anti-leak pattern)
```
done := make(chan struct{})
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            appendEvent(runDir, heartbeat event)
            sendCallback(runDir, "⏳ processing...")
        case <-done:
            return  // stops goroutine when cmd.Run() completes
        }
    }
}()
err = cmd.Run()
close(done)  // signals heartbeat to exit — MUST happen after cmd.Run()
```

### Status directory layout
```
controller/
  runs/<RunId>/
    status.json           — machine-readable status (accepted/running/completed/failed/cancelled)
    events.jsonl          — append-only event timeline (accepted, heartbeat, completed, failed, cancelled)
    incoming-question.md  — user's question
    cc-answer.md          — Claude Code's answer
    summary.md            — final summary (copied from answer on success)
    runner.pid            — PID for cancellation
    runner.exitcode.txt   — exit code (0=success, -1=cancelled, other=fail)
    callback-msg.md       — last callback message sent
  sessions/<SessionId>/
    session.json          — session metadata (turn_count, last_run_id, mode)
    transcript.jsonl      — append-only user/assistant turns
```

### Start-Process caveats (PowerShell only)
- `-RedirectStandardOutput` passes pipe handle to child → blocks cc-connect. Use `-WindowStyle Hidden` instead.
- `Start-Process` does NOT inherit parent process `$env:` changes. Set env vars persistently (`setx`) or add registry fallback in runner.

## Commands

| Script/Binary | Command | Purpose |
|---------------|---------|---------|
| `cc-controller.exe` | `/cc <question>` | **Session-aware CC对话（推荐）** — Go runner with heartbeat/callback (Go) |
| `cc-controller.exe` | `/问cc <question>` | Stateless async CC Q&A (legacy) (Go) |
| `cc-controller.exe` | `/cc结果 [RunId\|kind]` | CC ask status/result; omit RunId = latest; pass kind like "cc-session" (Go) |
| `cc-controller.exe` | `/问codex <question>` | Async Codex Q&A (Go) |
| `cc-controller.exe` | `/codex结果 [RunId]` | Codex ask status/result (Go) |
| `cc-controller.exe` | `/取消任务 [RunId]` | Cancel running task by RunId (omit = cancel latest) (Go) |
| `cc-controller.exe` | `/监控` | Stuck/zombie task monitor + auto-cleanup (Go) |
| `cc-controller.exe` | `/科研监控` | Research job monitor — GROMACS/Python/R/Docker detector (Go) |
| `check-install.ps1` | `/自检` | 12-item installation self-check (PS) |
| `submit-plan-review.ps1` | `/计划审查 <task>` | CC plan + Codex review (async, PS) |
| `show-plan-review.ps1` | `/查看审查 [RunId]` | Plan review status/result (PS) |
| `collect-md-status.ps1` | `/md状态检查 [path]` | Read-only MD workspace scan (PS) |
| `fix-controller.ps1` | `/修复controller <desc>` | CC auto-fix infrastructure (PS) |
| `grill-plan.ps1` | `/质询计划` | Q&A drill-down on latest review (PS) |
| `auto-callback-toggle.ps1` | `/自动回传 开/关` | Toggle auto-callback (PS) |

## Deployment

Scripts are edited in this repo, then copied to `<project>/controller/bin/`. Go source in `controller/cmd/cc-controller/`, built to `<project>/controller/cc-controller.exe`. Real config edits go directly to `<project>/cc-connect/config.toml` (never committed).

Build command:
```powershell
cd YOUR_PROJECT_ROOT\controller
C:\Go\bin\go.exe build -o cc-controller.exe .\cmd\cc-controller\
```

Commit convention: `feat:`, `fix:`, `refactor:`, `chore:` types. Two-commit pattern for feature + related infra changes.
