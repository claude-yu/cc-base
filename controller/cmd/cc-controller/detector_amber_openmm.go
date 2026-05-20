package main

import (
	"os"
	"path/filepath"
	"strings"
)

type amberOpenMMDetector struct{}

func (d *amberOpenMMDetector) Name() string     { return "amber_openmm" }
func (d *amberOpenMMDetector) StuckMinutes() int { return 60 }

func (d *amberOpenMMDetector) Match(dir string) (bool, int) {
	score := 0

	// Amber topology files — strong signal
	if n, _ := globExists(dir, "*.prmtop", "*.parm7"); n > 0 {
		score += 30
	}

	// Amber input/output files
	if n, _ := globExists(dir, "*.mdin"); n > 0 {
		score += 20
	}
	if n, _ := globExists(dir, "*.mdout"); n > 0 {
		score += 15
	}

	// Amber restart/trajectory
	if n, _ := globExists(dir, "*.rst7", "*.ncrst"); n > 0 {
		score += 10
	}
	if n, _ := globExists(dir, "*.mdcrd"); n > 0 {
		score += 10
	}

	// Amber inpcrd
	if n, _ := globExists(dir, "*.inpcrd"); n > 0 {
		score += 10
	}

	// LEaP signals
	if _, err := os.Stat(filepath.Join(dir, "leap.log")); err == nil {
		score += 15
	}
	if n, _ := globExists(dir, "tleap.in", "leap.in"); n > 0 {
		score += 10
	}

	// OpenMM-specific: Python scripts with openmm imports
	if hasOpenMMScript(dir) {
		score += 35
	}

	// NetCDF trajectory (.nc is shared with other tools, so only count if Amber context exists)
	if score >= 20 {
		if n, _ := globExists(dir, "*.nc"); n > 0 {
			score += 5
		}
	}

	if score < 40 {
		return false, 0
	}
	if score > 100 {
		score = 100
	}
	return true, score
}

func (d *amberOpenMMDetector) Inspect(dir string) ResearchStatus {
	rs := ResearchStatus{
		Detector:   "amber_openmm",
		WorkDir:    dir,
		Confidence: "medium",
	}

	_, kf := globExists(dir, "*.prmtop", "*.parm7", "*.mdin", "*.mdout",
		"*.rst7", "*.inpcrd", "leap.log", "*.mdcrd", "*.nc")
	if len(kf) > 10 {
		kf = kf[:10]
	}
	rs.KeyFiles = kf

	variant := detectAmberVariant(dir)
	if variant != "" {
		rs.Evidence = append(rs.Evidence, "variant: "+variant)
	}

	phase := detectAmberPhase(dir)
	if phase != "" {
		rs.ContextPhase = phase
		rs.Evidence = append(rs.Evidence, "phase: "+phase)
	}

	// Collect performance/energy info from mdout
	perfInfo := parseMdoutPerformance(dir)
	for _, p := range perfInfo {
		rs.Evidence = append(rs.Evidence, p)
	}

	// Check newest output file
	outputPatterns := []string{
		"*.mdout", "*.rst7", "*.ncrst", "*.mdcrd", "*.nc",
		"*.out", "*.log", "*.dcd", "*.csv",
	}
	newest, newestFile := newestModTime(dir, outputPatterns)
	if !newest.IsZero() {
		rs.LastUpdate = fmtMinsAgo(newest)
		rs.LastUpdateMins = minsAgo(newest)
	} else {
		rs.LastUpdateMins = -1
	}

	// Check logs for errors
	var logErrors []string
	logPatterns := []string{"*.mdout", "*.out", "*.log"}
	for _, p := range logPatterns {
		matches, _ := filepath.Glob(filepath.Join(dir, p))
		for _, m := range matches {
			tail := readTail(m, 80)
			hits := grepLines(tail, []string{
				"FATAL", "Segmentation fault", "ERROR",
				"Halting program",
				"Traceback", "RuntimeError", "CUDA out of memory",
				"FileNotFoundError",
			})
			hits = filterFalseErrors(hits)
			logErrors = append(logErrors, hits...)
		}
	}

	// Check completion
	hasFinished := mdoutHasCompletion(dir, variant)

	// Check cpptraj/pytraj analysis outputs (require actual products, not empty dirs)
	hasAnalysis := false
	for _, sub := range []string{"analysis", "cpptraj"} {
		subDir := filepath.Join(dir, sub)
		if fi, err := os.Stat(subDir); err == nil && fi.IsDir() {
			if n, _ := globExists(subDir, "*.dat", "*.agr", "*.csv", "*.out"); n > 0 {
				hasAnalysis = true
				rs.Evidence = append(rs.Evidence, sub+"/ 有分析产物")
				break
			}
		}
	}
	// Also check MMPBSA output at top level
	if !hasAnalysis {
		if n, _ := globExists(dir, "MMPBSA*.dat", "FINAL_RESULTS_MMPBSA.dat"); n > 0 {
			hasAnalysis = true
			rs.Evidence = append(rs.Evidence, "MMPBSA 结果文件存在")
		}
	}

	// Determine state
	if len(logErrors) > 0 {
		rs.State = "failed"
		rs.Confidence = "high"
		for _, e := range logErrors {
			if len(rs.Evidence) < 12 {
				rs.Evidence = append(rs.Evidence, "ERROR: "+e)
			}
		}
	} else if hasFinished {
		rs.State = "completed"
		rs.Confidence = "high"
		rs.Evidence = append(rs.Evidence, "mdout 包含 Final Performance Info / wallclock")
		if hasAnalysis {
			rs.Evidence = append(rs.Evidence, "检测到分析输出")
		} else {
			rs.NextActions = []string{"cpptraj 分析", "RMSD/RMSF/Rg", "MM-PBSA/GBSA"}
		}
	} else if hasAnalysis {
		rs.State = "completed"
		rs.Confidence = "medium"
		rs.Evidence = append(rs.Evidence, "检测到分析输出目录")
	} else if !newest.IsZero() {
		mins := minsAgo(newest)
		rs.Evidence = append(rs.Evidence, newestFile+" "+fmtMinsAgo(newest)+"更新")
		if mins <= d.StuckMinutes() {
			rs.State = "running"
			rs.Confidence = "medium"
		} else {
			rs.State = "stuck"
			rs.Confidence = "medium"
			rs.Warnings = append(rs.Warnings, newestFile+" 已 "+itoa(mins)+" 分钟未更新")
		}
	} else {
		hasTopo, _ := globExists(dir, "*.prmtop", "*.parm7")
		hasInp, _ := globExists(dir, "*.inpcrd", "*.mdin")
		if hasTopo > 0 || hasInp > 0 {
			rs.State = "idle"
			rs.Confidence = "medium"
			rs.Evidence = append(rs.Evidence, "有拓扑/输入文件但无输出更新")
		} else if strings.Contains(variant, "openmm") {
			rs.State = "idle"
			rs.Confidence = "medium"
			rs.Evidence = append(rs.Evidence, "有 OpenMM 脚本但无输出更新")
		} else {
			rs.State = "unknown"
			rs.Confidence = "low"
		}
	}

	if len(rs.Evidence) == 0 {
		rs.Evidence = []string{"无可用证据"}
	}
	return rs
}

// hasOpenMMScript checks if any .py file in the directory imports openmm.
func hasOpenMMScript(dir string) bool {
	pyFiles, _ := filepath.Glob(filepath.Join(dir, "*.py"))
	if len(pyFiles) > 20 {
		pyFiles = pyFiles[:20]
	}
	for _, py := range pyFiles {
		fi, err := os.Stat(py)
		if err != nil || fi.Size() > 1*1024*1024 {
			continue
		}
		lines := readHead(py, 50)
		for _, line := range lines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "openmm") || strings.Contains(lower, "simtk.openmm") {
				return true
			}
		}
	}
	return false
}

// detectAmberVariant returns "amber", "openmm", or "amber+openmm".
func detectAmberVariant(dir string) string {
	hasAmber := false
	hasOpenMM := false

	if n, _ := globExists(dir, "*.prmtop", "*.parm7", "*.mdin", "*.mdout"); n > 0 {
		hasAmber = true
	}
	if _, err := os.Stat(filepath.Join(dir, "leap.log")); err == nil {
		hasAmber = true
	}

	if hasOpenMMScript(dir) {
		hasOpenMM = true
	}
	// OpenMM system XML
	if n, _ := globExists(dir, "system.xml", "state.xml"); n > 0 {
		hasOpenMM = true
	}
	// DCD trajectory is often OpenMM
	if n, _ := globExists(dir, "*.dcd"); n > 0 {
		hasOpenMM = true
	}

	if hasAmber && hasOpenMM {
		return "amber+openmm"
	}
	if hasOpenMM {
		return "openmm"
	}
	if hasAmber {
		return "amber"
	}
	return ""
}

// detectAmberPhase identifies the current simulation phase.
func detectAmberPhase(dir string) string {
	phases := []struct {
		pattern string
		name    string
	}{
		{"*prod*.mdout", "production"},
		{"*md*.mdout", "production"},
		{"*equil*.mdout", "equilibration"},
		{"*npt*.mdout", "equilibration"},
		{"*nvt*.mdout", "equilibration"},
		{"*heat*.mdout", "heating"},
		{"*warm*.mdout", "heating"},
		{"*min*.mdout", "minimization"},
	}

	highest := ""
	phaseRank := map[string]int{
		"minimization":  1,
		"heating":       2,
		"equilibration": 3,
		"production":    4,
	}

	for _, p := range phases {
		if n, _ := globExists(dir, p.pattern); n > 0 {
			if phaseRank[p.name] > phaseRank[highest] {
				highest = p.name
			}
		}
	}

	if highest != "" {
		return highest
	}

	// OpenMM: check for DCD/reporter output without mdout
	if hasOpenMMScript(dir) {
		if n, _ := globExists(dir, "*.dcd", "*.csv"); n > 0 {
			return "production (OpenMM)"
		}
		return "setup (OpenMM)"
	}

	// LEaP-only = preparation
	if _, err := os.Stat(filepath.Join(dir, "leap.log")); err == nil {
		if n, _ := globExists(dir, "*.mdout"); n == 0 {
			return "preparation"
		}
	}

	return ""
}

// parseMdoutPerformance extracts performance and energy info from the latest mdout.
func parseMdoutPerformance(dir string) []string {
	var evidence []string

	mdouts, _ := filepath.Glob(filepath.Join(dir, "*.mdout"))
	if len(mdouts) == 0 {
		return evidence
	}

	// Pick the newest mdout
	var bestPath string
	var bestTime int64
	for _, m := range mdouts {
		fi, err := os.Stat(m)
		if err != nil {
			continue
		}
		if fi.ModTime().UnixNano() > bestTime {
			bestTime = fi.ModTime().UnixNano()
			bestPath = m
		}
	}
	if bestPath == "" {
		return evidence
	}

	tail := readTail(bestPath, 100)
	for i := len(tail) - 1; i >= 0; i-- {
		line := tail[i]
		if strings.Contains(line, "ns/day") || strings.Contains(line, "Performance") {
			evidence = append(evidence, strings.TrimSpace(line))
			break
		}
	}

	// Extract current NSTEP from tail
	nstep := parseLastNSTEP(tail)
	if nstep != "" {
		evidence = append(evidence, "last NSTEP: "+nstep)
	}

	return evidence
}

// parseLastNSTEP finds the last NSTEP value in mdout tail lines.
func parseLastNSTEP(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "NSTEP") && strings.Contains(line, "TIME") {
			// Header line, next line has values
			if i+1 < len(lines) {
				fields := strings.Fields(lines[i+1])
				if len(fields) >= 1 {
					return fields[0]
				}
			}
		}
	}
	return ""
}

// mdoutHasCompletion checks if any mdout file contains Amber completion markers,
// and optionally checks OpenMM logs if variant contains "openmm".
func mdoutHasCompletion(dir string, variant string) bool {
	mdouts, _ := filepath.Glob(filepath.Join(dir, "*.mdout"))
	for _, m := range mdouts {
		fi, err := os.Stat(m)
		if err != nil || fi.Size() > 10*1024*1024 {
			continue
		}
		tail := readTail(m, 50)
		for _, line := range tail {
			if strings.Contains(line, "Final Performance Info") ||
				strings.Contains(line, "wallclock") {
				return true
			}
		}
	}
	// Only check OpenMM log markers when variant indicates OpenMM
	if strings.Contains(variant, "openmm") {
		logFiles, _ := filepath.Glob(filepath.Join(dir, "*.log"))
		for _, lf := range logFiles {
			fi, err := os.Stat(lf)
			if err != nil || fi.Size() > 10*1024*1024 {
				continue
			}
			tail := readTail(lf, 30)
			for _, line := range tail {
				if strings.Contains(line, "Simulation complete") {
					return true
				}
			}
		}
	}
	return false
}
