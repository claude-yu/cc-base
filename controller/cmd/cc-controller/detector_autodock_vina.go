package main

import (
	"os"
	"path/filepath"
	"strings"
)

type autodockVinaDetector struct{}

func (d *autodockVinaDetector) Name() string     { return "autodock_vina" }
func (d *autodockVinaDetector) StuckMinutes() int { return 45 }

func (d *autodockVinaDetector) Match(dir string) (bool, int) {
	score := 0

	if hasVinaConfig(dir) {
		score += 35
	}

	if n, _ := globExists(dir, "vina*.log", "*_vina.log"); n > 0 {
		score += 30
	}

	if n, _ := globExists(dir, "*.dlg"); n > 0 {
		score += 35
	}

	if n, _ := globExists(dir, "*.dpf"); n > 0 {
		score += 20
	}

	if n, _ := globExists(dir, "*.gpf"); n > 0 {
		score += 15
	}

	if n, _ := globExists(dir, "autodock*.log", "autogrid*.log"); n > 0 {
		score += 25
	}

	if n, _ := globExists(dir, "*.pdbqt"); n > 0 {
		score += 15
	}

	if countVinaOutputPDBQT(dir) > 0 {
		score += 20
	}

	if score < 40 {
		return false, 0
	}
	if score > 100 {
		score = 100
	}
	return true, score
}

var vinaConfigKeywords = []string{
	"exhaustiveness", "center_x", "center_y", "center_z",
	"size_x", "size_y", "size_z", "num_modes", "energy_range",
}

func hasVinaConfig(dir string) bool {
	for _, name := range []string{"config.txt", "vina_config.txt", "conf.txt"} {
		lines := readHead(filepath.Join(dir, name), 30)
		if len(lines) == 0 {
			continue
		}
		hits := 0
		for _, line := range lines {
			lower := strings.ToLower(line)
			for _, kw := range vinaConfigKeywords {
				if strings.Contains(lower, kw) {
					hits++
					break
				}
			}
		}
		if hits >= 2 {
			return true
		}
	}
	return false
}

func isVinaOutputPDBQT(name string) bool {
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".pdbqt") {
		return false
	}
	base := strings.TrimSuffix(lower, ".pdbqt")
	return strings.HasPrefix(lower, "out") || strings.HasSuffix(base, "_out")
}

func countVinaOutputPDBQT(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && isVinaOutputPDBQT(e.Name()) {
			count++
		}
	}
	return count
}

type vinaLogResult struct {
	bestAffinity string
	modes        int
	completed    bool
}

func parseVinaLogs(dir string) vinaLogResult {
	var result vinaLogResult
	for _, p := range []string{"vina*.log", "*_vina.log"} {
		matches, _ := filepath.Glob(filepath.Join(dir, p))
		for _, m := range matches {
			lines := readHead(m, 200)
			inTable := false
			for _, line := range lines {
				lower := strings.ToLower(line)
				if strings.Contains(lower, "writing output") || strings.Contains(lower, "refine time") {
					result.completed = true
				}
				if strings.Contains(line, "mode |") {
					inTable = true
					continue
				}
				if inTable {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "rmsd") {
						continue
					}
					fields := strings.Fields(trimmed)
					if len(fields) >= 2 && looksNumeric(fields[0]) && looksNumeric(fields[1]) {
						result.modes++
						if result.bestAffinity == "" {
							result.bestAffinity = fields[1]
						}
					} else {
						inTable = false
					}
				}
			}
		}
	}
	return result
}

type dlgResult struct {
	bestEnergy string
	completed  bool
	runs       int
}

func parseDLG(dir string) dlgResult {
	var result dlgResult
	matches, _ := filepath.Glob(filepath.Join(dir, "*.dlg"))
	bestVal := 1e18
	for _, m := range matches {
		lines := readHead(m, 300)
		for _, line := range lines {
			if strings.Contains(line, "Estimated Free Energy of Binding") {
				parts := strings.Split(line, "=")
				if len(parts) >= 2 {
					fields := strings.Fields(strings.TrimSpace(parts[1]))
					if len(fields) > 0 && looksNumeric(fields[0]) {
						result.runs++
						val := parseFloat(fields[0])
						if val < bestVal {
							bestVal = val
							result.bestEnergy = fields[0]
						}
					}
				}
			}
		}
		tail := readTail(m, 50)
		for _, line := range tail {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "final docked state") ||
				strings.Contains(lower, "successful dock") ||
				strings.Contains(lower, "total number of runs") {
				result.completed = true
			}
		}
	}
	return result
}

func (d *autodockVinaDetector) Inspect(dir string) ResearchStatus {
	rs := ResearchStatus{
		Detector:   "autodock_vina",
		WorkDir:    dir,
		Confidence: "medium",
	}

	var kf []string
	for _, name := range []string{"config.txt", "vina_config.txt", "conf.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			kf = append(kf, name)
		}
	}
	for _, pattern := range []string{"vina*.log", "*_vina.log", "*.dlg", "*.dpf", "*.gpf", "autodock*.log", "autogrid*.log"} {
		if _, files := globExists(dir, pattern); len(files) > 0 {
			for _, f := range files {
				kf = append(kf, f)
				if len(kf) >= 10 {
					break
				}
			}
		}
		if len(kf) >= 10 {
			break
		}
	}
	if n, files := globExists(dir, "*.pdbqt"); n > 0 {
		for i, f := range files {
			if i >= 3 || len(kf) >= 12 {
				break
			}
			kf = append(kf, f)
		}
	}
	rs.KeyFiles = kf

	vina := parseVinaLogs(dir)
	dlg := parseDLG(dir)

	if vina.modes > 0 || vina.completed {
		rs.Evidence = append(rs.Evidence, "Vina 对接检测到")
		if vina.bestAffinity != "" {
			rs.Evidence = append(rs.Evidence, "best affinity: "+vina.bestAffinity+" kcal/mol")
		}
		if vina.modes > 0 {
			rs.Evidence = append(rs.Evidence, "modes: "+itoa(vina.modes))
		}
	}
	if dlg.runs > 0 || dlg.completed {
		rs.Evidence = append(rs.Evidence, "AutoDock4 对接检测到")
		if dlg.bestEnergy != "" {
			rs.Evidence = append(rs.Evidence, "best binding energy: "+dlg.bestEnergy+" kcal/mol")
		}
		if dlg.runs > 0 {
			rs.Evidence = append(rs.Evidence, "runs: "+itoa(dlg.runs))
		}
	}

	outCount := countVinaOutputPDBQT(dir)
	if outCount > 0 {
		rs.Evidence = append(rs.Evidence, itoa(outCount)+" 个输出 PDBQT")
	}

	pdbqtCount, _ := globExists(dir, "*.pdbqt")
	if pdbqtCount > 0 && vina.modes == 0 && !vina.completed && dlg.runs == 0 && !dlg.completed {
		rs.Evidence = append(rs.Evidence, itoa(pdbqtCount)+" 个 PDBQT 文件")
	}

	newest, newestFile := newestModTime(dir, []string{
		"vina*.log", "*_vina.log", "autodock*.log", "autogrid*.log",
		"*.dlg", "out*.pdbqt", "*_out.pdbqt", "*.log",
	})
	if !newest.IsZero() {
		rs.LastUpdate = fmtMinsAgo(newest)
		rs.LastUpdateMins = minsAgo(newest)
	} else {
		rs.LastUpdateMins = -1
	}

	var logErrors []string
	logPatterns := []string{"vina*.log", "*_vina.log", "autodock*.log", "autogrid*.log", "*.log"}
	errKeywords := []string{
		"ERROR", "FATAL", "failed", "could not open",
		"Parse error", "bad allocation", "Segmentation fault",
		"grid map not found", "core dumped", "Traceback",
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
	} else if vina.completed || dlg.completed {
		rs.State = "completed"
		rs.Confidence = "high"
		if vina.completed {
			rs.Evidence = append(rs.Evidence, "Vina 日志显示完成")
		}
		if dlg.completed {
			rs.Evidence = append(rs.Evidence, "AutoDock4 DLG 显示完成")
		}
		rs.NextActions = []string{"查看最优构象", "提取对接结果", "可视化检查"}
	} else if outCount > 0 {
		rs.State = "completed"
		rs.Confidence = "medium"
		rs.Evidence = append(rs.Evidence, "输出 PDBQT 存在")
		rs.NextActions = []string{"查看对接结果", "分析结合能"}
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
