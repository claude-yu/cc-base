# Research Job Monitor Plan

## v1 Implementation Status (2026-05-20)

**v1 宸插疄鐜?*锛歞etector.go + detector_docker.go + research_monitor.go + detector_test.go

| 缁勪欢 | 鐘舵€?| 璇存槑 |
|------|------|------|
| Detector 鎺ュ彛 + ResearchStatus | 鉁?Done | detector.go |
| Python Pipeline Detector | 鉁?Done | context.json 浼樺厛瑙ｆ瀽, pharmcell/tcga/generic 涓夌鏍煎紡 |
| R Pipeline Detector | 鉁?Done | research_context.json 瑙ｆ瀽, .Rout 閿欒妫€娴?|
| Generic CLI Detector | 鉁?Done | *.log/*.out 鍏滃簳, filterFalseErrors 璇垽杩囨护 |
| GROMACS Detector | 鉁?Done | md.log + 鍚庡鐞嗘枃浠舵娴? 瀹屾垚鍚庡垎鏋愬缓璁?|
| Docker Container Detector | 鉁?Done | prosettac/haddock3/colabfold/rosetta/rfdiffusion 绛?|
| /绉戠爺鐩戞帶 鍛戒护 + 鍒悕 | 鉁?Done | config.toml + template, 7 涓埆鍚?|
| 鎵嬫満绔憳瑕?+ report.md | 鉁?Done | research_monitor.go |
| 鍗曞厓娴嬭瘯 (37 tests) | 鉁?Done | detector_test.go |
| V5 鐪熷疄璺緞楠屾敹 | 鉁?Done | D:\research-work\work-9: 22 FS + 16 Docker = 38 浠诲姟 |
| Maestro/HADDOCK3/Rosetta detector | 鉂?Phase 2 | 寰呭悗缁?PR |
| 鏃ュ織鑴辨晱 | 鉂?v2 | 寰呭悗缁?|
| 鍚庡彴甯搁┗鐩戞帶 | 鉂?v2 | 寰呭悗缁?|

**Bug fixes during v1**:
- `filterFalseErrors`: "0 job(s) failed" 涓嶅啀瑙﹀彂 FAILED锛坣egated failure detection锛?- GROMACS post-processing: 鏈?md_fit.xtc/md_nojump.xtc 鐨勬棫妯℃嫙姝ｇ‘璇嗗埆涓?COMPLETED

## Summary

`/md鐘舵€佹鏌 涓嶅簲缁х画鎵╁睍鎴愬彧鏈嶅姟 GROMACS 鐨勫崟鐐瑰姛鑳姐€傛牴鎹幇鏈?skill 搴擄紝cc-base 闇€瑕佷竴涓€氱敤绉戠爺浠诲姟鐩戞帶妗嗘灦锛?
```text
/绉戠爺鐩戞帶
```

MD銆佸鎺ャ€丠ADDOCK3銆丮aestro銆丄utoDock銆丷osetta銆丳yRosetta銆丷銆丳ython銆佺敓淇?pipeline 閮戒綔涓?detector 鎺ュ叆銆?
鏍稿績鍘熷垯锛?
- 鎵嬫満绔彧鏄剧ず鐭憳瑕併€?- 璇︽儏鍐欏叆鏈湴鎶ュ憡銆?- 绗竴鐗堝彧璇伙紝涓嶅惎鍔ㄣ€佷笉鍋滄銆佷笉鍒犻櫎绉戠爺浠诲姟鏂囦欢銆?- 鍏堝仛閫氱敤 Python/R/CLI detector锛屽啀鍋氫笓鐢ㄧ瀛﹁蒋浠?detector銆?
## Skill Library Coverage

浠庡綋鍓?skill 搴撳彲瑙侊紝涓昏宸ヤ綔绫诲瀷鍖呮嫭锛?
| 棰嗗煙 | Skill | 甯歌鎵ц鏂瑰紡 | 鍏抽敭杈撳嚭/鐘舵€佹枃浠?|
|---|---|---|---|
| MD | `gromacs-md` | `gmx mdrun` / WSL / Docker | `.log`, `.cpt`, `.xtc`, `.trr`, `.edr`, `.gro`, `.tpr` |
| 淇℃伅椹卞姩瀵规帴 | `haddock3` | Docker / `haddock3 config.cfg` | `run-*`, `0_topoaa` 鑷?`8_caprieval`, `analysis`, `.out`, `.log` |
| 鍒嗗瓙瀵规帴 | `molecdock` | Python pipeline + Maestro/SeeSAR/infiniSee | `run_pipeline.py`, `config.py`, Glide/LigPrep/Prime logs, CSV summaries |
| AIDD | `aidd` | Python scripts / conda env | `run_pipeline.py --list`, reports, CSV, model outputs, traceback |
| 铔嬬櫧璁捐 | `protein-design` | Python + Rosetta/ESM/Foldseek | `run_pipeline.py --list`, `score.sc`, `.silent`, `.pt`, logs |
| 鐢熶俊閫氱敤 | `bioinfo` | Rscript pipeline | `run_pipeline.R`, phase outputs, CSV/PDF/RDS, `.Rout` |
| 鍗曠粏鑳?| `singlecell` | R/Python scripts | Seurat `.rds`, `.h5ad`, figures, logs, checkpoint outputs |
| 绌洪棿杞綍缁?| `spatial-transcriptomics` | Python pipeline | `.h5ad`, `.zarr`, `figures/`, `results/`, phase checkpoints |
| TCGA/棰勫悗 | `tcga-prognosis` | R scripts | `context.json`, R outputs, figures, model files |
| 闈剁偣鍙戠幇 | `miptd` | Python main/run pipeline | `main.py`, `run.py`, stage outputs, enrichment reports |
| 铏氭嫙缁欒嵂/鎵板姩 | `pharmcell` | Python pipeline | `scripts/run_pipeline.py`, `context.json`, phase outputs, reports |
| 鍥犳灉鎺ㄦ柇 | `causality` | Python pipeline | `run_pipeline.py`, JSON/CSV checkpoints, `analysis_report.md` |
| GEO/鏈哄櫒瀛︿範 | `geo-ml-repo` | R pipeline | `run_pipeline.R`, `research_context.R`, phase outputs, reports |
| MR/鍥犳灉/GEO ML | `mr-genomics`, `causality`, `geo-ml-repo` | R/Python | logs, CSV, model metrics, reports |

缁撹锛氱洿鎺ヤ负姣忎釜杞欢鍐欎竴涓?command 浼氬け鎺с€傚簲浣跨敤缁熶竴 detector framework銆?
## Command Design

鏂板涓诲懡浠わ細

```text
/绉戠爺鐩戞帶
```

鍒悕锛?
```text
/浠诲姟鐩戞帶
/杩愯鐩戞帶
/妫€鏌ヤ换鍔?```

淇濈暀鍏煎鍏ュ彛锛?
```text
/md鐘舵€佹鏌?```

浣嗗畠搴旂瓑浠蜂簬锛?
```text
/绉戠爺鐩戞帶 --detector gromacs
```

鎵嬫満绔緭鍑虹ず渚嬶細

```text
绉戠爺浠诲姟鐘舵€? running
绫诲瀷: GROMACS
椤圭洰: work-9
鏈€杩戞洿鏂? 4 鍒嗛挓鍓?鍏抽敭鏂囦欢: md.log
寤鸿: 缁х画绛夊緟
璇︽儏: runs/research-monitor-20260519-xxxx/report.md
```

## Detector Framework

寤鸿鏂板锛?
```text
cmd/cc-controller/research_monitor.go
cmd/cc-controller/detector.go
cmd/cc-controller/detectors/
```

Detector 鎺ュ彛锛?
```go
type Detector interface {
    Name() string
    Match(root string) (bool, int)   // matched, confidence score (0-100)
    Inspect(root string) ResearchStatus
}
```

澶?detector 璇勫垎鏈哄埗锛氭墍鏈?detector 瀵瑰悓涓€鐩綍鎵撳垎锛屼笉鏄涓€涓懡涓氨鍋溿€傞€夋渶楂樺垎 detector 浣滀负涓诲垽鏂紝鍏朵綑鍒椾负鍊欓€夈€備竴涓洰褰曞彲鑳藉悓鏃跺懡涓涓?detector锛堝 protein-design 鍚屾椂鏈?Python + Rosetta锛夈€?
```go
// 璇勫垎鍙傝€?// GROMACS: .tpr + md.log + .cpt          = 90
// Python:  run_pipeline.py + config.py   = 70
// R:       run_pipeline.R + config.R     = 70
// Generic: *.log                         = 30
```

鐘舵€佺粨鏋勶細

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

缁熶竴鐘舵€佸畾涔夛細

| 鐘舵€?| 鍚箟 |
|---|---|
| `running` | 鍏抽敭杈撳嚭鏂囦欢鏈€杩?N 鍒嗛挓鍐呮洿鏂帮紙涓诲垽鏂級鎴栨湁杩涚▼锛堣緟鍔╋級 |
| `completed` | 妫€娴嬪埌瀹屾垚鏍囧織銆佹渶缁堟姤鍛婃垨 context.json status=success |
| `stuck` | 鍏抽敭鏂囦欢瓒呰繃 stuck 闃堝€兼湭鏇存柊 |
| `failed` | 妫€娴嬪埌 fatal/error/traceback/non-zero exit |
| `idle` | 鏈夐」鐩枃浠朵絾娌℃湁杩愯浠诲姟 |
| `unknown` | 璇佹嵁涓嶈冻锛屼笉鑳藉垽鏂€?*unknown 姣旇鍒ゅソ** |

缃俊搴﹀畾涔夛細

| 绾у埆 | 鍚箟 | 绀轰緥 |
|---|---|---|
| `high` | 澶氭潯鐙珛璇佹嵁涓€鑷?| md.log 4 鍒嗛挓鍓嶆洿鏂?+ mdrun 杩涚▼瀛樺湪 |
| `medium` | 鍗曟潯寮鸿瘉鎹?| context.json current_phase=B 浣嗘棤杩涚▼ |
| `low` | 浠呮枃浠跺悕鍖归厤 | 鍙戠幇 run_pipeline.py 浣嗘棤鏃ュ織/checkpoint |

## Multi-Task Support

涓€涓」鐩洰褰曢噷鍙兘鍚屾椂鏈?MD銆丟lide銆丷 鍒嗘瀽銆俙/绉戠爺鐩戞帶` 涓嶅簲璇ュ彧杩斿洖涓€涓换鍔°€?
鎵弿娴佺▼锛?1. 瀵?root 涓嬫瘡涓瓙鐩綍锛坉epth 鈮?max_scan_depth锛夎繍琛屾墍鏈?detector
2. 鏀堕泦鎵€鏈?score > 0 鐨勫尮閰?3. 鍚堝苟鍚岀被锛堝悓涓€ detector 鐨勫涓瓙鐩綍鍙互鍚堝苟鎶ュ憡锛?4. 鎸夌姸鎬佷紭鍏堢骇鎺掑簭

鎵嬫満绔憳瑕佹帓搴忎紭鍏堢骇锛?
```text
failed > stuck > running > completed > idle > unknown
```

鐢ㄦ埛鏈€鎯冲厛鐪嬪埌鍑洪棶棰樼殑浠诲姟銆?
鎵嬫満绔浠诲姟杈撳嚭绀轰緥锛?
```text
鍙戠幇 3 绫讳换鍔?
1. [FAILED] R pipeline 鈥?Error in library(xxx)
2. [RUNNING] GROMACS 鈥?md.log 4鍒嗛挓鍓嶆洿鏂?3. [COMPLETED] Python 鈥?context.json 24/24 scripts done
璇︽儏: runs/research-monitor-xxx/report.md
```

## Implementation Constraints

v1 蹇呴』閬靛畧鐨?11 鏉¤璁＄害鏉燂細

### C1: context.json 鏄竴绛夊叕姘?
pharmcell / tcga-prognosis / geo-ml-repo / miptd 閮界敤 context.json 鎴?research_context.json 鍋氱粨鏋勫寲鐘舵€佽窡韪€傚綋 detector 鍙戠幇 context.json 鏃讹紝浼樺厛瑙ｆ瀽鍏?status/checkpoint/phase 瀛楁锛屾姤鍛婄粨鏋勫寲杩涘害锛堝 "Phase B, 18/24 scripts done"锛夛紝鑰屼笉鍙潬鏂囦欢鍚嶅垽鏂€?
瑙ｆ瀽浼樺厛绾э細
1. `context.json` / `pharmcell_context.json` / `tcga_prognosis_context.json` 鈥?script status / checkpoint booleans
2. `research_context.json` 鈥?phase fields population
3. `checkpoints/*.json` + `.done` files 鈥?stage completion count
4. 鏂囦欢淇敼鏃堕棿 鈥?running/stuck 鍒ゆ柇
5. 鏃ュ織鍐呭 鈥?error/completion detection

### C2: 鏂囦欢淇敼鏃堕棿 > 杩涚▼鍚?
Windows 涓?`python.exe` / `Rscript.exe` 鏄€氱敤杩涚▼鍚嶏紝鏃犳硶鍖哄垎椤圭洰銆倂1 涓诲垽鏂緷璧栨枃浠朵慨鏀规椂闂达細

```text
鍏抽敭杈撳嚭鏂囦欢鏈€杩?N 鍒嗛挓鏇存柊 鈫?running
瓒呰繃 stuck 闃堝€兼湭鏇存柊 鈫?stuck
鏈?completed marker/report 鈫?completed
鏈?traceback/Error 鈫?failed
```

杩涚▼妫€娴嬶紙tasklist锛変粎浣滆緟鍔╄瘉鎹紝鎻愬崌缃俊搴︺€?
### C3: 澶?Detector 璇勫垎锛屼笉鏄涓懡涓?
鎵€鏈?detector 瀵瑰悓涓€鐩綍鎵撳垎銆傞€夋渶楂樺垎浣滀负涓诲垽鏂紝鍏朵綑鍒椾负鍊欓€夈€備竴涓洰褰曞彲鑳藉悓鏃跺尮閰?Python + Rosetta銆?
### C4: 鏀寔澶氫换鍔″苟瀛?
涓€涓」鐩洰褰曞彲鑳藉悓鏃舵湁 MD + Glide + R 鍒嗘瀽銆傝繑鍥炴墍鏈夊尮閰嶄换鍔＄殑鍒楄〃锛屾寜鐘舵€佷紭鍏堢骇鎺掑簭銆?
### C5: Stuck 闃堝€兼寜 Detector 鍖哄垎

| Detector | 榛樿 stuck 闃堝€?| 鍘熷洜 |
|---|---|---|
| generic_cli | 30 min | 閫氱敤 |
| python_pipeline | 45 min | 澶фā鍨?鏁版嵁澶勭悊鍙兘杈冩參 |
| r_pipeline | 45 min | 澶ф暟鎹泦鍒嗘瀽鍙兘杈冩參 |
| gromacs | 30 min | mdrun 搴旀寔缁啓 log |
| maestro | 60 min | Glide/Prime 鍗曟杈冧箙 |
| haddock3 | 120 min | Docker 鏌愰樁娈靛彲鑳藉緢涔?|

鍏佽 `CC_RESEARCH_STUCK_MINUTES` 鐜鍙橀噺瑕嗙洊鎵€鏈?detector 鐨勯槇鍊笺€?
### C6: 鏃ュ織璇诲彇闄愰噺 + 鑴辨晱

```text
max_tail_lines = 80        # 鏈湴鎶ュ憡
mobile_tail_lines = 5      # 鎵嬫満绔憳瑕?```

鑴辨晱瑙勫垯锛坴1 鍙€夛紝v2 寮哄埗锛夛細
- 鏇挎崲 Windows 缁濆璺緞涓殑鐢ㄦ埛鍚?- 鎺╃爜 API key / token 妯″紡锛坄sk-...`, `Bearer ...`锛?
### C7: 瀹屾垚鍚庡缓璁寜 Detector 瀹氬埗

| Detector | 瀹屾垚鍚庢帹鑽?|
|---|---|
| gromacs | RMSD, RMSF, Rg, SASA, H-bond, MM-PBSA/GBSA |
| maestro | Docking score 鎺掑悕, MMGBSA, pose 妫€鏌?|
| haddock3 | caprieval, cluster 鍒嗘瀽, interface contacts |
| python_pipeline | 妫€鏌?metrics, Top-N, 妯″瀷鎬ц兘, 鏌ョ湅鎶ュ憡 |
| r_pipeline | 妫€鏌?PDF/CSV/RDS, 姹囨€绘姤鍛? 鍏抽敭鍥捐川閲?|
| generic_cli | 鏌ョ湅鏈€缁堣緭鍑? 妫€鏌?exit code |

### C8: 鐘舵€佸繀椤诲甫 Evidence

姣忎釜鐘舵€佸垽鏂繀椤昏緭鍑鸿嚦灏戜竴鏉?evidence銆備笉鍏佽绌?evidence 鍒楄〃銆?
```text
Evidence:
- found run_pipeline.py (confidence +20)
- context.json current_phase = B (confidence +30)
- latest output updated 6 min ago (confidence +20)
鈫?State: running, Confidence: medium
```

### C9: unknown 姣旇鍒ゅソ

璇佹嵁涓嶈冻鏃惰繑鍥?unknown + low confidence锛屼笉瑕佺寽娴嬨€?
```text
鐘舵€? unknown
缃俊搴? low
渚濇嵁: 鍙彂鐜?run_pipeline.py锛屾湭鍙戠幇鏃ュ織鎴?checkpoint
寤鸿: 纭鏄惁宸插惎鍔ㄤ换鍔?```

### C10: 鎵弿娣卞害鍜屾帓闄ょ洰褰?
```text
max_scan_depth = 3
exclude_dirs = [".git", "node_modules", "__pycache__", "venv", ".venv",
                "renv", ".snakemake", "trajectory", ".cache"]
```

detector 鍙湪娴呭眰鍖归厤鐗瑰緛鏂囦欢锛屼笉閫掑綊鎵弿 TB 绾ц建杩规垨 checkpoint 鐩綍銆傛枃浠跺ぇ灏忚秴杩?10MB 鐨勬棩蹇楀彧璇绘渶鍚?N 琛屻€?
### C11: 鐪熷疄椤圭洰楠屾敹

涓嶇敤 mock 鐩綍銆傝嚦灏戠敤浠ヤ笅鐪熷疄璺緞楠屾敹锛?
```text
D:\research-work\work-9    # GROMACS 涓変綋绯?```

纭 detector 鑳芥纭瘑鍒垨杩斿洖鍚堢悊鐨?unknown锛屽苟瑙ｉ噴渚濇嵁銆傚悗缁敤 miptd/pharmcell 椤圭洰鐩綍楠岃瘉 Python detector銆?
## Detector Priority

v1 涓嶅簲鍏堝啓鎵€鏈変笓鐢?detector銆傛帹鑽愰『搴忥細

### Phase 1: Generic Detectors

鍏堝疄鐜伴€氱敤 detector锛岃鐩栨渶澶?skill銆?
#### Python Pipeline Detector

鍖归厤锛?
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

杩涚▼锛?
- `python`
- `python.exe`
- `conda`

澶辫触鍏抽敭璇嶏細

```text
Traceback
ModuleNotFoundError
ImportError
CUDA out of memory
RuntimeError
ValueError
FileNotFoundError
```

瑕嗙洊锛?
- AIDD
- MIPTD
- PharmCell
- Causality
- spatial-transcriptomics
- protein-design
- PyRosetta
- Python-based docking utilities

#### R Pipeline Detector

鍖归厤锛?
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

杩涚▼锛?
- `Rscript`
- `Rterm`
- `R.exe`

澶辫触鍏抽敭璇嶏細

```text
Error in
Execution halted
there is no package called
cannot open file
object .* not found
subscript out of bounds
```

瑕嗙洊锛?
- bioinfo
- singlecell
- tcga-prognosis
- geo-ml-repo
- MR/causality/GEO ML 涓殑澶ч噺 R pipeline

#### Generic CLI Detector

鍖归厤锛?
- `*.log`
- `*.out`
- `*.err`
- `stdout.txt`
- `stderr.txt`
- `status.json`

琛屼负锛?
- 璇诲彇鏈€杩戜慨鏀规枃浠躲€?- 鎶ュ憡鏈€鍚?20 琛屾憳瑕併€?- 妫€娴?error/fatal/failed/success/done/completed銆?
瑕嗙洊锛?
- 灏氭湭鍐欎笓鐢?detector 鐨勪换鎰忓懡浠よ浠诲姟銆?
### Phase 2: Scientific Software Detectors

#### GROMACS Detector

鍖归厤锛?
- `.tpr`
- `.cpt`
- `.xtc`
- `.trr`
- `.edr`
- `.gro`
- `.top`
- `md.log`

杩涚▼锛?
- `gmx`
- `gmx_mpi`
- `mdrun`

妫€娴嬶細

- log 涓?step/time/ns/day/performance銆?- `Finished mdrun`銆?- fatal/error銆?- `.cpt/.xtc/.edr` 鏇存柊鏃堕棿銆?
瀹屾垚鍚庡缓璁細

- RMSD
- RMSF
- Rg
- SASA
- H-bond
- MM-PBSA/GBSA

#### Maestro/Schrodinger Detector

鍖归厤锛?
- `glide*.log`
- `ligprep*.log`
- `prime_mmgbsa*.log`
- `.mae`
- `.maegz`
- `.out`
- `.csv`

杩涚▼锛?
- `glide`
- `ligprep`
- `prime_mmgbsa`
- `jobcontrol`

妫€娴嬶細

- job finished / failed銆?- docking score 琛ㄦ槸鍚︾敓鎴愩€?- LigPrep 杈撳嚭鏁伴噺銆?- Prime MMGBSA 杈撳嚭 CSV銆?
#### HADDOCK3 Detector

鍖归厤锛?
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

杩涚▼锛?
- `haddock3`
- Docker container running haddock3

妫€娴嬶細

- 褰撳墠瀹屾垚鍒板摢涓?stage銆?- `8_caprieval` 鏄惁瀛樺湪銆?- caprieval/cluster 杈撳嚭鏄惁瀹屾暣銆?- Docker 鏄惁浠嶅湪杩愯銆?
#### AutoDock/Vina Detector

鍖归厤锛?
- `.pdbqt`
- `vina*.log`
- `*.dlg`
- `out*.pdbqt`

杩涚▼锛?
- `vina`
- `autodock4`
- `autogrid4`

妫€娴嬶細

- best affinity銆?- docking complete銆?- failed parsing receptor/ligand銆?
#### Rosetta/PyRosetta Detector

鍖归厤锛?
- `score.sc`
- `.silent`
- `flags`
- `rosetta*.log`
- `struct.db3`
- Python logs with PyRosetta init

杩涚▼锛?
- `rosetta`
- `rosetta_scripts`
- `python` with PyRosetta

妫€娴嬶細

- `score.sc` 鏄惁澧為暱銆?- JD2 complete / protocol failed銆?- Python traceback銆?- 杈撳嚭缁撴瀯鏁伴噺銆?
## Reporting

姣忔 `/绉戠爺鐩戞帶` 鍐欏叆锛?
```text
runs/research-monitor-YYYYMMDD-HHMMSS/
  summary.md
  report.md
  status.json
```

鎵嬫満绔彧鍙戦€?`summary.md` 鐨勭煭鎽樿銆?
`report.md` 鍖呭惈锛?
- detector 鍖归厤缁撴灉銆?- 鍏抽敭璇佹嵁鏂囦欢銆?- 鏈€杩戞棩蹇楃墖娈点€?- 鐘舵€佸垽鏂悊鐢便€?- 寤鸿涓嬩竴姝ャ€?
## Safety

v1 蹇呴』鍙銆?
绂佹锛?
- 涓嶅惎鍔?GROMACS/HADDOCK/R/Python銆?- 涓?kill 杩涚▼銆?- 涓嶅垹闄よ緭鍑恒€?- 涓嶇Щ鍔ㄦ枃浠躲€?- 涓嶈嚜鍔ㄨ繘鍏ュ垎鏋愰樁娈点€?
鍏佽锛?
- 鍒楃洰褰曘€?- 璇诲彇鏃ュ織銆?- 妫€鏌ヨ繘绋嬨€?- 璇诲彇鏂囦欢淇敼鏃堕棿銆?- 鍐?monitor 鎶ュ憡鍒?`runs/`銆?
## Config

鐜鍙橀噺锛?
```text
CC_RESEARCH_MONITOR_ROOT      # 榛樿 active_project.work_dir
CC_RESEARCH_MONITOR_MAX_LOG   # 榛樿 80 琛岋紙鎶ュ憡锛? 5 琛岋紙鎵嬫満锛?CC_RESEARCH_STUCK_MINUTES     # 瑕嗙洊鎵€鏈?detector 鐨?stuck 闃堝€硷紙榛樿鎸?detector 鍖哄垎锛?CC_RESEARCH_SCAN_DEPTH        # 榛樿 3
```

濡傛灉鏈缃?root锛?
```text
active_project.work_dir -> CC_WORK_DIR -> 褰撳墠鐩綍
```

榛樿 stuck 闃堝€硷紙鍒嗛挓锛夛細

| Detector | 闃堝€?|
|---|---|
| generic_cli | 30 |
| python_pipeline | 45 |
| r_pipeline | 45 |
| gromacs | 30 |
| maestro | 60 |
| haddock3 | 120 |

鎵弿鎺掗櫎鐩綍锛?
```text
.git, node_modules, __pycache__, venv, .venv, renv,
.snakemake, trajectory, .cache, .Rproj.user,
.ipynb_checkpoints, runs, sessions
```

## Implementation Plan

### Step 1: Add Research Monitor Command

鏂板锛?
```text
cc-controller research-monitor
```

閰嶇疆锛?
```text
/绉戠爺鐩戞帶
/浠诲姟鐩戞帶
/杩愯鐩戞帶
/妫€鏌ヤ换鍔?```

`/md鐘舵€佹鏌 淇濇寔鍏煎锛屾寚鍚?GROMACS detector銆?
### Step 2: Implement Generic Detectors

鍏堝仛锛?
- Python pipeline detector
- R pipeline detector
- Generic CLI detector

楠屾敹锛?
- 鑳借瘑鍒?`run_pipeline.py` 椤圭洰銆?- 鑳借瘑鍒?`main.py/run.py` 鍨?Python 椤圭洰銆?- 鑳借瘑鍒?`run_pipeline.R` 椤圭洰銆?- 鑳借瘑鍒櫘閫?`.log/.out/.err`銆?
### Step 3: Implement GROMACS Detector

杩佺Щ鐜版湁 `/md鐘舵€佹鏌 鑳藉姏鍒?detector framework銆?
楠屾敹锛?
- 鏈?`.tpr/.log/.cpt/.xtc` 鏃惰瘑鍒负 GROMACS銆?- 鑳藉垽鏂?running/completed/stuck/failed/idle銆?- 瀹屾垚鍚庡彧鎺ㄨ崘鍒嗘瀽璁″垝锛屼笉鑷姩鎵ц銆?
### Step 4: Add Docking Detectors

鎸変紭鍏堢骇锛?
1. Maestro/Schrodinger
2. HADDOCK3
3. AutoDock/Vina

### Step 5: Add Protein Design Detectors

鎸変紭鍏堢骇锛?
1. Rosetta
2. PyRosetta
3. Foldseek/ESM outputs

## Verification Checklist

### V1: Generic Python锛坮un_pipeline.py 鍨嬶級

鍦ㄥ惈 `run_pipeline.py` + `config.py` 鐨勯」鐩繍琛?`/绉戠爺鐩戞帶`銆?
棰勬湡锛?- detector = python_pipeline, score 鈮?70
- 杈撳嚭鏈€杩戞棩蹇?缁撴灉鐩綍
- 濡傛湁 context.json 鈫?瑙ｆ瀽 phase/status锛屾姤鍛婄粨鏋勫寲杩涘害
- 鏃犳棩蹇楁椂杩斿洖 unknown + low confidence锛屼笉璇姤 completed
- Evidence 鍒楄〃闈炵┖

### V2: Generic Python锛坢ain.py/run.py 鍨嬶級

鍦ㄥ惈 `main.py` 鎴?`run.py`锛堟棤 run_pipeline.py锛夌殑椤圭洰杩愯 `/绉戠爺鐩戞帶`銆?
棰勬湡锛?- detector = python_pipeline, score 鈮?50
- 鑳借瘑鍒?miptd 鍨嬮」鐩殑 checkpoints/ 鐩綍鍜?stage 鏂囦欢
- confidence 浣庝簬 V1锛堢己灏?run_pipeline.py 鐨勫己淇″彿锛?
### V3: context.json 缁撴瀯鍖栬В鏋?
鍦ㄥ惈 `context.json`锛坧harmcell/tcga-prognosis 鍨嬶級鐨勯」鐩繍琛?`/绉戠爺鐩戞帶`銆?
棰勬湡锛?- 瑙ｆ瀽 context.json 涓殑 script status / checkpoint fields
- 鎶ュ憡 "X/Y scripts completed" 鎴?"Phase N, checkpoint Z done"
- 涓嶅彧鎶ュ憡 "found context.json"

### V4: Generic R

鍦ㄥ惈 `run_pipeline.R` + `config.R` 鐨勯」鐩繍琛?`/绉戠爺鐩戞帶`銆?
棰勬湡锛?- detector = r_pipeline, score 鈮?70
- 璇嗗埆 `.Rout/.rds/results/figures`
- 濡傛湁 `research_context.json` 鈫?瑙ｆ瀽 phase fields

### V5: GROMACS锛堢湡瀹炶矾寰勶級

鍦?`D:\research-work\work-9` 杩愯 `/md鐘舵€佹鏌銆?
棰勬湡锛?- detector = gromacs, score 鈮?90
- 鎵嬫満绔煭鎽樿锛堚墹 5 琛?evidence锛?- 璇︽儏鍐?`report.md`锛堝惈 log tail銆佹枃浠舵椂闂淬€佺姸鎬佸垽鏂悊鐢憋級
- 瀹屾垚鍚庢帹鑽?RMSD/RMSF/Rg 绛夊垎鏋?- stuck 闃堝€?30 鍒嗛挓

### V6: 澶氫换鍔″苟瀛?
鍦ㄥ悓鏃跺惈 GROMACS + Python 鏂囦欢鐨勭洰褰曡繍琛?`/绉戠爺鐩戞帶`銆?
棰勬湡锛?- 杩斿洖澶氫釜浠诲姟鎽樿
- 鎸?failed > stuck > running > completed 鎺掑簭
- 姣忎釜浠诲姟鐙珛 evidence

### V7: Safety

纭鏃犱换浣曞惎鍔?鍋滄/鍒犻櫎琛屼负銆傛墍鏈夋搷浣滃彧璇?+ 鍐?monitor 鎶ュ憡鍒?runs/銆?
### V8: unknown 涓嶈鍒?
鍦ㄥ彧鏈?`main.py`锛堟棤鏃ュ織銆佹棤 checkpoint銆佹棤 context.json锛夌殑绌洪」鐩繍琛?`/绉戠爺鐩戞帶`銆?
棰勬湡锛?- State = unknown 鎴?idle
- Confidence = low
- Evidence 璇存槑 "found main.py but no logs/checkpoints"
- 涓嶆姤鍛?running 鎴?completed

## Recommended First PR

绗竴杞彧鍋氾細

1. `detector.go` 鈥?Detector 鎺ュ彛 + ResearchStatus 缁撴瀯 + 璇勫垎/鎺掑簭妗嗘灦
2. `research_monitor.go` 鈥?`/绉戠爺鐩戞帶` 鍛戒护澶勭悊 + 鎶ュ憡鐢熸垚 + 鎵嬫満绔憳瑕?3. Python pipeline detector 鈥?context.json 浼樺厛瑙ｆ瀽 + main.py/run.py 鍖归厤
4. R pipeline detector 鈥?research_context.json 瑙ｆ瀽 + config.R 鍖归厤
5. Generic CLI detector 鈥?*.log/*.out 鍏滃簳
6. GROMACS detector 鈥?杩佺Щ `/md鐘舵€佹鏌 鑳藉姏 + 瀹屾垚鍚庡垎鏋愬缓璁?7. main.go 璺敱 鈥?`research-monitor` subcommand + config.toml `/绉戠爺鐩戞帶` 娉ㄥ唽
8. 楠屾敹 V1-V8 鍏ㄩ儴閫氳繃

绗竴杞繀椤绘弧瓒筹細

- C1-C11 鍏ㄩ儴绾︽潫
- 姣忎釜鐘舵€佸甫 Evidence + Confidence
- 澶氫换鍔″苟瀛樿緭鍑?- stuck 闃堝€兼寜 detector 鍖哄垎
- 鏃ュ織 tail 闄愰噺锛堟姤鍛?80 琛岋紝鎵嬫満 5 琛岋級
- unknown > 璇垽

绗竴杞笉瑕佸仛锛?
- 涓嶅仛 Maestro/HADDOCK3/Rosetta/AutoDock detector锛圥hase 2+锛?- 涓嶈嚜鍔ㄨ窇鍚庣画鍒嗘瀽
- 涓嶅鐞嗕换鍔″彇娑?- 涓嶅仛鍚庡彴甯搁┗鐩戞帶
- 涓嶅仛鏃ュ織鑴辨晱锛坴2锛?
杩欐牱鍙互瑕嗙洊澶氭暟 Python/R 鐢熶俊 pipeline锛坢iptd/pharmcell/causality/geo-ml-repo/tcga-prognosis/bioinfo/singlecell锛夛紝鍚屾椂涓?MD 鍜屽悗缁鎺?铔嬬櫧璁捐 detector 鎵撳熀纭€銆傛鏋惰璁′娇鏂?detector 鍙渶瀹炵幇 Match + Inspect 涓や釜鏂规硶鍗冲彲鎺ュ叆銆?
