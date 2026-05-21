package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

func cmdExecCC(root string, args []string) {
	f := parseExecFlags(args)
	text := strings.TrimSpace(f.text)
	if text == "" || strings.HasPrefix(text, "--") {
		fmt.Fprintln(os.Stderr, `用法：
/cc <消息>        和 CC 连续对话
/cc结果 [RunId]   查看最近结果
/执行 RunId       确认执行型任务
/项目             查看当前项目
/切项目 <名称>    切换工作项目`)
		os.Exit(1)
	}

	// Resolve project: binding (per-chat) > active_project.json (global)
	project := resolveProjectForChat(root, f.platform, f.chatID)
	touchBinding(root, f.platform, f.chatID)

	sessionID := f.session
	if sessionID == "" {
		sessionID = "default"
	}
	sessionID = project.ProjectID + "-" + sessionID
	mode := f.mode
	auto := f.auto
	if auto && mode == "" {
		mode = classifyMode(text).String()
	}

	runID := genRunID("cc-session")
	runDir := filepath.Join(root, "runs", runID)
	os.MkdirAll(runDir, 0755)
	os.WriteFile(filepath.Join(runDir, "incoming-message.txt"), []byte(text), 0644)

	if f.chatID != "" {
		writeFile(filepath.Join(runDir, "runner.chat-id"), bindingKey(f.platform, f.chatID))
	}
	if f.replyProject != "" {
		writeFile(filepath.Join(runDir, "runner.reply-project"), f.replyProject)
	}

	sessionDir := filepath.Join(root, "sessions", sessionID)
	os.MkdirAll(sessionDir, 0755)

	// Create/update session.json
	sessionPath := filepath.Join(sessionDir, "session.json")
	sessionMeta := loadOrCreateSession(sessionPath, sessionID)
	sessionMeta.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	sessionMeta.LastRunID = runID
	sessionMeta.TurnCount++
	sessionMeta.Mode = "cc-session"
	writeJSON(sessionPath, sessionMeta)

	writeStatusJSON(runDir, statusJSON{
		RunID:        runID,
		Kind:         "cc-session",
		Status:       "accepted",
		Stage:        "accepted",
		SessionID:    sessionID,
		SessionScope: "project_default",
		StartedAt:    time.Now().UTC().Format(time.RFC3339),
	})
	appendEvent(runDir, eventEntry{
		Ts:      time.Now().UTC().Format(time.RFC3339),
		RunID:   runID,
		Type:    "accepted",
		Message: "session=" + sessionID + " mode=" + mode,
	})

	// execute_request: generate confirmation card, no background runner
	// MUST return before spawning any runner or calling Claude.
	if mode == "execute_request" {
		logRoute(root, text, "execute_request", "confirm_card", mode, runID, "classifyMode")
		// execute_request: use CC_EXECUTE_WORK_DIR (safe sandbox).
		// Never inherit CC_WORK_DIR (research project dir).
		workDir := os.Getenv("CC_EXECUTE_WORK_DIR")
		if workDir == "" || strings.Contains(workDir, "YOUR_PROJECT_ROOT") {
			fmt.Fprintln(os.Stderr, "错误: CC_EXECUTE_WORK_DIR 未正确配置，请设置为真实目录，例如 E:\\ai\\selfwork_ytl\\test")
			os.WriteFile(filepath.Join(runDir, "summary.md"), []byte("CC_EXECUTE_WORK_DIR 未正确配置，请设置为真实目录，例如 E:\\ai\\selfwork_ytl\\test"), 0644)
			updateStatusJSON(runDir, "failed", "config_error", 0)
			return
		}
		if _, err := os.Stat(workDir); err != nil {
			fmt.Fprintf(os.Stderr, "错误: CC_EXECUTE_WORK_DIR 目录不存在: %s\n", workDir)
			os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(fmt.Sprintf("CC_EXECUTE_WORK_DIR 目录不存在: %s", workDir)), 0644)
			updateStatusJSON(runDir, "failed", "config_error", 0)
			return
		}
		writeFile(filepath.Join(runDir, "runner.workdir"), workDir)
		confirmMsg := fmt.Sprintf(`[执行确认] (RunId: %s)

工作目录: %s（CC_EXECUTE_WORK_DIR）
权限:     --dangerously-skip-permissions（完整文件读写 + 命令执行）
文件范围: 仅限沙盒目录
回滚建议: 执行前建议先 git commit 保存当前状态；执行后可用 git diff 查看改动

确认执行:
  /执行 %s`, runID, workDir, runID)
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(confirmMsg), 0644)
		updateStatusJSON(runDir, "confirming", "awaiting_confirmation", 0)
		appendEvent(runDir, eventEntry{
			Ts:    time.Now().UTC().Format(time.RFC3339),
			RunID: runID,
			Type:  "confirming",
		})
		sendCallback(runDir, confirmMsg)
		queueAdd(root, runID, workDir, text)
		fmt.Printf("已生成执行确认 (Run ID: %s), 工作目录: %s\n", runID, workDir)
		return
	}

	// Result location queries: "结果在哪/文件在哪/输出在哪" → local handler
	if isResultLocationQuery(text) {
		os.WriteFile(filepath.Join(runDir, "is-status-query"), []byte("1"), 0644)
		logRoute(root, text, "result_location", "local_handler", mode, runID, "result_location")
		result := formatResultLocation(root, runID)
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(result), 0644)
		updateStatusJSON(runDir, "completed", "done", 0)
		setExitCode(runDir, 0)
		appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), RunID: runID, Type: "completed", ExitCode: 0})
		sendCallback(runDir, "[CC] "+result)
		fmt.Printf("已返回结果位置查询 (Run ID: %s)\n", runID)
		return
	}

	// Native status queries: handle in Go, skip Claude entirely.
	// isStatusQuery is more specific than classifyMode — if it matches,
	// always handle natively regardless of mode (execute already returned above).
	if isStatusQuery(text) {
		// Mark as status query so formatLatestStatus skips it
		os.WriteFile(filepath.Join(runDir, "is-status-query"), []byte("1"), 0644)
		detector := "status"
		if isMDQuery(text) {
			detector = "md_status"
		} else if isResearchQuery(text) {
			detector = "research_status"
		} else if isSystemStatusQuery(text) {
			detector = "system_status"
		}
		logRoute(root, text, "status_query", "local_handler", mode, runID, detector)
		result := resolveAndFormatStatus(root, runID, text)
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(result), 0644)
		updateStatusJSON(runDir, "completed", "done", 0)
		setExitCode(runDir, 0)
		appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), RunID: runID, Type: "completed", ExitCode: 0})
		sendCallback(runDir, "[CC] " + result)
		fmt.Printf("已返回本地状态查询 (Run ID: %s)\n", runID)
		return
	}

	var runnerWorkDir string
	if mode == "advice" || mode == "readonly" {
		runnerWorkDir = project.WorkDir
	}
	if runnerWorkDir != "" {
		writeFile(filepath.Join(runDir, "runner.workdir"), runnerWorkDir)
	}

	logRoute(root, text, mode, "claude_runner", mode, runID, "classifyMode")

	// Spawn background runner with session info and mode
	exe, _ := os.Executable()
	runner := exec.Command(exe, "run-cc", runID, "--session", sessionID, "--mode", mode)
	runner.Dir = root
	runner.Stdin = nil
	runner.Stdout = nil
	runner.Stderr = nil
	runner.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := runner.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start runner: %s\n", err)
		os.Exit(1)
	}
	writeFile(filepath.Join(runDir, "runner.pid"), fmt.Sprintf("%d", runner.Process.Pid))
	updateStatusJSON(runDir, "running", mode+"_running", runner.Process.Pid)

	preview := sanitizeQuestionPreview(text)
	sendCallback(runDir, fmt.Sprintf("收到，处理中...\n问题: %s", preview))

	fmt.Printf("已开始 CC 处理 (Run ID: %s, Session: %s, Mode: %s)\n", runID, sessionID, mode)
}

func cmdExecute(root, runID string) {
	if runID == "" || strings.ContainsAny(runID, "<>") {
		fmt.Fprintln(os.Stderr, "请使用 /执行 RunId（不要输入尖括号）")
		os.Exit(1)
	}
	runDir := filepath.Join(root, "runs", runID)
	data, err := os.ReadFile(filepath.Join(runDir, "status.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Run not found: %s\n", runID)
		os.Exit(1)
	}
	var s statusJSON
	if json.Unmarshal(data, &s) != nil {
		fmt.Fprintf(os.Stderr, "Invalid status.json for: %s\n", runID)
		os.Exit(1)
	}

	sandboxDir := ""
	if data, err := os.ReadFile(filepath.Join(runDir, "runner.workdir")); err == nil {
		sandboxDir = strings.TrimSpace(string(data))
	}
	sandboxEnv := os.Getenv("CC_EXECUTE_WORK_DIR")
	if sandboxEnv != "" && sandboxDir != "" && sandboxDir != sandboxEnv {
		fmt.Fprintf(os.Stderr, "警告: runner.workdir (%s) 与 CC_EXECUTE_WORK_DIR (%s) 不一致\n", sandboxDir, sandboxEnv)
	}
	if sandboxEnv == "" && sandboxDir == "" {
		fmt.Fprintln(os.Stderr, "警告: CC_EXECUTE_WORK_DIR 未设置，执行将使用默认目录")
	}

	exe, _ := os.Executable()
	runner := exec.Command(exe, "run-cc", runID, "--session", s.SessionID, "--mode", "execute")
	runner.Dir = root
	runner.Stdin = nil
	runner.Stdout = nil
	runner.Stderr = nil
	runner.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := runner.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start executor: %s\n", err)
		os.Exit(1)
	}
	writeFile(filepath.Join(runDir, "runner.pid"), fmt.Sprintf("%d", runner.Process.Pid))
	updateStatusJSON(runDir, "running", "executing", runner.Process.Pid)

	fmt.Printf("已开始执行 (Run ID: %s)\n", runID)
}

// buildExecuteSummary runs git status/diff in workDir and returns a
// formatted summary of changes. Errors are silently returned as text
// (not fatal — the workflow continues without the summary).
func buildExecuteSummary(workDir string) string {
	if workDir == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n--- 执行摘要 ---\n")

	// git status --short
	statusCmd := exec.Command("git", "status", "--short")
	statusCmd.Dir = workDir
	if out, err := statusCmd.Output(); err == nil && len(out) > 0 {
		sb.WriteString("修改文件:\n")
		sb.WriteString(string(out))
	} else if err != nil {
		sb.WriteString("git status: 未初始化 git 仓库或 git 不可用\n")
	} else {
		sb.WriteString("修改文件: 无（工作区干净）\n")
	}

	// git diff --stat
	diffCmd := exec.Command("git", "diff", "--stat")
	diffCmd.Dir = workDir
	if out, err := diffCmd.Output(); err == nil && len(out) > 0 {
		sb.WriteString("\n差异统计:\n")
		sb.WriteString(string(out))
	}

	sb.WriteString("\n验证建议:\n")
	sb.WriteString("  1. 用 git diff 查看具体改动\n")
	sb.WriteString("  2. 运行测试确认功能正常\n")
	sb.WriteString("  3. 如需回滚: git checkout -- <文件>\n")

	return sb.String()
}

type execFlags struct {
	text         string
	session      string
	mode         string
	replyProject string
	chatID       string
	platform     string
	auto         bool
}

func parseExecFlags(args []string) execFlags {
	var f execFlags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--text":
			i++
			if i >= len(args) {
				return f
			}
			parts := []string{args[i]}
			i++
			for i < len(args) && !strings.HasPrefix(args[i], "--") {
				parts = append(parts, args[i])
				i++
			}
			i--
			f.text = strings.Join(parts, " ")
		case "--session":
			i++
			if i < len(args) {
				f.session = args[i]
			}
		case "--auto":
			f.auto = true
		case "--mode":
			i++
			if i < len(args) {
				f.mode = args[i]
			}
		case "--reply-project":
			i++
			if i < len(args) {
				f.replyProject = args[i]
			}
		case "--chat-id":
			i++
			if i < len(args) {
				f.chatID = args[i]
			}
		case "--platform":
			i++
			if i < len(args) {
				f.platform = args[i]
			}
		}
	}
	return f
}

// statusQueryTrigger: compound phrases that unambiguously indicate status intent.
var statusQueryTrigger = []string{
	"查看状态", "查看运行", "查看进度",
	"最新状态", "最新运行", "运行状态", "运行结果",
	"cc状态", "任务状态", "任务进度",
	"继续", "进度如何", "go on", "goon",
	"看看结果", "结果如何", "任务怎么样了",
	"运行情况", "跑到哪了", "看看进度", "看看状态",
	"现在怎么样了", "进展怎么样了",
	"md进度", "模拟进度", "md状态", "md跑到哪", "模拟跑到哪",
	"gromacs进度", "gromacs状态", "md怎么样",
	"科研任务", "科研监控", "科研状态", "科研结果",
	"研究任务", "研究状态", "研究进度",
	"任务监控", "运行监控",
	"系统状态", "controller状态", "当前状态",
	"任务结果", "模拟结果",
	"跑完了吗", "完成了吗", "还在跑吗",
	"项目状态", "md任务",
	"动力学跑到哪", "轨迹出来",
	"gromacs现在怎么样",
	"模拟结束", "模拟完成", "动力学完成",
	"gromacs还在",
	"haddock状态", "haddock进度", "haddock监控",
	"schrodinger状态", "schrodinger进度",
	"maestro状态", "maestro进度", "glide状态",
	"rosetta状态", "rosetta进度",
	"vina状态", "vina进度",
	"autodock状态", "autodock进度",
	"alphafold状态", "alphafold进度",
	"colabfold状态", "colabfold进度",
	"蛋白折叠状态", "蛋白折叠进度",
	"amber状态", "amber进度",
	"openmm状态", "openmm进度",
	"gaussian状态", "gaussian进度",
	"g16状态", "g16进度",
	"g09状态", "g09进度",
	"python状态", "python进度",
}

// ultraShortStatusTrigger: single broad words that only match when the
// entire text is very short (≤8 runes). "状态" alone → local handler;
// "帮我总结一下当前项目状态" → Claude advice.
var ultraShortStatusTrigger = []string{
	"状态", "进度",
}

const ultraShortMaxRunes = 8

// adviceOverridePatterns: when these appear, the user wants Claude to
// analyze/summarize, not a simple status lookup.
var adviceOverridePatterns = []string{
	"帮我总结", "总结一下", "分析一下", "解释一下",
	"帮我分析", "帮我解释", "帮我看看",
	"详细说明", "说明一下",
}

func hasAdviceOverride(s string) bool {
	for _, p := range adviceOverridePatterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func isStatusQuery(text string) bool {
	lower := strings.ToLower(text)
	if hasQuestionPattern(lower) {
		return false
	}
	if hasAdviceOverride(lower) {
		return false
	}
	for _, s := range statusQueryTrigger {
		if strings.Contains(lower, s) {
			return true
		}
	}
	if len([]rune(strings.TrimSpace(lower))) <= ultraShortMaxRunes {
		for _, s := range ultraShortStatusTrigger {
			if strings.Contains(lower, s) {
				return true
			}
		}
	}
	return false
}

var mdQueryTrigger = []string{
	"md进度", "模拟进度", "md状态", "md跑到哪", "模拟跑到哪",
	"gromacs进度", "gromacs状态", "md怎么样",
	"md跑完", "md任务", "动力学跑到哪", "轨迹出来",
	"gromacs现在怎么样",
	"md还在跑", "模拟结束", "模拟完成", "动力学完成",
	"gromacs还在",
}

func isMDQuery(text string) bool {
	lower := strings.ToLower(text)
	for _, s := range mdQueryTrigger {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

var researchQueryTrigger = []string{
	"科研任务", "科研监控", "科研状态", "科研结果",
	"研究任务", "研究状态", "研究进度",
	"任务监控", "运行监控",
	"任务结果", "模拟结果",
	"haddock状态", "haddock进度", "haddock监控",
	"schrodinger状态", "schrodinger进度",
	"maestro状态", "maestro进度", "glide状态",
	"rosetta状态", "rosetta进度",
	"gromacs状态", "gromacs进度",
	"vina状态", "vina进度",
	"autodock状态", "autodock进度",
	"alphafold状态", "alphafold进度",
	"colabfold状态", "colabfold进度",
	"蛋白折叠状态", "蛋白折叠进度",
	"amber状态", "amber进度",
	"openmm状态", "openmm进度",
	"gaussian状态", "gaussian进度",
	"g16状态", "g16进度",
	"g09状态", "g09进度",
	"python状态", "python进度",
}

var detectorKeywords = []struct {
	keyword  string
	detector string
}{
	{"haddock", "haddock3"},
	{"schrodinger", "schrodinger"},
	{"maestro", "schrodinger"},
	{"glide", "schrodinger"},
	{"rosetta", "rosetta"},
	{"pyrosetta", "rosetta"},
	{"gromacs", "gromacs"},
	{"vina", "autodock_vina"},
	{"autodock", "autodock_vina"},
	{"autogrid", "autodock_vina"},
	{"python", "python_pipeline"},
	{"alphafold", "alphafold"},
	{"colabfold", "alphafold"},
	{"蛋白折叠", "alphafold"},
	{"amber", "amber_openmm"},
	{"openmm", "amber_openmm"},
	{"pmemd", "amber_openmm"},
	{"sander", "amber_openmm"},
	{"gaussian", "gaussian"},
	{"g16", "gaussian"},
	{"g09", "gaussian"},
	{"gjf", "gaussian"},
}

func extractDetectorKeyword(text string) string {
	lower := strings.ToLower(text)
	for _, dk := range detectorKeywords {
		if strings.Contains(lower, dk.keyword) {
			return dk.detector
		}
	}
	return ""
}

func isResearchQuery(text string) bool {
	lower := strings.ToLower(text)
	for _, s := range researchQueryTrigger {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

var systemStatusTrigger = []string{
	"系统状态", "controller状态", "当前状态",
}

func isSystemStatusQuery(text string) bool {
	lower := strings.ToLower(text)
	for _, s := range systemStatusTrigger {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

var resultLocationTrigger = []string{
	"结果在哪", "文件在哪", "输出在哪", "保存到哪", "保存在哪",
	"生成了什么文件", "产出在哪", "报告在哪",
	"结果放在哪", "文件放在哪", "输出放在哪",
}

func isResultLocationQuery(text string) bool {
	lower := strings.ToLower(text)
	for _, s := range resultLocationTrigger {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func formatResultLocation(root, currentRunID string) string {
	sessionID := currentSessionID(root)
	latest := findLatestMeaningfulRunForSession(filepath.Join(root, "runs"), currentRunID, sessionID)
	if latest == "" {
		return "暂无运行记录，无法定位结果文件"
	}
	latestDir := filepath.Join(root, "runs", latest)
	s := readStatusJSON(latestDir)

	var sb strings.Builder
	fmt.Fprintf(&sb, "最新任务: %s\n", latest)
	fmt.Fprintf(&sb, "状态: %s\n", humanStatus(s.Status))
	fmt.Fprintf(&sb, "目录: %s\n", latestDir)

	outputFiles := []string{
		"cc-answer.md", "codex-answer.md", "summary.md",
		"incoming-question.md", "incoming-message.txt",
	}
	var found []string
	for _, name := range outputFiles {
		p := filepath.Join(latestDir, name)
		if fi, err := os.Stat(p); err == nil && fi.Size() > 0 {
			found = append(found, name)
		}
	}
	if len(found) > 0 {
		fmt.Fprintf(&sb, "输出文件: %s\n", strings.Join(found, ", "))
	} else {
		sb.WriteString("输出文件: 暂无\n")
	}

	if data, err := os.ReadFile(filepath.Join(latestDir, "runner.workdir")); err == nil {
		sb.WriteString("工作目录: " + strings.TrimSpace(string(data)) + "\n")
	}

	sb.WriteString("\n详情: 查看 " + latest)
	return sb.String()
}

func resolveAndFormatStatus(root, currentRunID, text string) string {
	// isMDQuery routes to gromacs only; alphafold/colabfold are handled by isResearchQuery below
	if isMDQuery(text) {
		return formatResearchStatusByDetector(root, "gromacs")
	}
	if isResearchQuery(text) {
		if det := extractDetectorKeyword(text); det != "" {
			return formatResearchStatusByDetector(root, det)
		}
		return formatResearchStatus(root)
	}
	if isSystemStatusQuery(text) {
		return formatStatusShort(root)
	}
	return formatLatestStatus(root, currentRunID)
}

func formatResearchStatus(root string) string {
	results, runID := loadLatestResults(root)
	if results == nil {
		return "暂无科研监控数据，请先执行 科研监控 获取最新扫描"
	}
	runDir := filepath.Join(root, "runs", runID)
	return formatMobileSummary(results, runDir)
}

var detectorMatchAliases = map[string][]string{
	"alphafold":    {"alphafold", "colabfold"},
	"gromacs":      {"gromacs"},
	"schrodinger":  {"schrodinger"},
	"amber_openmm": {"amber_openmm"},
	"autodock_vina": {"autodock_vina"},
	"gaussian":     {"gaussian"},
}

func detectorMatches(resultDetector, filterDetector string) bool {
	rLower := strings.ToLower(resultDetector)
	aliases := detectorMatchAliases[filterDetector]
	if len(aliases) == 0 {
		aliases = []string{filterDetector}
	}
	for _, alias := range aliases {
		if rLower == alias || strings.Contains(rLower, alias) {
			return true
		}
	}
	return false
}

func formatResearchStatusByDetector(root, detector string) string {
	results, runID := loadLatestResults(root)
	if results == nil {
		return "暂无科研监控数据"
	}
	var filtered []ResearchStatus
	for _, r := range results {
		if detectorMatches(r.Detector, detector) {
			filtered = append(filtered, r)
		}
	}
	label := detectorShortName(detector)
	if len(filtered) == 0 {
		return "暂无 " + label + " 监控数据"
	}
	runDir := filepath.Join(root, "runs", runID)
	return formatMobileSummary(filtered, runDir, label)
}

func humanStatus(status string) string {
	switch status {
	case "accepted", "queued", "pending":
		return "已接收"
	case "running", "in_progress":
		return "运行中"
	case "done", "completed", "success":
		return "已完成"
	case "failed", "error":
		return "失败"
	case "cancelled", "canceled":
		return "已取消"
	case "confirming":
		return "等待确认"
	default:
		if status == "" {
			return "未知"
		}
		return status
	}
}

func formatLatestStatus(root, currentRunID string) string {
	runsRoot := filepath.Join(root, "runs")
	sessionID := currentSessionID(root)
	latest := findLatestMeaningfulRunForSession(runsRoot, currentRunID, sessionID)
	if latest == "" {
		if brief := briefResearchSummary(root); brief != "" {
			return brief + "\n详情: 科研监控"
		}
		return "暂无运行记录"
	}
	latestDir := filepath.Join(runsRoot, latest)
	s := readStatusJSON(latestDir)

	var sb strings.Builder
	fmt.Fprintf(&sb, "最新任务: %s\n", latest)
	fmt.Fprintf(&sb, "状态: %s\n", humanStatus(s.Status))
	if s.Stage != "" && s.Stage != s.Status {
		fmt.Fprintf(&sb, "阶段: %s\n", s.Stage)
	}

	for _, name := range []string{"incoming-question.md", "incoming-message.txt"} {
		if data, err := os.ReadFile(filepath.Join(latestDir, name)); err == nil {
			q := sanitizeQuestionPreview(strings.TrimSpace(string(data)))
			if q != "" {
				fmt.Fprintf(&sb, "问题: %s\n", q)
			}
			break
		}
	}

	ts := s.UpdatedAt
	if ts == "" {
		ts = s.StartedAt
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		fmt.Fprintf(&sb, "更新: %s\n", humanDuration(t))
	}

	if brief := briefResearchSummary(root); brief != "" {
		sb.WriteString("\n")
		sb.WriteString(brief)
	}

	sb.WriteString("\n详情: 查看 / 科研状态")
	return sb.String()
}

func findLatestMeaningfulRun(runsRoot, excludeID string) string {
	return findLatestMeaningfulRunForSession(runsRoot, excludeID, "")
}

func findLatestMeaningfulRunForSession(runsRoot, excludeID, sessionID string) string {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return ""
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() || !runIDPattern.MatchString(e.Name()) || e.Name() == excludeID {
			continue
		}
		if _, err := os.Stat(filepath.Join(runsRoot, e.Name(), "is-status-query")); err == nil {
			continue
		}
		runDir := filepath.Join(runsRoot, e.Name())
		s := readStatusJSON(runDir)
		if s.Status == "confirming" {
			continue
		}
		if sessionID != "" && s.SessionID != "" && s.SessionID != sessionID {
			continue
		}
		dirs = append(dirs, e.Name())
	}
	if len(dirs) == 0 {
		return ""
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i] > dirs[j] })
	return dirs[0]
}

func currentSessionID(root string) string {
	p := readActiveProject(root)
	if p.ProjectID == "" {
		return ""
	}
	return p.ProjectID + "-default"
}

var pastedCommandPrefixes = []string{
	"get-content", "select-object", "set-location", "write-output",
	"powershell", "cmd ", "cmd.exe",
	"cat ", "grep ", "ls ", "cd ", "dir ",
	"git ", "docker ", "npm ", "pip ",
	"c:\\", "C:\\", "g:\\", "G:\\",
}

func looksLikePastedCommand(q string) bool {
	lower := strings.ToLower(q)
	for _, p := range pastedCommandPrefixes {
		if strings.HasPrefix(lower, strings.ToLower(p)) {
			return true
		}
	}
	if strings.Contains(q, "\\>") || strings.Contains(q, "$ ") {
		return true
	}
	return false
}

func sanitizeQuestionPreview(q string) string {
	if hasPastedOutput(q) {
		return "命令输出诊断"
	}
	if looksLikePastedCommand(q) {
		return "命令/路径查询"
	}
	if len(q) > 80 {
		q = q[:80] + "..."
	}
	return q
}

func briefResearchSummary(root string) string {
	results, _ := loadLatestResults(root)
	if len(results) == 0 {
		return ""
	}
	detectorCounts := map[string][2]int{} // [total, running]
	for _, r := range results {
		if r.Bucket != "" {
			continue // skip archived/historical
		}
		name := detectorLabel(r.Detector)
		counts := detectorCounts[name]
		counts[0]++
		if r.State == "running" {
			counts[1]++
		}
		detectorCounts[name] = counts
	}
	if len(detectorCounts) == 0 {
		return ""
	}
	var parts []string
	for name, c := range detectorCounts {
		if c[1] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d个(%d运行中)", name, c[0], c[1]))
		} else {
			parts = append(parts, fmt.Sprintf("%s %d个", name, c[0]))
		}
	}
	return "科研任务: " + strings.Join(parts, ", ")
}

func detectorLabel(d string) string {
	switch d {
	case "gromacs":
		return "GROMACS"
	case "schrodinger":
		return "Schrödinger"
	case "haddock3":
		return "HADDOCK3"
	case "rosetta":
		return "Rosetta"
	case "python_pipeline":
		return "Python"
	case "r_pipeline":
		return "R"
	case "autodock_vina":
		return "AutoDock/Vina"
	case "alphafold":
		return "AlphaFold/ColabFold"
	case "amber_openmm":
		return "Amber/OpenMM"
	case "gaussian":
		return "Gaussian"
	case "docker":
		return "Docker"
	default:
		return d
	}
}

type routeLog struct {
	Ts          string `json:"ts"`
	TextPreview string `json:"text_preview"`
	TextHash    string `json:"text_hash"`
	Intent      string `json:"intent"`
	Handler     string `json:"handler"`
	Mode        string `json:"mode"`
	RunID       string `json:"run_id"`
	Topic       string `json:"topic"`
	Detector    string `json:"detector,omitempty"`
}

func logRoute(root, rawText, intent, handler, mode, runID, detector string) {
	preview := rawText
	if len(preview) > 80 {
		preview = preview[:80] + "..."
	}
	topic := "general"
	switch detector {
	case "md_status":
		topic = "md"
	case "research_status":
		topic = "research"
	case "system_status":
		topic = "system"
	case "status":
		topic = "latest_status"
	case "result_location":
		topic = "result_location"
	}
	if intent == "execute_request" {
		topic = "execute"
	}
	if handler == "claude_runner" && mode == "advice" {
		topic = "advice"
	}

	h := sha256.Sum256([]byte(rawText))
	entry := routeLog{
		Ts:          time.Now().UTC().Format(time.RFC3339),
		TextPreview: preview,
		TextHash:    fmt.Sprintf("sha256:%.8x", h),
		Intent:      intent,
		Handler:     handler,
		Mode:        mode,
		RunID:       runID,
		Topic:       topic,
		Detector:    detector,
	}
	data, _ := json.Marshal(entry)
	path := filepath.Join(root, "runs", "route.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(data)
	f.Write([]byte("\n"))
}

func loadOrCreateSession(path, sessionID string) sessionMeta {
	if data, err := os.ReadFile(path); err == nil {
		var m sessionMeta
		if json.Unmarshal(data, &m) == nil {
			return m
		}
	}
	return sessionMeta{
		SessionID:    sessionID,
		SessionScope: "project_default",
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		TurnCount:    0,
	}
}

func buildSessionContext(root, runID, sessionID string, currentText string) string {
	sessionDir := filepath.Join(root, "sessions", sessionID)
	transcriptPath := filepath.Join(sessionDir, "transcript.jsonl")

	var turns []transcriptEntry
	totalBytes := 0
	if data, err := os.ReadFile(transcriptPath); err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		start := len(lines) - (maxContextTurns * 2)
		if start < 0 {
			start = 0
		}
		for i := start; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			var e transcriptEntry
			if json.Unmarshal([]byte(line), &e) == nil {
				entrySize := len(e.Role) + len(e.Text)
				if totalBytes+entrySize > maxContextBytes {
					break
				}
				turns = append(turns, e)
				totalBytes += entrySize
			}
		}
	}

	var sb strings.Builder
	if len(turns) > 0 {
		sb.WriteString("--- 会话上下文（最近对话）---\n")
		for _, t := range turns {
			sb.WriteString(t.Role)
			sb.WriteString(": ")
			sb.WriteString(t.Text)
			sb.WriteString("\n")
		}
		sb.WriteString("\n--- 当前消息 ---\n")
	}
	sb.WriteString("user: ")
	sb.WriteString(currentText)
	return sb.String()
}
