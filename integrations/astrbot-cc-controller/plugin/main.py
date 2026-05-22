"""AstrBot plugin: cc-controller read-only scientific monitor for Feishu."""

from __future__ import annotations

import asyncio
import datetime
import json
import os
import re
import subprocess
from pathlib import Path

from astrbot.api import logger
from astrbot.api.event import AstrMessageEvent, filter
from astrbot.api.star import Context, Star

_DETECTOR_RE = re.compile(r"^[a-z_]+$")

_DETECTOR_ALIASES: dict[str, str] = {
    "maestro": "schrodinger",
    "glide": "schrodinger",
    "ligprep": "schrodinger",
    "desmond": "schrodinger",
    "薛定谔": "schrodinger",
    "pyrosetta": "rosetta",
    "colabfold": "alphafold",
    "amber": "amber_openmm",
    "openmm": "amber_openmm",
    "vina": "autodock_vina",
    "haddock": "haddock3",
}

_AMBIGUOUS_ALIASES: dict[str, list[str]] = {
    "对接": ["autodock_vina", "schrodinger", "haddock3", "rosetta"],
    "docking": ["autodock_vina", "schrodinger", "haddock3", "rosetta"],
    "dock": ["autodock_vina", "schrodinger", "haddock3", "rosetta"],
}

def _time_label(info: dict, today: str | None = None) -> str:
    modified = info.get("last_modified") or ""
    if not modified:
        return "未知"
    if today is None:
        today = datetime.date.today().isoformat()
    days = info.get("days_stale")
    if days is None and " " in modified:
        date_part = modified.split(" ", 1)[0]
        if date_part == today:
            days = 0
    if days is not None and days == 0:
        time_part = modified.split(" ", 1)[1] if " " in modified else modified
        time_part = ":".join(time_part.split(":")[:2])
        return f"今天 {time_part}"
    elif days is not None and days > 0:
        return f"{days}天前"
    return modified


_GROUP_BLOCKED_COMMANDS = {"提交审查", "审查结果", "审查统计", "确认归档"}
_GROUP_BLOCKED_MSG = "[限制] 该命令仅限私聊使用，群聊中不可用。"


def _is_group_chat(event: AstrMessageEvent) -> bool:
    origin = getattr(event, "unified_msg_origin", "") or ""
    return "GroupMessage" in origin


def _format_error(error: str) -> str:
    _TRANSLATIONS = {
        "TIMEOUT": "适配器超时（20秒）",
        "INVALID_COMMAND": "无效命令",
        "INVALID_DETECTOR": "无效检测器名称",
        "INJECTION_BLOCKED": "检测到注入攻击，已拦截",
        "INVALID_RUN_ID": "无效的 Run ID",
        "WORK_DIR_BLOCKED": "工作目录不在允许范围内",
    }
    if error in _TRANSLATIONS:
        return _TRANSLATIONS[error]
    if error.startswith("INVALID_TASK"):
        return "任务描述无效（至少2字符）"
    if error.startswith("CONTROLLER_ERROR:"):
        detail = error[len("CONTROLLER_ERROR:"):].strip()
        return f"控制器错误: {detail}"
    if error.startswith("NO_REVIEWS:"):
        return "无 AstrBot 审查记录"
    if error.startswith("NOT_FOUND:"):
        detail = error[len("NOT_FOUND:"):].strip()
        return f"未找到: {detail}"
    return error


ADAPTER_PATH = str(
    Path(__file__).resolve().parent.parent / "adapter.ps1"
)
SUBPROCESS_TIMEOUT = 25
MAX_DISPLAY_TASKS = 15


class Main(Star):
    def __init__(self, context: Context, config: dict | None = None) -> None:
        super().__init__(context)

    @staticmethod
    def _reply(event: AstrMessageEvent, text: str):
        return event.plain_result(text).stop_event()

    async def _call_adapter(
        self,
        command: str,
        detector: str | None = None,
        task_text: str | None = None,
        run_id: str | None = None,
    ) -> dict:
        env = os.environ.copy()
        env["PYTHONIOENCODING"] = "utf-8"
        env["PYTHONUTF8"] = "1"
        args = [
            "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass",
            "-File", ADAPTER_PATH,
            "-Command", command,
        ]
        if detector:
            args.extend(["-Detector", detector])
        if task_text:
            args.extend(["-TaskText", task_text])
        if run_id:
            args.extend(["-RunId", run_id])

        loop = asyncio.get_running_loop()
        try:
            result = await loop.run_in_executor(
                None,
                lambda: subprocess.run(
                    args,
                    capture_output=True,
                    text=True,
                    timeout=SUBPROCESS_TIMEOUT,
                    encoding="utf-8",
                    errors="replace",
                    env=env,
                ),
            )
        except subprocess.TimeoutExpired:
            return {"ok": False, "error": "TIMEOUT", "data": None}
        except FileNotFoundError:
            return {"ok": False, "error": "CONTROLLER_ERROR: powershell not found", "data": None}

        stdout = result.stdout.strip()
        if not stdout:
            return {"ok": False, "error": "CONTROLLER_ERROR: empty output from adapter", "data": None}

        try:
            return json.loads(stdout)
        except json.JSONDecodeError:
            return {"ok": False, "error": "CONTROLLER_ERROR: invalid JSON from adapter", "data": None}

    @staticmethod
    def _format_monitor(data: dict) -> str:
        scan = data.get("scan", {})
        summary = data.get("summary", {})
        tasks = data.get("tasks", [])

        lines = ["=== 科研监控 ==="]
        work_dir = scan.get("work_dir", "?")
        lines.append(f"工作目录: {_short_path(work_dir)}")
        lines.append(f"任务: {scan.get('total_tasks', 0)}")

        det = scan.get("detector_filter")
        if det:
            lines.append(f"过滤: {det}")

        by_state = summary.get("by_state", {})
        if by_state:
            state_parts = [f"{k}={v}" for k, v in sorted(by_state.items()) if v > 0]
            if state_parts:
                lines.append(f"状态: {' '.join(state_parts)}")

        if not tasks:
            lines.append("\n(无检测到的任务)")
            return "\n".join(lines)

        lines.append("")
        shown = tasks[:MAX_DISPLAY_TASKS]
        for t in shown:
            idx = t.get("index", "?")
            det_name = t.get("detector", "?")
            state = t.get("state", "?")
            conf = t.get("confidence", "?")
            score = t.get("score", "?")
            lines.append(f"#{idx} [{det_name}] {state} ({conf}/{score})")

            work_dir = t.get("work_dir", "")
            if work_dir:
                short = _short_path(work_dir)
                lines.append(f"   目录: {short}")

            evidence = t.get("evidence", [])
            if evidence:
                lines.append(f"   证据: {evidence[0]}")

            warnings = t.get("warnings", [])
            for w in warnings[:2]:
                lines.append(f"   警告: {w}")

        remaining = len(tasks) - len(shown)
        if remaining > 0:
            lines.append(f"\n...还有 {remaining} 个任务")

        return "\n".join(lines)

    @staticmethod
    def _format_status(data: dict) -> str:
        text = data.get("text", "")
        if not text:
            return "(系统状态: 无输出)"
        return f"=== 系统状态 ===\n{text}"

    @filter.command("科研监控", alias={"research"})
    async def research_monitor(self, event: AstrMessageEvent, detector: str | None = None) -> None:
        if detector:
            detector_lower = detector.lower().strip()
            if detector_lower in _AMBIGUOUS_ALIASES:
                options = "、".join(_AMBIGUOUS_ALIASES[detector_lower])
                yield self._reply(
                    event,
                    f'"{detector}" 可能对应多个检测器：{options}。请指定一个。',
                )
                return
            detector = _DETECTOR_ALIASES.get(detector_lower, detector_lower)
            if not _DETECTOR_RE.match(detector):
                yield self._reply(event, f"[错误] {_format_error('INVALID_DETECTOR')}")
                return

        envelope = await self._call_adapter("research-monitor", detector)

        if not envelope.get("ok"):
            error = envelope.get("error", "UNKNOWN")
            yield self._reply(event, f"[错误] {_format_error(error)}")
            return

        data = envelope.get("data")
        if not data:
            yield self._reply(event, "[错误] 控制器错误: 返回数据为空")
            return

        message = self._format_monitor(data)
        yield self._reply(event, message)

    @filter.command("系统状态", alias={"status"})
    async def system_status(self, event: AstrMessageEvent) -> None:
        envelope = await self._call_adapter("system-status")

        if not envelope.get("ok"):
            error = envelope.get("error", "UNKNOWN")
            yield self._reply(event, f"[错误] {_format_error(error)}")
            return

        data = envelope.get("data")
        if not data:
            yield self._reply(event, "[错误] 控制器错误: 返回数据为空")
            return

        message = self._format_status(data)
        yield self._reply(event, message)

    @filter.command("提交审查", alias={"submit-review"})
    async def submit_review(self, event: AstrMessageEvent, task: str | None = None) -> None:
        if _is_group_chat(event):
            yield self._reply(event, _GROUP_BLOCKED_MSG)
            return
        if not task or len(task.strip()) < 2:
            yield self._reply(event, "用法: /提交审查 <任务描述>\n示例: /提交审查 分析虚拟敲除DLG1的RMSD变化")
            return

        task_clean = task.strip()[:500]
        envelope = await self._call_adapter("submit-review", task_text=task_clean)

        if not envelope.get("ok"):
            error = envelope.get("error", "UNKNOWN")
            yield self._reply(event, f"[错误] {_format_error(error)}")
            return

        data = envelope.get("data")
        if not data:
            yield self._reply(event, "[错误] 控制器错误: 返回数据为空")
            return

        run_id = data.get("run_id", "?")
        lines = [
            "=== 审查已提交 ===",
            f"任务: {task_clean}",
            f"Run ID: {run_id}",
            "",
            "后台正在运行: Claude 生成计划 → Codex 审查",
            "预计 1-3 分钟完成",
            f"查看结果: /审查结果 {run_id}",
        ]
        yield self._reply(event, "\n".join(lines))

    @filter.command("审查结果", alias={"review-result"})
    async def show_review(self, event: AstrMessageEvent, run_id: str | None = None) -> None:
        if _is_group_chat(event):
            yield self._reply(event, _GROUP_BLOCKED_MSG)
            return
        envelope = await self._call_adapter("show-review", run_id=run_id)

        if not envelope.get("ok"):
            error = envelope.get("error", "UNKNOWN")
            yield self._reply(event, f"[错误] {_format_error(error)}")
            return

        data = envelope.get("data")
        if not data:
            yield self._reply(event, "[错误] 控制器错误: 返回数据为空")
            return

        message = self._format_review(data)
        yield self._reply(event, message)

    @filter.command("审查统计", alias={"review-stats"})
    async def review_stats(self, event: AstrMessageEvent) -> None:
        if _is_group_chat(event):
            yield self._reply(event, _GROUP_BLOCKED_MSG)
            return
        envelope = await self._call_adapter("review-stats")

        if not envelope.get("ok"):
            error = envelope.get("error", "UNKNOWN")
            yield self._reply(event, f"[错误] {_format_error(error)}")
            return

        data = envelope.get("data")
        if not data:
            yield self._reply(event, "[错误] 控制器错误: 返回数据为空")
            return

        message = self._format_stats(data)
        yield self._reply(event, message)

    @staticmethod
    def _format_stats(data: dict) -> str:
        total = data.get("total", 0)
        completed = data.get("completed", 0)
        failed = data.get("failed", 0)
        running = data.get("running", 0)
        verdicts = data.get("verdicts", {})
        failure_stages = data.get("failure_stages", {})
        avg_duration = data.get("avg_duration_seconds")
        recent_runs = data.get("recent_runs") or []

        lines = ["=== 审查统计 ==="]
        lines.append(f"总计: {total} | 完成: {completed} | 失败: {failed} | 运行中: {running}")

        if total == 0:
            lines.append("\n(尚无 AstrBot 审查记录)")
            return "\n".join(lines)

        v_parts = []
        for k in ("APPROVE", "REVISE", "BLOCK"):
            v = verdicts.get(k, 0)
            if v > 0:
                v_parts.append(f"{k}={v}")
        if v_parts:
            lines.append(f"判定: {' '.join(v_parts)}")

        if avg_duration is not None:
            minutes = int(avg_duration) // 60
            seconds = int(avg_duration) % 60
            lines.append(f"平均耗时: {minutes}m{seconds:02d}s")

        if failure_stages:
            fs_parts = [f"{k}={v}" for k, v in failure_stages.items()]
            lines.append(f"失败阶段: {' '.join(fs_parts)}")

        if recent_runs:
            lines.append("")
            lines.append("最近审查:")
            for r in recent_runs:
                rid = r.get("run_id", "?")
                rid_short = rid[:15] if len(rid) > 15 else rid
                status = r.get("status", "?")
                verdict = r.get("verdict", "?")
                dur = r.get("duration_seconds")
                dur_str = ""
                if dur is not None:
                    dur_str = f" {int(dur) // 60}m{int(dur) % 60:02d}s"
                task = r.get("task", "")
                task_short = task[:40] + "..." if len(task) > 40 else task
                lines.append(f"  {rid_short} [{status}] {verdict}{dur_str}")
                if task_short:
                    lines.append(f"    {task_short}")

        return "\n".join(lines)

    @filter.command("记录误判", alias={"记录检测器误判", "detector-intake"})
    async def detector_intake_template(self, event: AstrMessageEvent) -> None:
        lines = [
            "=== 检测器误判记录模板 ===",
            "把下面模板补全后，直接发给 Memory Steward Agent：",
            "",
            "/记录误判 【detector】 【expected_status】",
            "当前 monitor 显示 【observed_status】。",
            "证据：",
            "research-monitor JSON 显示 status=【observed_status】，evidence=[...]。",
            "日志/文件证据：【粘贴关键日志、结果文件或截图说明】。",
            "请整理成草案，不要写正式记忆。",
            "",
            "只口头确认、没有 JSON/日志/文件证据时，应返回 status: NEEDS_EVIDENCE。",
        ]
        yield self._reply(event, "\n".join(lines))

    @filter.command("转审", alias={"submit-detector-draft", "审查检测器草案", "提交检测器草案"})
    async def detector_draft_handoff(self, event: AstrMessageEvent) -> None:
        lines = [
            "=== Detector 草案转审话术 ===",
            "把上一条 status: DRAFT 草案贴给 Codex，并发送：",
            "",
            "请审查这条 detector intake 草案：",
            "",
            "【粘贴 AstrBot 生成的 status: DRAFT 草案】",
            "",
            "审查目标：",
            "1. 判断是否应该写入 detector-learning-log.md。",
            "2. 判断是否需要修改 detector 逻辑。",
            "3. 设计或补充对应 regression test。",
            "4. 运行必要的窄范围验证后再决定是否正式落地。",
        ]
        yield self._reply(event, "\n".join(lines))

    @filter.command("帮助", alias={"help", "命令列表"})
    async def show_help(self, event: AstrMessageEvent) -> None:
        text = (
            "=== cc-controller 命令列表 ===\n"
            "\n"
            "科研监控:\n"
            "  /科研监控 [detector]  — 查询任务状态\n"
            "  /系统状态            — 系统概览\n"
            "\n"
            "审查:\n"
            "  /提交审查 <任务>     — 提交 Claude+Codex 审查\n"
            "  /审查结果 [run_id]   — 查看审查结果\n"
            "  /审查统计            — 审查汇总\n"
            "\n"
            "记忆管家:\n"
            "  /记忆状态            — 记忆健康检查\n"
            "  /记忆记录            — 文件新鲜度\n"
            "  /记忆归档            — 归档候选扫描\n"
            "  /确认归档            — 执行归档\n"
            "  /recap              — 上下文恢复\n"
            "  /记录误判            — 检测器误判模板\n"
            "  /转审               — 草案转审话术\n"
            "\n"
            "其他:\n"
            "  /帮助               — 显示本列表"
        )
        yield self._reply(event, text)

    @filter.command("记忆状态", alias={"memory-status"})
    async def memory_status(self, event: AstrMessageEvent) -> None:
        envelope = await self._call_adapter("memory-status")

        if not envelope.get("ok"):
            error = envelope.get("error", "UNKNOWN")
            yield self._reply(event, f"[错误] {_format_error(error)}")
            return

        data = envelope.get("data")
        if not data:
            yield self._reply(event, "[错误] 控制器错误: 返回数据为空")
            return

        message = self._format_memory_status(data)
        yield self._reply(event, message)

    @filter.command("记忆记录", alias={"memory-record"})
    async def memory_record_status(self, event: AstrMessageEvent) -> None:
        envelope = await self._call_adapter("memory-record")

        if not envelope.get("ok"):
            error = envelope.get("error", "UNKNOWN")
            yield self._reply(event, f"[错误] {_format_error(error)}")
            return

        data = envelope.get("data")
        if not data:
            yield self._reply(event, "[错误] 控制器错误: 返回数据为空")
            return

        message = self._format_record_status(data)
        yield self._reply(event, message)

    @filter.command("记忆归档", alias={"memory-archive"})
    async def memory_archive(self, event: AstrMessageEvent) -> None:
        envelope = await self._call_adapter("memory-archive")

        if not envelope.get("ok"):
            error = envelope.get("error", "UNKNOWN")
            yield self._reply(event, f"[错误] {_format_error(error)}")
            return

        data = envelope.get("data")
        if not data:
            yield self._reply(event, "[错误] 控制器错误: 返回数据为空")
            return

        message = self._format_archive_candidates(data)
        yield self._reply(event, message)

    @filter.command("确认归档", alias={"archive-execute", "执行归档"})
    async def memory_archive_execute(self, event: AstrMessageEvent) -> None:
        if _is_group_chat(event):
            yield self._reply(event, _GROUP_BLOCKED_MSG)
            return
        envelope = await self._call_adapter("memory-archive-execute")

        if not envelope.get("ok"):
            error = envelope.get("error", "UNKNOWN")
            yield self._reply(event, f"[错误] {_format_error(error)}")
            return

        data = envelope.get("data")
        if not data:
            yield self._reply(event, "[错误] 控制器错误: 返回数据为空")
            return

        message = self._format_archive_result(data)
        yield self._reply(event, message)

    @staticmethod
    def _format_archive_result(data: dict) -> str:
        archived = data.get("archived_count", 0)
        before = data.get("progress_lines_before", 0)
        after = data.get("progress_lines_after", 0)
        arch_before = data.get("archive_lines_before", 0)
        arch_after = data.get("archive_lines_after", 0)
        entries = data.get("entries") or []

        if archived == 0:
            return "=== 归档结果 ===\n当前无可归档条目。"

        lines = ["=== 归档完成 ==="]
        lines.append(f"已移动 {archived} 个条目到 progress.archive.md")
        lines.append(f"progress.md: {before} → {after} lines")
        lines.append(f"progress.archive.md: {arch_before} → {arch_after} lines")

        if entries:
            lines.append("")
            lines.append("已归档:")
            for e in entries:
                text = e.get("text", "")
                if len(text) > 80:
                    text = text[:80] + "..."
                lines.append(f"- {text}")

        return "\n".join(lines)

    @filter.command("recap", alias={"memory-recap"})
    async def memory_recap(self, event: AstrMessageEvent) -> None:
        envelope = await self._call_adapter("memory-recap")

        if not envelope.get("ok"):
            error = envelope.get("error", "UNKNOWN")
            yield self._reply(event, f"[错误] {_format_error(error)}")
            return

        data = envelope.get("data")
        if not data:
            yield self._reply(event, "[错误] 控制器错误: 返回数据为空")
            return

        message = self._format_recap(data)
        yield self._reply(event, message)

    @staticmethod
    def _format_recap(data: dict) -> str:
        handoff_name = data.get("handoff_name")
        handoff_lines = data.get("handoff_lines", 0)
        handoff_content = data.get("handoff_content") or ""
        progress_lines = data.get("progress_lines", 0)
        progress_active = data.get("progress_active_content") or ""
        has_today = data.get("has_today_handoff", False)

        lines = ["=== 上下文恢复 ==="]

        if handoff_name:
            today_tag = " (今日)" if has_today else ""
            lines.append(f"Handoff: {handoff_name} ({handoff_lines} lines){today_tag}")
        else:
            lines.append("Handoff: 无")

        lines.append(f"Progress: {progress_lines} lines")

        # Extract key sections from handoff
        if handoff_content:
            lines.append("")
            lines.append("--- Handoff 摘要 ---")
            handoff_sections = _extract_sections(
                handoff_content,
                ["Current Goal", "Phase Just Ended", "Pending", "Next", "Risks", "Blockers"],
                max_lines=12,
            )
            if handoff_sections:
                lines.append(handoff_sections)
            else:
                # Fallback: first 500 chars if no recognized sections
                lines.append(handoff_content[:500] + ("..." if len(handoff_content) > 500 else ""))

        # Extract key sections from progress
        if progress_active:
            lines.append("")
            lines.append("--- Progress 摘要 ---")
            progress_sections = _extract_sections(
                progress_active,
                ["Pinned", "Decisions"],
                max_lines=10,
            )
            if progress_sections:
                lines.append(progress_sections)
            else:
                lines.append(progress_active[:500] + ("..." if len(progress_active) > 500 else ""))

        if not handoff_content and not progress_active:
            lines.append("")
            lines.append("(无可用的上下文数据)")

        return "\n".join(lines)

    @staticmethod
    def _format_memory_status(data: dict) -> str:
        def file_line(label: str, info: dict) -> str:
            if not info or not info.get("exists"):
                return f"- {label}: missing"
            lines = info.get("lines", 0)
            return f"- {label}: {lines} lines, {_time_label(info)}"

        lines = ["=== 记忆状态检查 ==="]
        noise = data.get("noise_assessment", "unknown")
        archive_candidates = data.get("archive_candidates_count", 0)
        lines.append(f"噪声评估: {noise}")
        lines.append(f"归档候选: {archive_candidates}")
        lines.append("")
        lines.append("文件:")
        lines.append(file_line("progress.md", data.get("progress", {})))

        handoff = data.get("latest_handoff", {})
        if handoff.get("exists"):
            handoff_name = handoff.get("name", "handoff-YYYY-MM-DD.md")
            lines.append(file_line(handoff_name, handoff))
        else:
            lines.append("- handoff-YYYY-MM-DD.md: missing")

        lines.append(file_line("memory-index.md", data.get("memory_index", {})))
        lines.append(file_line("detector-learning-log.md", data.get("detector_learning_log", {})))
        lines.append(file_line("skills-audit.md", data.get("skills_audit", {})))
        lines.append(file_line("progress.archive.md", data.get("progress_archive", {})))

        gaps = data.get("gaps") or []
        if gaps:
            lines.append("")
            lines.append("缺口:")
            for gap in gaps[:5]:
                lines.append(f"- {gap}")

        _REC_TRANSLATIONS: dict[str, str] = {
            "Archive stale completed items soon.": "有归档候选条目，建议尽快归档。",
            "Run /记忆归档 with Codex/progress-recorder before major new work.": "噪声较高，建议在开始新工作前执行 /记忆归档。",
            "No immediate archive required.": "当前无需归档。",
            "Resolve missing memory files or confirm they are intentionally absent.": "部分记忆文件缺失，请确认是否需要补建。",
        }

        recs = data.get("recommendations") or []
        if recs:
            lines.append("")
            lines.append("建议:")
            for rec in recs[:4]:
                display_rec = _REC_TRANSLATIONS.get(rec, rec)
                lines.append(f"- {display_rec}")

        lines.append("")
        lines.append("只读检查完成；未写入或修改任何记忆文件。")
        return "\n".join(lines)

    @staticmethod
    def _format_archive_candidates(data: dict) -> str:
        progress_lines = data.get("progress_lines", 0)
        progress_modified = data.get("progress_last_modified") or "?"
        archive_lines = data.get("archive_lines", 0)
        archive_modified = data.get("archive_last_modified") or "?"
        candidates = data.get("candidates") or []
        candidate_count = data.get("candidate_count", 0)
        recommendation = data.get("recommendation", "")

        lines = ["=== 记忆归档检查 ==="]
        lines.append(f"progress.md: {progress_lines} lines (updated {progress_modified})")
        lines.append(f"progress.archive.md: {archive_lines} lines (updated {archive_modified})")
        lines.append(f"归档候选: {candidate_count}")

        if candidates:
            lines.append("")
            lines.append("候选条目:")
            for idx, c in enumerate(candidates, 1):
                ln = c.get("line_number", "?")
                text = c.get("text", "")
                reason = c.get("reason", "")
                reason_label = reason
                if reason.startswith("stale"):
                    # "stale (17 days)" -> "stale 17天"
                    m = re.search(r"\((\d+) days?\)", reason)
                    if m:
                        reason_label = f"stale {m.group(1)}天"
                lines.append(f"{idx}. [L{ln}] {text} [{reason_label}]")

        if recommendation:
            # Translate known English recommendation patterns
            if recommendation == "No archive candidates found":
                rec_display = "当前无归档候选条目"
            else:
                m = re.match(r"^(\d+) items can be archived to progress\.archive\.md$", recommendation)
                if m:
                    rec_display = f"{m.group(1)} 个条目可归档到 progress.archive.md"
                else:
                    rec_display = recommendation
            lines.append("")
            lines.append(f"建议: {rec_display}")

        if candidates:
            lines.append("")
            lines.append("如需执行归档，把下面发给 Codex / progress-recorder：")
            lines.append("")
            lines.append("请将 progress.md 中以下条目移入 progress.archive.md：")
            for c in candidates:
                ln = c.get("line_number", "?")
                text = c.get("text", "")
                # Truncate for the instruction
                if len(text) > 80:
                    text = text[:80] + "..."
                lines.append(f"- L{ln}: {text}")
            lines.append("保留摘要引用，不删除活跃条目。")
        elif not recommendation or recommendation == "No archive candidates found":
            pass
        else:
            lines.append("如需执行归档，请发给 Codex / progress-recorder。")

        if candidates:
            lines.append("")
            lines.append("回复 /确认归档 执行归档。")

        lines.append("")
        lines.append("只读检查完成；未写入或修改任何记忆文件。")
        return "\n".join(lines)

    @staticmethod
    def _format_record_status(data: dict) -> str:
        today = data.get("today", "?")
        files = data.get("files", {})
        actions = data.get("actions_needed") or []
        recommendation = data.get("recommendation", "")

        lines = ["=== 记忆记录状态 ==="]
        lines.append(f"日期: {today}")
        lines.append("")
        lines.append("文件状态:")

        progress = files.get("progress", {})
        if progress.get("exists"):
            lines.append(f"- progress.md: {progress.get('lines', 0)} lines, {_time_label(progress, today)}")
        else:
            lines.append("- progress.md: missing")

        handoff = files.get("handoff_today", {})
        handoff_name = handoff.get("name", "handoff-YYYY-MM-DD.md")
        if handoff.get("exists"):
            lines.append(f"- {handoff_name}: {handoff.get('lines', 0)} lines, {_time_label(handoff, today)}")
        else:
            if handoff.get("last_modified"):
                lines.append(f"- {handoff_name}: 今日无 (最近: {handoff.get('lines', 0)} lines, {handoff.get('last_modified', '?')})")
            else:
                lines.append(f"- {handoff_name}: missing")

        for key, label in [
            ("memory_index", "memory-index.md"),
            ("detector_log", "detector-learning-log.md"),
            ("skills_audit", "skills-audit.md"),
        ]:
            info = files.get(key, {})
            if info.get("exists"):
                lines.append(f"- {label}: {info.get('lines', 0)} lines, {_time_label(info, today)}")
            else:
                lines.append(f"- {label}: missing")

        _ACTION_MAP = {
            "needs new handoff today": "需要创建今日 handoff",
            "progress.md not updated today": "progress.md 今日未更新",
        }
        _REC_MAP = {
            "all_fresh": "当前记忆文件基本是最新的。如有新进展，建议执行 /record 更新 progress.md 和今日 handoff。",
            "minor_updates": "有少量文件需要更新。建议执行 /record 补齐。",
            "full_refresh": "多个记忆文件需要更新。建议尽快执行 /record 进行全面刷新。",
        }

        if actions:
            lines.append("")
            lines.append("待办:")
            for action in actions:
                display = _ACTION_MAP.get(action)
                if display is None:
                    # e.g. "detector-learning-log.md 1d stale" -> "detector-learning-log.md 1天未更新"
                    m = re.match(r"^(.+?) (\d+)d stale$", action)
                    if m:
                        display = f"{m.group(1)} {m.group(2)}天未更新"
                    else:
                        display = action
                lines.append(f"- {display}")

        if recommendation:
            lines.append("")
            rec_text = _REC_MAP.get(recommendation, recommendation)
            lines.append(f"建议: {rec_text}")

        lines.append("")
        lines.append("只读检查完成；未写入或修改任何记忆文件。")
        return "\n".join(lines)

    @staticmethod
    def _format_review(data: dict) -> str:
        run_id = data.get("run_id", "?")
        status = data.get("status", "unknown")
        stage = data.get("stage", "")
        verdict = data.get("verdict", "UNKNOWN")
        failure_reason = data.get("failure_reason")
        failure_stage = data.get("failure_stage")
        next_step = data.get("next_step")
        task = data.get("task", "")
        summary = data.get("summary", "")

        verdict_icon = {
            "APPROVE": "[APPROVE]",
            "REVISE": "[REVISE]",
            "BLOCK": "[BLOCK]",
        }.get(verdict, f"[{verdict}]")

        lines = [f"=== 审查结果 {verdict_icon} ==="]
        lines.append(f"Run ID: {run_id}")
        lines.append(f"状态: {status}")
        if stage:
            lines.append(f"阶段: {stage}")

        if task:
            lines.append(f"任务: {task[:100]}")

        if status == "running":
            lines.append("\n审查仍在进行中，请稍后再查看。")
            return "\n".join(lines)

        if verdict != "UNKNOWN":
            lines.append(f"判定: {verdict}")

        if failure_reason:
            lines.append(f"失败阶段: {failure_stage or stage or 'unknown'}")
            lines.append(f"失败原因: {failure_reason}")
            if next_step:
                lines.append(f"下一步: {next_step}")

        if summary:
            lines.append("")
            if len(summary) > 1000:
                summary = summary[:1000] + "...(截断)"
            lines.append(summary)

        return "\n".join(lines)


def _extract_sections(content: str, section_names: list[str], max_lines: int = 12) -> str:
    """Extract named ## sections from markdown content."""
    sections: dict[str, list[str]] = {}
    current_name = ""
    current_lines: list[str] = []

    for line in content.split("\n"):
        if line.startswith("## "):
            if current_name:
                sections[current_name] = current_lines
            current_name = line[3:].strip()
            current_lines = []
        elif current_name:
            current_lines.append(line)

    if current_name:
        sections[current_name] = current_lines

    result_parts = []
    for name in section_names:
        if name in sections:
            section_lines = sections[name]
            # Strip trailing empty lines
            while section_lines and not section_lines[-1].strip():
                section_lines.pop()
            if not section_lines:
                continue
            truncated = section_lines[:max_lines]
            result_parts.append(f"## {name}")
            result_parts.extend(truncated)
            if len(section_lines) > max_lines:
                result_parts.append(f"  ...({len(section_lines) - max_lines} lines omitted)")
            result_parts.append("")

    return "\n".join(result_parts).rstrip()


def _short_path(full_path: str, parts: int = 3) -> str:
    p = Path(full_path)
    components = p.parts
    if len(components) <= parts:
        return full_path
    return ".../" + "/".join(components[-parts:])
