package main

import (
	"os"
	"path/filepath"
	"strings"
)

type gaussianDetector struct{}

func (d *gaussianDetector) Name() string      { return "gaussian" }
func (d *gaussianDetector) StuckMinutes() int  { return 120 }

func (d *gaussianDetector) Match(dir string) (bool, int) {
	score := 0

	// Gaussian input files — strong signal
	if n, _ := globExists(dir, "*.gjf"); n > 0 {
		score += 25
	}
	if n, _ := globExists(dir, "*.com"); n > 0 {
		score += 25
	}

	// Checkpoint files
	if n, _ := globExists(dir, "*.chk"); n > 0 {
		score += 15
	}
	if n, _ := globExists(dir, "*.fchk"); n > 0 {
		score += 15
	}

	// Gaussian log content confirmation — only count if log is actually Gaussian
	logFiles, _ := filepath.Glob(filepath.Join(dir, "*.log"))
	if len(logFiles) > 20 {
		logFiles = logFiles[:20]
	}
	hasGaussianLog := false
	hasSCFOrOpt := false
	for _, lf := range logFiles {
		if !isGaussianLog(lf) {
			continue
		}
		hasGaussianLog = true
		// Check for SCF/optimization/frequency markers
		tail := readTail(lf, 100)
		markers := grepLines(tail, []string{"SCF Done", "Optimization completed", "Stationary point found", "Frequencies --"})
		if len(markers) > 0 {
			hasSCFOrOpt = true
		}
	}
	if hasGaussianLog {
		score += 40
	}
	if hasSCFOrOpt {
		score += 10
	}

	if score < 40 {
		return false, 0
	}
	if score > 100 {
		score = 100
	}
	return true, score
}

func (d *gaussianDetector) Inspect(dir string) ResearchStatus {
	rs := ResearchStatus{
		Detector:   "gaussian",
		WorkDir:    dir,
		Confidence: "medium",
	}

	// Collect key files
	_, kf := globExists(dir, "*.gjf", "*.com", "*.chk", "*.fchk")
	// Add only confirmed Gaussian logs to key files
	logFiles, _ := filepath.Glob(filepath.Join(dir, "*.log"))
	if len(logFiles) > 20 {
		logFiles = logFiles[:20]
	}
	var gaussianLogs []string
	for _, lf := range logFiles {
		if isGaussianLog(lf) {
			gaussianLogs = append(gaussianLogs, lf)
			kf = append(kf, filepath.Base(lf))
		}
	}
	if len(kf) > 10 {
		kf = kf[:10]
	}
	rs.KeyFiles = kf

	// Check newest output file
	outputPatterns := []string{"*.chk", "*.fchk"}
	// Add confirmed Gaussian log paths for newest check
	// We use glob patterns for chk/fchk, but for logs we check individually below
	newest, newestFile := newestModTime(dir, outputPatterns)
	// Also check Gaussian logs for newest
	for _, gl := range gaussianLogs {
		fi, err := os.Stat(gl)
		if err != nil {
			continue
		}
		if fi.ModTime().After(newest) {
			newest = fi.ModTime()
			newestFile = filepath.Base(gl)
		}
	}
	if !newest.IsZero() {
		rs.LastUpdate = fmtMinsAgo(newest)
		rs.LastUpdateMins = minsAgo(newest)
	} else {
		rs.LastUpdateMins = -1
	}

	// Read Gaussian log tails for errors and completion
	var logErrors []string
	hasCompletion := false
	phase := ""
	for _, gl := range gaussianLogs {
		fi, err := os.Stat(gl)
		if err != nil || fi.Size() > 10*1024*1024 {
			continue
		}
		tail := readTail(gl, 100)

		// Check errors
		errHits := grepLines(tail, []string{
			"Error termination",
			"Convergence failure",
			"Erroneous write",
			"Link died",
		})
		errHits = filterFalseErrors(errHits)
		logErrors = append(logErrors, errHits...)

		// Check completion
		completionHits := grepLines(tail, []string{"Normal termination of Gaussian"})
		if len(completionHits) > 0 {
			hasCompletion = true
			rs.Evidence = append(rs.Evidence, "Normal termination of Gaussian")
		}

		// Phase detection (highest phase wins)
		for _, line := range tail {
			if strings.Contains(line, "Frequencies --") {
				phase = "frequency"
			}
			if phase != "frequency" {
				if strings.Contains(line, "Optimization completed") || strings.Contains(line, "Stationary point found") {
					phase = "optimization"
				}
			}
			if phase == "" {
				if strings.Contains(line, "SCF Done") {
					phase = "scf"
				}
			}
		}

		// Collect evidence: SCF Done energy, markers, termination
		scfLines := grepLines(tail, []string{"SCF Done"})
		if len(scfLines) > 0 {
			rs.Evidence = append(rs.Evidence, scfLines[len(scfLines)-1])
		}
		optLines := grepLines(tail, []string{"Optimization completed", "Stationary point found"})
		for _, ol := range optLines {
			if len(rs.Evidence) < 12 {
				rs.Evidence = append(rs.Evidence, ol)
			}
		}
		freqLines := grepLines(tail, []string{"Frequencies --"})
		if len(freqLines) > 0 && len(rs.Evidence) < 12 {
			rs.Evidence = append(rs.Evidence, freqLines[0])
		}
	}

	// Set phase; fallback to "input" if gjf/com exist but no log phase
	if phase == "" {
		hasInput, _ := globExists(dir, "*.gjf", "*.com")
		if hasInput > 0 {
			phase = "input"
		}
	}
	if phase != "" {
		rs.ContextPhase = phase
		rs.Evidence = append(rs.Evidence, "phase: "+phase)
	}

	// Add newest file update info to evidence
	if !newest.IsZero() && len(rs.Evidence) < 12 {
		rs.Evidence = append(rs.Evidence, newestFile+" "+fmtMinsAgo(newest)+"更新")
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
	} else if hasCompletion {
		rs.State = "completed"
		rs.Confidence = "high"
		rs.NextActions = []string{"检查收敛结果", "提取能量/频率", "formchk 生成 .fchk"}
	} else if !newest.IsZero() {
		mins := minsAgo(newest)
		if mins <= d.StuckMinutes() {
			rs.State = "running"
			rs.Confidence = "medium"
		} else {
			rs.State = "stuck"
			rs.Confidence = "medium"
			rs.Warnings = append(rs.Warnings, newestFile+" 已 "+itoa(mins)+" 分钟未更新")
		}
	} else {
		hasInput, _ := globExists(dir, "*.gjf", "*.com")
		if hasInput > 0 {
			rs.State = "idle"
			rs.Confidence = "medium"
			rs.Evidence = append(rs.Evidence, "有输入文件但无输出更新")
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

// isGaussianLog reads the head of a file and checks for Gaussian-specific markers.
// Returns false for non-Gaussian .log files (size guard: skip > 10MB).
func isGaussianLog(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() > 10*1024*1024 {
		return false
	}
	lines := readHead(path, 50)
	for _, line := range lines {
		if strings.Contains(line, "Entering Gaussian System") || strings.Contains(line, "Gaussian, Inc.") {
			return true
		}
	}
	return false
}
