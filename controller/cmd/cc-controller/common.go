package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
// Priority: CC_WORK_DIR env > active_project.json > "."
func resolveProjectWorkDir() string {
	if dir := os.Getenv("CC_WORK_DIR"); dir != "" {
		return dir
	}
	if dir := readActiveProjectWorkDir(); dir != "" {
		return dir
	}
	return "."
}

func readActiveProjectWorkDir() string {
	root := resolveControllerRootSilent()
	if root == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "active_project.json"))
	if err != nil {
		return ""
	}
	var proj struct {
		WorkDir string `json:"work_dir"`
	}
	if json.Unmarshal(data, &proj) != nil || proj.WorkDir == "" {
		return ""
	}
	if fi, err := os.Stat(proj.WorkDir); err == nil && fi.IsDir() {
		return proj.WorkDir
	}
	return ""
}

// resolveControllerRootSilent is like resolveControllerRoot but returns ""
// instead of calling os.Exit on failure.
func resolveControllerRootSilent() string {
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
	return ""
}

func writeFile(path, content string) {
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte(content), 0644)
}

func sendCallback(runDir, message string) {
	callbackPath := filepath.Join(runDir, "callback-msg.md")
	os.WriteFile(callbackPath, []byte(message), 0644)

	// Prefer fork binaries over PATH (PATH has npm .cmd wrapper that isn't our fork).
	var ccConnect string
	if exe, exeErr := os.Executable(); exeErr == nil {
		dir := filepath.Dir(exe)
		for _, name := range []string{"cc-connect-fork-v4c.exe", "cc-connect-fork-v4b.exe", "cc-connect-fork-v4.exe", "cc-connect-fork-v3.exe", "cc-connect-fork.exe"} {
			c := filepath.Join(dir, "..", "cc-connect", name)
			if _, err := os.Stat(c); err == nil {
				ccConnect = c
				break
			}
		}
	}
	if ccConnect == "" {
		for _, c := range []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "cc-connect", "cc-connect.exe"),
			"cc-connect.exe",
		} {
			if _, err := os.Stat(c); err == nil {
				ccConnect = c
				break
			}
		}
	}
	if ccConnect == "" {
		if p, err := exec.LookPath("cc-connect"); err == nil {
			ccConnect = p
		}
	}
	if ccConnect == "" {
		logCallbackError(runDir, "cc-connect not found in fork dir, LOCALAPPDATA, or PATH")
		return
	}

	// Use reply-project if specified, otherwise default to "cc".
	project := "cc"
	if data, err := os.ReadFile(filepath.Join(runDir, "runner.reply-project")); err == nil {
		if p := strings.TrimSpace(string(data)); p != "" {
			project = p
		}
	}

	if err, stderrText := runCCConnectSend(ccConnect, project, "", message); err != nil {
		errMsg := fmt.Sprintf("cc-connect send failed (project=%s): %v", project, err)
		if stderrText != "" {
			errMsg += " stderr: " + strings.TrimSpace(stderrText)
		}

		sessionKey := resolveCallbackSessionKey(runDir, ccConnect, project)
		if sessionKey != "" {
			if retryErr, retryStderr := runCCConnectSend(ccConnect, project, sessionKey, message); retryErr == nil {
				return
			} else {
				logCallbackError(runDir, errMsg)
				retryMsg := fmt.Sprintf("cc-connect send retry failed (project=%s, session=%s): %v", project, redactSessionKey(sessionKey), retryErr)
				if retryStderr != "" {
					retryMsg += " stderr: " + strings.TrimSpace(retryStderr)
				}
				logCallbackError(runDir, retryMsg)
			}
		} else {
			logCallbackError(runDir, errMsg)
		}

		runID := filepath.Base(runDir)
		fallback := fmt.Sprintf("✅ 任务已完成，但结果发送失败（消息可能过长）。\n/结果 %s 查看完整内容", runID)
		if fallbackErr, fallbackStderr := runCCConnectSend(ccConnect, project, sessionKey, fallback); fallbackErr != nil {
			msg := "fallback send also failed: " + fallbackErr.Error()
			if fallbackStderr != "" {
				msg += " stderr: " + strings.TrimSpace(fallbackStderr)
			}
			logCallbackError(runDir, msg)
		}
	}
}

func runCCConnectSend(ccConnect, project, sessionKey, message string) (error, string) {
	args := []string{"send", "--stdin", "-p", project}
	if strings.TrimSpace(sessionKey) != "" {
		args = append(args, "-s", sessionKey)
	}
	cmd := exec.Command(ccConnect, args...)
	cmd.Stdin = strings.NewReader(message)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	return err, stderr.String()
}

func resolveCallbackSessionKey(runDir, ccConnect, project string) string {
	if key := strings.TrimSpace(os.Getenv("CC_SESSION_KEY")); key != "" {
		return key
	}
	if data, err := os.ReadFile(filepath.Join(runDir, "runner.cc-session-key")); err == nil {
		if key := strings.TrimSpace(string(data)); key != "" {
			return key
		}
	}
	if data, err := os.ReadFile(filepath.Join(runDir, "runner.chat-id")); err == nil {
		if key := sessionKeyFromBindingKey(strings.TrimSpace(string(data))); key != "" {
			return key
		}
	}
	return resolveCCConnectActiveSessionKey(ccConnect, project, "")
}

func resolveCCConnectActiveSessionKey(ccConnect, project, preferredPlatform string) string {
	out, err := exec.Command(ccConnect, "sessions", "list").Output()
	if err != nil {
		return ""
	}
	sessionProject, platform := parseLatestSessionProjectPlatform(string(out), project)
	if sessionProject == "" {
		return ""
	}
	path := filepath.Join(os.Getenv("USERPROFILE"), ".cc-connect", "sessions", sessionProject+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if preferredPlatform != "" {
		platform = preferredPlatform
	}
	return chooseActiveSessionKey(extractActiveSessionKeys(string(data)), platform)
}

func sessionKeyFromBindingKey(key string) string {
	parts := strings.SplitN(key, bindingDelimiter, 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return parts[0] + ":" + parts[1]
}

func parseLatestSessionProjectPlatform(listOutput, project string) (string, string) {
	for _, line := range strings.Split(listOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !isNumeric(fields[0]) {
			continue
		}
		sessionProject := fields[1]
		if sessionProject == project || strings.HasPrefix(sessionProject, project+"_") {
			return sessionProject, fields[2]
		}
	}
	return "", ""
}

func extractActiveSessionKeys(sessionJSON string) []string {
	re := regexp.MustCompile(`(?s)"active_session"\s*:\s*\{(.*?)\}`)
	m := re.FindStringSubmatch(sessionJSON)
	if len(m) != 2 {
		return nil
	}
	keyRe := regexp.MustCompile(`"([^"]+)"\s*:\s*"[^"]+"`)
	matches := keyRe.FindAllStringSubmatch(m[1], -1)
	keys := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) == 2 {
			keys = append(keys, match[1])
		}
	}
	return keys
}

func chooseActiveSessionKey(keys []string, platform string) string {
	if len(keys) == 0 {
		return ""
	}
	prefix := platform + ":"
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			return key
		}
	}
	return keys[0]
}

func redactSessionKey(key string) string {
	if key == "" {
		return ""
	}
	parts := strings.SplitN(key, ":", 2)
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + ":..."
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
