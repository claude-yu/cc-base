# Feishu / AstrBot Setup

This guide installs the cc-base AstrBot integration for Feishu/Lark.

## Boundary

Feishu is not the full control channel. It is for:

- scientific job monitoring
- controller status checks
- review submission and review result lookup
- memory status, recap, and bounded archive helpers
- detector false-positive / false-negative intake templates

Do not expose general execution, shell commands, or `/执行` through Feishu.

## Files

```text
integrations/astrbot-cc-controller/
  CONTRACT.md
  adapter.ps1
  smoke.ps1
  plugin/
    main.py
    metadata.yaml
  personas/
  skills/
```

## Install

1. Install AstrBot and enable the Lark/Feishu platform.
2. Confirm the Feishu bot can receive private chat messages.
3. Link the bundled plugin into AstrBot:

```powershell
$repo = "C:\cc-base"
$target = "C:\Users\$env:USERNAME\.astrbot\data\plugins\astrbot_cc_controller"
New-Item -ItemType Directory -Force -Path (Split-Path $target) | Out-Null
cmd /c mklink /J "$target" "$repo\integrations\astrbot-cc-controller\plugin"
```

If junctions are not suitable, copy the plugin folder instead.

4. Restart AstrBot.
5. Run smoke tests:

```powershell
cd C:\cc-base\integrations\astrbot-cc-controller
powershell -NoProfile -ExecutionPolicy Bypass -File .\smoke.ps1
```

## Configure Adapter Paths

`adapter.ps1` is the only component that calls local cc-base commands. Keep it narrow:

- hardcode or safely resolve `cc-controller.exe`
- resolve active project from `controller/active_project.json`
- reject work dirs outside allowed roots
- pass detector names as argument arrays, never through string-built shell
- return a single JSON envelope on stdout

## Commands

| Command | Purpose |
| --- | --- |
| `/科研监控` | scan active project |
| `/科研监控 <detector>` | scan with detector filter |
| `/系统状态` | controller status summary |
| `/提交审查 <任务>` | submit async review job |
| `/审查结果 [RunId]` | show AstrBot-marked review result |
| `/审查统计` | aggregate AstrBot review runs |
| `/记录误判` | detector intake template |
| `/转审` | detector draft handoff template |
| `/记忆状态` | memory status summary |
| `/记忆记录` | memory update recommendation |
| `/记忆归档` | archive preview |
| `/确认归档` | bounded archive execution |
| `/recap` | continuation recap |
| `/帮助` | command help |

## Validation

A working install should pass:

```text
/科研监控
/科研监控 r_pipeline
/科研监控 gromacs
/系统状态
/帮助
```

For review commands, submit a small harmless task first and verify `/审查结果` only returns AstrBot-marked runs.

## Safety Notes

- Keep Feishu private-chat only until group gates are reviewed.
- Keep execution commands unavailable.
- Keep adapter output as JSON only.
- Do not let user input choose arbitrary file paths or binaries.
- Do not read legacy WeChat review runs unless they have the AstrBot marker file.

