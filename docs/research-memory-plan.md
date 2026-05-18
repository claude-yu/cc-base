# cc-base v2.3 Research-Memory Bridge Plan

## Summary

本计划不创建新的 memory 系统，而是桥接 auto-memory（索引）、progress/handoff（时间线）、homunculus（instinct）、项目本地 `memory/`（科研证据）四方。

v2.3 第一版只建立最小可用的 research-memory bridge：记录科研项目中的聊天日志、关键决策、踩坑修复和证据指针。它面向生信、单细胞、空间转录组、AI 蛋白、AIDD、分子对接、分子动力学等科研计算项目。第一版不接 byterover，不实现 fact_store，不预建大而空的目录树。

## Authority Boundaries

- **auto-memory 是索引权威**：`~/.claude/projects/<dir-hash>/memory/MEMORY.md` 继续作为稳定记忆入口。research-memory 不新增平行的 `soul.md` 或 `memory.md`。
- **progress/handoff 是时间线权威**：`progress.md` 记录项目阶段、Pinned、Risks、TODO 和关键进展；`handoff-YYYY-MM-DD.md` 记录交接摘要。项目本地 `memory/` 不重复这些时间线内容。
- **homunculus 是 instinct 权威**：默认路径为 `~/.claude/homunculus/projects/<project-id>/instincts/`，可由 `CC_HOMUNCULUS_HOME` 覆盖。research-memory 不新增 `memory/instincts/`。
- **项目本地 `memory/` 是证据仓库**：只保存原始聊天、决策、踩坑、归档和科研证据链。auto-memory/MEMORY.md 通过指针引用这些文件。

Tie-breaker：

- 跨项目通用规则写入 feedback memory，例如“Claude CLI 中文输入必须走 stdin pipe”。
- 本项目专属约束写入 `progress.md ## Pinned`，例如“本项目真实 config 不进 GitHub”。
- 同一条同时满足两边时，只写 feedback memory，Pinned 中使用 `[[feedback-name]]` 引用。

## Minimal Local Layout

项目内只预期以下最小结构，其他文件按真实需求长出来：

```text
project/
  progress.md
  handoff-YYYY-MM-DD.md

  memory/
    chat/
      YYYY-MM-DD.jsonl
      YYYY-MM-DD.summary.md
    research/
      decisions.md
      pitfalls.md
    archive/
```

不在 v2.3 预建：

- `soul.md`
- `memory.md`
- `working/`
- `facts/`
- `workflows/`
- `hypotheses.md`
- `targets.md`
- `compounds.md`
- `instincts/`

扩展规则：

- 当同类假设达到 3 条以上，再创建 `research/hypotheses.md`。
- 当候选靶点或化合物需要独立追踪时，再创建 `research/targets.md` 或 `research/compounds.md`。
- 通用 SOP 保持在技能中，例如 `gromacs-md`、`molecdock`、`bioinfo`、`tcga-prognosis`。项目本地 memory 只记录本项目偏离点、参数选择、失败修复和证据链，不复制 SOP 全文。

## MEMORY.md Pointer Format

auto-memory 的 `MEMORY.md` 指向项目文件时使用双字段，避免跨机器路径失效：

```yaml
work_dir: "E:\\path\\to\\project"
rel_path: "memory/research/decisions.md"
```

读取规则：

- 优先用当前 `CC_WORK_DIR + rel_path` 定位。
- 如果不存在，再用 `work_dir + rel_path` 作为本机 fallback。
- 不在 `MEMORY.md` 中只写不可迁移的绝对路径，也不只写无法独立解析的相对路径。

## Chat Log Policy

v2.3 的聊天日志是 research-memory 的原始材料，但不替代 instinct 和 progress。

环境变量：

- `CC_CHAT_LOG_DIR`：默认 `<project>/memory/chat`
- `CC_CHAT_LOG_MODE`：默认 `full`，可设 `metadata` 或 `off`

Schema 要求：

```json
{
  "ts": "2026-05-18T12:00:00+08:00",
  "record_type": "message",
  "channel": "wechat",
  "direction": "in",
  "lifecycle": "started",
  "signal": null,
  "command": "计划审查",
  "alias": "grill",
  "run_id": "20260518-120000",
  "work_dir": "E:\\path\\to\\project",
  "project_id": "4d3e4eb9a6c2",
  "text": "脱敏后的原文",
  "text_hash": "sha256...",
  "meta": {}
}
```

字段规则：

- `record_type` 表示日志行类型：`message | signal_patch`，默认 `message`。
- `lifecycle` 只表示执行生命周期：`started | completed | failed | replied`。
- `signal` 只表示学习信号：`corrected | confirmed | ignored | null`。
- 学习信号采用 append-only 回填：原始消息行不修改；后续新增同 `run_id` 的 `record_type: "signal_patch"` 行，只填写 `signal` 和必要上下文。
- 同一 `run_id` 有多条 `signal_patch` 时，以 `ts` 最新的一条作为最终 signal；历史 patch 保留供回溯，不删除。
- `work_dir` 记录人类可读路径，`project_id` 使用 `CC_WORK_DIR` SHA256 前 12 hex，与 homunculus 对齐。
- 默认 `full` 模式保留脱敏后的原文；`metadata` 模式移除 `text`，保留 `text_hash`、command、run_id、project_id、lifecycle、signal。
- `cancelled` 是合法 lifecycle，用于用户中断、撤回、手动停止或后台任务被取消。

脱敏规则：

- 日志写入前执行正则脱敏，不通过默认裁字段解决隐私问题。
- 至少屏蔽 token、Bearer、API key、微信/飞书凭据、内网 IP、可识别账号、绝对项目私密路径。
- `text_hash` 基于脱敏后的 `text` 计算，便于去重和 signal patch 对齐。
- 原始 chat 目录必须被 Git 忽略。

Git ignore 规则：

```gitignore
memory/chat/*
!memory/chat/.gitkeep
memory/archive/
```

如后续决定提交 `YYYY-MM-DD.summary.md`，必须显式白名单摘要文件；原始 JSONL 和 archive 永远不提交。

## v1 Known Gaps

- v2.3 第一版只记录命中 controller command 的消息。
- 普通微信/飞书文字、默认 agent 自由聊天、agent 回复目前无法完整捕获。
- 完整消息层日志依赖 cc-connect message hook，需要 fork 或 upstream PR。
- 不做 catch-all alias，因为它会和默认 agent 路由、alias 路由、fallback 行为冲突。

## Research Files

`memory/research/decisions.md` 与 `memory/research/pitfalls.md` 使用相同最小条目格式，最新条目放顶部。

```markdown
## 2026-05-18: 标题（≤20 字）

**Status:** active
**Context:** 一句话场景
**Decision/Pitfall:** 实际内容
**Evidence:** 文件、run_id、日志、图表或 PR 链接
**Linked:** [[memory-name]] / progress.md#section
**Superseded by:** （仅 superseded 时填写）
```

Status 定义：

- `active`：当前有效。
- `superseded`：有更新条目替代它，必须填写 `Superseded by`。
- `deprecated`：已失效但没有替代，例如假设被证伪、流程下线。

Append-only 规则：

- 默认只追加新条目，不回改历史。
- 唯一例外：允许修改旧条目的 `Status` 和 `Superseded by`。
- `Context`、`Decision/Pitfall`、`Evidence`、`Linked` 一律不回改；如果写错或结论变化，追加新条目。

## Grill-Me Integration

Grill-Me 是 research-memory 的科研计划质询入口，不是独立记忆系统。

触发场景：

- 新科研项目启动前，例如单细胞/空间/AIDD/docking/MD 大流程开跑前。
- `/计划审查` 返回 `REVISE` 或 `BLOCK`，用户不确定如何改。
- 准备执行高成本或不可逆步骤，例如大规模下载、长时间 MD、批量 docking、覆盖结果目录。
- 项目阶段收尾或快结束时，用于复盘假设、证据链、遗漏验证和可沉淀 skill。

行为规则：

- 一次只问一个问题，每题附推荐答案。
- 如果问题可通过读取 `progress.md`、handoff、最近 plan-review、`memory/research/*.md` 解决，先自行读取，不问用户。
- 质询输出不直接写核心记忆；只有用户确认后的结论才追加到 `decisions.md` 或 `pitfalls.md`。
- 可沉淀为跨项目经验的内容进入 feedback memory；项目特有内容进入 `progress.md ## Pinned` 或本地 `research/` 文件。

输出沉淀：

- 计划选择、参数选择、证据链补强写入 `decisions.md`。
- 失败模式、风险暴露、错误假设写入 `pitfalls.md`。
- 重复出现的操作偏好仍交给 homunculus/continuous-learning-v2 instinct 管线，不写入本地 `memory/instincts/`。

## Lazy Maintenance

v2.3 不引入 cron、Windows 计划任务或后台守护进程。所有整理都走 lazy 模式，由用户触发 `/记忆整理` 或等价 controller command。

整理动作：

- 扫描 `memory/chat/*.jsonl`，为当天或指定日期生成 `YYYY-MM-DD.summary.md`。
- 原始 JSONL 保留 7 天；超过 7 天的日志移动到 `memory/archive/YYYY-MM/`。
- 检查 `decisions.md` 和 `pitfalls.md` 中低价值、重复、被替代条目，生成整理建议，不自动删除。
- 不处理 homunculus instinct retire；instinct 生命周期继续由 continuous-learning-v2/homunculus 管。
- v2.3 采用隔离策略：continuous-learning-v2 只消费自己的 hook 事件，不读取 `memory/chat/*.jsonl`；chat 日志只供人和 agent 阅读。将 chat 转为 c-l-v2 observation 留到 v2.4 之后再评估。

## Domain Rules

生信、单细胞、空间转录组：

- 只在本项目 memory 中记录数据集、批次、QC 阈值、聚类 resolution、注释依据、关键 marker、空间区域、细胞互作结论和证据链接。
- 不复制通用 scRNA-seq 或空间分析 SOP；通用流程属于对应 skill。
- 结论必须区分数据支持、推测和待验证假设。

AI 蛋白、AIDD、分子对接、MD：

- 只记录本项目的结构版本、配体来源、参数偏离、失败修复、docking pose 证据、MD 指标摘要和关键判断。
- 不复制 GROMACS、docking、AIDD 的通用执行手册；通用流程属于对应 skill。
- docking 分数不能单独作为机制证据；MD 结论必须链接 RMSD/RMSF/Rg/SASA/H-bond/MM-PBSA 等指标。

## Implementation Scope

只更新计划和后续规范，不在本阶段改动脚本行为：

- 重写 `docs/research-memory-plan.md` 为 bridge plan。
- 后续正式实现时，再新增 `docs/research-memory.md`、`/记忆整理` 命令和 chat log 写入逻辑。
- 不改 `SKILL.md`、config、controller 脚本或真实本机配置，除非进入实施阶段。

## Test Plan

- 边界检查：计划中明确 auto-memory、progress/handoff、homunculus、项目本地 memory 四方职责，不存在第二套入口。
- 路径检查：MEMORY.md 指针使用 `work_dir + rel_path` 双字段，并定义 `CC_WORK_DIR` 优先解析规则。
- 日志检查：chat schema 拆分 `lifecycle` 和 `signal`，包含 `work_dir` 与 `project_id`。
- Signal 检查：同一 `run_id` 多条 `signal_patch` 能按最新 `ts` 解析最终 signal。
- 中断检查：`cancelled` lifecycle 不计入 `failed` 统计。
- Git ignore 检查：`memory/chat/.gitkeep` 可提交，`memory/chat/*.jsonl` 和 `memory/archive/` 不可提交。
- Known gap 检查：文档明确 v2.3 不捕获普通聊天和 agent 回复，不使用 catch-all alias。
- 目录收缩检查：v2.3 只要求 `chat/`、`research/decisions.md`、`research/pitfalls.md`、`archive/`，不预建空壳目录。
- 真实项目试运行：在一个 MD 或单细胞项目中使用 14 天；从未写过的目录删除，写过但少于 3 条的扩展文件标记 deprecated。

## Assumptions

- auto-memory 的实际默认路径是 `~/.claude/projects/<dir-hash>/memory/MEMORY.md`。
- homunculus 的实际默认路径是 `~/.claude/homunculus/`，不是 `~/.Codex/homunculus/`。
- v2.3 首先解决记忆边界和降噪，不引入 fact_store、byterover、hindsight 或语义检索。
- fact_store 等到出现真实检索失败案例后再设计，例如“我们在哪个数据集验证过某靶点”无法回答。
- v2.3 上线 14 天后必须审计：从未写过的目录删除；写过但少于 3 条的扩展文件标记 deprecated，防止 research-memory 自己变成新噪音源。
