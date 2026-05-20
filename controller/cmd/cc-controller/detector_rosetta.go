package main

import (
	"os"
	"path/filepath"
	"strings"
)

type rosettaDetector struct{}

func (d *rosettaDetector) Name() string     { return "rosetta" }
func (d *rosettaDetector) StuckMinutes() int { return 90 }

func (d *rosettaDetector) Match(dir string) (bool, int) {
	score := 0

	if _, err := os.Stat(filepath.Join(dir, "score.sc")); err == nil {
		score += 40
	}

	if n, _ := globExists(dir, "*.silent"); n > 0 {
		score += 25
	}

	for _, name := range []string{"flags", "rosetta_flags.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			score += 20
			break
		}
	}
	if n, _ := globExists(dir, "*.flags", "*.options"); n > 0 && score < 20 {
		score += 15
	}

	if _, err := os.Stat(filepath.Join(dir, "rosetta_scripts.xml")); err == nil {
		score += 20
	}

	if hasRosettaLog(dir) {
		score += 15
	}

	if hasPyRosettaScript(dir) {
		score += 30
	}

	if score < 40 {
		return false, 0
	}
	if score > 100 {
		score = 100
	}
	return true, score
}

func (d *rosettaDetector) Inspect(dir string) ResearchStatus {
	rs := ResearchStatus{
		Detector:   "rosetta",
		WorkDir:    dir,
		Confidence: "medium",
	}

	var kf []string
	for _, name := range []string{"score.sc", "flags", "rosetta_flags.txt", "rosetta_scripts.xml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			kf = append(kf, name)
		}
	}
	if n, files := globExists(dir, "*.silent"); n > 0 {
		for _, f := range files {
			kf = append(kf, f)
			if len(kf) >= 8 {
				break
			}
		}
	}
	if n, files := globExists(dir, "*.flags", "*.options"); n > 0 {
		for _, f := range files {
			kf = append(kf, f)
			if len(kf) >= 10 {
				break
			}
		}
	}
	rs.KeyFiles = kf

	isPyRosetta := hasPyRosettaScript(dir)
	if isPyRosetta {
		rs.Evidence = append(rs.Evidence, "PyRosetta 脚本检测到")
	}

	scoreSC := parseScoreSC(dir)
	if scoreSC.rows > 0 {
		rs.Evidence = append(rs.Evidence, "score.sc: "+itoa(scoreSC.rows)+" 行结果")
		if scoreSC.bestScore != "" {
			rs.Evidence = append(rs.Evidence, "best score: "+scoreSC.bestScore)
		}
	}

	silentCount, _ := globExists(dir, "*.silent")
	if silentCount > 0 {
		rs.Evidence = append(rs.Evidence, itoa(silentCount)+" 个 silent 文件")
	}

	pdbOut := countOutputPDBs(dir)
	if pdbOut > 0 {
		rs.Evidence = append(rs.Evidence, itoa(pdbOut)+" 个输出 PDB")
	}

	newest, newestFile := newestModTime(dir, []string{
		"score.sc", "*.silent", "*.log", "*.out", "*.pdb",
	})
	if !newest.IsZero() {
		rs.LastUpdate = fmtMinsAgo(newest)
		rs.LastUpdateMins = minsAgo(newest)
	} else {
		rs.LastUpdateMins = -1
	}

	var logErrors []string
	logPatterns := []string{"*.log", "*.out"}
	errKeywords := []string{
		"ERROR", "FATAL", "Traceback", "RuntimeError",
		"FileNotFoundError", "ModuleNotFoundError", "Exception",
		"Segmentation fault", "core dumped",
	}
	for _, p := range logPatterns {
		matches, _ := filepath.Glob(filepath.Join(dir, p))
		for _, m := range matches {
			tail := readTail(m, 80)
			hits := grepLines(tail, errKeywords)
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
	} else if scoreSC.rows > 0 && hasRosettaCompletion(dir) {
		rs.State = "completed"
		rs.Confidence = "high"
		rs.Evidence = append(rs.Evidence, "score.sc 存在且日志显示完成")
		rs.NextActions = []string{"查看 score.sc 排名", "提取最优结构", "可视化检查"}
	} else if scoreSC.rows > 0 {
		rs.State = "completed"
		rs.Confidence = "medium"
		rs.Evidence = append(rs.Evidence, "score.sc 存在 ("+itoa(scoreSC.rows)+" 行)")
		rs.NextActions = []string{"查看 score.sc 排名", "提取最优结构"}
	} else if !newest.IsZero() && minsAgo(newest) <= d.StuckMinutes() {
		rs.State = "running"
		rs.Confidence = "medium"
		rs.Evidence = append(rs.Evidence, newestFile+" "+fmtMinsAgo(newest)+"更新")
	} else if !newest.IsZero() {
		rs.State = "stuck"
		rs.Confidence = "medium"
		rs.Warnings = append(rs.Warnings, newestFile+" 已 "+itoa(minsAgo(newest))+" 分钟未更新")
	} else if len(kf) > 0 {
		rs.State = "idle"
		rs.Confidence = "medium"
		rs.Evidence = append(rs.Evidence, "输入文件存在但无输出")
	} else {
		rs.State = "unknown"
		rs.Confidence = "low"
	}

	if len(rs.Evidence) == 0 {
		rs.Evidence = []string{"无可用证据"}
	}
	return rs
}

type scoreSCInfo struct {
	rows      int
	bestScore string
}

func parseScoreSC(dir string) scoreSCInfo {
	path := filepath.Join(dir, "score.sc")
	lines := readHead(path, 100)
	if len(lines) == 0 {
		return scoreSCInfo{}
	}

	dataRows := 0
	scoreIdx := -1
	bestScore := ""
	bestVal := 1e18

	headerSeen := false
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "SEQUENCE:" {
			continue
		}
		if fields[0] == "SCORE:" && !headerSeen {
			if len(fields) > 1 && !looksNumeric(fields[1]) {
				headerSeen = true
				for j, f := range fields {
					if f == "total_score" || f == "score" {
						scoreIdx = j
						break
					}
				}
				continue
			}
		}

		if fields[0] == "SCORE:" && len(fields) > 1 && looksNumeric(fields[1]) {
			dataRows++
			if scoreIdx > 0 && scoreIdx < len(fields) {
				val := parseFloat(fields[scoreIdx])
				if val < bestVal {
					bestVal = val
					bestScore = fields[scoreIdx]
				}
			}
		}
	}
	return scoreSCInfo{rows: dataRows, bestScore: bestScore}
}

func looksNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	start := 0
	if s[0] == '-' || s[0] == '+' {
		start = 1
	}
	hasDot := false
	digits := 0
	for i := start; i < len(s); i++ {
		if s[i] == '.' && !hasDot {
			hasDot = true
		} else if s[i] >= '0' && s[i] <= '9' {
			digits++
		} else {
			return false
		}
	}
	return digits > 0
}

func parseFloat(s string) float64 {
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}
	intPart := 0.0
	fracPart := 0.0
	fracDiv := 1.0
	inFrac := false
	for _, c := range s {
		if c == '.' {
			inFrac = true
			continue
		}
		if c < '0' || c > '9' {
			break
		}
		if inFrac {
			fracPart = fracPart*10 + float64(c-'0')
			fracDiv *= 10
		} else {
			intPart = intPart*10 + float64(c-'0')
		}
	}
	val := intPart + fracPart/fracDiv
	if neg {
		return -val
	}
	return val
}

func hasRosettaLog(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.log"))
	for _, m := range matches {
		lines := readHead(m, 30)
		for _, line := range lines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "rosetta") ||
				strings.Contains(lower, "protocols.jd2") ||
				strings.Contains(lower, "core.init") ||
				strings.Contains(lower, "core.scoring") {
				return true
			}
		}
	}
	return false
}

func hasPyRosettaScript(dir string) bool {
	pyFiles, _ := filepath.Glob(filepath.Join(dir, "*.py"))
	for _, f := range pyFiles {
		lines := readHead(f, 30)
		for _, line := range lines {
			if strings.Contains(line, "pyrosetta") || strings.Contains(line, "PyRosetta") {
				return true
			}
		}
	}
	return false
}

func hasRosettaCompletion(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.log"))
	for _, m := range matches {
		tail := readTail(m, 30)
		for _, line := range tail {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "reported success") ||
				strings.Contains(lower, "protocols.jd2.jobdistributor") && strings.Contains(lower, "no more") {
				return true
			}
		}
	}
	return false
}

func countOutputPDBs(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".pdb") && !strings.HasPrefix(name, "input") {
			count++
		}
	}
	return count
}
