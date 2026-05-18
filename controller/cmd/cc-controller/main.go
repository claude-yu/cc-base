package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const timeFormat = "20060102-150405"

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	root := resolveControllerRoot()
	args := os.Args[2:]

	switch cmd {
	case "ask-cc":
		cmdAsk(root, "cc-ask", "run-cc", args)
	case "ask-codex":
		cmdAsk(root, "codex-ask", "run-codex", args)
	case "run-cc":
		mustHaveArgs(args, 1, "usage: cc-controller run-cc <RunId>")
		runCC(root, args[0])
	case "run-codex":
		mustHaveArgs(args, 1, "usage: cc-controller run-codex <RunId>")
		runCodex(root, args[0])
	case "show":
		mustHaveArgs(args, 1, "usage: cc-controller show <RunId>")
		showRun(root, args[0])
	case "cancel":
		if len(args) >= 1 && args[0] != "" {
			cancelTask(root, args[0])
		} else {
			cancelLatest(root)
		}
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `cc-controller — async ask/callback pipeline

Commands:
  ask-cc <text>       Ask Claude Code asynchronously
  ask-codex <text>    Ask Codex asynchronously
  run-cc <RunId>      Background runner for ask-cc
  run-codex <RunId>   Background runner for ask-codex
  show <RunId>        Show run result
  cancel [RunId]      Cancel a running task (omit RunId to cancel latest)`)
		os.Exit(1)
	os.Exit(1)
}

func mustHaveArgs(args []string, n int, msg string) {
	if len(args) < n {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

func resolveControllerRoot() string {
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
	fmt.Fprintln(os.Stderr, "FATAL: cannot resolve controller root. Set CONTROLLER_ROOT env.")
	os.Exit(1)
	return ""
}

func genRunID(suffix string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%s-%s-%s", time.Now().Format(timeFormat), suffix, hex.EncodeToString(b))
}

func readInput(args []string) string {
	if len(args) > 0 {
		return strings.Join(args, " ")
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return ""
	}
	return string(bytes.TrimSpace(data))
}

func writeFile(path, content string) {
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte(content), 0644)
}

// ── ask-cc / ask-codex (entry from cc-connect) ──

func cmdAsk(root, suffix, runnerName string, args []string) {
	text := readInput(args)
	if text == "" {
		fmt.Fprintln(os.Stderr, "no input provided")
		os.Exit(1)
	}

	runID := genRunID(suffix)
	runDir := filepath.Join(root, "runs", runID)
	os.MkdirAll(runDir, 0755)
	os.WriteFile(filepath.Join(runDir, "incoming-question.md"), []byte(text), 0644)

	exe, _ := os.Executable()
	runner := exec.Command(exe, runnerName, runID)
	runner.Dir = root
	runner.Stdin = nil
	runner.Stdout = nil
	runner.Stderr = nil
	runner.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := runner.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start runner: %s\n", err)
		os.Exit(1)
	}
	// Write PID for cancellation tracking
	writeFile(filepath.Join(runDir, "runner.pid"), fmt.Sprintf("%d", runner.Process.Pid))

	fmt.Println(runID)
}

// ── run-cc (background runner) ──

func resolveClaudeCmd() string {
	if cmd := os.Getenv("CLAUDE_CMD"); cmd != "" {
		return cmd
	}
	// Check common install locations
	candidates := []string{
		filepath.Join(os.Getenv("USERPROFILE"), ".claude", "bin", "claude.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "claude", "claude.exe"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Fall back to PATH lookup
	if p, err := exec.LookPath("claude"); err == nil {
		return p
	}
	return "claude"
}

func resolveWorkDir() string {
	if dir := os.Getenv("CC_WORK_DIR"); dir != "" {
		return dir
	}
	return "."
}

func runCC(root, runID string) {
	runDir := filepath.Join(root, "runs", runID)
	writeFile(filepath.Join(runDir, "runner.pid"), fmt.Sprintf("%d", os.Getpid()))

	question, err := os.ReadFile(filepath.Join(runDir, "incoming-question.md"))
	if err != nil {
		writeError(runDir, err)
		return
	}

	claudeCmd := resolveClaudeCmd()
	workDir := resolveWorkDir()

	systemPrompt := "You are Claude Code acting as an advice-only assistant. Do not modify files. Do not run shell commands. Do not spawn subagents. Read files if needed, but return concise, structured output. Answer the user's question directly."

	cmd := exec.Command(claudeCmd, "-p", "--system-prompt", systemPrompt, "--output-format", "text", "--no-session-persistence")
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewReader(question)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()

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
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(rawAnswer), 0644)
		sendCallback(runDir, fmt.Sprintf("[CC] 已回复 (RunId: %s)\n%s", runID, rawAnswer))
	} else {
		errText := rawAnswer
		if errText == "" {
			errText = strings.TrimSpace(stderr.String())
		}
		if errText == "" {
			errText = "(no output)"
		}
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(errText), 0644)
		sendCallback(runDir, fmt.Sprintf("[CC] 调用失败 (RunId: %s). 检查 Claude CLI / CLAUDE_PROXY / /修复controller", runID))
	}

	os.WriteFile(filepath.Join(runDir, "runner.exitcode.txt"), []byte("0"), 0644)
}

// ── run-codex (background runner) ──

func resolveCodexCmd() string {
	if cmd := os.Getenv("CODEX_CMD"); cmd != "" {
		return cmd
	}
	if p, err := exec.LookPath("codex"); err == nil {
		return p
	}
	return "codex"
}

func setCodexProxy() {
	proxy := os.Getenv("CODEX_PROXY")
	if proxy == "" {
		return
	}
	os.Setenv("ALL_PROXY", proxy)
	os.Setenv("HTTP_PROXY", proxy)
	os.Setenv("HTTPS_PROXY", proxy)
}

var codexNoisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^Reading prompt from stdin`),
	regexp.MustCompile(`(?i)^OpenAI Codex v`),
	regexp.MustCompile(`^-{4,}$`),
	regexp.MustCompile(`(?i)^tokens used`),
	regexp.MustCompile(`^\s*\d{1,3}(,\d{3})+\s*$`),
	regexp.MustCompile(`(?i)^(success:|成功:)`),
	regexp.MustCompile(`(?i)^System\.Management\.Automation\.RemoteException$`),
}

func cleanCodexOutput(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	skipMetadata := false

	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")

		// Skip known noise
		if matched, _ := regexp.MatchString(`(?i)^Reading prompt from stdin`, trimmed); matched {
			continue
		}
		if matched, _ := regexp.MatchString(`(?i)^OpenAI Codex v`, trimmed); matched {
			skipMetadata = true
			continue
		}
		if matched, _ := regexp.MatchString(`^-{4,}$`, trimmed); matched {
			continue
		}
		if matched, _ := regexp.MatchString(`System\.Management\.Automation\.RemoteException`, trimmed); matched {
			continue
		}
		if matched, _ := regexp.MatchString(`(?i)^tokens used`, trimmed); matched {
			continue
		}
		if matched, _ := regexp.MatchString(`^\s*\d{1,3}(,\d{3})+\s*$`, trimmed); matched {
			continue
		}
		if matched, _ := regexp.MatchString(`(?i)^(success:|成功:)`, trimmed); matched {
			continue
		}

		if skipMetadata {
			lower := strings.ToLower(trimmed)
			if strings.TrimSpace(trimmed) == "codex" {
				skipMetadata = false
				continue
			}
			if strings.HasPrefix(lower, "workdir:") ||
				strings.HasPrefix(lower, "model:") ||
				strings.HasPrefix(lower, "provider:") ||
				strings.HasPrefix(lower, "approval:") ||
				strings.HasPrefix(lower, "sandbox:") ||
				strings.HasPrefix(lower, "reasoning:") ||
				strings.HasPrefix(lower, "session id:") {
				continue
			}
			if strings.TrimSpace(trimmed) == "user" {
				continue
			}
			continue
		}

		kept = append(kept, trimmed)
	}

	clean := strings.TrimSpace(strings.Join(kept, "\n"))
	if clean == "" {
		clean = strings.TrimSpace(text)
	}

	deduped := removeDuplicateOutput(clean)
	if deduped != "" {
		return deduped
	}
	return clean
}

func removeDuplicateOutput(text string) string {
	blocks := regexp.MustCompile(`\n{2,}`).Split(text, -1)
	nonEmpty := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if t := strings.TrimSpace(b); t != "" {
			nonEmpty = append(nonEmpty, t)
		}
	}

	if len(nonEmpty) >= 2 && len(nonEmpty)%2 == 0 {
		half := len(nonEmpty) / 2
		first := strings.Join(nonEmpty[:half], "\n\n")
		second := strings.Join(nonEmpty[half:], "\n\n")
		if first == second {
			return first
		}
	}

	lines := strings.Split(text, "\n")
	if len(lines) >= 4 && len(lines)%2 == 0 {
		half := len(lines) / 2
		first := strings.TrimSpace(strings.Join(lines[:half], "\n"))
		second := strings.TrimSpace(strings.Join(lines[half:], "\n"))
		if first == second && first != "" {
			return first
		}
	}

	return ""
}

func runCodex(root, runID string) {
	runDir := filepath.Join(root, "runs", runID)
	writeFile(filepath.Join(runDir, "runner.pid"), fmt.Sprintf("%d", os.Getpid()))

	question, err := os.ReadFile(filepath.Join(runDir, "incoming-question.md"))
	if err != nil {
		writeError(runDir, err)
		return
	}

	codexCmd := resolveCodexCmd()
	setCodexProxy()

	systemPrompt := `Reply in the same language as the user's question. If the question contains Chinese, use Simplified Chinese.

You are Codex acting as an independent technical advisor.
Do not read files. Do not run commands. Do not modify anything.
Answer the user's question directly based on your knowledge.

At the end of your answer, include a "建议下一步" section:

## 建议下一步
- P1 ...
- P2 ...
- P3 ...

If no action is needed, write:
- 无需后续操作。`

	fullPrompt := systemPrompt + "\n\n---\n" + string(question)

	cmd := exec.Command(codexCmd, "exec", "--sandbox", "read-only", "--skip-git-repo-check", "--ephemeral")
	cmd.Stdin = strings.NewReader(fullPrompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	rawAnswer := stdout.String()
	cleanAnswer := cleanCodexOutput(rawAnswer)

	os.WriteFile(filepath.Join(runDir, "codex-answer.raw.md"), []byte(rawAnswer), 0644)
	os.WriteFile(filepath.Join(runDir, "codex-answer.md"), []byte(cleanAnswer), 0644)
	os.WriteFile(filepath.Join(runDir, "codex-answer.exitcode.txt"), []byte(fmt.Sprintf("%d", exitCode)), 0644)

	if exitCode == 0 {
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(cleanAnswer), 0644)
		sendCallback(runDir, fmt.Sprintf("[Codex] 已回复 (RunId: %s)\n%s", runID, cleanAnswer))
	} else {
		errText := rawAnswer
		if errText == "" {
			errText = strings.TrimSpace(stderr.String())
		}
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(errText), 0644)
		sendCallback(runDir, fmt.Sprintf("[Codex] 调用失败 (RunId: %s). 检查 CODEX_PROXY / /修复controller", runID))
	}

	os.WriteFile(filepath.Join(runDir, "runner.exitcode.txt"), []byte("0"), 0644)
}

// ── cancel ──

func cancelTask(root, runID string) {
	runDir := filepath.Join(root, "runs", runID)
	pidPath := filepath.Join(runDir, "runner.pid")

	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "找不到运行中任务的 PID (RunId: %s)\n", runID)
		os.Exit(1)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "无效 PID 文件: %s\n", pidPath)
		os.Exit(1)
	}

	kill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	kill.Stdout = nil
	kill.Stderr = nil
	if err := kill.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "取消失败 (RunId: %s, PID %d 可能已退出)\n", runID, pid)
		os.Exit(1)
	}

	msg := fmt.Sprintf("[Cancelled] 任务已取消 (RunId: %s)", runID)
	os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(msg), 0644)
	os.WriteFile(filepath.Join(runDir, "runner.exitcode.txt"), []byte("-1"), 0644)
	sendCallback(runDir, fmt.Sprintf("[Cancelled] (RunId: %s)\n任务已被用户取消。\n/查看审查 %s 可查看最终状态。", runID, runID))

	fmt.Printf("已取消: %s (PID %d)\n", runID, pid)
}

func cancelLatest(root string) {
	entries, err := os.ReadDir(filepath.Join(root, "runs"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "没有运行中的任务")
		os.Exit(1)
	}

	var running []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pidPath := filepath.Join(root, "runs", e.Name(), "runner.pid")
		pidData, err := os.ReadFile(pidPath)
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
		if err != nil {
			continue
		}
		if isProcessRunning(pid) {
			running = append(running, e.Name())
		}
	}

	if len(running) == 0 {
		fmt.Fprintln(os.Stderr, "没有运行中的任务")
		os.Exit(1)
	}

	sort.Slice(running, func(i, j int) bool { return running[i] > running[j] })
	cancelTask(root, running[0])
}

func isProcessRunning(pid int) bool {
	check := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	out, err := check.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), strconv.Itoa(pid))
}

// ── shared helpers ──

func sendCallback(runDir, message string) {
	// Write callback file for debugging/recovery
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
			return
		}
	}
	cmd := exec.Command(ccConnect, "send", "--stdin", "-p", "cc")
	cmd.Stdin = strings.NewReader(message)
	cmd.Run()
}

func writeError(runDir string, err error) {
	msg := fmt.Sprintf("Error: %s", err.Error())
	os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(msg), 0644)
	os.WriteFile(filepath.Join(runDir, "runner.exitcode.txt"), []byte("1"), 0644)
}

func showRun(root, runID string) {
	runDir := filepath.Join(root, "runs", runID)

	for _, name := range []string{"incoming-question.md", "cc-answer.md", "codex-answer.md", "summary.md"} {
		data, err := os.ReadFile(filepath.Join(runDir, name))
		if err != nil {
			continue
		}
		fmt.Printf("=== %s ===\n%s\n\n", name, string(data))
	}
}
