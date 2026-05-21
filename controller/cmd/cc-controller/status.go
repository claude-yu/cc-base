package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const staleThreshold = 10 * time.Minute

func writeStatusJSON(runDir string, s statusJSON) {
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(filepath.Join(runDir, "status.json"), data, 0644)
}

func updateStatusJSON(runDir, status, stage string, pid int) {
	path := filepath.Join(runDir, "status.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var s statusJSON
	if json.Unmarshal(data, &s) != nil {
		return
	}
	s.Status = status
	s.Stage = stage
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if pid > 0 {
		s.PID = pid
	}
	writeJSON(path, s)
}

func setExitCode(runDir string, code int) {
	os.WriteFile(filepath.Join(runDir, "runner.exitcode.txt"), []byte(fmt.Sprintf("%d", code)), 0644)
	path := filepath.Join(runDir, "status.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var s statusJSON
	if json.Unmarshal(data, &s) != nil {
		return
	}
	c := code
	s.ExitCode = &c
	writeJSON(path, s)
}

func appendEvent(runDir string, e eventEntry) {
	path := filepath.Join(runDir, "events.jsonl")
	data, _ := json.Marshal(e)
	os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(data)
	f.Write([]byte("\n"))
}

func appendTranscript(root, sessionID, runID, role, text string) {
	sessionDir := filepath.Join(root, "sessions", sessionID)
	path := filepath.Join(sessionDir, "transcript.jsonl")
	e := transcriptEntry{
		Ts:    time.Now().UTC().Format(time.RFC3339),
		RunID: runID,
		Role:  role,
		Text:  text,
	}
	data, _ := json.Marshal(e)
	os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(data)
	f.Write([]byte("\n"))
}

func writeJSON(path string, v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, data, 0644)
}

func showRun(root, runID, kind string) {
	runsRoot := filepath.Join(root, "runs")
	if runID == "" && kind != "" {
		runID = findLatestRunByKind(runsRoot, kind)
		if runID == "" {
			fmt.Fprintf(os.Stderr, "没有 %s 类型的 run 记录\n", kind)
			os.Exit(1)
		}
	}
	if runID == "" {
		sessionID := currentSessionID(root)
		runID = findLatestMeaningfulRunForSession(runsRoot, "", sessionID)
		if runID == "" {
			runID = findLatestRun(runsRoot)
		}
		if runID == "" {
			fmt.Fprintln(os.Stderr, "没有任何 run 记录")
			os.Exit(1)
		}
	}
	runDir := filepath.Join(runsRoot, runID)
	if fi, err := os.Stat(runDir); err != nil || !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "Run not found: %s\n", runID)
		os.Exit(1)
	}

	if data, err := os.ReadFile(filepath.Join(runDir, "status.json")); err == nil {
		fmt.Printf("%s\n\n", string(data))
	} else {
		fmt.Printf("Run ID: %s\n状态: %s\n\n", runID, runStatus(runDir))
	}

	for _, name := range []string{"incoming-question.md", "incoming-message.txt", "cc-answer.md", "codex-answer.md", "summary.md"} {
		data, err := os.ReadFile(filepath.Join(runDir, name))
		if err != nil {
			continue
		}
		fmt.Printf("=== %s ===\n%s\n\n", name, string(data))
	}
}

func findLatestRunByKind(runsRoot, kind string) string {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return ""
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && runIDPattern.MatchString(e.Name()) && strings.Contains(e.Name(), kind) {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		return ""
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i] > dirs[j] })
	return dirs[0]
}

func findLatestRun(runsRoot string) string {
	return findLatestRunExcluding(runsRoot, "")
}

func findLatestRunExcluding(runsRoot, excludeID string) string {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return ""
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() || !runIDPattern.MatchString(e.Name()) || e.Name() == excludeID {
			continue
		}
		// Skip status query runs (they have an is-status-query marker)
		if _, err := os.Stat(filepath.Join(runsRoot, e.Name(), "is-status-query")); err == nil {
			continue
		}
		dirs = append(dirs, e.Name())
	}
	if len(dirs) == 0 {
		return ""
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i] > dirs[j] })
	return dirs[0]
}

func runStatus(runDir string) string {
	if data, err := os.ReadFile(filepath.Join(runDir, "runner.exitcode.txt")); err == nil {
		switch code := trimToken(string(data)); code {
		case "0":
			return "DONE"
		case "-1":
			return "CANCELLED"
		default:
			return "FAILED (exit " + code + ")"
		}
	}
	if data, err := os.ReadFile(filepath.Join(runDir, "runner.pid")); err == nil {
		if pid, err := strconv.Atoi(trimToken(string(data))); err == nil && isProcessRunning(pid) {
			return "RUNNING"
		}
	}
	return "UNKNOWN"
}

// readStatusJSON reads and parses a run's status.json.
func readStatusJSON(runDir string) statusJSON {
	data, err := os.ReadFile(filepath.Join(runDir, "status.json"))
	if err != nil {
		return statusJSON{Status: "unknown"}
	}
	var s statusJSON
	if json.Unmarshal(data, &s) != nil {
		return statusJSON{Status: "unknown"}
	}
	return s
}

func humanDuration(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	default:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	}
}

type runSummary struct {
	RunID      string
	Status     string
	Stage      string
	UpdatedAgo string
	Question   string
}

func summarizeRun(runDir, runID string) runSummary {
	s := readStatusJSON(runDir)
	updated := s.UpdatedAt
	if updated == "" {
		updated = s.StartedAt
	}
	ago := "?"
	if t, err := time.Parse(time.RFC3339, updated); err == nil {
		ago = humanDuration(t)
	}
	q := ""
	for _, name := range []string{"incoming-question.md", "incoming-message.txt"} {
		if data, err := os.ReadFile(filepath.Join(runDir, name)); err == nil {
			q = strings.TrimSpace(string(data))
			if len(q) > 80 {
				q = q[:80] + "..."
			}
			break
		}
	}
	return runSummary{
		RunID:      runID,
		Status:     runStatus(runDir),
		Stage:      s.Stage,
		UpdatedAgo: ago,
		Question:   q,
	}
}

func findStaleRuns(runsRoot string) []runSummary {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return nil
	}
	var stale []runSummary
	for _, e := range entries {
		if !e.IsDir() || !runIDPattern.MatchString(e.Name()) {
			continue
		}
		runDir := filepath.Join(runsRoot, e.Name())
		s := readStatusJSON(runDir)
		if s.Status != "running" && s.Status != "accepted" {
			continue
		}
		ts := s.UpdatedAt
		if ts == "" {
			ts = s.StartedAt
		}
		if ts == "" {
			continue
		}
		updated, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		if time.Since(updated) > staleThreshold {
			stale = append(stale, summarizeRun(runDir, e.Name()))
		}
	}
	sort.Slice(stale, func(i, j int) bool { return stale[i].RunID > stale[j].RunID })
	return stale
}

func findActiveRuns(runsRoot string) []runSummary {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return nil
	}
	var active []runSummary
	for _, e := range entries {
		if !e.IsDir() || !runIDPattern.MatchString(e.Name()) {
			continue
		}
		runDir := filepath.Join(runsRoot, e.Name())
		if runStatus(runDir) == "RUNNING" {
			active = append(active, summarizeRun(runDir, e.Name()))
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].RunID > active[j].RunID })
	return active
}

// cmdStatusShort shows a condensed 3-line status for mobile screens.
func formatStatusShort(root string) string {
	runsRoot := filepath.Join(root, "runs")
	p := readActiveProject(root)
	var sb strings.Builder

	fmt.Fprintf(&sb, "📂 %s  %s\n", p.Name, p.WorkDir)

	active := findActiveRuns(runsRoot)
	pruned := queuePrune(root)
	queueEntries := readQueue(root)
	if len(active) > 0 {
		latest := active[0]
		fmt.Fprintf(&sb, "▶ 活动 %d 个 | %s (%s)", len(active), latest.Stage, latest.UpdatedAgo)
	} else {
		fmt.Fprintf(&sb, "▶ 活动 0 个")
	}
	if len(queueEntries) > 0 {
		fmt.Fprintf(&sb, " | 待确认 %d", len(queueEntries))
	}
	if pruned > 0 {
		fmt.Fprintf(&sb, " (已清理 %d 过期)", pruned)
	}
	sb.WriteString("\n")

	latestDone := findLatestCompletedRun(runsRoot)
	if latestDone.RunID != "" {
		result := latestDone.Status
		if latestDone.Question != "" {
			q := latestDone.Question
			if len(q) > 40 {
				q = q[:40] + "…"
			}
			result += " " + q
		}
		fmt.Fprintf(&sb, "✓ 最近完成: %s (%s)\n", result, latestDone.UpdatedAgo)
	} else {
		sb.WriteString("✓ 最近完成: 无\n")
	}
	return sb.String()
}

func cmdStatusShort(root string) {
	fmt.Print(formatStatusShort(root))
}

func findLatestCompletedRun(runsRoot string) runSummary {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return runSummary{}
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && runIDPattern.MatchString(e.Name()) {
			dirs = append(dirs, e.Name())
		}
	}
	// Sort newest first
	sort.Slice(dirs, func(i, j int) bool { return dirs[i] > dirs[j] })
	for _, d := range dirs {
		runDir := filepath.Join(runsRoot, d)
		st := runStatus(runDir)
		if st == "DONE" || st == "CANCELLED" || strings.HasPrefix(st, "FAILED") {
			return summarizeRun(runDir, d)
		}
	}
	return runSummary{}
}

// cmdStatus shows a comprehensive system status dashboard.
func cmdStatus(root string) {
	runsRoot := filepath.Join(root, "runs")

	// ── Project info ──
	p := readActiveProject(root)
	session := p.ProjectID + "-default"
	fmt.Printf("=== 系统状态 ===\n")
	fmt.Printf("项目:   %s\n", p.Name)
	fmt.Printf("目录:   %s\n", p.WorkDir)
	fmt.Printf("Session: %s\n", session)
	fmt.Println()

	// ── Waiting queue (auto-prune stale entries) ──
	pruned := queuePrune(root)
	if pruned > 0 {
		fmt.Printf("(已自动清理 %d 个过期待确认任务)\n", pruned)
	}
	fmt.Println(describeQueue(root))
	fmt.Println()

	// ── Active runs ──
	active := findActiveRuns(runsRoot)
	if len(active) > 0 {
		fmt.Printf("⚠️  活动任务: %d 个\n", len(active))
		for _, r := range active {
			fmt.Printf("  RunId:  %s\n", r.RunID)
			fmt.Printf("  状态:   %s (%s)\n", r.Stage, r.UpdatedAgo)
			if r.Question != "" {
				fmt.Printf("  消息:   %s\n", r.Question)
			}
			fmt.Printf("  取消:   /取消任务 %s\n", r.RunID)
			fmt.Println()
		}
	} else {
		fmt.Println("活动任务: 无")
		fmt.Println()
	}

	// ── Stale runs ──
	stale := findStaleRuns(runsRoot)
	if len(stale) > 0 {
		fmt.Printf("⚠️  疑似卡住 (超过 %d 分钟无更新): %d 个\n", int(staleThreshold.Minutes()), len(stale))
		for _, r := range stale {
			fmt.Printf("  RunId:  %s\n", r.RunID)
			fmt.Printf("  状态:   %s (最后更新 %s)\n", r.Stage, r.UpdatedAgo)
			if r.Question != "" {
				fmt.Printf("  消息:   %s\n", r.Question)
			}
			fmt.Printf("  取消:   /取消任务 %s\n", r.RunID)
			fmt.Println()
		}
	} else {
		fmt.Println("卡住检测: 无 (10分钟阈值)")
		fmt.Println()
	}

	// ── Latest records by kind ──
	fmt.Println("=== 最近记录 ===")
	latestCC := findLatestRunByKind(runsRoot, "cc-session")
	if latestCC != "" {
		s := summarizeRun(filepath.Join(runsRoot, latestCC), latestCC)
		fmt.Printf("/cc:   %s → %s (%s)\n", latestCC, s.Status, s.UpdatedAgo)
	} else {
		fmt.Println("/cc:   无记录")
	}
	latestCodex := findLatestRunByKind(runsRoot, "codex-ask")
	if latestCodex != "" {
		s := summarizeRun(filepath.Join(runsRoot, latestCodex), latestCodex)
		fmt.Printf("Codex: %s → %s (%s)\n", latestCodex, s.Status, s.UpdatedAgo)
	} else {
		fmt.Println("Codex: 无记录")
	}
	fmt.Println()

	// ── Quick actions ──
	fmt.Println("=== 快速操作 ===")
	fmt.Println("/cc <消息>        开始对话")
	fmt.Println("/项目             查看项目信息")
	fmt.Println("/切项目 <名称>    切换项目")
}
