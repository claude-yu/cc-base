package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// --- Detector Interface & Types ---

type Detector interface {
	Name() string
	StuckMinutes() int
	Match(dir string) (bool, int) // matched, score 0-100
	Inspect(dir string) ResearchStatus
}

type ResearchStatus struct {
	Detector       string   `json:"detector"`
	State          string   `json:"state"`      // running|completed|stuck|failed|idle|unknown
	Confidence     string   `json:"confidence"` // high|medium|low
	Score          int      `json:"score"`
	Index          int      `json:"index"`                      // 1-based stable index (assigned post-sort)
	Bucket         string   `json:"bucket,omitempty"`           // active_failed|historical_failed|archived_failed
	WorkDir        string   `json:"work_dir"`
	KeyFiles       []string `json:"key_files"`
	LastUpdate     string   `json:"last_update,omitempty"`
	LastUpdateMins int      `json:"last_update_mins,omitempty"` // minutes since last activity (-1 = unknown)
	Evidence       []string `json:"evidence"`
	Warnings       []string `json:"warnings,omitempty"`
	NextActions    []string `json:"next_actions,omitempty"`
	ContextPhase   string   `json:"context_phase,omitempty"`
}

var statePriority = map[string]int{
	"failed": 0, "stuck": 1, "running": 2,
	"completed": 3, "idle": 4, "unknown": 5,
}

func allDetectors() []Detector {
	return []Detector{
		&gromacsDetector{},
		&schrodingerDetector{},
		&haddock3Detector{},
		&rosettaDetector{},
		&autodockVinaDetector{},
		&alphafoldDetector{},
		&amberOpenMMDetector{},
		&gaussianDetector{},
		&pythonDetector{},
		&rDetector{},
		&genericCLIDetector{},
	}
}

var detectorAliases = map[string]string{
	"maestro":    "schrodinger",
	"glide":      "schrodinger",
	"ligprep":    "schrodinger",
	"desmond":    "schrodinger",
	"pyrosetta":  "rosetta",
	"colabfold":  "alphafold",
	"amber":      "amber_openmm",
	"openmm":     "amber_openmm",
	"vina":       "autodock_vina",
	"haddock":    "haddock3",
}

func resolveDetectorAlias(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	if canonical, ok := detectorAliases[lower]; ok {
		return canonical
	}
	for _, d := range allDetectors() {
		if strings.EqualFold(d.Name(), lower) {
			return d.Name()
		}
	}
	return ""
}

// --- Shared Helpers ---

var scanExcludeDirs = map[string]bool{
	".git": true, "node_modules": true, "__pycache__": true,
	"venv": true, ".venv": true, "renv": true, ".snakemake": true,
	"trajectory": true, ".cache": true, ".Rproj.user": true,
	".ipynb_checkpoints": true, "runs": true, "sessions": true,
}

func globExists(dir string, patterns ...string) (int, []string) {
	count := 0
	var found []string
	for _, p := range patterns {
		matches, _ := filepath.Glob(filepath.Join(dir, p))
		for _, m := range matches {
			count++
			found = append(found, filepath.Base(m))
		}
	}
	return count, found
}

func newestModTime(dir string, patterns []string) (time.Time, string) {
	var newest time.Time
	var newestFile string
	for _, p := range patterns {
		matches, _ := filepath.Glob(filepath.Join(dir, p))
		for _, m := range matches {
			if fi, err := os.Stat(m); err == nil && fi.ModTime().After(newest) {
				newest = fi.ModTime()
				newestFile = filepath.Base(m)
			}
		}
	}
	return newest, newestFile
}

func minsAgo(t time.Time) int {
	if t.IsZero() {
		return -1
	}
	return int(time.Since(t).Minutes())
}

func readTail(path string, n int) []string {
	fi, err := os.Stat(path)
	if err != nil {
		return nil
	}
	const maxRead = 1024 * 1024 // 1MB cap
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	size := fi.Size()
	readSize := size
	if readSize > maxRead {
		readSize = maxRead
		f.Seek(size-maxRead, 0)
	}
	buf := make([]byte, readSize)
	nr, _ := f.Read(buf)
	buf = buf[:nr]

	lines := strings.Split(string(buf), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func readHead(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	buf := make([]byte, 8192)
	nr, _ := f.Read(buf)
	if nr == 0 {
		return nil
	}
	lines := strings.Split(string(buf[:nr]), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return lines
}

func grepLines(lines []string, keywords []string) []string {
	var hits []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, kw := range keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				trimmed := strings.TrimSpace(line)
				if len(trimmed) > 150 {
					trimmed = trimmed[:150] + "..."
				}
				hits = append(hits, trimmed)
				break
			}
		}
	}
	return hits
}

func isNegatedFailure(line string) bool {
	lower := strings.ToLower(line)
	if !strings.Contains(lower, "failed") && !strings.Contains(lower, "error") {
		return false
	}
	if strings.Contains(lower, "0 job") && strings.Contains(lower, "failed") {
		return true
	}
	if strings.Contains(lower, "0 task") && strings.Contains(lower, "failed") {
		return true
	}
	if (strings.Contains(lower, "succeeded") || strings.Contains(lower, "completed")) &&
		strings.Contains(lower, "; 0") && strings.Contains(lower, "failed") {
		return true
	}
	return false
}

func filterFalseErrors(hits []string) []string {
	var filtered []string
	for _, h := range hits {
		if !isNegatedFailure(h) {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

func parseJSON(path string) map[string]interface{} {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() > 10*1024*1024 {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	return m
}

func fmtMinsAgo(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	m := minsAgo(t)
	if m < 1 {
		return "<1 分钟前"
	}
	if m < 60 {
		return strings.Replace(strings.Replace("%d 分钟前", "%d", itoa(m), 1), "", "", 0)
	}
	h := m / 60
	return itoa(h) + " 小时 " + itoa(m%60) + " 分钟前"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

// --- GROMACS Detector ---

type gromacsDetector struct{}

func (d *gromacsDetector) Name() string      { return "gromacs" }
func (d *gromacsDetector) StuckMinutes() int  { return 30 }

func (d *gromacsDetector) Match(dir string) (bool, int) {
	score := 0
	if n, _ := globExists(dir, "*.tpr"); n > 0 {
		score += 30
	}
	if n, _ := globExists(dir, "md.log", "*.log"); n > 0 {
		for _, p := range []string{"md.log"} {
			if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
				score += 25
				break
			}
		}
		if score < 30 { // only generic *.log, not md.log
			score += 10
		}
	}
	if n, _ := globExists(dir, "*.cpt"); n > 0 {
		score += 20
	}
	if n, _ := globExists(dir, "*.xtc", "*.trr", "*.edr"); n > 0 {
		score += 15
	}
	if score < 40 {
		return false, 0
	}
	if score > 100 {
		score = 100
	}
	return true, score
}

func (d *gromacsDetector) Inspect(dir string) ResearchStatus {
	rs := ResearchStatus{
		Detector: d.Name(),
		WorkDir:  dir,
	}

	// Collect key files
	_, keyFiles := globExists(dir, "*.tpr", "md.log", "*.cpt", "*.xtc", "*.trr", "*.edr", "*.gro", "*.top")
	rs.KeyFiles = keyFiles

	// Check newest output file
	newest, newestFile := newestModTime(dir, []string{"md.log", "*.cpt", "*.xtc", "*.edr", "*.trr"})
	if !newest.IsZero() {
		rs.LastUpdate = fmtMinsAgo(newest)
		rs.LastUpdateMins = minsAgo(newest)
	} else {
		rs.LastUpdateMins = -1
	}

	// Read md.log tail
	mdLogPath := filepath.Join(dir, "md.log")
	var logErrors []string
	var hasFinished bool

	tail := readTail(mdLogPath, 80)
	if len(tail) > 0 {
		rs.Evidence = append(rs.Evidence, "md.log: "+itoa(len(tail))+" 行可读")

		errPatterns := []string{"Fatal error", "GROMACS error", "Segmentation fault", "nan", "Halting program"}
		logErrors = grepLines(tail, errPatterns)

		for _, line := range tail {
			if strings.Contains(line, "Finished mdrun") {
				hasFinished = true
				break
			}
		}

		// Extract performance from last lines
		for i := len(tail) - 1; i >= 0 && i >= len(tail)-20; i-- {
			if strings.Contains(tail[i], "ns/day") || strings.Contains(tail[i], "Performance") {
				rs.Evidence = append(rs.Evidence, strings.TrimSpace(tail[i]))
				break
			}
		}
	}

	// Check for post-processing outputs (trjconv results → simulation was completed)
	hasPostProcessing := false
	postFiles := []string{"md_fit.xtc", "md_nojump.xtc", "md_cluster.xtc"}
	var foundPost []string
	for _, pf := range postFiles {
		if _, err := os.Stat(filepath.Join(dir, pf)); err == nil {
			hasPostProcessing = true
			foundPost = append(foundPost, pf)
		}
	}
	// Also check for analysis XVG files
	xvgCount, _ := globExists(dir, "*.xvg")
	if xvgCount > 0 {
		hasPostProcessing = true
	}

	// Determine state
	if len(logErrors) > 0 {
		rs.State = "failed"
		rs.Confidence = "high"
		for _, e := range logErrors {
			rs.Evidence = append(rs.Evidence, "ERROR: "+e)
		}
	} else if hasFinished {
		rs.State = "completed"
		rs.Confidence = "high"
		rs.Evidence = append(rs.Evidence, "md.log 包含 Finished mdrun")
		if hasPostProcessing {
			rs.Evidence = append(rs.Evidence, "已完成后处理: "+strings.Join(foundPost, ", "))
		} else {
			rs.NextActions = []string{"RMSD", "RMSF", "Rg", "SASA", "H-bond", "MM-PBSA/GBSA"}
		}
	} else if hasPostProcessing {
		rs.State = "completed"
		rs.Confidence = "medium"
		rs.Evidence = append(rs.Evidence, "检测到后处理文件: "+strings.Join(foundPost, ", "))
		if xvgCount > 0 {
			rs.Evidence = append(rs.Evidence, itoa(xvgCount)+" 个 XVG 分析文件")
		}
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
		tprN, _ := globExists(dir, "*.tpr")
		if tprN > 0 {
			rs.State = "idle"
			rs.Confidence = "medium"
			rs.Evidence = append(rs.Evidence, "有 .tpr 但无输出文件更新")
		} else {
			rs.State = "unknown"
			rs.Confidence = "low"
			rs.Evidence = append(rs.Evidence, "证据不足")
		}
	}

	if len(rs.Evidence) == 0 {
		rs.Evidence = []string{"无可用证据"}
	}
	return rs
}

// --- Python Pipeline Detector ---

type pythonDetector struct{}

func (d *pythonDetector) Name() string      { return "python_pipeline" }
func (d *pythonDetector) StuckMinutes() int  { return 45 }

func (d *pythonDetector) Match(dir string) (bool, int) {
	score := 0

	if _, err := os.Stat(filepath.Join(dir, "run_pipeline.py")); err == nil {
		score += 35
	}
	if _, err := os.Stat(filepath.Join(dir, "config.py")); err == nil {
		score += 15
	}
	for _, name := range []string{"main.py", "run.py"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			score += 20
			break
		}
	}
	// context.json is a strong signal for structured pipelines
	for _, name := range []string{"context.json", "pharmcell_context.json", "tcga_prognosis_context.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			score += 20
			break
		}
	}
	if n, _ := globExists(dir, "checkpoints"); n > 0 {
		if fi, err := os.Stat(filepath.Join(dir, "checkpoints")); err == nil && fi.IsDir() {
			score += 10
		}
	}

	if score < 20 {
		return false, 0
	}
	if score > 100 {
		score = 100
	}
	return true, score
}

func (d *pythonDetector) Inspect(dir string) ResearchStatus {
	rs := ResearchStatus{
		Detector: d.Name(),
		WorkDir:  dir,
	}

	// Collect key files
	_, keyFiles := globExists(dir, "run_pipeline.py", "main.py", "run.py", "config.py",
		"context.json", "pharmcell_context.json", "tcga_prognosis_context.json")
	rs.KeyFiles = keyFiles

	// --- C1: context.json as first-class citizen ---
	ctxParsed := false
	for _, ctxName := range []string{"context.json", "pharmcell_context.json", "tcga_prognosis_context.json"} {
		ctxPath := filepath.Join(dir, ctxName)
		ctx := parseJSON(ctxPath)
		if ctx == nil {
			continue
		}
		ctxParsed = true
		rs.Evidence = append(rs.Evidence, ctxName+" 已解析")

		phase := extractContextPhase(ctx, ctxName)
		if phase != "" {
			rs.ContextPhase = phase
			rs.Evidence = append(rs.Evidence, "进度: "+phase)
		}

		if hasContextError(ctx) {
			rs.Evidence = append(rs.Evidence, ctxName+" 中检测到 failed 状态")
		}
		break
	}

	// Check checkpoints directory
	cpDir := filepath.Join(dir, "checkpoints")
	if fi, err := os.Stat(cpDir); err == nil && fi.IsDir() {
		cpCount, _ := globExists(cpDir, "*.json")
		doneCount, _ := globExists(cpDir, "*.done")
		if cpCount > 0 {
			rs.Evidence = append(rs.Evidence, "checkpoints/: "+itoa(cpCount)+" json, "+itoa(doneCount)+" done")
		}
	}

	// Check results/figures/outputs
	for _, sub := range []string{"results", "figures", "outputs"} {
		if fi, err := os.Stat(filepath.Join(dir, sub)); err == nil && fi.IsDir() {
			rs.Evidence = append(rs.Evidence, sub+"/ 目录存在")
		}
	}

	// Check for completion markers
	hasReport := false
	for _, rpt := range []string{"REPORT.md", "analysis_report.md", "run_report.md"} {
		if _, err := os.Stat(filepath.Join(dir, rpt)); err == nil {
			hasReport = true
			rs.Evidence = append(rs.Evidence, rpt+" 已生成")
			break
		}
	}

	// Check newest output for running/stuck
	outputPatterns := []string{"*.csv", "*.json", "*.h5ad", "*.png", "*.pdf"}
	subDirPatterns := []string{}
	for _, sub := range []string{"results", "outputs", "figures", "checkpoints"} {
		for _, ext := range []string{"*.csv", "*.json", "*.png", "*.h5ad"} {
			subDirPatterns = append(subDirPatterns, filepath.Join(sub, ext))
		}
	}
	allPatterns := append(outputPatterns, subDirPatterns...)
	newest, newestFile := newestModTime(dir, allPatterns)
	if !newest.IsZero() {
		rs.LastUpdate = fmtMinsAgo(newest)
		rs.LastUpdateMins = minsAgo(newest)
	} else {
		rs.LastUpdateMins = -1
	}

	// Check logs for errors
	var logErrors []string
	for _, logPattern := range []string{"*.log", "logs/*.log"} {
		logMatches, _ := filepath.Glob(filepath.Join(dir, logPattern))
		for _, logPath := range logMatches {
			tail := readTail(logPath, 80)
			pyErrors := []string{"Traceback", "ModuleNotFoundError", "ImportError",
				"CUDA out of memory", "RuntimeError", "FileNotFoundError", "MemoryError"}
			hits := grepLines(tail, pyErrors)
			logErrors = append(logErrors, hits...)
		}
	}

	// Determine state
	if len(logErrors) > 0 {
		rs.State = "failed"
		rs.Confidence = "high"
		for _, e := range logErrors {
			if len(rs.Evidence) < 10 {
				rs.Evidence = append(rs.Evidence, "ERROR: "+e)
			}
		}
	} else if hasReport {
		rs.State = "completed"
		rs.Confidence = "high"
		rs.NextActions = []string{"查看报告", "检查 metrics/Top-N", "验证输出完整性"}
	} else if ctxParsed && rs.ContextPhase != "" {
		// Use context.json status if available
		if strings.Contains(rs.ContextPhase, "完成") || strings.Contains(rs.ContextPhase, "done") || strings.Contains(rs.ContextPhase, "24/24") {
			rs.State = "completed"
			rs.Confidence = "high"
		} else if !newest.IsZero() && minsAgo(newest) <= d.StuckMinutes() {
			rs.State = "running"
			rs.Confidence = "high" // context.json + recent output = high
		} else if !newest.IsZero() {
			rs.State = "stuck"
			rs.Confidence = "medium"
			rs.Warnings = append(rs.Warnings, newestFile+" 已 "+itoa(minsAgo(newest))+" 分钟未更新")
		} else {
			rs.State = "idle"
			rs.Confidence = "medium"
		}
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
		rs.State = "unknown"
		rs.Confidence = "low"
		rs.Evidence = append(rs.Evidence, "发现 Python 项目文件但无日志或 checkpoint")
	}

	if len(rs.Evidence) == 0 {
		rs.Evidence = []string{"无可用证据"}
	}
	return rs
}

// extractContextPhase reads structured progress from context.json variants.
func extractContextPhase(ctx map[string]interface{}, filename string) string {
	// pharmcell: scripts map with status fields
	if scripts, ok := ctx["scripts"].(map[string]interface{}); ok {
		total := len(scripts)
		done := 0
		failed := 0
		for _, v := range scripts {
			if script, ok := v.(map[string]interface{}); ok {
				if s, _ := script["status"].(string); s == "success" || s == "completed" {
					done++
				} else if s == "failed" {
					failed++
				}
			}
		}
		parts := itoa(done) + "/" + itoa(total) + " scripts done"
		if failed > 0 {
			parts += ", " + itoa(failed) + " failed"
		}
		return parts
	}

	// tcga-prognosis: checkpoints with boolean flags
	if cp, ok := ctx["checkpoints"].(map[string]interface{}); ok {
		total := len(cp)
		done := 0
		for _, v := range cp {
			switch val := v.(type) {
			case bool:
				if val {
					done++
				}
			case []interface{}:
				if len(val) > 0 {
					done++
				}
			case float64:
				if val > 0 {
					done++
				}
			case string:
				if val != "" {
					done++
				}
			}
		}
		return itoa(done) + "/" + itoa(total) + " checkpoints filled"
	}

	// Generic: look for current_phase, phase, status
	if phase, ok := ctx["current_phase"].(string); ok {
		return "current_phase = " + phase
	}
	if phase, ok := ctx["phase"].(string); ok {
		return "phase = " + phase
	}
	if execSummary, ok := ctx["execution_summary"].(map[string]interface{}); ok {
		if completed, ok := execSummary["total_completed"].(float64); ok {
			total := 0.0
			if t, ok := execSummary["total_scripts"].(float64); ok {
				total = t
			}
			if total > 0 {
				return itoa(int(completed)) + "/" + itoa(int(total)) + " scripts done"
			}
		}
	}

	return ""
}

func hasContextError(ctx map[string]interface{}) bool {
	if scripts, ok := ctx["scripts"].(map[string]interface{}); ok {
		for _, v := range scripts {
			if script, ok := v.(map[string]interface{}); ok {
				if s, _ := script["status"].(string); s == "failed" {
					return true
				}
			}
		}
	}
	return false
}

// --- R Pipeline Detector ---

type rDetector struct{}

func (d *rDetector) Name() string      { return "r_pipeline" }
func (d *rDetector) StuckMinutes() int  { return 45 }

func (d *rDetector) Match(dir string) (bool, int) {
	score := 0

	if _, err := os.Stat(filepath.Join(dir, "run_pipeline.R")); err == nil {
		score += 35
	}
	if _, err := os.Stat(filepath.Join(dir, "config.R")); err == nil {
		score += 15
	}
	if _, err := os.Stat(filepath.Join(dir, "research_context.R")); err == nil {
		score += 10
	}
	// context.json from R pipelines
	for _, name := range []string{"research_context.json", "context.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			score += 15
			break
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "renv.lock")); err == nil {
		score += 5
	}
	if n, _ := globExists(dir, "*.R"); n > 0 && score < 20 {
		score += 10
	}
	// results/ dir with output files is a strong R project signal
	if fi, err := os.Stat(filepath.Join(dir, "results")); err == nil && fi.IsDir() {
		if n, _ := globExists(dir, "results/*.rds", "results/*.csv", "results/*.RData"); n > 0 {
			score += 10
		}
	}
	if n, _ := globExists(dir, "*.rds", "*.RData"); n > 0 && score < 20 {
		score += 5
	}

	if score < 20 {
		return false, 0
	}
	if score > 100 {
		score = 100
	}
	return true, score
}

func (d *rDetector) Inspect(dir string) ResearchStatus {
	rs := ResearchStatus{
		Detector: d.Name(),
		WorkDir:  dir,
	}

	_, keyFiles := globExists(dir, "run_pipeline.R", "config.R", "research_context.R",
		"research_context.json", "context.json", "renv.lock")
	rs.KeyFiles = keyFiles

	// C1: Parse research_context.json or context.json
	ctxParsed := false
	for _, ctxName := range []string{"research_context.json", "context.json"} {
		ctx := parseJSON(filepath.Join(dir, ctxName))
		if ctx == nil {
			continue
		}
		ctxParsed = true
		rs.Evidence = append(rs.Evidence, ctxName+" 已解析")
		phase := extractContextPhase(ctx, ctxName)
		if phase != "" {
			rs.ContextPhase = phase
			rs.Evidence = append(rs.Evidence, "进度: "+phase)
		}
		break
	}

	// Check output directories
	for _, sub := range []string{"results", "figures", "processed"} {
		if fi, err := os.Stat(filepath.Join(dir, sub)); err == nil && fi.IsDir() {
			rs.Evidence = append(rs.Evidence, sub+"/ 目录存在")
		}
	}

	// Check R output files
	rdsCount, _ := globExists(dir, "*.rds", "*.RData", "results/*.rds")
	routCount, _ := globExists(dir, "*.Rout")
	if rdsCount > 0 {
		rs.Evidence = append(rs.Evidence, itoa(rdsCount)+" 个 RDS/RData 文件")
	}

	// Check newest output
	newest, newestFile := newestModTime(dir, []string{
		"*.rds", "*.RData", "*.Rout", "*.csv", "*.pdf", "*.png",
		"results/*.csv", "results/*.rds", "figures/*.png", "figures/*.pdf",
		"processed/*.csv", "processed/*.rds",
	})
	if !newest.IsZero() {
		rs.LastUpdate = fmtMinsAgo(newest)
		rs.LastUpdateMins = minsAgo(newest)
	} else {
		rs.LastUpdateMins = -1
	}

	// Check .Rout logs for errors
	var logErrors []string
	routFiles, _ := filepath.Glob(filepath.Join(dir, "*.Rout"))
	for _, rout := range routFiles {
		tail := readTail(rout, 80)
		rErrors := []string{"Error in", "Execution halted", "there is no package called",
			"cannot open file", "not found", "subscript out of bounds"}
		hits := grepLines(tail, rErrors)
		logErrors = append(logErrors, hits...)
	}

	// Determine state
	if len(logErrors) > 0 {
		rs.State = "failed"
		rs.Confidence = "high"
		for _, e := range logErrors {
			if len(rs.Evidence) < 10 {
				rs.Evidence = append(rs.Evidence, "ERROR: "+e)
			}
		}
	} else if ctxParsed && rs.ContextPhase != "" && (strings.Contains(rs.ContextPhase, "完成") || strings.Contains(rs.ContextPhase, "done")) {
		rs.State = "completed"
		rs.Confidence = "high"
		rs.NextActions = []string{"检查 PDF/CSV/RDS 输出", "汇总报告", "关键图质量"}
	} else if !newest.IsZero() {
		mins := minsAgo(newest)
		rs.Evidence = append(rs.Evidence, newestFile+" "+fmtMinsAgo(newest)+"更新")
		if mins <= d.StuckMinutes() {
			rs.State = "running"
			if ctxParsed {
				rs.Confidence = "high"
			} else {
				rs.Confidence = "medium"
			}
		} else {
			rs.State = "stuck"
			rs.Confidence = "medium"
			rs.Warnings = append(rs.Warnings, newestFile+" 已 "+itoa(mins)+" 分钟未更新")
		}
	} else if routCount > 0 || rdsCount > 0 {
		rs.State = "idle"
		rs.Confidence = "medium"
		rs.Evidence = append(rs.Evidence, "有 R 输出文件但未检测到活动更新")
	} else {
		rs.State = "unknown"
		rs.Confidence = "low"
		rs.Evidence = append(rs.Evidence, "发现 R 项目文件但无日志或输出")
	}

	if len(rs.Evidence) == 0 {
		rs.Evidence = []string{"无可用证据"}
	}
	return rs
}

// --- Generic CLI Detector ---

type genericCLIDetector struct{}

func (d *genericCLIDetector) Name() string      { return "generic_cli" }
func (d *genericCLIDetector) StuckMinutes() int  { return 30 }

func (d *genericCLIDetector) Match(dir string) (bool, int) {
	score := 0
	if n, _ := globExists(dir, "*.log"); n > 0 {
		score += 15
	}
	if n, _ := globExists(dir, "*.out"); n > 0 {
		score += 10
	}
	if n, _ := globExists(dir, "*.err", "stderr.txt"); n > 0 {
		score += 5
	}
	if _, err := os.Stat(filepath.Join(dir, "stdout.txt")); err == nil {
		score += 10
	}
	if _, err := os.Stat(filepath.Join(dir, "status.json")); err == nil {
		score += 10
	}

	if score < 25 {
		return false, 0
	}
	if score > 100 {
		score = 100
	}
	return true, score
}

func (d *genericCLIDetector) Inspect(dir string) ResearchStatus {
	rs := ResearchStatus{
		Detector: d.Name(),
		WorkDir:  dir,
	}

	_, keyFiles := globExists(dir, "*.log", "*.out", "*.err", "stdout.txt", "stderr.txt", "status.json")
	rs.KeyFiles = keyFiles

	// Find newest log/out file
	newest, newestFile := newestModTime(dir, []string{"*.log", "*.out", "stdout.txt"})
	if !newest.IsZero() {
		rs.LastUpdate = fmtMinsAgo(newest)
		rs.LastUpdateMins = minsAgo(newest)
	} else {
		rs.LastUpdateMins = -1
	}

	// Read the most recent log file
	var allErrors []string
	var hasSuccess bool
	logPatterns := []string{"*.log", "*.out"}
	for _, p := range logPatterns {
		matches, _ := filepath.Glob(filepath.Join(dir, p))
		for _, m := range matches {
			tail := readTail(m, 80)
			errHits := grepLines(tail, []string{"error", "fatal", "failed", "FATAL", "ERROR", "FAILED"})
			errHits = filterFalseErrors(errHits)
			allErrors = append(allErrors, errHits...)
			successHits := grepLines(tail, []string{"success", "done", "completed", "finished", "SUCCESS", "DONE"})
			if len(successHits) > 0 {
				hasSuccess = true
			}
		}
	}

	// Check stderr
	stderrPath := filepath.Join(dir, "stderr.txt")
	if stderrTail := readTail(stderrPath, 20); len(stderrTail) > 0 {
		errHits := grepLines(stderrTail, []string{"error", "fatal", "failed"})
		errHits = filterFalseErrors(errHits)
		allErrors = append(allErrors, errHits...)
	}

	if len(allErrors) > 0 {
		rs.State = "failed"
		rs.Confidence = "medium"
		for _, e := range allErrors {
			if len(rs.Evidence) < 8 {
				rs.Evidence = append(rs.Evidence, "ERROR: "+e)
			}
		}
	} else if hasSuccess {
		rs.State = "completed"
		rs.Confidence = "medium"
		rs.Evidence = append(rs.Evidence, "日志中检测到 success/done/completed")
		rs.NextActions = []string{"查看最终输出", "检查 exit code"}
	} else if !newest.IsZero() {
		mins := minsAgo(newest)
		rs.Evidence = append(rs.Evidence, newestFile+" "+fmtMinsAgo(newest)+"更新")
		if mins <= d.StuckMinutes() {
			rs.State = "running"
			rs.Confidence = "low"
		} else {
			rs.State = "stuck"
			rs.Confidence = "low"
			rs.Warnings = append(rs.Warnings, newestFile+" 已 "+itoa(mins)+" 分钟未更新")
		}
	} else {
		rs.State = "unknown"
		rs.Confidence = "low"
		rs.Evidence = append(rs.Evidence, "发现日志文件但无法确定状态")
	}

	if len(rs.Evidence) == 0 {
		rs.Evidence = []string{"无可用证据"}
	}
	return rs
}
