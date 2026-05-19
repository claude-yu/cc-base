package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultScanDepth        = 3
	defaultMaxLogLines      = 80
	defaultMobileLines      = 5
	recentThresholdMins     = 24 * 60     // <24h = active_failed
	archivedThresholdMins   = 7 * 24 * 60 // >7d = archived_failed
)

func cmdResearchMonitor(root string, args []string) {
	filterDetector := ""
	var positional []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--detector" && i+1 < len(args) {
			filterDetector = args[i+1]
			i++
		} else {
			positional = append(positional, args[i])
		}
	}

	subcmd := strings.TrimSpace(strings.Join(positional, " "))

	switch {
	case subcmd == "" || subcmd == "刷新":
		doFullScan(root, filterDetector)
	case subcmd == "历史":
		showFiltered(root, "historical")
	case subcmd == "归档":
		showFiltered(root, "archived")
	case subcmd == "全部":
		showFiltered(root, "all")
	case isNumeric(subcmd):
		showTaskDetail(root, atoiSafe(subcmd))
	default:
		if strings.HasSuffix(subcmd, " 全部") {
			showTaskByKeyword(root, strings.TrimSuffix(subcmd, " 全部"), "all")
		} else if strings.HasSuffix(subcmd, " 归档") {
			showTaskByKeyword(root, strings.TrimSuffix(subcmd, " 归档"), "archived")
		} else {
			showTaskByKeyword(root, subcmd, "default")
		}
	}
}

func doFullScan(root, filterDetector string) {
	workDir := resolveMonitorRoot()

	scanDepth := defaultScanDepth
	if v := os.Getenv("CC_RESEARCH_SCAN_DEPTH"); v != "" {
		if n := atoiSafe(v); n > 0 {
			scanDepth = n
		}
	}

	results := scanProject(workDir, scanDepth, filterDetector)

	if filterDetector == "" || strings.HasPrefix(filterDetector, "docker") {
		dockerResults := scanDockerContainers()
		results = append(results, dockerResults...)
	}

	results = deduplicateByWorkDir(results)

	sort.SliceStable(results, func(i, j int) bool {
		pi := statePriority[results[i].State]
		pj := statePriority[results[j].State]
		if pi != pj {
			return pi < pj
		}
		return results[i].Score > results[j].Score
	})

	for i := range results {
		results[i].Index = i + 1
		results[i].Bucket = classifyBucket(results[i])
	}

	runID := genRunID("research-monitor")
	runDir := filepath.Join(root, "runs", runID)
	os.MkdirAll(runDir, 0755)

	writeDetailReport(runDir, results, workDir)
	writeStatusJSON(runDir, statusJSON{
		RunID:     runID,
		Kind:      "research-monitor",
		Status:    "completed",
		Stage:     "completed",
		StartedAt: time.Now().Format(timeFormat),
		UpdatedAt: time.Now().Format(timeFormat),
	})

	writeFile(filepath.Join(root, "latest-monitor-run.txt"), runID)

	summary := formatMobileSummary(results, runDir)
	fmt.Print(summary)
}

func deduplicateByWorkDir(results []ResearchStatus) []ResearchStatus {
	bestScore := map[string]int{}
	for _, rs := range results {
		key := dedupeKey(rs)
		if rs.Score > bestScore[key] {
			bestScore[key] = rs.Score
		}
	}
	var deduped []ResearchStatus
	seen := map[string]bool{}
	for _, rs := range results {
		key := dedupeKey(rs)
		if seen[key] {
			continue
		}
		if rs.Score == bestScore[key] {
			deduped = append(deduped, rs)
			seen[key] = true
		}
	}
	return deduped
}

func dedupeKey(rs ResearchStatus) string {
	if rs.WorkDir != "" {
		return rs.WorkDir
	}
	if len(rs.KeyFiles) > 0 {
		return rs.Detector + ":" + rs.KeyFiles[0]
	}
	return rs.Detector
}

func classifyBucket(rs ResearchStatus) string {
	if rs.State == "stuck" {
		if rs.LastUpdateMins > archivedThresholdMins {
			return "archived_stuck"
		}
		return ""
	}
	if rs.State != "failed" {
		return ""
	}
	if rs.LastUpdateMins < 0 {
		return "active_failed"
	}
	if rs.LastUpdateMins <= recentThresholdMins {
		return "active_failed"
	}
	if rs.LastUpdateMins <= archivedThresholdMins {
		return "historical_failed"
	}
	return "archived_failed"
}

func absTimeShort(mins int) string {
	if mins < 0 {
		return ""
	}
	return time.Now().Add(-time.Duration(mins) * time.Minute).Format("01-02 15:04")
}

func absTimeFull(mins int) string {
	if mins < 0 {
		return ""
	}
	return time.Now().Add(-time.Duration(mins) * time.Minute).Format("2006-01-02 15:04:05")
}

func taskLine(rs ResearchStatus, timeMode string) string {
	line := itoa(rs.Index) + ". [" + stateTag(rs) + "]"
	switch timeMode {
	case "short":
		if t := absTimeShort(rs.LastUpdateMins); t != "" {
			line += " " + t
		}
	case "full":
		if t := absTimeFull(rs.LastUpdateMins); t != "" {
			line += " " + t
		}
	}
	line += " " + detectorShortName(rs.Detector) + " — " + filepath.Base(rs.WorkDir)
	return line
}

func isArchived(rs ResearchStatus) bool {
	return rs.Bucket == "archived_failed" || rs.Bucket == "archived_stuck"
}

func scanTimeFromRunID(runID string) string {
	if len(runID) < 15 {
		return ""
	}
	t, err := time.Parse("20060102-150405", runID[:15])
	if err != nil {
		return ""
	}
	return t.Format("01-02 15:04")
}

func loadLatestResults(root string) ([]ResearchStatus, string) {
	data, err := os.ReadFile(filepath.Join(root, "latest-monitor-run.txt"))
	if err != nil {
		return nil, ""
	}
	runID := strings.TrimSpace(string(data))
	if runID == "" {
		return nil, ""
	}
	jdata, err := os.ReadFile(filepath.Join(root, "runs", runID, "monitor-results.json"))
	if err != nil {
		return nil, ""
	}
	var results []ResearchStatus
	if json.Unmarshal(jdata, &results) != nil {
		return nil, ""
	}
	return results, runID
}

func showTaskDetail(root string, index int) {
	results, _ := loadLatestResults(root)
	if results == nil {
		fmt.Println("尚无扫描结果，请先执行 /科研监控")
		return
	}
	for _, rs := range results {
		if rs.Index == index {
			fmt.Print(formatTaskDetail(rs))
			return
		}
	}
	fmt.Println("未找到任务 #" + itoa(index) + "（共 " + itoa(len(results)) + " 个任务）")
}

const keywordMaxDisplay = 10

func showTaskByKeyword(root, keyword, mode string) {
	results, runID := loadLatestResults(root)
	if results == nil {
		fmt.Println("尚无扫描结果，请先执行 /科研监控")
		return
	}
	lower := strings.ToLower(keyword)
	var matches []ResearchStatus
	for _, rs := range results {
		if strings.Contains(strings.ToLower(rs.Detector), lower) ||
			strings.Contains(strings.ToLower(filepath.Base(rs.WorkDir)), lower) ||
			strings.Contains(strings.ToLower(rs.ContextPhase), lower) {
			matches = append(matches, rs)
		}
	}
	if len(matches) == 0 {
		fmt.Println("未找到匹配 \"" + keyword + "\" 的任务")
		return
	}
	if len(matches) == 1 {
		fmt.Print(formatTaskDetail(matches[0]))
		return
	}

	tsSuffix := ""
	if ts := scanTimeFromRunID(runID); ts != "" {
		tsSuffix = " (" + ts + " 扫描)"
	}

	var sb strings.Builder

	switch mode {
	case "archived":
		var arch []ResearchStatus
		for _, rs := range matches {
			if isArchived(rs) {
				arch = append(arch, rs)
			}
		}
		if len(arch) == 0 {
			fmt.Println("\"" + keyword + "\" 无归档任务")
			return
		}
		sb.WriteString("\"" + keyword + "\" 归档: " + itoa(len(arch)) + " 个" + tsSuffix + "\n")
		for _, rs := range arch {
			sb.WriteString(taskLine(rs, "short") + "\n")
		}

	case "all":
		sb.WriteString("匹配 \"" + keyword + "\": " + itoa(len(matches)) + " 个" + tsSuffix + "\n")
		for _, rs := range matches {
			sb.WriteString(taskLine(rs, "short") + "\n")
		}

	default:
		var recent, arch []ResearchStatus
		for _, rs := range matches {
			if isArchived(rs) {
				arch = append(arch, rs)
			} else {
				recent = append(recent, rs)
			}
		}
		sb.WriteString("匹配 \"" + keyword + "\": " + itoa(len(matches)) + " 个任务" + tsSuffix + "\n")
		if len(recent) > 0 {
			display := recent
			truncated := false
			if len(recent) > keywordMaxDisplay {
				display = recent[:keywordMaxDisplay]
				truncated = true
			}
			for _, rs := range display {
				sb.WriteString(taskLine(rs, "short") + "\n")
			}
			if truncated {
				sb.WriteString("... 显示 " + itoa(len(display)) + "/" + itoa(len(recent)) + "。更多: /科研监控 " + keyword + " 全部\n")
			}
		}
		if len(arch) > 0 {
			sb.WriteString("\n归档 (>7天): " + itoa(len(arch)) + " 个（/科研监控 " + keyword + " 归档）\n")
		}
		if len(recent) == 0 && len(arch) > 0 {
			sb.WriteString("近期无匹配，全部已归档\n")
		}
	}

	sb.WriteString("\n用 /科研监控 <序号> 查看详情")
	fmt.Print(sb.String())
}

func showFiltered(root, filter string) {
	results, runID := loadLatestResults(root)
	if results == nil {
		fmt.Println("尚无扫描结果，请先执行 /科研监控")
		return
	}
	tsSuffix := ""
	if ts := scanTimeFromRunID(runID); ts != "" {
		tsSuffix = " (" + ts + " 扫描)"
	}

	switch filter {
	case "historical":
		var hist, arch []ResearchStatus
		for _, rs := range results {
			switch rs.Bucket {
			case "historical_failed":
				hist = append(hist, rs)
			case "archived_failed", "archived_stuck":
				arch = append(arch, rs)
			}
		}
		if len(hist) == 0 && len(arch) == 0 {
			fmt.Println("无历史/归档异常")
			return
		}
		var sb strings.Builder
		if tsSuffix != "" {
			sb.WriteString("数据" + tsSuffix + "\n\n")
		}
		if len(hist) > 0 {
			sb.WriteString("历史异常 (1-7天): " + itoa(len(hist)) + " 个\n")
			for _, rs := range hist {
				sb.WriteString(taskLine(rs, "short") + "\n")
			}
		}
		if len(arch) > 0 {
			if len(hist) > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("归档 (>7天): " + itoa(len(arch)) + " 个（/科研监控 归档）\n")
		}
		sb.WriteString("\n用 /科研监控 <序号> 查看详情")
		fmt.Print(sb.String())

	case "archived":
		var arch []ResearchStatus
		for _, rs := range results {
			if isArchived(rs) {
				arch = append(arch, rs)
			}
		}
		if len(arch) == 0 {
			fmt.Println("无归档任务")
			return
		}
		var sb strings.Builder
		sb.WriteString("归档任务 (>7天): " + itoa(len(arch)) + " 个" + tsSuffix + "\n")
		display := arch
		if len(arch) > keywordMaxDisplay {
			display = arch[:keywordMaxDisplay]
		}
		for _, rs := range display {
			sb.WriteString(taskLine(rs, "short") + "\n")
		}
		if len(arch) > keywordMaxDisplay {
			sb.WriteString("... 显示 " + itoa(keywordMaxDisplay) + "/" + itoa(len(arch)) + "。更多: /科研监控 全部\n")
		}
		sb.WriteString("\n用 /科研监控 <序号> 查看详情")
		fmt.Print(sb.String())

	case "all":
		var sb strings.Builder
		sb.WriteString("全部任务: " + itoa(len(results)) + " 个" + tsSuffix + "\n\n")
		for _, rs := range results {
			sb.WriteString(taskLine(rs, "short") + "\n")
		}
		sb.WriteString("\n用 /科研监控 <序号> 查看详情")
		fmt.Print(sb.String())
	}
}

func formatTaskDetail(rs ResearchStatus) string {
	var sb strings.Builder
	sb.WriteString("#" + itoa(rs.Index) + " [" + strings.ToUpper(rs.State) + "] " + detectorShortName(rs.Detector) + "\n\n")

	sb.WriteString("Detector: " + rs.Detector + "\n")
	sb.WriteString("WorkDir: " + rs.WorkDir + "\n")
	if rs.ContextPhase != "" {
		sb.WriteString("Phase: " + rs.ContextPhase + "\n")
	}
	if rs.LastUpdateMins >= 0 {
		sb.WriteString("最近更新: " + absTimeFull(rs.LastUpdateMins) + "\n")
	}
	if rs.Bucket != "" {
		sb.WriteString("分类: " + bucketLabel(rs.Bucket) + "\n")
	}
	sb.WriteString("Confidence: " + rs.Confidence + " | Score: " + itoa(rs.Score) + "\n")

	if len(rs.KeyFiles) > 0 {
		sb.WriteString("\n关键文件:\n")
		for _, f := range rs.KeyFiles {
			sb.WriteString("- " + f + "\n")
		}
	}

	sb.WriteString("\n判断依据:\n")
	for _, e := range rs.Evidence {
		sb.WriteString("- " + e + "\n")
	}

	if len(rs.Warnings) > 0 {
		sb.WriteString("\n警告:\n")
		for _, w := range rs.Warnings {
			sb.WriteString("- " + w + "\n")
		}
	}

	if len(rs.NextActions) > 0 {
		sb.WriteString("\n建议下一步:\n")
		for _, a := range rs.NextActions {
			sb.WriteString("- " + a + "\n")
		}
	}

	return sb.String()
}

func stateTag(rs ResearchStatus) string {
	switch rs.Bucket {
	case "archived_failed":
		return "ARCHIVED"
	case "archived_stuck":
		return "STUCK/归档"
	case "historical_failed":
		return "FAILED/历史"
	default:
		return strings.ToUpper(rs.State)
	}
}

func bucketLabel(bucket string) string {
	switch bucket {
	case "active_failed":
		return "近期失败 (<24h)"
	case "historical_failed":
		return "历史失败 (1-7天)"
	case "archived_failed":
		return "归档失败 (>7天)"
	case "archived_stuck":
		return "归档卡住 (>7天)"
	default:
		return bucket
	}
}

func resolveMonitorRoot() string {
	if v := os.Getenv("CC_RESEARCH_MONITOR_ROOT"); v != "" {
		return v
	}
	return resolveProjectWorkDir()
}

// scanProject walks workDir up to maxDepth, running all detectors on each directory.
func scanProject(workDir string, maxDepth int, filterDetector string) []ResearchStatus {
	detectors := allDetectors()
	if filterDetector != "" {
		var filtered []Detector
		for _, d := range detectors {
			if d.Name() == filterDetector {
				filtered = append(filtered, d)
			}
		}
		if len(filtered) > 0 {
			detectors = filtered
		}
	}

	type dirMatch struct {
		dir      string
		detector string
	}
	seen := map[dirMatch]bool{}
	var results []ResearchStatus

	walkFn := func(dir string, depth int) {
		for _, det := range detectors {
			key := dirMatch{dir, det.Name()}
			if seen[key] {
				continue
			}
			matched, score := det.Match(dir)
			if !matched {
				continue
			}
			seen[key] = true
			rs := det.Inspect(dir)
			rs.Score = score
			results = append(results, rs)
		}
	}

	walkFn(workDir, 0)
	walkDirs(workDir, 1, maxDepth, walkFn)

	return results
}

func walkDirs(dir string, currentDepth, maxDepth int, fn func(string, int)) {
	if currentDepth > maxDepth {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if scanExcludeDirs[name] || strings.HasPrefix(name, ".") {
			continue
		}
		subDir := filepath.Join(dir, name)
		fn(subDir, currentDepth)
		walkDirs(subDir, currentDepth+1, maxDepth, fn)
	}
}

func formatMobileSummary(results []ResearchStatus, runDir string) string {
	if len(results) == 0 {
		return "科研监控: 未发现任何可识别的科研任务\n建议: 确认 CC_WORK_DIR 或 CC_RESEARCH_MONITOR_ROOT 指向科研项目目录\n"
	}

	activeFailed := 0
	historicalFailed := 0
	archivedCount := 0
	stateCounts := map[string]int{}

	for _, rs := range results {
		switch rs.Bucket {
		case "active_failed":
			activeFailed++
		case "historical_failed":
			historicalFailed++
		case "archived_failed", "archived_stuck":
			archivedCount++
		default:
			stateCounts[rs.State]++
		}
	}

	failedDisplay := activeFailed + historicalFailed

	var sb strings.Builder
	sb.WriteString("科研监控: " + itoa(len(results)) + " 个任务 (" + time.Now().Format("01-02 15:04") + ")\n")

	var parts []string
	if failedDisplay > 0 {
		parts = append(parts, "FAILED "+itoa(failedDisplay))
	}
	if c := stateCounts["stuck"]; c > 0 {
		parts = append(parts, "STUCK "+itoa(c))
	}
	if c := stateCounts["running"]; c > 0 {
		parts = append(parts, "RUNNING "+itoa(c))
	}
	if c := stateCounts["completed"]; c > 0 {
		parts = append(parts, "COMPLETED "+itoa(c))
	}
	if archivedCount > 0 {
		parts = append(parts, "ARCHIVED "+itoa(archivedCount))
	}
	if c := stateCounts["idle"]; c > 0 {
		parts = append(parts, "IDLE "+itoa(c))
	}
	if len(parts) > 0 {
		sb.WriteString(strings.Join(parts, " | ") + "\n")
	}

	// High-priority: active_failed + active stuck (not archived_stuck)
	var alerts []ResearchStatus
	for _, rs := range results {
		if rs.Bucket == "active_failed" || (rs.State == "stuck" && rs.Bucket == "") {
			alerts = append(alerts, rs)
		}
	}
	if len(alerts) > 0 {
		sb.WriteString("\n高优先级异常:\n")
		for i, rs := range alerts {
			if i >= 3 {
				sb.WriteString("... 还有 " + itoa(len(alerts)-3) + " 个\n")
				break
			}
			line := itoa(rs.Index) + ". [" + strings.ToUpper(rs.State) + "] " + detectorShortName(rs.Detector)
			if rs.ContextPhase != "" {
				line += " — " + rs.ContextPhase
			} else if len(rs.Evidence) > 0 {
				ev := firstUsefulEvidence(rs.Evidence)
				if len(ev) > 50 {
					ev = ev[:50] + "..."
				}
				line += " — " + ev
			}
			sb.WriteString(line + "\n")
		}
	} else if stateCounts["running"] == 0 {
		sb.WriteString("\n当前无活跃异常或运行中任务。\n")
	}

	// Historical + archived merged hint
	if historicalFailed > 0 || archivedCount > 0 {
		sb.WriteString("\n")
		var hParts []string
		if historicalFailed > 0 {
			hParts = append(hParts, itoa(historicalFailed)+" 个历史")
		}
		if archivedCount > 0 {
			hParts = append(hParts, itoa(archivedCount)+" 个归档")
		}
		sb.WriteString("历史/归档: " + strings.Join(hParts, "，") + "（/科研监控 历史）\n")
	}

	// Running tasks (max 2)
	var running []ResearchStatus
	for _, rs := range results {
		if rs.State == "running" {
			running = append(running, rs)
		}
	}
	if len(running) > 0 {
		sb.WriteString("\n运行中:\n")
		for i, rs := range running {
			if i >= 2 {
				sb.WriteString("... 还有 " + itoa(len(running)-2) + " 个运行中\n")
				break
			}
			line := itoa(rs.Index) + ". " + detectorShortName(rs.Detector)
			if rs.ContextPhase != "" {
				line += " — " + rs.ContextPhase
			} else if rs.LastUpdate != "" {
				line += " — " + rs.LastUpdate + "更新"
			}
			sb.WriteString(line + "\n")
		}
	}

	sb.WriteString("\n查看详情: /科研监控 <序号>\n")
	sb.WriteString("刷新: /科研监控 刷新\n")

	return sb.String()
}

func detectorShortName(det string) string {
	if strings.HasPrefix(det, "docker:") {
		return strings.ToUpper(strings.TrimPrefix(det, "docker:"))
	}
	switch det {
	case "gromacs":
		return "GROMACS"
	case "python_pipeline":
		return "Python"
	case "r_pipeline":
		return "R"
	case "generic_cli":
		return "CLI"
	default:
		return det
	}
}

func firstUsefulEvidence(evidence []string) string {
	for _, e := range evidence {
		if strings.HasPrefix(e, "ERROR:") || strings.HasPrefix(e, "LOG-ERROR:") {
			return strings.TrimPrefix(strings.TrimPrefix(e, "ERROR: "), "LOG-ERROR: ")
		}
	}
	if len(evidence) > 0 {
		return evidence[0]
	}
	return ""
}

type detectorGroup struct {
	detector string
	results  []ResearchStatus
}

func groupByDetector(results []ResearchStatus) []detectorGroup {
	order := []string{}
	m := map[string][]ResearchStatus{}
	for _, rs := range results {
		if _, seen := m[rs.Detector]; !seen {
			order = append(order, rs.Detector)
		}
		m[rs.Detector] = append(m[rs.Detector], rs)
	}
	var groups []detectorGroup
	for _, det := range order {
		groups = append(groups, detectorGroup{detector: det, results: m[det]})
	}
	return groups
}

func writeDetailReport(runDir string, results []ResearchStatus, workDir string) {
	var sb strings.Builder
	sb.WriteString("# 科研任务监控报告\n\n")
	sb.WriteString("扫描目录: " + workDir + "\n")
	sb.WriteString("扫描时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	sb.WriteString("发现任务: " + itoa(len(results)) + "\n\n")

	if len(results) == 0 {
		sb.WriteString("未发现任何可识别的科研任务。\n\n")
		sb.WriteString("可能原因:\n")
		sb.WriteString("- 工作目录不正确\n")
		sb.WriteString("- 项目尚未初始化（无 .tpr/.py/.R 等特征文件）\n")
		sb.WriteString("- 扫描深度不足（当前: " + itoa(defaultScanDepth) + "，可通过 CC_RESEARCH_SCAN_DEPTH 调整）\n")
	}

	for _, rs := range results {
		sb.WriteString("---\n\n")
		sb.WriteString("## #" + itoa(rs.Index) + " " + rs.Detector + " — " + strings.ToUpper(rs.State))
		if rs.Bucket != "" {
			sb.WriteString(" [" + rs.Bucket + "]")
		}
		sb.WriteString("\n\n")

		sb.WriteString("| 属性 | 值 |\n|---|---|\n")
		sb.WriteString("| Detector | " + rs.Detector + " |\n")
		sb.WriteString("| State | " + rs.State + " |\n")
		sb.WriteString("| Bucket | " + rs.Bucket + " |\n")
		sb.WriteString("| Confidence | " + rs.Confidence + " |\n")
		sb.WriteString("| Score | " + itoa(rs.Score) + " |\n")
		sb.WriteString("| WorkDir | " + rs.WorkDir + " |\n")
		if rs.LastUpdate != "" {
			sb.WriteString("| LastUpdate | " + rs.LastUpdate + " |\n")
		}
		if rs.ContextPhase != "" {
			sb.WriteString("| Phase | " + rs.ContextPhase + " |\n")
		}
		sb.WriteString("\n")

		if len(rs.KeyFiles) > 0 {
			sb.WriteString("### Key Files\n\n")
			for _, f := range rs.KeyFiles {
				sb.WriteString("- " + f + "\n")
			}
			sb.WriteString("\n")
		}

		sb.WriteString("### Evidence\n\n")
		for _, e := range rs.Evidence {
			sb.WriteString("- " + e + "\n")
		}
		sb.WriteString("\n")

		if len(rs.Warnings) > 0 {
			sb.WriteString("### Warnings\n\n")
			for _, w := range rs.Warnings {
				sb.WriteString("- ⚠ " + w + "\n")
			}
			sb.WriteString("\n")
		}

		if len(rs.NextActions) > 0 {
			sb.WriteString("### Next Actions\n\n")
			for _, a := range rs.NextActions {
				sb.WriteString("- " + a + "\n")
			}
			sb.WriteString("\n")
		}
	}

	writeFile(filepath.Join(runDir, "report.md"), sb.String())

	jsonData, _ := json.MarshalIndent(results, "", "  ")
	writeFile(filepath.Join(runDir, "monitor-results.json"), string(jsonData))
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return n
}
