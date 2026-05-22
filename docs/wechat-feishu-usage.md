# WeChat + Feishu Usage Model

cc-base supports two different mobile surfaces. They should not have identical permissions.

## WeChat / QQ: Main Control

Use WeChat when you are the operator and need full control.

Typical commands:

```text
/cc <message>
/项目
/切项目 <name-or-path>
/科研监控
/审查 security
/计划审查 <task>
/执行 <RunId>
/取消任务 <RunId>
/记忆状态
```

WeChat can be configured with execution commands because it is the private control channel. Execution still goes through review/approval gates where configured.

## Feishu / AstrBot: Read-only and Review Surface

Use Feishu when you want quick project visibility or a safer team-facing interface.

Typical commands:

```text
/科研监控
/科研监控 r_pipeline
/系统状态
/提交审查 <task>
/审查结果
/审查统计
/记忆状态
/recap
```

Feishu should not expose general shell execution or arbitrary file writes.

## Recommended Daily Workflow

1. Switch or verify the active project from WeChat or local CLI.
2. Use Feishu `/科研监控` to check job state during the day.
3. Use Feishu `/提交审查` for lightweight review requests.
4. Use WeChat only when an approved action must be executed.
5. Use `/recap` or `/记忆状态` before handing context to another agent.

## Detector Alias Rules

Aliases are intentionally explicit:

```text
maestro/glide/ligprep/desmond/薛定谔 -> schrodinger
pyrosetta -> rosetta
colabfold -> alphafold
amber/openmm -> amber_openmm
vina -> autodock_vina
haddock -> haddock3
```

Ambiguous terms like `对接`, `docking`, and `dock` ask the user to choose. This prevents an AutoDock/Schrodinger/HADDOCK/Rosetta misroute.

## Permission Matrix

| Capability | WeChat | Feishu |
| --- | --- | --- |
| Monitor jobs | yes | yes |
| Filter detectors | yes | yes |
| Submit reviews | yes | yes |
| View review results | yes | yes, AstrBot-marked only |
| Execute approved work | yes | no |
| Kill/cancel local tasks | yes | no |
| Memory recap/status | yes | yes |
| Memory archive execution | operator only | bounded helper only, keep private |

## Expansion Rule

Before enabling Feishu group chats or any execute-class command, add and review:

- group-chat allowlist
- explicit user authorization
- command-level permission table
- audit log for every accepted command
- timeout and redaction tests

