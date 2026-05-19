package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
	updateStatusJSON(runDir, "cancelled", "cancelled", 0)
	appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), RunID: runID, Type: "cancelled"})
	sendCallback(runDir, fmt.Sprintf("[Cancelled] (RunId: %s)\n任务已被用户取消。\n/cc结果 %s 可查看最终状态。", runID, runID))

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
