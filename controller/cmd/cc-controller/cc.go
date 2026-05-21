package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func resolveClaudeCmd() string {
	if cmd := os.Getenv("CLAUDE_CMD"); cmd != "" {
		return cmd
	}
	candidates := []string{
		filepath.Join(os.Getenv("USERPROFILE"), ".claude", "bin", "claude.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "claude", "claude.exe"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("claude"); err == nil {
		return p
	}
	return "claude"
}

func runCC(root, runID, sessionID, mode string) {
	runDir := filepath.Join(root, "runs", runID)
	writeFile(filepath.Join(runDir, "runner.pid"), fmt.Sprintf("%d", os.Getpid()))
	updateStatusJSON(runDir, "running", mode+"_running", os.Getpid())

	questionPath := filepath.Join(runDir, "incoming-question.md")
	if _, err := os.Stat(questionPath); os.IsNotExist(err) {
		questionPath = filepath.Join(runDir, "incoming-message.txt")
	}
	question, err := os.ReadFile(questionPath)
	if err != nil {
		writeError(runDir, err)
		return
	}

	claudeCmd := resolveClaudeCmd()

	workDir := readActiveProject(root).WorkDir
	if data, err := os.ReadFile(filepath.Join(runDir, "runner.workdir")); err == nil {
		if wd := strings.TrimSpace(string(data)); wd != "" {
			workDir = wd
		}
	}

	modePrompt := buildModePrompt(mode)

	projectContext := ""
	if isProjectContextQuery(string(question)) {
		projectContext = buildProjectMemoryContext(root, workDir)
	}

	var fullInput string
	if sessionID != "" {
		sessionNote := "\n\n注意：你当前运行在 cc-controller session-aware runner 中。当前 session 的最近对话会被注入上下文，所以你可以记住本 session 内前文。但这不是永久记忆；不要说「新会话不会保留」作为默认免责声明。只有用户要求永久记忆时，再建议写入 CLAUDE.md / memory。"
		context := buildSessionContext(root, runID, sessionID, string(question))
		fullInput = modePrompt + sessionNote + projectContext + "\n\n" + context
	} else {
		fullInput = modePrompt + projectContext + "\n\nuser: " + string(question)
	}

	var claudeArgs []string
	if mode == "execute" {
		claudeArgs = []string{"-p", "--dangerously-skip-permissions", "--output-format", "text", "--no-session-persistence"}
	} else {
		claudeArgs = []string{"-p", "--dangerously-skip-permissions", "--system-prompt", modePrompt, "--output-format", "text", "--no-session-persistence"}
	}
	if m := os.Getenv("CC_MODEL"); m != "" {
		claudeArgs = append(claudeArgs, "--model", m)
	}
	cmd := exec.Command(claudeCmd, claudeArgs...)
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewReader([]byte(fullInput))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan struct{})
	go func() {
		initialDelay := 90 * time.Second
		if mode == "execute" {
			initialDelay = 30 * time.Second
		}
		timer := time.NewTimer(initialDelay)
		defer timer.Stop()
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		start := time.Now()
		lastOutLen := 0
		sendHeartbeat := func() {
			elapsed := int(time.Since(start).Seconds())
			outLen := stdout.Len() + stderr.Len()
			extra := "工作目录: " + workDir
			if outLen > lastOutLen {
				extra += fmt.Sprintf("\n输出: %d 字节 (活跃)", outLen)
			} else if outLen > 0 {
				extra += fmt.Sprintf("\n输出: %d 字节 (等待中)", outLen)
			}
			lastOutLen = outLen
			appendEvent(runDir, eventEntry{
				Ts:         time.Now().UTC().Format(time.RFC3339),
				RunID:      runID,
				Type:       "heartbeat",
				Stage:      mode + "_running",
				ElapsedSec: elapsed,
			})
			sendCallback(runDir, heartbeatMsg("CC", runID, elapsed, extra))
		}
		for {
			select {
			case <-timer.C:
				sendHeartbeat()
			case <-ticker.C:
				sendHeartbeat()
			case <-done:
				return
			}
		}
	}()

	err = cmd.Run()
	close(done)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	rawAnswer := strings.TrimSpace(stdout.String())

	os.WriteFile(filepath.Join(runDir, "cc-answer.raw.md"), []byte(rawAnswer), 0644)
	os.WriteFile(filepath.Join(runDir, "cc-answer.md"), []byte(rawAnswer), 0644)
	os.WriteFile(filepath.Join(runDir, "cc-answer.exitcode.txt"), []byte(fmt.Sprintf("%d", exitCode)), 0644)

	if exitCode == 0 {
		summaryAnswer := rawAnswer
		if mode == "execute" {
			summaryAnswer += buildExecuteSummary(workDir)
		}
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(summaryAnswer), 0644)
		updateStatusJSON(runDir, "completed", "done", 0)
		setExitCode(runDir, 0)
		appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), RunID: runID, Type: "completed", ExitCode: 0})

		if sessionID != "" {
			appendTranscript(root, sessionID, runID, "user", string(question))
			appendTranscript(root, sessionID, runID, "assistant", rawAnswer)
		}

		var callbackMsg string
		if mode == "execute" {
			callbackMsg = fmt.Sprintf("[CC] 执行完成 (RunId: %s)\n工作目录: %s\n%s", runID, workDir, rawAnswer)
			execSummary := buildExecuteSummary(workDir)
			if strings.Contains(execSummary, "修改文件:\n") {
				callbackMsg += "\n文件已修改，用 /cc结果 " + runID + " 查看详情"
			}
		} else if sessionID != "" {
			callbackMsg = fmt.Sprintf("[CC] 已回复 (RunId: %s, Session: %s)\n%s", runID, sessionID, rawAnswer)
		} else {
			callbackMsg = fmt.Sprintf("[CC] 已回复 (RunId: %s)\n%s", runID, rawAnswer)
		}
		sendCallback(runDir, callbackMsg)
	} else {
		errText := rawAnswer
		if errText == "" {
			errText = strings.TrimSpace(stderr.String())
		}
		if errText == "" {
			errText = "(no output)"
		}
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(errText), 0644)
		updateStatusJSON(runDir, "failed", "failed", 0)
		setExitCode(runDir, exitCode)
		appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), RunID: runID, Type: "failed", ExitCode: exitCode})
		sendCallback(runDir, fmt.Sprintf("[CC] 调用失败 (RunId: %s). 检查 Claude CLI / CLAUDE_PROXY / /修复controller", runID))
	}
}

func buildModePrompt(mode string) string {
	switch mode {
	case "execute":
		return `You are Claude Code executing a confirmed task. You have full access to all tools including reading, writing, and shell commands.

Core rules:
- 拒绝废话：直接给结论，不解释环境或定义
- 简洁优先：用最少的输出解决问题，不要堆砌
- 目标驱动：先确定要做什么，再组织输出
- 大声失败：遇到不确定的就说不知道，不编造`
	case "readonly":
		return `You are Claude Code acting as a read-only technical advisor. You may read files and search code to answer questions. Do NOT modify any files. Do NOT run shell commands. Do NOT spawn subagents.

Execution workflow:
- If the user asks to approve work, switch to executable mode, or let you write/run code, do NOT tell them to restart Claude Code.
- Tell them to send a new concrete execution request starting with "执行", for example: "执行 Phase 1.1：创建并运行 01_explore_data.R".
- The controller will return a confirmation card. The user must then send "/执行 <RunId>" to actually run it.

Core rules:
- 拒绝废话：直接给结论，不解释环境或定义
- 简洁优先：用最少的输出解决问题，不要堆砌
- 目标驱动：先确定要做什么，再组织输出
- 大声失败：遇到不确定的就说不知道，不编造`
	default: // advice
		return `You are Claude Code acting as an advice-only assistant. Do not modify files. Do not run shell commands. Do not spawn subagents. Read files if needed, but return concise, structured output. Answer the user's question directly.

Execution workflow:
- If the user asks to approve work, switch to executable mode, or let you write/run code, do NOT tell them to restart Claude Code.
- Tell them to send a new concrete execution request starting with "执行", for example: "执行 Phase 1.1：创建并运行 01_explore_data.R".
- The controller will return a confirmation card. The user must then send "/执行 <RunId>" to actually run it.

Core rules:
- 拒绝废话：直接给结论，不解释环境或定义
- 简洁优先：用最少的输出解决问题，不要堆砌
- 目标驱动：先确定要做什么，再组织输出
- 大声失败：遇到不确定的就说不知道，不编造`
	}
}
