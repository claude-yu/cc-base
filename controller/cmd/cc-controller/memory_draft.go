package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func cmdMemoryDraft(root string, args []string) {
	mode := "summary"
	if len(args) > 0 && args[0] != "" {
		mode = strings.ToLower(args[0])
	}

	switch mode {
	case "summary", "总结":
		cmdMemoryDraftGenerate(root, "summary")
	case "record", "记录":
		cmdMemoryDraftGenerate(root, "record")
	case "status", "状态":
		cmdMemoryStatus(root)
	case "patch", "补丁":
		cmdMemoryPatch(root, args[1:])
	case "review-patch", "审查补丁":
		cmdMemoryPatchReview(root, args[1:])
	case "apply", "应用":
		cmdMemoryApply(root, args[1:])
	default:
		fmt.Fprintln(os.Stderr, "未知模式（可用: summary, record, status, patch, review-patch, apply）")
		os.Exit(1)
	}
}

func cmdMemoryStatus(root string) {
	workDir := resolveMemoryRoot(root)

	progressPath := filepath.Join(workDir, "progress.md")
	progressLines := 0
	progressExists := false
	if data, err := os.ReadFile(progressPath); err == nil {
		progressExists = true
		progressLines = strings.Count(string(data), "\n") + 1
	}

	today := time.Now().Format("2006-01-02")
	handoffPath := filepath.Join(workDir, "handoff-"+today+".md")
	handoffStatus := "未创建"
	if fi, err := os.Stat(handoffPath); err == nil {
		handoffStatus = fmt.Sprintf("已更新 (%s)", fi.ModTime().Format("15:04"))
	} else {
		latestHandoff := findLatestHandoff(workDir)
		if latestHandoff != "" {
			if fi2, err := os.Stat(latestHandoff); err == nil {
				handoffStatus = fmt.Sprintf("最近: %s (%s)", filepath.Base(latestHandoff), fi2.ModTime().Format("01-02 15:04"))
			}
		}
	}

	archivePath := filepath.Join(workDir, "progress.archive.md")
	archiveLines := 0
	if data, err := os.ReadFile(archivePath); err == nil {
		archiveLines = strings.Count(string(data), "\n") + 1
	}

	var sb strings.Builder
	sb.WriteString("📋 记忆状态\n\n")

	if progressExists {
		sb.WriteString(fmt.Sprintf("progress.md: %d 行", progressLines))
		if progressLines > 120 {
			sb.WriteString(" ⚠️ 建议 archive")
		} else if progressLines > 80 {
			sb.WriteString(" （接近上限）")
		} else {
			sb.WriteString(" ✅")
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("progress.md: 不存在\n")
	}

	sb.WriteString(fmt.Sprintf("handoff: %s\n", handoffStatus))

	if archiveLines > 0 {
		sb.WriteString(fmt.Sprintf("archive: %d 行\n", archiveLines))
	}

	// Count recent runs
	runsDir := filepath.Join(root, "runs")
	recentRuns := countRecentRuns(runsDir, 24*time.Hour)
	sb.WriteString(fmt.Sprintf("最近 24h runs: %d\n", recentRuns))

	if progressLines > 120 {
		sb.WriteString("\n建议: 运行 /archive 清理 progress.md 中的历史内容")
	}

	fmt.Print(sb.String())
}

func cmdMemoryDraftGenerate(root, mode string) {
	workDir := resolveMemoryRoot(root)

	runID := genRunID("memory-draft")
	runDir := filepath.Join(root, "runs", runID)
	os.MkdirAll(runDir, 0755)

	now := time.Now().UTC().Format(time.RFC3339)
	writeStatusJSON(runDir, statusJSON{
		RunID: runID, Kind: "memory-draft", Status: "running",
		Stage: "drafting", StartedAt: now, UpdatedAt: now,
	})

	sendCallback(runDir, fmt.Sprintf("⏳ 生成%s草稿中\nRun ID: %s", modeLabel(mode), runID))

	context := gatherMemoryContext(root, workDir)
	if context == "" {
		msg := "没有找到可用的项目记忆文件"
		writeError(runDir, fmt.Errorf("%s", msg))
		sendCallback(runDir, msg)
		fmt.Println(msg)
		os.Exit(1)
	}

	systemPrompt := memoryDraftSystemPrompt(mode)
	userPrompt := memoryDraftUserPrompt(mode, context)

	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		start := time.Now()
		for {
			select {
			case <-ticker.C:
				elapsed := int(time.Since(start).Seconds())
				sendCallback(runDir, heartbeatMsg("记忆草稿", runID, elapsed, ""))
			case <-done:
				return
			}
		}
	}()

	resp, err := doReviewCall([]chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, CodexBackendGLM)

	if err != nil {
		writeError(runDir, err)
		sendCallback(runDir, fmt.Sprintf("❌ 草稿生成失败: %s\nRun ID: %s", truncate(err.Error(), 200), runID))
		os.Exit(1)
	}

	os.WriteFile(filepath.Join(runDir, "memory-draft.md"), []byte(resp), 0600)

	shortSummary := extractDraftSummary(resp, mode, runID)
	os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(shortSummary), 0644)

	writeStatusJSON(runDir, statusJSON{
		RunID: runID, Kind: "memory-draft", Status: "completed",
		Stage: "done", StartedAt: now, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	setExitCode(runDir, 0)
	appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), RunID: runID, Type: "completed"})

	sendCallback(runDir, shortSummary)
	fmt.Println(shortSummary)
}

func resolveMemoryRoot(root string) string {
	if dir := os.Getenv("CC_MEMORY_ROOT"); dir != "" {
		return filepath.Clean(dir)
	}
	return filepath.Dir(root)
}

func gatherMemoryContext(root, workDir string) string {
	var parts []string

	if data, err := os.ReadFile(filepath.Join(workDir, "progress.md")); err == nil {
		content := string(data)
		if len(content) > 8000 {
			content = content[:8000] + "\n... (截断)"
		}
		parts = append(parts, "## progress.md\n"+content)
	}

	today := time.Now().Format("2006-01-02")
	handoffPath := filepath.Join(workDir, "handoff-"+today+".md")
	if data, err := os.ReadFile(handoffPath); err == nil {
		parts = append(parts, "## handoff (today)\n"+string(data))
	} else {
		latestHandoff := findLatestHandoff(workDir)
		if latestHandoff != "" {
			if data, err := os.ReadFile(latestHandoff); err == nil {
				content := string(data)
				if len(content) > 4000 {
					content = content[:4000] + "\n... (截断)"
				}
				parts = append(parts, "## handoff (latest: "+filepath.Base(latestHandoff)+")\n"+content)
			}
		}
	}

	runsDir := filepath.Join(root, "runs")
	recentSummaries := gatherRecentRunSummaries(runsDir, 5)
	if len(recentSummaries) > 0 {
		parts = append(parts, "## 最近完成的 runs\n"+strings.Join(recentSummaries, "\n---\n"))
	}

	return strings.Join(parts, "\n\n")
}

func gatherRecentRunSummaries(runsDir string, limit int) []string {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})

	var summaries []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		summaryPath := filepath.Join(runsDir, e.Name(), "summary.md")
		data, err := os.ReadFile(summaryPath)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		if len(content) > 1000 {
			content = content[:1000] + "..."
		}
		summaries = append(summaries, fmt.Sprintf("### %s\n%s", e.Name(), content))
		if len(summaries) >= limit {
			break
		}
	}
	return summaries
}

func countRecentRuns(runsDir string, window time.Duration) int {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return 0
	}
	cutoff := time.Now().Add(-window)
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if fi, err := e.Info(); err == nil && fi.ModTime().After(cutoff) {
			count++
		}
	}
	return count
}

func modeLabel(mode string) string {
	switch mode {
	case "summary":
		return "摘要"
	case "record":
		return "进度记录"
	default:
		return mode
	}
}

func extractDraftSummary(draft, mode, runID string) string {
	lines := strings.SplitN(draft, "\n", 8)
	preview := strings.Join(lines, "\n")
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}

	label := modeLabel(mode)
	return fmt.Sprintf("📝 %s草稿已生成\n\n%s\n\n完整草稿: runs/%s/memory-draft.md\nRun ID: %s\n\n⚠️ 这是草稿，确认后再落盘到 progress.md/handoff", label, preview, runID, runID)
}

func memoryDraftSystemPrompt(mode string) string {
	switch mode {
	case "summary":
		return `你是科研项目的记忆管理助手。根据提供的项目上下文（progress.md、handoff、最近 runs），生成一份简洁的阶段摘要。

要求：
- 中文输出
- 聚焦"完成了什么"、"关键决策"、"下一步"
- 不超过 500 字
- 不要发明不存在的事实
- 格式：先一句话总结，再分点列出关键内容`

	case "record":
		return `你是科研项目的记忆管理助手。根据提供的项目上下文，生成 progress.md 和 handoff 的更新草稿。

要求：
- 中文输出
- progress.md 格式：Current Objective / Current Status / Decisions / Completed / Next Steps / Risks
- handoff 格式：Current Goal / Phase Just Ended / Completed / Decisions / Important Files / Next Actions
- 只包含活跃信息，历史内容标注"建议 archive"
- 不要发明不存在的事实
- 先输出 "## progress.md 更新建议"，再输出 "## handoff 更新建议"`

	default:
		return "你是科研项目的记忆管理助手。"
	}
}

func memoryDraftUserPrompt(mode, context string) string {
	today := time.Now().Format("2006-01-02")
	switch mode {
	case "summary":
		return fmt.Sprintf("今天是 %s。请根据以下项目上下文生成阶段摘要草稿：\n\n%s", today, context)
	case "record":
		return fmt.Sprintf("今天是 %s。请根据以下项目上下文生成 progress.md 和 handoff-%s.md 的更新草稿：\n\n%s", today, today, context)
	default:
		return context
	}
}
