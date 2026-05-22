# cc-controller AstrBot Integration Contract v1

> v1 = read-only + review (no execute). No shell, no credentials.

## Command Whitelist

| Command | Invocation | Output |
|---------|-----------|--------|
| `research-monitor` | `cc-controller.exe research-monitor --format json [--detector X]` | MonitorOutput JSON |
| `system-status` | `cc-controller.exe status` | Plain text (3-line condensed) |
| `submit-review` | `bin/submit-plan-review.ps1 <task>` | Run ID (async, background runner) |
| `show-review` | Read marked `runs/<RunId>/` files directly | Review status + summary JSON |
| `review-stats` | Scan all AstrBot-marked `runs/` directories | Aggregated stats JSON |
| `detector-intake` | Local plugin template only | Detector misclassification intake prompt template |
| `submit-detector-draft` | Local plugin template only | Codex/reviewer handoff prompt template |
| `memory-status` | Read fixed project memory file metadata only | Memory health status summary |
| `memory-record` | Read-only memory file freshness check | Memory file staleness report with update recommendations |
| `memory-archive` | Read-only progress.md archive candidate scan | Archive candidate list with line numbers, reasons, and recommendation |
| `memory-recap` | Read-only handoff + progress.md content retrieval | Recap context summary with handoff and active progress content |
| `memory-archive-execute` | Scan progress.md candidates + move to progress.archive.md | Archive execution result with line count changes |
| `help` | Local plugin only | Command list with categories |

No general execute commands are permitted in v1. No `/纭鎵ц`, no `/鎵ц`.

`memory-archive-execute` is the sole write command, limited to moving completed/stale entries from `progress.md` to `progress.archive.md`. It does not accept user-specified line numbers 鈥?it scans and moves only pattern-matched candidates (DONE/completed items + entries older than 14 days).

Detector intake and memory steward helper commands are read-only except `memory-archive-execute`. They do not write drafts or mutate code.

Review isolation: `submit-review` must write `astrbot-review.json` into the created run directory. `show-review` only exposes runs with that marker, including explicit Run ID lookups. This prevents Feishu from returning legacy WeChat/QQ plan-review runs.

## Allowed Detectors (11)

```
gromacs, schrodinger, haddock3, rosetta, autodock_vina,
alphafold, amber_openmm, gaussian, python_pipeline,
r_pipeline, generic_cli
```

Validation: must match `^[a-z_]+$` AND be in the whitelist above.

## Detector Aliases (plugin layer only)

Aliases are resolved in the Python plugin before calling adapter.ps1.
The adapter and cc-controller only see canonical names.

| Alias | Canonical |
|-------|-----------|
| maestro, glide, ligprep, desmond, 钖涘畾璋?| schrodinger |
| pyrosetta | rosetta |
| colabfold | alphafold |
| amber, openmm | amber_openmm |
| vina | autodock_vina |
| haddock | haddock3 |

Ambiguous aliases (prompt user to choose):
- 瀵规帴, docking, dock 鈫?autodock_vina / schrodinger / haddock3 / rosetta

## Binary Path

```
C:\cc-base\controller\cc-controller.exe
```

## Work Directory Resolution

1. Read `C:\cc-base\controller\active_project.json` 鈫?use `work_dir` field
2. Fallback: `C:\cc-base` (only if active_project.json missing or invalid)

Set via `CC_RESEARCH_MONITOR_ROOT` env var before calling cc-controller.
Must never fall back to AstrBot cwd or `.`.

## JSON Envelope

All adapter output is a single JSON object on stdout. No Write-Host, no mixed text.

```json
{
  "ok": true,
  "command": "research-monitor",
  "detector": null,
  "data": { },
  "error": null,
  "ts": "2026-05-22T12:00:00Z"
}
```

### `data` field schema

**`research-monitor`**: `data` = cc-controller MonitorOutput as-is (no wrapping, no transformation):

```json
{
  "scan": {
    "run_id": "...",
    "work_dir": "...",
    "scanned_at": "...",
    "scan_depth": 3,
    "detector_filter": "",
    "total_tasks": 0
  },
  "summary": {
    "by_state": {},
    "by_bucket": {}
  },
  "tasks": []
}
```

**`system-status`**: `data` = raw text wrapped:

```json
{
  "text": "<raw cc-controller status output>"
}
```

**`submit-review`**: `data` = async submission confirmation:

```json
{
  "run_id": "20260522-123456-plan-review",
  "message": "瀹℃煡宸叉彁浜わ紝鍚庡彴杩愯涓?,
  "task": "<user task text>"
}
```

**`show-review`**: `data` = review status + results:

```json
{
  "run_id": "20260522-123456-plan-review",
  "status": "completed",
  "stage": "done",
  "verdict": "APPROVE",
  "failure_reason": null,
  "failure_stage": null,
  "next_step": null,
  "task": "<task text>",
  "summary": "<summary.md content, max 2000 chars>"
}
```

For failed runs, `failure_reason`, `failure_stage`, and `next_step` are populated by `adapter.ps1`. The adapter prefers the latest structured failed/error event from `events.jsonl`, then falls back to a redacted `background-err.log` summary. The plugin must only format these fields and must not read raw logs.

**`review-stats`**: `data` = aggregated AstrBot review run statistics:

```json
{
  "total": 3,
  "completed": 1,
  "failed": 1,
  "running": 1,
  "verdicts": { "APPROVE": 0, "REVISE": 1, "BLOCK": 0 },
  "failure_stages": { "codex_review": 1 },
  "avg_duration_seconds": 142,
  "recent_runs": [
    {
      "run_id": "20260522-143858-plan-review",
      "status": "completed",
      "verdict": "REVISE",
      "task": "task text (max 80 chars)",
      "duration_seconds": 142
    }
  ]
}
```

`avg_duration_seconds` is computed from completed runs only (marker `created_at` to `runner.exitcode.txt` LastWriteTime). `recent_runs` shows the 5 most recent AstrBot-marked runs. Only runs with `astrbot-review.json` marker are counted.

## Error Codes

| Code | When |
|------|------|
| `INVALID_COMMAND` | Command not in whitelist |
| `INVALID_DETECTOR` | Detector not in whitelist or fails regex |
| `TIMEOUT` | cc-controller did not respond within 20s |
| `CONTROLLER_ERROR` | Non-zero exit code or invalid output |
| `INJECTION_BLOCKED` | Detector contains special characters |
| `WORK_DIR_BLOCKED` | work_dir from active_project.json not under allowed roots |

Error envelope example:

```json
{
  "ok": false,
  "command": "research-monitor",
  "detector": "gromacs; rm -rf /",
  "data": null,
  "error": "INJECTION_BLOCKED",
  "ts": "2026-05-22T12:00:00Z"
}
```

## Security Constraints

1. No `shell=True` / no `Invoke-Expression` / no string interpolation in args
2. Detector must match `^[a-z_]+$` AND be in whitelist
3. No env var passthrough except `CC_RESEARCH_MONITOR_ROOT`
4. Binary path hardcoded, not user-configurable
5. 20-second timeout on all subprocess calls
6. No credential files read (no config.toml access)
7. Adapter outputs ONLY the JSON envelope to stdout
8. **work_dir whitelist (#88)**: `research-monitor` and `submit-review` validate work_dir against allowed roots (`C:\cc-base`, `D:\research-work`, `E:\ai\inquring`). Out-of-bounds paths return `WORK_DIR_BLOCKED`.
9. **Path redaction (#88)**: All user-facing text (review summaries, task snippets, failure reasons) passes through `Redact-ReviewText` which replaces `[A-Za-z]:\\...` with `<path>`.
10. **Group chat gate (#88)**: Plugin restricts `/鎻愪氦瀹℃煡`, `/瀹℃煡缁撴灉`, `/瀹℃煡缁熻`, `/纭褰掓。` to private chat only. Group chat (`GroupMessage` in `unified_msg_origin`) receives a rejection message.

## Timeout

20 seconds. Adapter must capture both stdout and stderr while enforcing timeout.


