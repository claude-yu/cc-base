package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type schrodingerDetector struct{}

func (d *schrodingerDetector) Name() string     { return "schrodinger" }
func (d *schrodingerDetector) StuckMinutes() int { return 120 }

type schrodingerJobType int

const (
	sjUnknown schrodingerJobType = iota
	sjGlideDock
	sjGlideGrid
	sjLigPrep
	sjProteinPrep
	sjProtProtDock
)

func classifySchrodingerDir(dirName string) schrodingerJobType {
	lower := strings.ToLower(dirName)
	switch {
	case strings.HasPrefix(lower, "glide-dock_") || strings.HasPrefix(lower, "glide_sp_") ||
		strings.HasPrefix(lower, "glide_xp_") || strings.HasPrefix(lower, "glide_htvs_"):
		return sjGlideDock
	case strings.HasPrefix(lower, "glide-grid_") || strings.HasPrefix(lower, "glide_grid_"):
		return sjGlideGrid
	case strings.HasPrefix(lower, "ligprep"):
		return sjLigPrep
	case strings.HasPrefix(lower, "proteinprep"):
		return sjProteinPrep
	case strings.HasPrefix(lower, "prot-prot-docking_") || strings.HasPrefix(lower, "prot-prot_"):
		return sjProtProtDock
	default:
		return sjUnknown
	}
}

func schrodingerJobLabel(jt schrodingerJobType) string {
	switch jt {
	case sjGlideDock:
		return "Glide Docking"
	case sjGlideGrid:
		return "Glide Grid"
	case sjLigPrep:
		return "LigPrep"
	case sjProteinPrep:
		return "Protein Prep"
	case sjProtProtDock:
		return "Protein-Protein Docking"
	default:
		return "Schrodinger"
	}
}

func (d *schrodingerDetector) Match(dir string) (bool, int) {
	dirName := filepath.Base(dir)
	score := 0

	jt := classifySchrodingerDir(dirName)
	if jt == sjGlideDock {
		score += 70
	} else if jt != sjUnknown {
		score += 60
	}

	pvCount, _ := globExists(dir, "*_pv.maegz")
	libCount, _ := globExists(dir, "*_lib.maegz")
	if pvCount > 0 || libCount > 0 {
		score += 30
	}

	inCount, _ := globExists(dir, "*.in")
	if inCount > 0 && score > 0 {
		score += 10
	}

	maegzCount, _ := globExists(dir, "*.maegz")
	if maegzCount > 0 && score > 0 {
		score += 5
	}

	// If no directory name match, check for Schrodinger-specific files
	if score == 0 {
		_, logFiles := globExists(dir, "*.log")
		for _, lf := range logFiles {
			if isSchrodingerLog(filepath.Join(dir, lf)) {
				score += 55
				break
			}
		}
	}

	if score < 50 {
		return false, 0
	}
	if score > 100 {
		score = 100
	}
	return true, score
}

func isSchrodingerLog(path string) bool {
	lines := readHead(path, 30)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "glide") || strings.Contains(lower, "schrodinger") ||
			strings.Contains(lower, "ligprep") || strings.Contains(lower, "impact") ||
			strings.Contains(lower, "prime ") || strings.Contains(lower, "macromodel") {
			return true
		}
	}
	return false
}

func (d *schrodingerDetector) Inspect(dir string) ResearchStatus {
	dirName := filepath.Base(dir)
	jt := classifySchrodingerDir(dirName)

	rs := ResearchStatus{
		Detector:   "schrodinger",
		WorkDir:    dir,
		Confidence: "medium",
	}

	if jt != sjUnknown {
		rs.ContextPhase = schrodingerJobLabel(jt)
	}

	// Collect key files
	var kf []string
	_, pvFiles := globExists(dir, "*_pv.maegz")
	kf = append(kf, pvFiles...)
	_, libFiles := globExists(dir, "*_lib.maegz")
	kf = append(kf, libFiles...)
	_, csvFiles := globExists(dir, "*.csv")
	kf = append(kf, csvFiles...)
	_, inFiles := globExists(dir, "*.in")
	kf = append(kf, inFiles...)
	if len(kf) > 8 {
		kf = kf[:8]
	}
	rs.KeyFiles = kf

	// Find primary log (match dir name, exclude _subjobs, prefer newest)
	primaryLog := findSchrodingerLog(dir, dirName)
	if primaryLog == "" {
		inCount, _ := globExists(dir, "*.in")
		if inCount > 0 {
			rs.State = "idle"
			rs.Evidence = append(rs.Evidence, "输入文件存在但无日志")
		} else {
			rs.State = "unknown"
		}
		return rs
	}

	rs.Evidence = append(rs.Evidence, "日志: "+filepath.Base(primaryLog))

	// File age
	if fi, err := os.Stat(primaryLog); err == nil {
		rs.LastUpdate = fmtMinsAgo(fi.ModTime())
		rs.LastUpdateMins = minsAgo(fi.ModTime())
	}

	// Output file counts
	pvCount := len(pvFiles)
	libCount := len(libFiles)
	maegzCount, _ := globExists(dir, "*.maegz")
	csvCount := len(csvFiles)

	// Parse log for state
	tail := readTail(primaryLog, 60)
	state, evidence := parseSchrodingerLog(tail, jt)
	rs.State = state
	rs.Evidence = append(rs.Evidence, evidence...)

	// Output summary
	if maegzCount > 0 || csvCount > 0 {
		parts := []string{}
		if maegzCount > 0 {
			parts = append(parts, itoa(maegzCount)+" maegz")
		}
		if csvCount > 0 {
			parts = append(parts, itoa(csvCount)+" csv")
		}
		rs.Evidence = append(rs.Evidence, "输出: "+strings.Join(parts, ", "))
	}
	if pvCount > 0 {
		rs.Evidence = append(rs.Evidence, "Pose viewer: "+itoa(pvCount)+" _pv.maegz")
	}
	if libCount > 0 {
		rs.Evidence = append(rs.Evidence, "Pose library: "+itoa(libCount)+" _lib.maegz")
	}

	// Override state based on output files
	if rs.State == "running" || rs.State == "unknown" {
		if pvCount > 0 || libCount > 0 {
			rs.State = "completed"
			rs.Confidence = "high"
			rs.Evidence = append(rs.Evidence, "输出文件存在，判定完成")
		}
	}

	// Stuck detection
	if rs.State == "running" && rs.LastUpdateMins > d.StuckMinutes() {
		rs.State = "stuck"
		rs.Evidence = append(rs.Evidence, "日志 "+itoa(rs.LastUpdateMins)+" 分钟未更新")
	}

	if rs.State == "completed" || rs.State == "failed" {
		rs.Confidence = "high"
	}

	return rs
}

func findSchrodingerLog(dir, dirName string) string {
	// 1. Exact match: dirName.log
	exact := filepath.Join(dir, dirName+".log")
	if fi, err := os.Stat(exact); err == nil && !fi.IsDir() {
		return exact
	}

	// 2. All .log files excluding _subjobs.log
	matches, _ := filepath.Glob(filepath.Join(dir, "*.log"))
	var candidates []string
	for _, m := range matches {
		base := filepath.Base(m)
		if strings.HasSuffix(base, "_subjobs.log") || strings.HasSuffix(base, "_launch.log") {
			continue
		}
		candidates = append(candidates, m)
	}
	if len(candidates) == 0 {
		return ""
	}

	// 3. Return newest
	var newest string
	var newestTime time.Time
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.ModTime().After(newestTime) {
			newestTime = fi.ModTime()
			newest = c
		}
	}
	return newest
}

func parseSchrodingerLog(tail []string, jt schrodingerJobType) (string, []string) {
	var evidence []string
	hasErrors := false
	hasCompleted := false

	for _, line := range tail {
		lower := strings.ToLower(strings.TrimSpace(line))
		trimmed := strings.TrimSpace(line)

		// Definitive success/failure summary — early return
		if strings.Contains(lower, "job(s) succeeded") {
			if strings.Contains(lower, "0 job(s) failed") || !strings.Contains(lower, "job(s) failed") {
				evidence = append(evidence, "完成: "+trimmed)
				return "completed", evidence
			}
			evidence = append(evidence, "部分失败: "+trimmed)
			return "failed", evidence
		}

		// Ambiguous completion markers — defer, errors take priority
		if strings.Contains(lower, "all jobs have completed") ||
			strings.Contains(lower, "all jobs succeeded") ||
			strings.Contains(lower, "exiting glide") {
			evidence = append(evidence, "完成: "+trimmed)
			hasCompleted = true
			continue
		}

		// Docking score
		if strings.Contains(lower, "best docking score") {
			evidence = append(evidence, trimmed)
		}

		// Pose writing
		if strings.Contains(lower, "writing") && (strings.Contains(lower, "poses to") || strings.Contains(lower, "_pv.maegz")) {
			evidence = append(evidence, trimmed)
		}

		// Elapsed time at end (Schrodinger pattern)
		if strings.Contains(lower, "total elapsed") || strings.Contains(lower, "total cpu time") {
			evidence = append(evidence, trimmed)
		}

		// Errors
		if (strings.Contains(lower, "error") || strings.Contains(lower, "fatal")) &&
			!isNegatedFailure(line) && !strings.Contains(lower, "error_handler") {
			hasErrors = true
			if len(evidence) < 8 {
				evidence = append(evidence, "ERROR: "+trimmed)
			}
		}
	}

	if hasErrors {
		return "failed", evidence
	}
	if hasCompleted {
		return "completed", evidence
	}
	return "running", evidence
}
