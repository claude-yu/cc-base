package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type waitingEntry struct {
	Index     int    `json:"index"`
	RunID     string `json:"run_id"`
	Title     string `json:"title"`
	Summary   string `json:"summary,omitempty"`
	WorkDir   string `json:"work_dir"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func queuePath(root string) string {
	return filepath.Join(root, "waiting_queue.json")
}

func readQueue(root string) []waitingEntry {
	data, err := os.ReadFile(queuePath(root))
	if err != nil {
		return nil
	}
	var entries []waitingEntry
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	return entries
}

func writeQueue(root string, entries []waitingEntry) {
	writeJSON(queuePath(root), entries)
}

func queueNextIndex(entries []waitingEntry) int {
	if len(entries) == 0 {
		return 1
	}
	maxIdx := 0
	for _, e := range entries {
		if e.Index > maxIdx {
			maxIdx = e.Index
		}
	}
	return maxIdx + 1
}

func queueAdd(root, runID, workDir string, text string) {
	title := text
	if len([]rune(title)) > 50 {
		title = string([]rune(title)[:50]) + "..."
	}
	entries := readQueue(root)
	entry := waitingEntry{
		Index:     queueNextIndex(entries),
		RunID:     runID,
		Title:     title,
		Summary:   text,
		WorkDir:   workDir,
		Status:    "waiting_confirmation",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	entries = append(entries, entry)
	writeQueue(root, entries)
}

func queueRemove(root, runID string) {
	entries := readQueue(root)
	var kept []waitingEntry
	for _, e := range entries {
		if e.RunID != runID {
			kept = append(kept, e)
		}
	}
	writeQueue(root, kept)
}

func queueRemoveByIndex(root string, index int) {
	entries := readQueue(root)
	var kept []waitingEntry
	for _, e := range entries {
		if e.Index != index {
			kept = append(kept, e)
		}
	}
	writeQueue(root, kept)
}

func queueFindByIndex(entries []waitingEntry, index int) *waitingEntry {
	for i := range entries {
		if entries[i].Index == index {
			return &entries[i]
		}
	}
	return nil
}

// queuePrune removes entries whose run no longer exists or is no longer
// awaiting_confirmation. Returns count of pruned entries.
func queuePrune(root string) int {
	entries := readQueue(root)
	var kept []waitingEntry
	pruned := 0
	for _, e := range entries {
		runDir := filepath.Join(root, "runs", e.RunID)
		s := readStatusJSON(runDir)
		if s.Stage == "awaiting_confirmation" {
			kept = append(kept, e)
		} else {
			pruned++
		}
	}
	writeQueue(root, kept)
	return pruned
}

// cmdExecuteQueue handles `/执行` with no args.
func cmdExecuteQueue(root string) {
	pruned := queuePrune(root)
	entries := readQueue(root)
	if pruned > 0 {
		fmt.Fprintf(os.Stderr, "已清理 %d 个已失效的等待任务\n", pruned)
	}
	switch len(entries) {
	case 0:
		fmt.Fprintln(os.Stderr, "当前没有待执行任务")
		os.Exit(1)
	case 1:
		e := entries[0]
		fmt.Printf("将执行 #%d：%s\n", e.Index, e.Title)
		fmt.Printf("RunId: %s\n", e.RunID)
		fmt.Printf("工作目录: %s\n\n", e.WorkDir)
		queueRemove(root, e.RunID)
		cmdExecute(root, e.RunID)
	default:
		fmt.Printf("发现 %d 个待执行任务，请回复：\n\n", len(entries))
		for _, e := range entries {
			fmt.Printf("/执行 %d\n", e.Index)
		}
		fmt.Println()
		for _, e := range entries {
			fmt.Printf("%d. %s\n", e.Index, e.Title)
			fmt.Printf("   RunId: %s\n", e.RunID)
			fmt.Printf("   工作目录: %s\n", e.WorkDir)
			fmt.Println()
		}
		os.Exit(1)
	}
}

// cmdExecuteQueueIndex handles `/执行 N` where N is a queue index.
func cmdExecuteQueueIndex(root string, idxStr string) {
	pruned := queuePrune(root)
	if pruned > 0 {
		fmt.Fprintf(os.Stderr, "已清理 %d 个已失效的等待任务\n", pruned)
	}
	entries := readQueue(root)
	idx := 0
	for _, c := range idxStr {
		idx = idx*10 + int(c-'0')
	}
	e := queueFindByIndex(entries, idx)
	if e == nil {
		fmt.Fprintf(os.Stderr, "未找到 #%d 等待任务\n", idx)
		os.Exit(1)
	}
	fmt.Printf("将执行 #%d：%s\n", e.Index, e.Title)
	fmt.Printf("RunId: %s\n", e.RunID)
	fmt.Printf("工作目录: %s\n\n", e.WorkDir)
	queueRemove(root, e.RunID)
	cmdExecute(root, e.RunID)
}

// cmdCancelSmart handles `/取消` with no args.
func cmdCancelSmart(root string) {
	pruned := queuePrune(root)
	if pruned > 0 {
		fmt.Fprintf(os.Stderr, "已清理 %d 个已失效的等待任务\n", pruned)
	}

	// First, check for running tasks.
	entries, err := os.ReadDir(filepath.Join(root, "runs"))
	runningFound := false
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pidPath := filepath.Join(root, "runs", e.Name(), "runner.pid")
			pidData, err := os.ReadFile(pidPath)
			if err != nil {
				continue
			}
			pid := 0
			for _, c := range string(pidData) {
				if c >= '0' && c <= '9' {
					pid = pid*10 + int(c-'0')
				} else if pid > 0 {
					break
				}
			}
			if pid > 0 && isProcessRunning(pid) {
				fmt.Printf("已优先取消正在运行的任务：\nRunId: %s\nwaiting 队列未处理。\n", e.Name())
				cancelTask(root, e.Name())
				runningFound = true
				break
			}
		}
	}
	if runningFound {
		return
	}

	// No running tasks — check waiting queue.
	waiting := readQueue(root)
	switch len(waiting) {
	case 0:
		fmt.Fprintln(os.Stderr, "没有运行中的任务，也没有待执行的等待任务")
		os.Exit(1)
	case 1:
		e := waiting[0]
		fmt.Printf("将取消 #%d：%s\n", e.Index, e.Title)
		fmt.Printf("RunId: %s\n\n", e.RunID)
		cancelTask(root, e.RunID)
		queueRemove(root, e.RunID)
	default:
		fmt.Printf("发现 %d 个等待任务，请回复：\n\n", len(waiting))
		for _, e := range waiting {
			fmt.Printf("/取消 %d\n", e.Index)
		}
		fmt.Println()
		for _, e := range waiting {
			fmt.Printf("%d. %s\n", e.Index, e.Title)
			fmt.Printf("   RunId: %s\n", e.RunID)
			fmt.Println()
		}
		os.Exit(1)
	}
}

// cmdCancelQueueIndex handles `/取消 N` where N is a queue index.
func cmdCancelQueueIndex(root string, idxStr string) {
	entries := readQueue(root)
	idx := 0
	for _, c := range idxStr {
		idx = idx*10 + int(c-'0')
	}
	e := queueFindByIndex(entries, idx)
	if e == nil {
		fmt.Fprintf(os.Stderr, "未找到 #%d 等待任务\n", idx)
		os.Exit(1)
	}
	fmt.Printf("将取消 #%d：%s\n", e.Index, e.Title)
	fmt.Printf("RunId: %s\n\n", e.RunID)
	cancelTask(root, e.RunID)
	queueRemove(root, e.RunID)
}

func describeQueue(root string) string {
	entries := readQueue(root)
	if len(entries) == 0 {
		return "等待执行: 无"
	}
	s := fmt.Sprintf("等待执行: %d 个\n", len(entries))
	for _, e := range entries {
		s += fmt.Sprintf("  #%d  %s\n", e.Index, e.Title)
	}
	return s
}
