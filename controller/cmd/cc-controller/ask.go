package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const deduplicationWindow = 5 * time.Second

// findDuplicateRun checks the most recent 3 runs with the given suffix.
// If any was created within deduplicationWindow and has identical question text,
// it returns that run's ID. Otherwise it returns "".
func findDuplicateRun(runsRoot, suffix, questionText string) string {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return ""
	}

	// Collect run dirs that match the suffix.
	var candidates []string
	for _, e := range entries {
		if e.IsDir() && runIDPattern.MatchString(e.Name()) && strings.Contains(e.Name(), suffix) {
			candidates = append(candidates, e.Name())
		}
	}
	if len(candidates) == 0 {
		return ""
	}

	// Sort descending (newest first) — run IDs are timestamp-prefixed.
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] > candidates[j] })

	// Only check the most recent 3.
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}

	now := time.Now()
	trimmedQuestion := strings.TrimSpace(questionText)

	for _, name := range candidates {
		// Parse timestamp from the run ID prefix (first 15 chars: "20060102-150405").
		if len(name) < len(timeFormat) {
			continue
		}
		t, err := time.ParseInLocation(timeFormat, name[:len(timeFormat)], time.Now().Location())
		if err != nil {
			continue
		}
		if now.Sub(t) > deduplicationWindow {
			continue
		}

		// Compare question text.
		runDir := filepath.Join(runsRoot, name)
		data, err := os.ReadFile(filepath.Join(runDir, "incoming-question.md"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == trimmedQuestion {
			return name
		}
	}
	return ""
}

func genRunID(suffix string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%s-%s-%s", time.Now().Format(timeFormat), suffix, hex.EncodeToString(b))
}

func cmdAsk(root, suffix, runnerName string, args []string) {
	// Extract --reply-project flag before reading input text.
	replyProject := ""
	var textArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--reply-project" {
			i++
			if i < len(args) {
				replyProject = args[i]
			}
		} else {
			textArgs = append(textArgs, args[i])
		}
	}

	text := readInput(textArgs)
	if text == "" {
		fmt.Fprintln(os.Stderr, "no input provided")
		os.Exit(1)
	}

	// Deduplication: reject identical questions within 5 seconds.
	if dup := findDuplicateRun(filepath.Join(root, "runs"), suffix, text); dup != "" {
		fmt.Printf("重复请求已忽略（5秒内相同问题）\nRun ID: %s\n", dup)
		return
	}

	runID := genRunID(suffix)
	runDir := filepath.Join(root, "runs", runID)
	os.MkdirAll(runDir, 0755)
	os.WriteFile(filepath.Join(runDir, "incoming-question.md"), []byte(strings.TrimSpace(text)), 0644)

	if replyProject != "" {
		writeFile(filepath.Join(runDir, "runner.reply-project"), replyProject)
	}

	writeStatusJSON(runDir, statusJSON{
		RunID:     runID,
		Kind:      suffix,
		Status:    "accepted",
		Stage:     "accepted",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	})
	appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), RunID: runID, Type: "accepted"})

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
	writeFile(filepath.Join(runDir, "runner.pid"), fmt.Sprintf("%d", runner.Process.Pid))
	updateStatusJSON(runDir, "running", "prepare", runner.Process.Pid)

	label := "Codex"
	if suffix == "cc-ask" {
		label = "CC"
	}
	fmt.Printf("已开始询问 %s...\nRun ID: %s\n完成后自动回传结果\n", label, runID)
}
