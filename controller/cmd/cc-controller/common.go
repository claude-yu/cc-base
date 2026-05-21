package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func mustHaveArgs(args []string, n int, msg string) {
	if len(args) < n {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

func resolveControllerRoot() string {
	if dir := os.Getenv("CC_CONTROLLER_DIR"); dir != "" {
		return dir
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if filepath.Base(dir) == "bin" {
			return filepath.Dir(dir)
		}
		if fi, err := os.Stat(filepath.Join(dir, "runs")); err == nil && fi.IsDir() {
			return dir
		}
	}
	if root := os.Getenv("CONTROLLER_ROOT"); root != "" {
		return root
	}
	fmt.Fprintln(os.Stderr, "FATAL: cannot resolve controller root. Set CC_CONTROLLER_DIR or CONTROLLER_ROOT.")
	os.Exit(1)
	return ""
}

// resolveProjectWorkDir returns the per-research-project working directory.
// Switch projects by changing CC_WORK_DIR only — controller dir and sandbox
// stay fixed.
func resolveProjectWorkDir() string {
	if dir := os.Getenv("CC_WORK_DIR"); dir != "" {
		return dir
	}
	return "."
}

func writeFile(path, content string) {
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte(content), 0644)
}

func sendCallback(runDir, message string) {
	callbackPath := filepath.Join(runDir, "callback-msg.md")
	os.WriteFile(callbackPath, []byte(message), 0644)

	ccConnect, err := exec.LookPath("cc-connect")
	if err != nil {
		candidates := []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "cc-connect", "cc-connect.exe"),
			"cc-connect.exe",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				ccConnect = c
				break
			}
		}
		if ccConnect == "" {
			logCallbackError(runDir, "cc-connect not found on PATH or in known locations")
			return
		}
	}

	// Use reply-project if specified, otherwise default to "cc".
	project := "cc"
	if data, err := os.ReadFile(filepath.Join(runDir, "runner.reply-project")); err == nil {
		if p := strings.TrimSpace(string(data)); p != "" {
			project = p
		}
	}

	cmd := exec.Command(ccConnect, "send", "--stdin", "-p", project)
	cmd.Stdin = strings.NewReader(message)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := fmt.Sprintf("cc-connect send failed (project=%s): %v", project, err)
		if s := stderr.String(); s != "" {
			errMsg += " stderr: " + strings.TrimSpace(s)
		}
		logCallbackError(runDir, errMsg)

		runID := filepath.Base(runDir)
		fallback := fmt.Sprintf("✅ 任务已完成，但结果发送失败（消息可能过长）。\n/结果 %s 查看完整内容", runID)
		retry := exec.Command(ccConnect, "send", "--stdin", "-p", project)
		retry.Stdin = strings.NewReader(fallback)
		if retryErr := retry.Run(); retryErr != nil {
			logCallbackError(runDir, "fallback send also failed: "+retryErr.Error())
		}
	}
}

func logCallbackError(runDir, errMsg string) {
	logPath := filepath.Join(runDir, "callback-error.log")
	entry := fmt.Sprintf("[%s] %s\n", time.Now().UTC().Format(time.RFC3339), errMsg)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(entry)
}

// heartbeatMsg builds a heartbeat callback message with escalation at 3/5/10 min.
func heartbeatMsg(label, runID string, elapsed int, extra string) string {
	var prefix, suffix string
	switch {
	case elapsed >= 600:
		prefix = "⚠️"
		suffix = fmt.Sprintf("\n运行已超 %d 分钟，可能卡住\n/取消任务 %s", elapsed/60, runID)
	case elapsed >= 300:
		prefix = "⏳"
		suffix = fmt.Sprintf("\n运行较久（%d分钟），如需取消: /取消任务 %s", elapsed/60, runID)
	default:
		prefix = "⏳"
	}
	msg := fmt.Sprintf("%s %s 处理中\nRun ID: %s", prefix, label, runID)
	if extra != "" {
		msg += "\n" + extra
	}
	msg += fmt.Sprintf("\n已用时: %ds", elapsed)
	msg += suffix
	return msg
}

func writeError(runDir string, err error) {
	msg := fmt.Sprintf("Error: %s", err.Error())
	os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(msg), 0644)
	os.WriteFile(filepath.Join(runDir, "runner.exitcode.txt"), []byte("1"), 0644)
	appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), Type: "error", Message: err.Error()})
}

// trimToken strips a leading UTF-8 BOM (legacy PowerShell-written files have one)
// plus surrounding whitespace, so single-token files compare reliably.
func trimToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, string(rune(0xFEFF)))
	return strings.TrimSpace(s)
}

// isNumeric returns true if s contains only ASCII digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// readInput returns the first arg or stdin, trimming whitespace.
func readInput(args []string) string {
	if len(args) > 0 {
		return strings.Join(args, " ")
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		return ""
	}
	return strings.TrimSpace(string(data))
}
