# Research Job Monitor Plan

## v1 Implementation Status (2026-05-20)

**v1 已实现**：detector.go + detector_docker.go + research_monitor.go + detector_test.go

| 组件 | 状态 | 说明 |
|------|------|------|
| Detector 接口 + ResearchStatus | ✅ Done | detector.go |
| Python Pipeline Detector | ✅ Done | context.json 优先解析, pharmcell/tcga/generic 三种格式 |
| R Pipeline Detector | ✅ Done | research_context.json 解析, .Rout 错误检测 |
| Generic CLI Detector | ✅ Done | *.log/*.out 兜底, filterFalseErrors 误判过滤 |
| GROMACS Detector | ✅ Done | md.log + 后处理文件检测, 完成后分析建议 |
| Docker Container Detector | ✅ Done | prosettac/haddock3/colabfold/rosetta/rfdiffusion 等 |
| /科研监控 命令 + 别名 | ✅ Done | config.toml + template, 7 个别名 |
| 手机端摘要 + report.md | ✅ Done | research_monitor.go |
| 单元测试 (37 tests) | ✅ Done | detector_test.go |
| V5 真实路径验收 | ✅ Done | G:\proteinwork\work-9: 22 FS + 16 Docker = 38 任务 |
| Maestro/HADDOCK3/Rosetta detector | ❌ Phase 2 | 待后续 PR |
| 日志脱敏 | ❌ v2 | 待后续 |
| 后台常驻监控 | ❌ v2 | 待后续 |

**Bug fixes during v1**:
- `filterFalseErrors`: "0 job(s) failed" 不再触发 FAILED（negated failure detection）
- GROMACS post-processing: 有 md_fit.xtc/md_nojump.xtc 的旧模拟正确识别为 COMPLETED

## Summary

`/md状态检查` 不应继续扩展成只服务 GROMACS 的单点功能。根据现有 skill 库，cc-base 需要一个通用科研任务监控框架：

```text
/科研监控
```

MD、对接、HADDOCK3、Maestro、AutoDock、Rosetta、PyRosetta、R、Python、生信 pipeline 都作为 detector 接入。

核心原则：

- 手机端只显示短摘要。
- 详情写入本地报告。
- 第一版只读，不启动、不停止、不删除科研任务文件。
- 先做通用 Python/R/CLI detector，再做专用科学软件 detector。

## Skill Library Coverage

从当前 skill 库可见，主要工作类型包括：

| 领域 | Skill | 常见执行方式 | 关键输出/状态文件 |
|---|---|---|---|
| MD | `gromacs-md` | `gmx mdrun` / WSL / Docker | `.log`, `.cpt`, `.xtc`, `.trr`, `.edr`, `.gro`, `.tpr` |
| 信息驱动对接 | `haddock3` | Docker / `haddock3 config.cfg` | `run-*`, `0_topoaa` 至 `8_caprieval`, `analysis`, `.out`, `.log` |
| 分子对接 | `molecdock` | Python pipeline + Maestro/SeeSAR/infiniSee | `run_pipeline.py`, `config.py`, Glide/LigPrep/Prime logs, CSV summaries |
| AIDD | `aidd` | Python scripts / conda env | `run_pipeline.py --list`, reports, CSV, model outputs, traceback |
| 蛋白设计 | `protein-design` | Python + Rosetta/ESM/Foldseek | `run_pipeline.py --list`, `score.sc`, `.silent`, `.pt`, logs |
| 生信通用 | `bioinfo` | Rscript pipeline | `run_pipeline.R`, phase outputs, CSV/PDF/RDS, `.Rout` |
| 单细胞 | `singlecell` | R/Python scripts | Seurat `.rds`, `.h5ad`, figures, logs, checkpoint outputs |
| 空间转录组 | `spatial-transcriptomics` | Python pipeline | `.h5ad`, `.zarr`, `figures/`, `results/`, phase checkpoints |
| TCGA/预后 | `tcga-prognosis` | R scripts | `context.json`, R outputs, figures, model files |
| 靶点发现 | `miptd` | Python main/run pipeline | `main.py`, `run.py`, stage outputs, enrichment reports |
| 虚拟给药/扰动 | `pharmcell` | Python pipeline | `scripts/run_pipeline.py`, `context.json`, phase outputs, reports |
| 因果推断 | `causality` | Python pipeline | `run_pipeline.py`, JSON/CSV checkpoints, `analysis_report.md` |
| GEO/机器学习 | `geo-ml-repo` | R pipeline | `run_pipeline.R`, `research_context.R`, phase outputs, reports |
| MR/因果/GEO ML | `mr-genomics`, `causality`, `geo-ml-repo` | R/Python | logs, CSV, model metrics, reports |

结论：直接为每个软件写一个 command 会失控。应使用统一 detector framework。

## Command Design

新增主命令：

```text
/科研监控
```

别名：

```text
/任务监控
/运行监控
/检查任务
```

保留兼容入口：

```text
/md状态检查
```

但它应等价于：

```text
/科研监控 --detector gromacs
```

手机端输出示例：

```text
科研任务状态: running
类型: GROMACS
项目: work-9
最近更新: 4 分钟前
关键文件: md.log
建议: 继续等待
详情: runs/research-monitor-20260519-xxxx/report.md
```

## Detector Framework

建议新增：

```text
cmd/cc-controller/research_monitor.go
cmd/cc-controller/detector.go
cmd/cc-controller/detectors/
```

Detector 接口：

```go
type Detector interface {
    Name() string
    Match(root string) (bool, int)   // matched, confidence score (0-100)
    Inspect(root string) ResearchStatus
}
```

多 detector 评分机制：所有 detector 对同一目录打分，不是第一个命中就停。选最高分 detector 作为主判断，其余列为候选。一个目录可能同时命中多个 detector（如 protein-design 同时有 Python + Rosetta）。

```go
// 评分参考
// GROMACS: .tpr + md.log + .cpt          = 90
// Python:  run_pipeline.py + config.py   = 70
// R:       run_pipeline.R + config.R     = 70
// Generic: *.log                         = 30
```

状态结构：

```go
type ResearchStatus struct {
    Detector      string
    State         string     // running | completed | stuck | failed | idle | unknown
    Confidence    string     // high | medium | low
    WorkDir       string
    MainProcess   string     // auxiliary only on Windows (python.exe too generic)
    KeyFiles      []string
    LastUpdate    time.Time
    Evidence      []string   // MANDATORY: every status must explain why
    Warnings      []string
    NextActions   []string   // detector-specific recommendations
    DetailReport  string
    ContextPhase  string     // from context.json if available, e.g. "Phase B, 18/24 scripts"
}
```

统一状态定义：

| 状态 | 含义 |
|---|---|
| `running` | 关键输出文件最近 N 分钟内更新（主判断）或有进程（辅助） |
| `completed` | 检测到完成标志、最终报告或 context.json status=success |
| `stuck` | 关键文件超过 stuck 阈值未更新 |
| `failed` | 检测到 fatal/error/traceback/non-zero exit |
| `idle` | 有项目文件但没有运行任务 |
| `unknown` | 证据不足，不能判断。**unknown 比误判好** |

置信度定义：

| 级别 | 含义 | 示例 |
|---|---|---|
| `high` | 多条独立证据一致 | md.log 4 分钟前更新 + mdrun 进程存在 |
| `medium` | 单条强证据 | context.json current_phase=B 但无进程 |
| `low` | 仅文件名匹配 | 发现 run_pipeline.py 但无日志/checkpoint |

## Multi-Task Support

一个项目目录里可能同时有 MD、Glide、R 分析。`/科研监控` 不应该只返回一个任务。

扫描流程：
1. 对 root 下每个子目录（depth ≤ max_scan_depth）运行所有 detector
2. 收集所有 score > 0 的匹配
3. 合并同类（同一 detector 的多个子目录可以合并报告）
4. 按状态优先级排序

手机端摘要排序优先级：

```text
failed > stuck > running > completed > idle > unknown
```

用户最想先看到出问题的任务。

手机端多任务输出示例：

```text
发现 3 类任务:
1. [FAILED] R pipeline — Error in library(xxx)
2. [RUNNING] GROMACS — md.log 4分钟前更新
3. [COMPLETED] Python — context.json 24/24 scripts done
详情: runs/research-monitor-xxx/report.md
```

## Implementation Constraints

v1 必须遵守的 11 条设计约束：

### C1: context.json 是一等公民

pharmcell / tcga-prognosis / geo-ml-repo / miptd 都用 context.json 或 research_context.json 做结构化状态跟踪。当 detector 发现 context.json 时，优先解析其 status/checkpoint/phase 字段，报告结构化进度（如 "Phase B, 18/24 scripts done"），而不只靠文件名判断。

解析优先级：
1. `context.json` / `pharmcell_context.json` / `tcga_prognosis_context.json` — script status / checkpoint booleans
2. `research_context.json` — phase fields population
3. `checkpoints/*.json` + `.done` files — stage completion count
4. 文件修改时间 — running/stuck 判断
5. 日志内容 — error/completion detection

### C2: 文件修改时间 > 进程名

Windows 上 `python.exe` / `Rscript.exe` 是通用进程名，无法区分项目。v1 主判断依赖文件修改时间：

```text
关键输出文件最近 N 分钟更新 → running
超过 stuck 阈值未更新 → stuck
有 completed marker/report → completed
有 traceback/Error → failed
```

进程检测（tasklist）仅作辅助证据，提升置信度。

### C3: 多 Detector 评分，不是首个命中

所有 detector 对同一目录打分。选最高分作为主判断，其余列为候选。一个目录可能同时匹配 Python + Rosetta。

### C4: 支持多任务并存

一个项目目录可能同时有 MD + Glide + R 分析。返回所有匹配任务的列表，按状态优先级排序。

### C5: Stuck 阈值按 Detector 区分

| Detector | 默认 stuck 阈值 | 原因 |
|---|---|---|
| generic_cli | 30 min | 通用 |
| python_pipeline | 45 min | 大模型/数据处理可能较慢 |
| r_pipeline | 45 min | 大数据集分析可能较慢 |
| gromacs | 30 min | mdrun 应持续写 log |
| maestro | 60 min | Glide/Prime 单步较久 |
| haddock3 | 120 min | Docker 某阶段可能很久 |

允许 `CC_RESEARCH_STUCK_MINUTES` 环境变量覆盖所有 detector 的阈值。

### C6: 日志读取限量 + 脱敏

```text
max_tail_lines = 80        # 本地报告
mobile_tail_lines = 5      # 手机端摘要
```

脱敏规则（v1 可选，v2 强制）：
- 替换 Windows 绝对路径中的用户名
- 掩码 API key / token 模式（`sk-...`, `Bearer ...`）

### C7: 完成后建议按 Detector 定制

| Detector | 完成后推荐 |
|---|---|
| gromacs | RMSD, RMSF, Rg, SASA, H-bond, MM-PBSA/GBSA |
| maestro | Docking score 排名, MMGBSA, pose 检查 |
| haddock3 | caprieval, cluster 分析, interface contacts |
| python_pipeline | 检查 metrics, Top-N, 模型性能, 查看报告 |
| r_pipeline | 检查 PDF/CSV/RDS, 汇总报告, 关键图质量 |
| generic_cli | 查看最终输出, 检查 exit code |

### C8: 状态必须带 Evidence

每个状态判断必须输出至少一条 evidence。不允许空 evidence 列表。

```text
Evidence:
- found run_pipeline.py (confidence +20)
- context.json current_phase = B (confidence +30)
- latest output updated 6 min ago (confidence +20)
→ State: running, Confidence: medium
```

### C9: unknown 比误判好

证据不足时返回 unknown + low confidence，不要猜测。

```text
状态: unknown
置信度: low
依据: 只发现 run_pipeline.py，未发现日志或 checkpoint
建议: 确认是否已启动任务
```

### C10: 扫描深度和排除目录

```text
max_scan_depth = 3
exclude_dirs = [".git", "node_modules", "__pycache__", "venv", ".venv",
                "renv", ".snakemake", "trajectory", ".cache"]
```

detector 只在浅层匹配特征文件，不递归扫描 TB 级轨迹或 checkpoint 目录。文件大小超过 10MB 的日志只读最后 N 行。

### C11: 真实项目验收

不用 mock 目录。至少用以下真实路径验收：

```text
G:\proteinwork\work-9    # GROMACS 三体系
```

确认 detector 能正确识别或返回合理的 unknown，并解释依据。后续用 miptd/pharmcell 项目目录验证 Python detector。

## Detector Priority

v1 不应先写所有专用 detector。推荐顺序：

### Phase 1: Generic Detectors

先实现通用 detector，覆盖最多 skill。

#### Python Pipeline Detector

匹配：

- `run_pipeline.py`
- `main.py`
- `run.py`
- `config.py`
- `context.json`
- `*.log`
- `traceback`
- `*.h5ad`
- `results/`
- `figures/`
- `REPORT.md`
- `analysis_report.md`

进程：

- `python`
- `python.exe`
- `conda`

失败关键词：

```text
Traceback
ModuleNotFoundError
ImportError
CUDA out of memory
RuntimeError
ValueError
FileNotFoundError
```

覆盖：

- AIDD
- MIPTD
- PharmCell
- Causality
- spatial-transcriptomics
- protein-design
- PyRosetta
- Python-based docking utilities

#### R Pipeline Detector

匹配：

- `run_pipeline.R`
- `config.R`
- `research_context.R`
- `context.json`
- `*.Rout`
- `*.RData`
- `*.rds`
- `renv.lock`
- `figures/`
- `results/`

进程：

- `Rscript`
- `Rterm`
- `R.exe`

失败关键词：

```text
Error in
Execution halted
there is no package called
cannot open file
object .* not found
subscript out of bounds
```

覆盖：

- bioinfo
- singlecell
- tcga-prognosis
- geo-ml-repo
- MR/causality/GEO ML 中的大量 R pipeline

#### Generic CLI Detector

匹配：

- `*.log`
- `*.out`
- `*.err`
- `stdout.txt`
- `stderr.txt`
- `status.json`

行为：

- 读取最近修改文件。
- 报告最后 20 行摘要。
- 检测 error/fatal/failed/success/done/completed。

覆盖：

- 尚未写专用 detector 的任意命令行任务。

### Phase 2: Scientific Software Detectors

#### GROMACS Detector

匹配：

- `.tpr`
- `.cpt`
- `.xtc`
- `.trr`
- `.edr`
- `.gro`
- `.top`
- `md.log`

进程：

- `gmx`
- `gmx_mpi`
- `mdrun`

检测：

- log 中 step/time/ns/day/performance。
- `Finished mdrun`。
- fatal/error。
- `.cpt/.xtc/.edr` 更新时间。

完成后建议：

- RMSD
- RMSF
- Rg
- SASA
- H-bond
- MM-PBSA/GBSA

#### Maestro/Schrodinger Detector

匹配：

- `glide*.log`
- `ligprep*.log`
- `prime_mmgbsa*.log`
- `.mae`
- `.maegz`
- `.out`
- `.csv`

进程：

- `glide`
- `ligprep`
- `prime_mmgbsa`
- `jobcontrol`

检测：

- job finished / failed。
- docking score 表是否生成。
- LigPrep 输出数量。
- Prime MMGBSA 输出 CSV。

#### HADDOCK3 Detector

匹配：

- `run-*`
- `config.cfg`
- `0_topoaa/`
- `1_rigidbody/`
- `2_seletop/`
- `3_flexref/`
- `4_emref/`
- `5_rmsdmatrix/`
- `6_clustrmsd/`
- `7_seletopclusts/`
- `8_caprieval/`

进程：

- `haddock3`
- Docker container running haddock3

检测：

- 当前完成到哪个 stage。
- `8_caprieval` 是否存在。
- caprieval/cluster 输出是否完整。
- Docker 是否仍在运行。

#### AutoDock/Vina Detector

匹配：

- `.pdbqt`
- `vina*.log`
- `*.dlg`
- `out*.pdbqt`

进程：

- `vina`
- `autodock4`
- `autogrid4`

检测：

- best affinity。
- docking complete。
- failed parsing receptor/ligand。

#### Rosetta/PyRosetta Detector

匹配：

- `score.sc`
- `.silent`
- `flags`
- `rosetta*.log`
- `struct.db3`
- Python logs with PyRosetta init

进程：

- `rosetta`
- `rosetta_scripts`
- `python` with PyRosetta

检测：

- `score.sc` 是否增长。
- JD2 complete / protocol failed。
- Python traceback。
- 输出结构数量。

## Reporting

每次 `/科研监控` 写入：

```text
runs/research-monitor-YYYYMMDD-HHMMSS/
  summary.md
  report.md
  status.json
```

手机端只发送 `summary.md` 的短摘要。

`report.md` 包含：

- detector 匹配结果。
- 关键证据文件。
- 最近日志片段。
- 状态判断理由。
- 建议下一步。

## Safety

v1 必须只读。

禁止：

- 不启动 GROMACS/HADDOCK/R/Python。
- 不 kill 进程。
- 不删除输出。
- 不移动文件。
- 不自动进入分析阶段。

允许：

- 列目录。
- 读取日志。
- 检查进程。
- 读取文件修改时间。
- 写 monitor 报告到 `runs/`。

## Config

环境变量：

```text
CC_RESEARCH_MONITOR_ROOT      # 默认 active_project.work_dir
CC_RESEARCH_MONITOR_MAX_LOG   # 默认 80 行（报告）/ 5 行（手机）
CC_RESEARCH_STUCK_MINUTES     # 覆盖所有 detector 的 stuck 阈值（默认按 detector 区分）
CC_RESEARCH_SCAN_DEPTH        # 默认 3
```

如果未设置 root：

```text
active_project.work_dir -> CC_WORK_DIR -> 当前目录
```

默认 stuck 阈值（分钟）：

| Detector | 阈值 |
|---|---|
| generic_cli | 30 |
| python_pipeline | 45 |
| r_pipeline | 45 |
| gromacs | 30 |
| maestro | 60 |
| haddock3 | 120 |

扫描排除目录：

```text
.git, node_modules, __pycache__, venv, .venv, renv,
.snakemake, trajectory, .cache, .Rproj.user,
.ipynb_checkpoints, runs, sessions
```

## Implementation Plan

### Step 1: Add Research Monitor Command

新增：

```text
cc-controller research-monitor
```

配置：

```text
/科研监控
/任务监控
/运行监控
/检查任务
```

`/md状态检查` 保持兼容，指向 GROMACS detector。

### Step 2: Implement Generic Detectors

先做：

- Python pipeline detector
- R pipeline detector
- Generic CLI detector

验收：

- 能识别 `run_pipeline.py` 项目。
- 能识别 `main.py/run.py` 型 Python 项目。
- 能识别 `run_pipeline.R` 项目。
- 能识别普通 `.log/.out/.err`。

### Step 3: Implement GROMACS Detector

迁移现有 `/md状态检查` 能力到 detector framework。

验收：

- 有 `.tpr/.log/.cpt/.xtc` 时识别为 GROMACS。
- 能判断 running/completed/stuck/failed/idle。
- 完成后只推荐分析计划，不自动执行。

### Step 4: Add Docking Detectors

按优先级：

1. Maestro/Schrodinger
2. HADDOCK3
3. AutoDock/Vina

### Step 5: Add Protein Design Detectors

按优先级：

1. Rosetta
2. PyRosetta
3. Foldseek/ESM outputs

## Verification Checklist

### V1: Generic Python（run_pipeline.py 型）

在含 `run_pipeline.py` + `config.py` 的项目运行 `/科研监控`。

预期：
- detector = python_pipeline, score ≥ 70
- 输出最近日志/结果目录
- 如有 context.json → 解析 phase/status，报告结构化进度
- 无日志时返回 unknown + low confidence，不误报 completed
- Evidence 列表非空

### V2: Generic Python（main.py/run.py 型）

在含 `main.py` 或 `run.py`（无 run_pipeline.py）的项目运行 `/科研监控`。

预期：
- detector = python_pipeline, score ≥ 50
- 能识别 miptd 型项目的 checkpoints/ 目录和 stage 文件
- confidence 低于 V1（缺少 run_pipeline.py 的强信号）

### V3: context.json 结构化解析

在含 `context.json`（pharmcell/tcga-prognosis 型）的项目运行 `/科研监控`。

预期：
- 解析 context.json 中的 script status / checkpoint fields
- 报告 "X/Y scripts completed" 或 "Phase N, checkpoint Z done"
- 不只报告 "found context.json"

### V4: Generic R

在含 `run_pipeline.R` + `config.R` 的项目运行 `/科研监控`。

预期：
- detector = r_pipeline, score ≥ 70
- 识别 `.Rout/.rds/results/figures`
- 如有 `research_context.json` → 解析 phase fields

### V5: GROMACS（真实路径）

在 `G:\proteinwork\work-9` 运行 `/md状态检查`。

预期：
- detector = gromacs, score ≥ 90
- 手机端短摘要（≤ 5 行 evidence）
- 详情写 `report.md`（含 log tail、文件时间、状态判断理由）
- 完成后推荐 RMSD/RMSF/Rg 等分析
- stuck 阈值 30 分钟

### V6: 多任务并存

在同时含 GROMACS + Python 文件的目录运行 `/科研监控`。

预期：
- 返回多个任务摘要
- 按 failed > stuck > running > completed 排序
- 每个任务独立 evidence

### V7: Safety

确认无任何启动/停止/删除行为。所有操作只读 + 写 monitor 报告到 runs/。

### V8: unknown 不误判

在只有 `main.py`（无日志、无 checkpoint、无 context.json）的空项目运行 `/科研监控`。

预期：
- State = unknown 或 idle
- Confidence = low
- Evidence 说明 "found main.py but no logs/checkpoints"
- 不报告 running 或 completed

## Recommended First PR

第一轮只做：

1. `detector.go` — Detector 接口 + ResearchStatus 结构 + 评分/排序框架
2. `research_monitor.go` — `/科研监控` 命令处理 + 报告生成 + 手机端摘要
3. Python pipeline detector — context.json 优先解析 + main.py/run.py 匹配
4. R pipeline detector — research_context.json 解析 + config.R 匹配
5. Generic CLI detector — *.log/*.out 兜底
6. GROMACS detector — 迁移 `/md状态检查` 能力 + 完成后分析建议
7. main.go 路由 — `research-monitor` subcommand + config.toml `/科研监控` 注册
8. 验收 V1-V8 全部通过

第一轮必须满足：

- C1-C11 全部约束
- 每个状态带 Evidence + Confidence
- 多任务并存输出
- stuck 阈值按 detector 区分
- 日志 tail 限量（报告 80 行，手机 5 行）
- unknown > 误判

第一轮不要做：

- 不做 Maestro/HADDOCK3/Rosetta/AutoDock detector（Phase 2+）
- 不自动跑后续分析
- 不处理任务取消
- 不做后台常驻监控
- 不做日志脱敏（v2）

这样可以覆盖多数 Python/R 生信 pipeline（miptd/pharmcell/causality/geo-ml-repo/tcga-prognosis/bioinfo/singlecell），同时为 MD 和后续对接/蛋白设计 detector 打基础。框架设计使新 detector 只需实现 Match + Inspect 两个方法即可接入。
