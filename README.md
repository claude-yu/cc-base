# cc-base

cc-base is a Windows-first remote control kit for Claude Code, Codex, scientific job monitoring, code review, and memory maintenance. It is designed for researchers who want to control a local workstation from WeChat and inspect research jobs from Feishu/Lark through AstrBot.

Current public boundary:

- WeChat / QQ through cc-connect is the main control entrance. It can run conversations, review flows, approved execution, project switching, and scientific monitoring.
- Feishu / Lark through AstrBot is the read-only and review entrance. It can inspect scientific job status, submit review jobs, view review results, and run memory recap/status helpers. It must not expose general shell execution.
- cc-controller is the local Go core. It owns the command routing, scientific detectors, review commands, queue/status files, and JSON output contracts.

## What You Get

| Area | Entry | Purpose |
| --- | --- | --- |
| WeChat control | cc-connect commands | Remote Claude Code / Codex workflows from mobile chat |
| Feishu read-only view | AstrBot plugin | Scientific monitor, review status, memory status from Feishu |
| Scientific monitor | `cc-controller research-monitor` | Detect GROMACS, Schrodinger, HADDOCK3, Rosetta, Vina, AlphaFold, Amber/OpenMM, Gaussian, Python, R, Docker/CLI jobs |
| Review workflow | `/审查`, `/提交审查`, `/审查结果` | DeepSeek/GLM/Codex-assisted independent reviews depending on channel |
| Memory maintenance | `memory-health`, `memory-draft`, AstrBot memory commands | Health scan, patch/review/apply pipeline, recap/archive helpers |
| Multi-project sessions | `/项目`, `/切项目` | Keep per-project working context and active work directory |

## Repository Layout

```text
cc-base/
  SKILL.md                         # skill entrypoint for Claude/Codex agents
  README.md                        # this guide
  install.ps1                      # installer for a local cc-base deployment
  scripts/
    config.toml.template           # safe cc-connect config template, no secrets
    start.ps1                      # cc-connect starter
    bin/*.ps1                      # pipeline wrappers and helpers
  controller/
    go.mod
    cmd/cc-controller/*.go          # Go controller source and tests
  integrations/
    astrbot-cc-controller/
      CONTRACT.md                  # AstrBot adapter contract and JSON schemas
      adapter.ps1                  # Feishu-safe PowerShell adapter
      smoke.ps1                    # offline smoke tests
      plugin/                      # AstrBot plugin package
      personas/                    # optional Feishu persona prompts
      skills/                      # optional helper skills for detector/memory work
  docs/
    wechat-setup.md
    feishu-astrbot-setup.md
    wechat-feishu-usage.md
    env-vars.md
    research-job-monitor-plan.md
```

## Prerequisites

| Tool | Required For | Notes |
| --- | --- | --- |
| Windows + PowerShell 5.1+ | all flows | The scripts are Windows-first. PowerShell 7 is optional. |
| Go 1.21+ | cc-controller build | `go build` and `go test` are used locally. |
| Node.js 18+ | cc-connect and Claude/Codex CLIs | Install from nodejs.org or your package manager. |
| Claude Code CLI | WeChat `/cc`, planning, execution | Install and login before use. |
| Codex CLI | optional review / second opinion | Required only for native Codex workflows. |
| cc-connect | WeChat / QQ gateway | The config template is in `scripts/config.toml.template`. |
| AstrBot | Feishu / Lark gateway | Used with the plugin under `integrations/astrbot-cc-controller/plugin`. |
| Feishu / Lark bot | Feishu channel | Configure inside AstrBot. |
| Enterprise WeChat bot or supported cc-connect WeChat channel | WeChat channel | Put real tokens only in local config, never commit them. |

## Install cc-base

Clone the repository and install into your working directory:

```powershell
git clone https://github.com/claude-yu/cc-base.git
cd cc-base
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -ProjectDir "C:\cc-base"
```

Build the Go controller:

```powershell
cd C:\cc-base\controller
go test .\cmd\cc-controller\...
go build -o cc-controller.exe .\cmd\cc-controller\
```

The installer copies scripts and templates into your project. The sensitive runtime config is local-only:

```text
C:\cc-base\cc-connect\config.toml
```

Do not commit `config.toml`, tokens, account IDs, API keys, run logs, or generated `active_project.json`.

## WeChat Setup

WeChat uses cc-connect as the chat gateway. The gateway maps chat commands to `cc-controller.exe` and the PowerShell wrappers.

1. Copy the safe template:

```powershell
Copy-Item .\scripts\config.toml.template C:\cc-base\cc-connect\config.toml
```

2. Edit only the local config:

```text
C:\cc-base\cc-connect\config.toml
```

Set at least:

```toml
[settings]
controller_dir = "C:\\cc-base\\controller"
work_dir = "C:\\cc-base"

# Fill your real gateway tokens/accounts locally.
# Never commit this file.
```

3. Start cc-connect:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "C:\cc-base\scripts\start.ps1"
```

4. In WeChat, test read-only commands first:

```text
/状态
/项目
/科研监控
/审查 general
/记忆状态
```

5. Use execution commands only after you understand the safety boundary:

```text
/cc 帮我看看当前项目状态
/计划审查 帮我设计一个安全的分析流程
/执行 <RunId>
/取消任务 <RunId>
```

Important cc-connect rule: messages that begin with `/` bypass alias lookup. If you want `/某命令` to work, that command must exist as a real `[[commands]]` entry in `config.toml`. Aliases only work for non-slash first-word matching.

## Feishu / AstrBot Setup

Feishu uses AstrBot plus the bundled plugin. This channel is intentionally narrower than WeChat.

### 1. Install AstrBot

Install and configure AstrBot according to the AstrBot documentation. Enable the Lark/Feishu platform and make sure your bot can receive private chat messages before adding this plugin.

### 2. Copy or link the plugin

The plugin source is here:

```text
integrations\astrbot-cc-controller\plugin
```

Recommended local development setup on Windows:

```powershell
$repo = "C:\cc-base"
$astr = "C:\Users\$env:USERNAME\.astrbot\data\plugins\astrbot_cc_controller"
New-Item -ItemType Directory -Force -Path (Split-Path $astr) | Out-Null
cmd /c mklink /J "$astr" "$repo\integrations\astrbot-cc-controller\plugin"
```

If junctions are inconvenient, copy the folder instead:

```powershell
Copy-Item -Recurse -Force "C:\cc-base\integrations\astrbot-cc-controller\plugin" "C:\Users\$env:USERNAME\.astrbot\data\plugins\astrbot_cc_controller"
```

Restart AstrBot after linking or copying.

### 3. Point the adapter at your controller

Edit `integrations\astrbot-cc-controller\adapter.ps1` if your installation path differs from the default. The adapter must know where `cc-controller.exe` and the local project root are.

The expected model is:

- adapter reads the active project from `controller\active_project.json`
- adapter sets `CC_RESEARCH_MONITOR_ROOT` before calling `cc-controller.exe`
- adapter rejects work dirs outside allowed roots
- adapter returns exactly one JSON envelope on stdout

Run the smoke test from the integration directory:

```powershell
cd C:\cc-base\integrations\astrbot-cc-controller
powershell -NoProfile -ExecutionPolicy Bypass -File .\smoke.ps1
```

Expected result: all smoke tests pass. The current suite checks JSON output, detector filters, invalid command rejection, injection blocking, review result isolation, memory helpers, and archive safety.

## Feishu Commands

| Feishu Command | Alias | Capability | Safety |
| --- | --- | --- | --- |
| `/科研监控` | `/research` | Scan active research project | read-only |
| `/科研监控 gromacs` | `/research gromacs` | Filter by detector | read-only |
| `/系统状态` | `/status` | Show condensed controller status | read-only |
| `/提交审查 <任务>` | `/submit-review <任务>` | Submit async review job | review-only, no shell execution from chat |
| `/审查结果 [RunId]` | `/review-result [RunId]` | Show AstrBot-marked review result | read-only |
| `/审查统计` | `/review-stats` | Aggregate AstrBot review runs | read-only |
| `/记录误判` | `/detector-intake` | Show detector false-positive/false-negative intake template | local template only |
| `/转审` | `/submit-detector-draft` | Show detector draft handoff template | local template only |
| `/记忆状态` | `/memory-status` | Check memory file presence and freshness | read-only |
| `/记忆记录` | `/memory-record` | Show memory update recommendations | read-only |
| `/记忆归档` | `/memory-archive` | Preview archive candidates | read-only preview |
| `/确认归档` | `/archive-execute` | Move stale completed memory entries | bounded write to progress files only |
| `/recap` | `/memory-recap` | Show handoff + progress continuation context | read-only |
| `/帮助` | `/help` | Command list | local plugin only |

Feishu must not expose:

```text
/执行
/确认执行
/批准执行
shell commands
arbitrary PowerShell
arbitrary file writes
```

## Detector Names and Aliases

Canonical detector names:

```text
gromacs, schrodinger, haddock3, rosetta, autodock_vina,
alphafold, amber_openmm, gaussian, python_pipeline,
r_pipeline, generic_cli
```

Common aliases:

| User Input | Canonical Detector |
| --- | --- |
| `maestro`, `glide`, `ligprep`, `desmond`, `薛定谔` | `schrodinger` |
| `pyrosetta` | `rosetta` |
| `colabfold` | `alphafold` |
| `amber`, `openmm` | `amber_openmm` |
| `vina` | `autodock_vina` |
| `haddock` | `haddock3` |

Ambiguous words such as `对接`, `docking`, and `dock` are not silently mapped. The Feishu plugin asks the user to choose a specific detector.

## Channel Boundary

| Channel | Role | Permissions | Recommended Use |
| --- | --- | --- | --- |
| WeChat / QQ via cc-connect | Main control entrance | Full configured command set, including approved execution | Private operator control |
| Feishu / Lark via AstrBot | Team visibility and review entrance | Read-only + review submission/result + bounded memory archive | Project status sharing and safer mobile checks |
| CLI | Local admin | Full local access | Build, test, deploy, emergency repair |

This split is deliberate. Keep Feishu useful for status and review, but keep destructive execution out of group chats.

## Review and Memory Environment Variables

Only set keys you actually use:

```powershell
setx CC_DEEPSEEK_API_KEY "..."
setx CC_GLM_API_KEY "..."
setx CC_CODEX_BACKEND "native_codex"
setx CC_RESEARCH_MONITOR_ROOT "C:\cc-base"
```

Do not put API keys into Git-tracked config files. The adapter allows only the narrow environment needed by the command it calls.

## Common Operations

Build and test controller:

```powershell
cd C:\cc-base\controller
go test .\cmd\cc-controller\...
go build -o cc-controller.exe .\cmd\cc-controller\
```

Run monitor locally:

```powershell
.\cc-controller.exe research-monitor --work-dir "D:\research-work\work_12\虚拟敲除" --format json
.\cc-controller.exe research-monitor --detector r_pipeline --format json
```

Run AstrBot adapter directly:

```powershell
cd C:\cc-base\integrations\astrbot-cc-controller
powershell -NoProfile -ExecutionPolicy Bypass -File .\adapter.ps1 -Command research-monitor
powershell -NoProfile -ExecutionPolicy Bypass -File .\adapter.ps1 -Command research-monitor -Detector gromacs
powershell -NoProfile -ExecutionPolicy Bypass -File .\adapter.ps1 -Command system-status
```

## Troubleshooting

| Problem | Likely Cause | Fix |
| --- | --- | --- |
| WeChat command does nothing | command missing in `[[commands]]` | Register slash commands explicitly in `config.toml`. |
| Feishu returns invalid JSON | adapter mixed stdout text with JSON | Ensure adapter writes only one JSON envelope to stdout. Use stderr for diagnostics. |
| Feishu detects wrong project | stale `active_project.json` or wrong allowed root | Switch project from WeChat/CLI, then rerun `/科研监控`. |
| Detector alias not recognized | alias missing from Go or Python layer | Add it to both `resolveDetectorAlias()` and `_DETECTOR_ALIASES`. |
| Chinese text is garbled | PowerShell encoding mismatch | Use UTF-8 scripts and pipe Chinese text via stdin where needed. See `rules/encoding.md`. |
| API review fails | missing API key in runtime env | Set `CC_DEEPSEEK_API_KEY` or `CC_GLM_API_KEY` in the process/user environment. |
| Feishu group risk | group chat exposes commands broadly | Keep execution commands disabled. Add explicit group-chat gates before expanding. |

## Security Checklist Before Sharing

Before pushing or sending this repo to a friend:

```powershell
git status --short
git grep -n "token\|secret\|password\|api_key\|apikey\|authorization\|Bearer"
```

Confirm these are not tracked:

```text
config.toml
*.bak
*.exe
controller/runs/
controller/active_project.json
controller/latest-monitor-run.txt
controller/waiting_queue.json
__pycache__/
*.pyc
```

## Further Docs

- `docs/wechat-setup.md` - WeChat/cc-connect setup notes
- `docs/feishu-astrbot-setup.md` - Feishu/AstrBot setup guide
- `docs/wechat-feishu-usage.md` - how the two channels should be used together
- `integrations/astrbot-cc-controller/CONTRACT.md` - adapter contract and schemas
- `docs/research-job-monitor-plan.md` - detector design notes
- `docs/env-vars.md` - environment variable reference


