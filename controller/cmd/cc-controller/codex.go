package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var codexNoisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^Reading prompt from stdin`),
	regexp.MustCompile(`(?i)^OpenAI Codex v`),
	regexp.MustCompile(`^-{4,}$`),
	regexp.MustCompile(`(?i)^tokens used`),
	regexp.MustCompile(`^\s*\d{1,3}(,\d{3})+\s*$`),
	regexp.MustCompile(`(?i)^(success:|成功:)`),
	regexp.MustCompile(`(?i)^System\.Management\.Automation\.RemoteException$`),
}

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

func cleanCodexOutput(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	skipMetadata := false

	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")

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
		if matched, _ := regexp.MatchString(`(?i).*(?:终止|terminate|killed|process).*PID\s+\d+`, trimmed); matched {
			continue
		}
		if matched, _ := regexp.MatchString(`(?i).*PID\s+\d+.*(?:终止|terminate|killed|process)`, trimmed); matched {
			continue
		}
		if !utf8.ValidString(trimmed) && strings.Contains(strings.ToUpper(trimmed), "PID") {
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

Core rules:
- 拒绝废话：直接给结论，不解释环境或定义
- 简洁优先：用最少的输出解决问题，不要堆砌
- 目标驱动：先确定要回答什么，再组织输出
- 大声失败：遇到不确定的就说不知道，不编造

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

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		start := time.Now()
		for {
			select {
			case <-ticker.C:
				elapsed := int(time.Since(start).Seconds())
				appendEvent(runDir, eventEntry{
					Ts:         time.Now().UTC().Format(time.RFC3339),
					RunID:      runID,
					Type:       "heartbeat",
					Stage:      "codex_running",
					ElapsedSec: elapsed,
				})
				sendCallback(runDir, fmt.Sprintf("⏳ Codex 处理中\nRun ID: %s\n已用时: %ds", runID, elapsed))
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

	rawAnswer := stdout.String()
	cleanAnswer := cleanCodexOutput(rawAnswer)

	os.WriteFile(filepath.Join(runDir, "codex-answer.raw.md"), []byte(rawAnswer), 0644)
	os.WriteFile(filepath.Join(runDir, "codex-answer.md"), []byte(cleanAnswer), 0644)
	os.WriteFile(filepath.Join(runDir, "codex-answer.exitcode.txt"), []byte(fmt.Sprintf("%d", exitCode)), 0644)

	if exitCode == 0 {
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(cleanAnswer), 0644)
		updateStatusJSON(runDir, "completed", "done", 0)
		setExitCode(runDir, 0)
		appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), RunID: runID, Type: "completed", ExitCode: 0})
		sendCallback(runDir, fmt.Sprintf("[Codex] 已回复 (RunId: %s)\n%s", runID, cleanAnswer))
	} else {
		errText := rawAnswer
		if errText == "" {
			errText = strings.TrimSpace(stderr.String())
		}
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(errText), 0644)
		updateStatusJSON(runDir, "failed", "failed", 0)
		setExitCode(runDir, exitCode)
		appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), RunID: runID, Type: "failed", ExitCode: exitCode})
		sendCallback(runDir, fmt.Sprintf("[Codex] 调用失败 (RunId: %s). 检查 CODEX_PROXY / /修复controller", runID))
	}
}
