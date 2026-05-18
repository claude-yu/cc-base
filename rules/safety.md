# 安全约束规则

## 权限控制

- `--dangerously-skip-permissions` 只允许在两个脚本中使用：
  - `execute-approved.ps1`（需 Codex APPROVE）
  - `execute-manual-approved.ps1`（需 USER_ACCEPTED_WITH_REVISE + manual-approval.md + 非 BLOCK）

## 审计要求

- 所有 run 必须写审计文件到 `controller/runs/`
- 每个 run 目录包含：request.md, cc-plan.md, codex-review.md, verdict.md, summary.md

## 路径限制

**`/修复controller` 只能修改**：
- `controller/` 目录
- `cc-connect/` 目录
- `~/.cc-connect/config.toml`

**禁止碰**：
- 用户项目数据目录（`$env:CC_WORK_DIR` 指向的实际工作区）
- MD 轨迹/结果文件
- 任何非基础设施文件

## Config 修改规则

- 所有 config 修改只改**源文件**（`<project>/cc-connect/config.toml`）
- 重启 cc-connect 时自动同步到部署位置
- 紧急情况可手动同步：`Copy-Item -LiteralPath "源" -Destination "部署" -Force`

## admin_from 位置

- `admin_from` 必须在 `[[projects]]` 级别，不能只在 platform options 中
- 否则会出现 "administrator permission required" 错误
