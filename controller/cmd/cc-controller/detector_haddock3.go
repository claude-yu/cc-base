package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// HADDOCK3 stages in expected execution order.
var haddock3Stages = []string{
	"0_topoaa",
	"1_rigidbody",
	"2_seletop",
	"3_flexref",
	"4_emref",
	"5_rmsdmatrix",
	"6_clustrmsd",
	"7_seletopclusts",
	"8_caprieval",
}

type haddock3Detector struct{}

func (d *haddock3Detector) Name() string     { return "haddock3" }
func (d *haddock3Detector) StuckMinutes() int { return 120 }

func (d *haddock3Detector) Match(dir string) (bool, int) {
	score := 0

	if _, err := os.Stat(filepath.Join(dir, "config.cfg")); err == nil {
		if isHaddock3Config(filepath.Join(dir, "config.cfg")) {
			score += 40
		}
	}

	stageCount := countHaddock3Stages(dir)
	if stageCount > 0 {
		score += 30 + stageCount*5
	}

	if n, _ := globExists(dir, "run-*"); n > 0 {
		score += 10
	}

	if score < 50 {
		return false, 0
	}
	if score > 100 {
		score = 100
	}
	return true, score
}

func (d *haddock3Detector) Inspect(dir string) ResearchStatus {
	rs := ResearchStatus{
		Detector:   "haddock3",
		WorkDir:    dir,
		Confidence: "medium",
	}

	var kf []string
	if _, err := os.Stat(filepath.Join(dir, "config.cfg")); err == nil {
		kf = append(kf, "config.cfg")
	}

	completed, current := classifyHaddock3Stages(dir)
	totalStages := len(completed)
	if current != "" {
		totalStages++
	}
	rs.ContextPhase = formatStageProgress(completed, current)

	for _, s := range completed {
		kf = append(kf, s+"/")
	}
	if current != "" {
		kf = append(kf, current+"/")
	}
	if len(kf) > 10 {
		kf = kf[:10]
	}
	rs.KeyFiles = kf

	rs.Evidence = append(rs.Evidence, "stages: "+itoa(totalStages)+"/"+itoa(len(haddock3Stages)))
	if rs.ContextPhase != "" {
		rs.Evidence = append(rs.Evidence, rs.ContextPhase)
	}

	hasCaprieval := false
	for _, s := range completed {
		if strings.HasPrefix(s, "8_caprieval") {
			hasCaprieval = true
			break
		}
	}
	if current != "" && strings.HasPrefix(current, "8_caprieval") {
		hasCaprieval = true
	}

	if hasCaprieval {
		capri := findCaprievalResults(dir)
		rs.Evidence = append(rs.Evidence, capri...)
	}

	newest, newestFile := newestStageModTime(dir, completed, current)
	if !newest.IsZero() {
		rs.LastUpdate = fmtMinsAgo(newest)
		rs.LastUpdateMins = minsAgo(newest)
	} else {
		rs.LastUpdateMins = -1
	}

	var logErrors []string
	logPatterns := []string{"*.log", "*.out", "run-*/haddock3.log"}
	for _, p := range logPatterns {
		matches, _ := filepath.Glob(filepath.Join(dir, p))
		for _, m := range matches {
			tail := readTail(m, 80)
			hits := grepLines(tail, []string{
				"Traceback", "Error", "FAILED", "fatal",
				"FileNotFoundError", "RuntimeError", "Exception",
			})
			hits = filterFalseErrors(hits)
			logErrors = append(logErrors, hits...)
		}
	}

	if len(logErrors) > 0 {
		rs.State = "failed"
		rs.Confidence = "high"
		for _, e := range logErrors {
			if len(rs.Evidence) < 12 {
				rs.Evidence = append(rs.Evidence, "ERROR: "+e)
			}
		}
	} else if hasCaprieval && stageHasOutput(dir, "8_caprieval") {
		rs.State = "completed"
		rs.Confidence = "high"
		rs.Evidence = append(rs.Evidence, "8_caprieval 完成，判定 HADDOCK3 run 结束")
		rs.NextActions = []string{"查看 caprieval 排名", "提取最优 cluster", "检查 RMSD/iRMSD"}
	} else if current != "" && !newest.IsZero() && minsAgo(newest) <= d.StuckMinutes() {
		rs.State = "running"
		rs.Confidence = "medium"
		rs.Evidence = append(rs.Evidence, newestFile+" "+fmtMinsAgo(newest)+"更新")
	} else if current != "" && !newest.IsZero() {
		rs.State = "stuck"
		rs.Confidence = "medium"
		rs.Warnings = append(rs.Warnings, newestFile+" 已 "+itoa(minsAgo(newest))+" 分钟未更新")
	} else if len(completed) > 0 && current == "" {
		lastCompleted := completed[len(completed)-1]
		if strings.HasPrefix(lastCompleted, "8_caprieval") {
			rs.State = "completed"
			rs.Confidence = "medium"
		} else {
			rs.State = "idle"
			rs.Confidence = "medium"
			rs.Evidence = append(rs.Evidence, "最后完成 stage: "+lastCompleted+"，之后无新 stage")
		}
	} else if totalStages == 0 {
		rs.State = "idle"
		rs.Confidence = "medium"
		rs.Evidence = append(rs.Evidence, "config.cfg 存在但无 stage 目录")
	} else {
		rs.State = "unknown"
		rs.Confidence = "low"
	}

	if len(rs.Evidence) == 0 {
		rs.Evidence = []string{"无可用证据"}
	}
	return rs
}

func isHaddock3Config(path string) bool {
	lines := readHead(path, 30)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "haddock") || strings.Contains(lower, "topoaa") ||
			strings.Contains(lower, "rigidbody") || strings.Contains(lower, "flexref") ||
			strings.Contains(lower, "caprieval") || strings.Contains(lower, "molecules") {
			return true
		}
	}
	return false
}

func countHaddock3Stages(dir string) int {
	count := 0
	for _, stage := range haddock3Stages {
		if fi, err := os.Stat(filepath.Join(dir, stage)); err == nil && fi.IsDir() {
			count++
		}
	}
	return count
}

func classifyHaddock3Stages(dir string) (completed []string, current string) {
	var found []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) > 1 && name[0] >= '0' && name[0] <= '9' && strings.Contains(name, "_") {
			found = append(found, name)
		}
	}
	sort.Strings(found)
	if len(found) == 0 {
		return nil, ""
	}

	for i, stage := range found {
		if stageHasOutput(dir, stage) {
			completed = append(completed, stage)
		} else if i == len(found)-1 {
			current = stage
		} else {
			completed = append(completed, stage)
		}
	}
	return completed, current
}

var stageIgnoreFiles = map[string]bool{
	"__init__.py": true,
	".gitkeep":    true,
	".gitignore":  true,
	".DS_Store":   true,
}

func stageHasOutput(dir, stage string) bool {
	stageDir := filepath.Join(dir, stage)
	entries, err := os.ReadDir(stageDir)
	if err != nil {
		return false
	}
	fileCount := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if stageIgnoreFiles[name] || strings.HasSuffix(name, ".pyc") || strings.HasSuffix(name, ".tmp") {
			continue
		}
		fileCount++
	}
	return fileCount >= 2
}

func formatStageProgress(completed []string, current string) string {
	if len(completed) == 0 && current == "" {
		return ""
	}
	var sb strings.Builder
	if len(completed) > 0 {
		sb.WriteString("completed: ")
		if len(completed) <= 3 {
			sb.WriteString(strings.Join(completed, ", "))
		} else {
			sb.WriteString(completed[0] + ".." + completed[len(completed)-1])
			sb.WriteString(" (" + itoa(len(completed)) + " stages)")
		}
	}
	if current != "" {
		if sb.Len() > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString("current: " + current)
	}
	return sb.String()
}

func findCaprievalResults(dir string) []string {
	var evidence []string
	caprDir := ""
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "8_caprieval") {
			caprDir = filepath.Join(dir, e.Name())
			break
		}
	}
	if caprDir == "" {
		return evidence
	}

	if n, files := globExists(caprDir, "capri_ss.tsv", "capri_clt.tsv"); n > 0 {
		evidence = append(evidence, "caprieval 结果: "+strings.Join(files, ", "))
	}

	clusterDir := filepath.Join(caprDir, "cluster")
	if fi, err := os.Stat(clusterDir); err == nil && fi.IsDir() {
		clusterCount, _ := globExists(clusterDir, "cluster_*")
		if clusterCount > 0 {
			evidence = append(evidence, "clusters: "+itoa(clusterCount)+" 个")
		}
	}

	return evidence
}

func newestStageModTime(dir string, completed []string, current string) (time.Time, string) {
	var stages []string
	stages = append(stages, completed...)
	if current != "" {
		stages = append(stages, current)
	}

	var patterns []string
	for _, s := range stages {
		patterns = append(patterns, filepath.Join(s, "*"))
	}
	patterns = append(patterns, "*.log", "*.out")

	return newestModTime(dir, patterns)
}
