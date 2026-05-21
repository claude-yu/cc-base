package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func cmdReviewLocal(root string, args []string) {
	preset := "security"
	if len(args) > 0 && args[0] != "" {
		preset = strings.ToLower(args[0])
	}

	p, ok := reviewPresets[preset]
	if !ok {
		fmt.Fprintf(os.Stderr, "未知 preset: %s（可用: security, routing, general）\n", preset)
		os.Exit(1)
	}

	workDir := resolveProjectWorkDir()

	diff := getLocalDiff(workDir)
	if diff == "" {
		fmt.Println("当前没有可审查改动")
		os.Exit(0)
	}
	if len(diff) > 64*1024 {
		fmt.Fprintln(os.Stderr, "diff 过大（超过 64KB），请缩小范围")
		os.Exit(1)
	}

	runID := genRunID("review-local")
	runDir := filepath.Join(root, "runs", runID)
	os.MkdirAll(runDir, 0755)

	now := time.Now().UTC().Format(time.RFC3339)
	writeStatusJSON(runDir, statusJSON{
		RunID: runID, Kind: "review-local", Status: "running",
		Stage: "reviewing", StartedAt: now, UpdatedAt: now,
	})
	os.WriteFile(filepath.Join(runDir, "review-diff.txt"), []byte(diff), 0644)

	sendCallback(runDir, fmt.Sprintf("⏳ 代码审查中（%s）\nRun ID: %s", preset, runID))

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		start := time.Now()
		for {
			select {
			case <-ticker.C:
				elapsed := int(time.Since(start).Seconds())
				sendCallback(runDir, heartbeatMsg("审查", runID, elapsed, "preset: "+preset))
			case <-done:
				return
			}
		}
	}()

	backend := p.defaultBackend
	resp, err := callReviewBackend(diff, backend, p.systemPrompt)
	close(done)

	if err != nil {
		writeError(runDir, err)
		sendCallback(runDir, fmt.Sprintf("❌ 审查失败: %s\nRun ID: %s", truncate(err.Error(), 200), runID))
		os.Exit(1)
	}

	report, parseErr := extractReviewJSON(resp)
	if parseErr != nil {
		resp2, retryErr := callReviewBackendRetry(diff, resp, backend, p.systemPrompt)
		if retryErr != nil {
			writeError(runDir, retryErr)
			sendCallback(runDir, fmt.Sprintf("❌ 审查解析失败\nRun ID: %s", runID))
			os.Exit(1)
		}
		report, parseErr = extractReviewJSON(resp2)
		if parseErr != nil {
			writeError(runDir, fmt.Errorf("JSON parse: %w", parseErr))
			sendCallback(runDir, fmt.Sprintf("❌ 审查结果无法解析\nRun ID: %s", runID))
			os.Exit(1)
		}
	}

	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(filepath.Join(runDir, "review-report.json"), reportJSON, 0644)

	summary := formatReviewMobile(report, preset, runID)
	os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(summary), 0644)

	writeStatusJSON(runDir, statusJSON{
		RunID: runID, Kind: "review-local", Status: "completed",
		Stage: "done", StartedAt: now, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	sendCallback(runDir, summary)
	fmt.Println(summary)
}

func getLocalDiff(workDir string) string {
	cmd := exec.Command("git", "diff")
	cmd.Dir = workDir
	unstaged, _ := cmd.Output()

	cmd2 := exec.Command("git", "diff", "--cached")
	cmd2.Dir = workDir
	staged, _ := cmd2.Output()

	combined := strings.TrimSpace(string(unstaged)) + "\n" + strings.TrimSpace(string(staged))
	return strings.TrimSpace(combined)
}

func formatReviewMobile(report *ReviewReport, preset, runID string) string {
	var sb strings.Builder

	var icon string
	switch report.Verdict {
	case VerdictPass:
		icon = "✅"
	case VerdictWarn:
		icon = "⚠️"
	case VerdictFail:
		icon = "❌"
	default:
		icon = "🔍"
	}
	sb.WriteString(fmt.Sprintf("%s 审查结果: %s (%s)\n", icon, report.Verdict, preset))

	if report.Summary != "" {
		sb.WriteString(report.Summary + "\n")
	}

	count := 0
	for _, f := range report.Findings {
		sev := strings.ToUpper(f.Severity)
		if sev != "CRITICAL" && sev != "HIGH" && sev != "MEDIUM" {
			continue
		}
		count++
		sb.WriteString(fmt.Sprintf("\n[%s] %s", sev, f.Title))
		if f.File != "" {
			if f.Line > 0 {
				sb.WriteString(fmt.Sprintf(" (%s:%d)", f.File, f.Line))
			} else {
				sb.WriteString(fmt.Sprintf(" (%s)", f.File))
			}
		}
	}

	lowCount := 0
	for _, f := range report.Findings {
		if strings.ToUpper(f.Severity) == "LOW" {
			lowCount++
		}
	}
	if lowCount > 0 {
		sb.WriteString(fmt.Sprintf("\n\n另有 %d 个 LOW 级别发现（见完整报告）", lowCount))
	}

	sb.WriteString(fmt.Sprintf("\n\nRun ID: %s", runID))
	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
