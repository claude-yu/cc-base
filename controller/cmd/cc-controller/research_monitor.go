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

	// Mobile summary to stdout (cc-connect returns this)
	summary := formatMobileSummary(results, runDir)
	fmt.Print(summary)

	// Callback
	sendCallback(runDir, summary)
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

// formatMobileSummary generates a short mobile-friendly status.
func formatMobileSummary(results []ResearchStatus, runDir string) string {
	if len(results) == 0 {
		return "科研监控: 未发现任何可识别的科研任务\n建议: 确认 CC_WORK_DIR 或 CC_RESEARCH_MONITOR_ROOT 指向科研项目目录\n"
	}

	var sb strings.Builder
	sb.WriteString("科研监控: " + itoa(len(results)) + " 个任务\n")

	for i, rs := range results {
		if i >= 5 {
			sb.WriteString("... 还有 " + itoa(len(results)-5) + " 个任务\n")
			break
		}
		stateTag := strings.ToUpper(rs.State)
		line := itoa(i+1) + ". [" + stateTag + "] " + rs.Detector
		if rs.Confidence != "" {
			line += " " + rs.Confidence
		}

		// Show the most useful evidence line
		if rs.ContextPhase != "" {
			line += " — " + rs.ContextPhase
		} else if len(rs.Evidence) > 0 {
			ev := rs.Evidence[0]
			if len(ev) > 60 {
				ev = ev[:60] + "..."
			}
			line += " — " + ev
		}
		sb.WriteString(line + "\n")

		// Show top warnings for failed/stuck
		if (rs.State == "failed" || rs.State == "stuck") && len(rs.Evidence) > 1 {
			for j := 1; j < len(rs.Evidence) && j <= defaultMobileLines; j++ {
				ev := rs.Evidence[j]
				if len(ev) > 80 {
					ev = ev[:80] + "..."
				}
				sb.WriteString("   " + ev + "\n")
			}
		}

		// Show next actions for completed
		if rs.State == "completed" && len(rs.NextActions) > 0 {
			sb.WriteString("   建议: " + strings.Join(rs.NextActions, ", ") + "\n")
		}
	}

	relDir := filepath.Base(filepath.Dir(runDir)) + "/" + filepath.Base(runDir)
	sb.WriteString("详情: runs/" + filepath.Base(runDir) + "/report.md\n")
	_ = relDir
	return sb.String()
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
