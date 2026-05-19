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
	defaultScanDepth    = 3
	defaultMaxLogLines  = 80
	defaultMobileLines  = 5
)

// cmdResearchMonitor is the entry point for /科研监控.
func cmdResearchMonitor(root string, args []string) {
	workDir := resolveMonitorRoot()

	// Parse optional --detector flag for compat (/md状态检查 → --detector gromacs)
	filterDetector := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--detector" && i+1 < len(args) {
			filterDetector = args[i+1]
			i++
		}
	}

	scanDepth := defaultScanDepth
	if v := os.Getenv("CC_RESEARCH_SCAN_DEPTH"); v != "" {
		if n := atoiSafe(v); n > 0 {
			scanDepth = n
		}
	}

	results := scanProject(workDir, scanDepth, filterDetector)

	// Append Docker container scan (unless filtering to a specific detector)
	if filterDetector == "" || strings.HasPrefix(filterDetector, "docker") {
		dockerResults := scanDockerContainers()
		results = append(results, dockerResults...)
	}

	// Sort: failed > stuck > running > completed > idle > unknown
	sort.SliceStable(results, func(i, j int) bool {
		pi := statePriority[results[i].State]
		pj := statePriority[results[j].State]
		if pi != pj {
			return pi < pj
		}
		return results[i].Score > results[j].Score
	})

	// Write detail report to runs/
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

	// Mobile summary to stdout (cc-connect captures and sends to user)
	summary := formatMobileSummary(results, runDir)
	fmt.Print(summary)
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

	// Depth 0: root itself
	walkFn(workDir, 0)

	// Depth 1..maxDepth: subdirectories
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

const recentThresholdMins = 24 * 60 // 24h: recent vs historical

// formatMobileSummary generates a compact mobile-friendly status.
func formatMobileSummary(results []ResearchStatus, runDir string) string {
	if len(results) == 0 {
		return "科研监控: 未发现任何可识别的科研任务\n建议: 确认 CC_WORK_DIR 或 CC_RESEARCH_MONITOR_ROOT 指向科研项目目录\n"
	}

	// Count by state
	counts := map[string]int{}
	for _, rs := range results {
		counts[rs.State]++
	}

	var sb strings.Builder
	sb.WriteString("科研监控: " + itoa(len(results)) + " 个任务\n")

	// State count line
	stateOrder := []string{"failed", "stuck", "running", "completed", "idle", "unknown"}
	var parts []string
	for _, s := range stateOrder {
		if c := counts[s]; c > 0 {
			parts = append(parts, strings.ToUpper(s)+" "+itoa(c))
		}
	}
	if len(parts) > 0 {
		sb.WriteString(strings.Join(parts, " | ") + "\n")
	}

	// Split into recent (<24h) and historical (>24h) failures/stuck
	var recentAlerts []ResearchStatus
	var histAlerts []ResearchStatus
	for _, rs := range results {
		if rs.State != "failed" && rs.State != "stuck" {
			continue
		}
		if rs.LastUpdateMins >= 0 && rs.LastUpdateMins > recentThresholdMins {
			histAlerts = append(histAlerts, rs)
		} else {
			recentAlerts = append(recentAlerts, rs)
		}
	}

	// Recent high-priority alerts (max 3, one line each)
	if len(recentAlerts) > 0 {
		sb.WriteString("\n高优先级异常:\n")
		for i, rs := range recentAlerts {
			if i >= 3 {
				sb.WriteString("... 还有 " + itoa(len(recentAlerts)-3) + " 个近期异常\n")
				break
			}
			line := itoa(i+1) + ". [" + strings.ToUpper(rs.State) + "] " + detectorShortName(rs.Detector)
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
	}

	// Historical failures — grouped by detector
	if len(histAlerts) > 0 {
		sb.WriteString("\n历史异常 (>24h): " + itoa(len(histAlerts)) + " 个\n")
		groups := groupByDetector(histAlerts)
		for _, g := range groups {
			age := ""
			if g.results[0].LastUpdate != "" {
				age = " (" + g.results[0].LastUpdate + ")"
			}
			sb.WriteString("- " + detectorShortName(g.detector) + " x" + itoa(len(g.results)) + age + "\n")
		}
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
			line := "- " + detectorShortName(rs.Detector)
			if rs.ContextPhase != "" {
				line += " — " + rs.ContextPhase
			} else if rs.LastUpdate != "" {
				line += " — " + rs.LastUpdate + "更新"
			}
			sb.WriteString(line + "\n")
		}
	}

	sb.WriteString("\n详情: runs/" + filepath.Base(runDir) + "/report.md\n")
	return sb.String()
}

func detectorShortName(det string) string {
	// "docker:haddock3" → "HADDOCK3"
	// "gromacs" → "GROMACS"
	// "python_pipeline" → "Python"
	// "r_pipeline" → "R"
	// "generic_cli" → "CLI"
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

// writeDetailReport writes the full report.md to the run directory.
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

	for i, rs := range results {
		sb.WriteString("---\n\n")
		sb.WriteString("## " + itoa(i+1) + ". " + rs.Detector + " — " + strings.ToUpper(rs.State) + "\n\n")

		sb.WriteString("| 属性 | 值 |\n|---|---|\n")
		sb.WriteString("| Detector | " + rs.Detector + " |\n")
		sb.WriteString("| State | " + rs.State + " |\n")
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

	// Also write machine-readable status.json with all results
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
