# Chat-Instinct 学习系统

cc-base 内置的聊天入口学习能力。通过记录用户的命令使用模式，逐步积累为可复用的 instinct。

## 架构

```
用户通过微信/飞书/QQ 发命令
    │
    ▼ controller 脚本自动记录
┌──────────────────────────────────────┐
│  observations.jsonl                  │
│  (command_start/end, errors, etc.)   │
└──────────────────────────────────────┘
    │
    ▼ /进化习惯（用户触发，不自动）
┌──────────────────────────────────────┐
│  evolved-candidates.md               │
│  (候选列表，用户确认后才写 instinct) │
└──────────────────────────────────────┘
    │
    ▼ 用户确认
┌──────────────────────────────────────┐
│  instincts/personal/*.yaml           │
│  (id, trigger, confidence, domain)   │
└──────────────────────────────────────┘
```

## 自动记录的事件

以下 controller 命令会自动记录 observation：

| 脚本 | 记录点 |
|------|--------|
| `submit-plan-review.ps1` | command_start（含任务描述） |
| `fix-controller.ps1` | command_start + command_end（含 exit code） |
| `collect-md-status.ps1` | command_start + command_end |
| `execute-approved.ps1` | command_start + command_end（含 RunId + exit code） |
| `execute-manual-approved.ps1` | command_start + command_end（含 RunId + exit code） |
| `grill-plan.ps1` | command_start（含质询上下文） |

## 存储位置

```
$CC_INSTINCT_HOME/projects/<project-id>/
├── observations.jsonl          # 自动记录
├── instincts/
│   ├── personal/               # 进化生成的 instinct yaml
│   └── inherited/              # 导入的 instinct
└── evolved/
    └── evolved-candidates.md   # 进化候选（待用户确认）
```

- `project-id` = `CC_WORK_DIR` 的 SHA256 前 12 hex
- 默认 `CC_INSTINCT_HOME` = `~/.cc-base/instincts`

## Observation 格式

每行一个 JSON：

```json
{"timestamp":"2026-05-18T15:30:00+08:00","event":"command_start","command":"计划审查","detail":"分析蛋白结合位点","platform":"weixin"}
```

## Instinct 格式

```yaml
---
id: prefer-readonly-first
trigger: "when user asks to check status"
confidence: 0.7
domain: "workflow"
scope: project
project_id: "a1b2c3d4e5f6"
---

# Prefer Readonly First

## Action
Always use readonly commands before execution commands.

## Evidence
- Observed 8 instances of status check before execute
- User rejected direct execution on 2026-05-10
```

## 用户操作

| 操作 | 方式 |
|------|------|
| 查看学习状态 | `/学习状态` 或别名 `学习状态` |
| 触发进化分析 | `/进化习惯` 或别名 `进化` |
| 删除某个 instinct | 直接删除 `instincts/personal/<id>.yaml` |
| 禁用某个 instinct | 编辑 yaml，设 `confidence: 0` |

## 进化规则

- observation 不足 10 条时不建议进化
- `/进化习惯` 只生成候选列表（`evolved-candidates.md`），**不自动写 skill**
- 用户确认后才创建 instinct yaml 文件
- 同一 instinct 在 2+ 项目出现且 confidence >= 0.8 时，可考虑提升为全局

## 与 continuous-learning-v2 的关系

| 特性 | chat-instinct（内置） | continuous-learning-v2（可选增强） |
|------|----------------------|----------------------------------|
| 记录方式 | controller 脚本内嵌 | PreToolUse/PostToolUse hooks |
| 触发 | 聊天命令自动 | Claude Code session 级 |
| 范围 | 命令使用模式 | 工具调用 + 代码操作模式 |
| 依赖 | 无（cc-base 内置） | 需配置 hooks |
| yaml 格式 | 共享 | 共享 |

两者可共存，共享 instinct 理念和 yaml 格式。chat-instinct 不依赖 continuous-learning-v2。
