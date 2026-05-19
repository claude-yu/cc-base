package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func cmdExecCC(root string, args []string) {
	text, sessionID, mode, auto := parseExecFlags(args)
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "--") {
		fmt.Fprintln(os.Stderr, `用法：
/cc <消息>        和 CC 连续对话
/cc结果 [RunId]   查看最近结果
/执行 RunId       确认执行型任务
/项目             查看当前项目
/切项目 <名称>    切换工作项目`)
		os.Exit(1)
	}
	if sessionID == "" {
		sessionID = "default"
	}
	sessionID = resolveSessionID(root, sessionID)
	if auto && mode == "" {
		mode = classifyMode(text).String()
	}

	runID := genRunID("cc-session")
	runDir := filepath.Join(root, "runs", runID)
	os.MkdirAll(runDir, 0755)
	os.WriteFile(filepath.Join(runDir, "incoming-message.txt"), []byte(text), 0644)

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
		// execute_request: use CC_EXECUTE_WORK_DIR (safe sandbox).
		// Never inherit CC_WORK_DIR (research project dir).
		workDir := os.Getenv("CC_EXECUTE_WORK_DIR")
		if workDir == "" {
			workDir = "YOUR_PROJECT_ROOT\\test"
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
		fmt.Printf("已生成执行确认 (Run ID: %s), 工作目录: %s\n", runID, workDir)
		return
	}

	// Native status queries: handle in Go, skip Claude entirely.
	if mode == "readonly" && isStatusQuery(text) {
		result := formatLatestStatus(root, runID)
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(result), 0644)
		updateStatusJSON(runDir, "completed", "done", 0)
		setExitCode(runDir, 0)
		appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), RunID: runID, Type: "completed", ExitCode: 0})
		sendCallback(runDir, "[CC] " + result)
		fmt.Printf("已返回本地状态查询 (Run ID: %s)\n", runID)
		return
	}

	// Resolve workdir for the runner based on mode.
	//   advice → project dir (active_project.json / CC_WORK_DIR)
	//   readonly (project query, not status) → project dir
	//   execute → handled above via execute_request
	var runnerWorkDir string
	if mode == "advice" || mode == "readonly" {
		runnerWorkDir = readActiveProject(root).WorkDir
	}
	if runnerWorkDir != "" {
		writeFile(filepath.Join(runDir, "runner.workdir"), runnerWorkDir)
	}

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
	if sandboxEnv == "" {
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

func parseExecFlags(args []string) (text, session, mode string, auto bool) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--text":
			i++
			if i >= len(args) {
				return "", session, mode, auto
			}
			parts := []string{args[i]}
			i++
			for i < len(args) && !strings.HasPrefix(args[i], "--") {
				parts = append(parts, args[i])
				i++
			}
			i--
			text = strings.Join(parts, " ")
		case "--session":
			i++
			if i < len(args) {
				session = args[i]
			}
		case "--auto":
			auto = true
		case "--mode":
			i++
			if i < len(args) {
				mode = args[i]
			}
		}
	}
	return
}

// isStatusQuery detects queries about controller/runs status that can be
// handled natively in Go without spawning Claude.
var statusQueryTrigger = []string{
	"查看状态", "查看运行", "查看进度",
	"最新状态", "最新运行", "运行状态", "运行结果",
	"cc状态", "任务状态", "任务进度",
	"继续", "进度如何", "go on", "goon",
}

func isStatusQuery(text string) bool {
	lower := strings.ToLower(text)
	for _, s := range statusQueryTrigger {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// formatLatestStatus reads the latest run and formats a human-readable summary.
func formatLatestStatus(root, currentRunID string) string {
	latest := findLatestRun(filepath.Join(root, "runs"))
	if latest == "" {
		return "暂无运行记录"
	}
	latestDir := filepath.Join(root, "runs", latest)
	status := runStatus(latestDir)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("最新运行: %s\n状态: %s", latest, status))

	if data, err := os.ReadFile(filepath.Join(latestDir, "summary.md")); err == nil && len(data) > 0 {
		sb.WriteString("\n\n概要:\n")
		sb.WriteString(string(data))
	}
	if data, err := os.ReadFile(filepath.Join(latestDir, "status.json")); err == nil {
		sb.WriteString("\n\n详情:\n")
		sb.WriteString(string(data))
	}
	return sb.String()
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
