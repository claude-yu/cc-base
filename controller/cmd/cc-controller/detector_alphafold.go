package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type alphafoldDetector struct{}

func (d *alphafoldDetector) Name() string     { return "alphafold" }
func (d *alphafoldDetector) StuckMinutes() int { return 45 }

func (d *alphafoldDetector) Match(dir string) (bool, int) {
	score := 0

	if _, err := os.Stat(filepath.Join(dir, "ranking_debug.json")); err == nil {
		score += 30
	}
	if _, err := os.Stat(filepath.Join(dir, "features.pkl")); err == nil {
		score += 20
	}
	if _, err := os.Stat(filepath.Join(dir, "timings.json")); err == nil {
		score += 15
	}
	if fi, err := os.Stat(filepath.Join(dir, "msas")); err == nil && fi.IsDir() {
		score += 15
	}
	if n, _ := globExists(dir, "ranked_*.pdb"); n > 0 {
		score += 20
	}
	if n, _ := globExists(dir, "unrelaxed_model_*.pdb"); n > 0 {
		score += 10
	}
	if n, _ := globExists(dir, "relaxed_model_*.pdb"); n > 0 {
		score += 10
	}

	if n, _ := globExists(dir, "ranking_scores.csv"); n > 0 {
		score += 30
	}
	if n, _ := globExists(dir, "*_model.cif"); n > 0 {
		score += 25
	}
	if countSeedDirs(dir) > 0 {
		score += 20
	}
	if n, _ := globExists(dir, "*_confidences.json"); n > 0 {
		score += 10
	}
	if n, _ := globExists(dir, "*_summary_confidences.json"); n > 0 {
		score += 10
	}

	if n, _ := globExists(dir, "*_scores.json"); n > 0 {
		score += 20
	}
	if n, _ := globExists(dir, "*_unrelaxed_rank_*.pdb"); n > 0 {
		score += 20
	}
	if n, _ := globExists(dir, "*_plddt.png", "*_PAE.png"); n > 0 {
		score += 15
	}
	if n, _ := globExists(dir, "*.done.txt"); n > 0 {
		score += 15
	}
	if n, _ := globExists(dir, "*.a3m"); n > 0 && score >= 20 {
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

func countSeedDirs(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "seed-") {
			count++
		}
	}
	return count
}

func (d *alphafoldDetector) Inspect(dir string) ResearchStatus {
	rs := ResearchStatus{
		Detector:   "alphafold",
		WorkDir:    dir,
		Confidence: "medium",
	}

	variant := detectAFVariant(dir)

	var kf []string
	switch variant {
	case "af2":
		for _, name := range []string{"ranking_debug.json", "features.pkl", "timings.json"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				kf = append(kf, name)
			}
		}
		if _, files := globExists(dir, "ranked_*.pdb"); len(files) > 0 {
			kf = append(kf, files...)
		}
	case "af3":
		if _, err := os.Stat(filepath.Join(dir, "ranking_scores.csv")); err == nil {
			kf = append(kf, "ranking_scores.csv")
		}
		if _, files := globExists(dir, "*_model.cif"); len(files) > 0 {
			for i, f := range files {
				if i >= 5 {
					break
				}
				kf = append(kf, f)
			}
		}
	case "colabfold":
		if _, files := globExists(dir, "*_scores.json"); len(files) > 0 {
			for i, f := range files {
				if i >= 3 {
					break
				}
				kf = append(kf, f)
			}
		}
		if _, files := globExists(dir, "*_unrelaxed_rank_*.pdb"); len(files) > 0 {
			for i, f := range files {
				if i >= 5 {
					break
				}
				kf = append(kf, f)
			}
		}
		if _, files := globExists(dir, "*.done.txt"); len(files) > 0 {
			kf = append(kf, files...)
		}
	}
	if _, files := globExists(dir, "*.a3m"); len(files) > 0 {
		for i, f := range files {
			if i >= 2 || len(kf) >= 12 {
				break
			}
			kf = append(kf, f)
		}
	}
	rs.KeyFiles = kf
	rs.Evidence = append(rs.Evidence, "variant: "+variant)

	rankedPDBs, _ := globExists(dir, "ranked_*.pdb")
	unrelaxedPDBs, _ := globExists(dir, "unrelaxed_model_*.pdb")
	relaxedPDBs, _ := globExists(dir, "relaxed_model_*.pdb")
	cifModels, _ := globExists(dir, "*_model.cif")
	cfRankPDBs, _ := globExists(dir, "*_unrelaxed_rank_*.pdb")
	doneCount, _ := globExists(dir, "*.done.txt")
	seedDirs := countSeedDirs(dir)

	bestPLDDT := ""
	switch variant {
	case "af2":
		bestPLDDT = parseRankingDebug(dir)
		if rankedPDBs > 0 {
			rs.Evidence = append(rs.Evidence, itoa(rankedPDBs)+" ranked PDB")
		}
		if unrelaxedPDBs > 0 {
			rs.Evidence = append(rs.Evidence, itoa(unrelaxedPDBs)+" unrelaxed model PDB")
		}
		if relaxedPDBs > 0 {
			rs.Evidence = append(rs.Evidence, itoa(relaxedPDBs)+" relaxed model PDB")
		}
	case "af3":
		bestPLDDT = parseRankingScoresCSV(dir)
		if cifModels > 0 {
			rs.Evidence = append(rs.Evidence, itoa(cifModels)+" model CIF")
		}
		if seedDirs > 0 {
			rs.Evidence = append(rs.Evidence, itoa(seedDirs)+" seed directories")
		}
	case "colabfold":
		bestPLDDT = parseCFScoresJSON(dir)
		if cfRankPDBs > 0 {
			rs.Evidence = append(rs.Evidence, itoa(cfRankPDBs)+" ranked PDB")
		}
		if doneCount > 0 {
			rs.Evidence = append(rs.Evidence, itoa(doneCount)+" done markers")
		}
	}
	if bestPLDDT != "" {
		rs.Evidence = append(rs.Evidence, "best pLDDT: "+bestPLDDT)
	}

	hasMSA := false
	if fi, err := os.Stat(filepath.Join(dir, "msas")); err == nil && fi.IsDir() {
		hasMSA = true
		rs.Evidence = append(rs.Evidence, "msas/ 目录存在")
	}
	a3mCount, _ := globExists(dir, "*.a3m")
	if a3mCount > 0 {
		hasMSA = true
	}

	allPatterns := []string{
		"ranking_debug.json", "features.pkl", "timings.json",
		"ranked_*.pdb", "unrelaxed_model_*.pdb", "relaxed_model_*.pdb",
		"ranking_scores.csv", "*_model.cif", "*_confidences.json",
		"*_scores.json", "*_unrelaxed_rank_*.pdb", "*.done.txt",
		"*_plddt.png", "*_PAE.png", "*.a3m", "*.log",
	}
	newest, newestFile := newestModTime(dir, allPatterns)
	if !newest.IsZero() {
		rs.LastUpdate = fmtMinsAgo(newest)
		rs.LastUpdateMins = minsAgo(newest)
	} else {
		rs.LastUpdateMins = -1
	}

	var logErrors []string
	for _, p := range []string{"*.log"} {
		matches, _ := filepath.Glob(filepath.Join(dir, p))
		for _, m := range matches {
			tail := readTail(m, 80)
			hits := grepLines(tail, []string{
				"ERROR", "FATAL", "Traceback", "RuntimeError",
				"CUDA out of memory", "FileNotFoundError", "MemoryError",
				"Segmentation fault", "core dumped",
			})
			hits = filterFalseErrors(hits)
			logErrors = append(logErrors, hits...)
		}
	}

	completed := false
	switch variant {
	case "af2":
		if _, err := os.Stat(filepath.Join(dir, "ranking_debug.json")); err == nil && rankedPDBs > 0 {
			completed = true
		}
	case "af3":
		if cifModels > 0 {
			if _, err := os.Stat(filepath.Join(dir, "ranking_scores.csv")); err == nil {
				completed = true
			}
		}
	case "colabfold":
		if doneCount > 0 && cfRankPDBs > 0 {
			completed = true
		}
	}

	phase := afPhase(hasMSA, unrelaxedPDBs > 0 || cifModels > 0 || cfRankPDBs > 0,
		relaxedPDBs > 0, completed,
		func() bool { _, err := os.Stat(filepath.Join(dir, "features.pkl")); return err == nil }())

	rs.ContextPhase = phase

	if len(logErrors) > 0 {
		rs.State = "failed"
		rs.Confidence = "high"
		for _, e := range logErrors {
			if len(rs.Evidence) < 12 {
				rs.Evidence = append(rs.Evidence, "ERROR: "+e)
			}
		}
	} else if completed {
		rs.State = "completed"
		rs.Confidence = "high"
		rs.NextActions = []string{"查看最优模型 pLDDT", "可视化检查结构", "运行后续分析"}
	} else if !newest.IsZero() && minsAgo(newest) <= d.StuckMinutes() {
		rs.State = "running"
		rs.Confidence = "medium"
		rs.Evidence = append(rs.Evidence, newestFile+" "+fmtMinsAgo(newest)+"更新")
	} else if !newest.IsZero() {
		hasPartial := unrelaxedPDBs > 0 || cifModels > 0 || cfRankPDBs > 0 || hasMSA
		if hasPartial {
			rs.State = "stuck"
		} else {
			rs.State = "failed"
		}
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

func detectAFVariant(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "ranking_debug.json")); err == nil {
		return "af2"
	}
	if n, _ := globExists(dir, "unrelaxed_model_*.pdb"); n > 0 {
		return "af2"
	}
	if _, err := os.Stat(filepath.Join(dir, "features.pkl")); err == nil {
		return "af2"
	}

	if n, _ := globExists(dir, "*_model.cif"); n > 0 {
		return "af3"
	}
	if _, err := os.Stat(filepath.Join(dir, "ranking_scores.csv")); err == nil {
		// ranking_scores.csv without *_model.cif could be ColabFold batch output
		if cfn, _ := globExists(dir, "*_scores.json", "*_unrelaxed_rank_*.pdb"); cfn == 0 {
			return "af3"
		}
	}
	if countSeedDirs(dir) > 0 {
		return "af3"
	}

	if n, _ := globExists(dir, "*_scores.json"); n > 0 {
		return "colabfold"
	}
	if n, _ := globExists(dir, "*_unrelaxed_rank_*.pdb"); n > 0 {
		return "colabfold"
	}
	if n, _ := globExists(dir, "*.done.txt"); n > 0 {
		return "colabfold"
	}
	if n, _ := globExists(dir, "*_plddt.png", "*_PAE.png"); n > 0 {
		return "colabfold"
	}

	return "af2"
}

func parseRankingDebug(dir string) string {
	path := filepath.Join(dir, "ranking_debug.json")
	fi, err := os.Stat(path)
	if err != nil || fi.Size() > 10*1024*1024 {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var m map[string]interface{}
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	plddts, ok := m["plddts"].(map[string]interface{})
	if !ok {
		return ""
	}
	best := -1.0
	for _, v := range plddts {
		if val, ok := v.(float64); ok && val > best {
			best = val
		}
	}
	if best < 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", best)
}

func parseRankingScoresCSV(dir string) string {
	path := filepath.Join(dir, "ranking_scores.csv")
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return ""
	}
	scoreIdx := -1
	for i, h := range header {
		lower := strings.ToLower(strings.TrimSpace(h))
		if lower == "ranking_score" || lower == "score" || lower == "plddt" {
			scoreIdx = i
			break
		}
	}
	if scoreIdx < 0 && len(header) > 1 {
		scoreIdx = 1
	}
	if scoreIdx < 0 {
		return ""
	}
	bestVal := ""
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}
		if scoreIdx >= len(row) {
			continue
		}
		val := strings.TrimSpace(row[scoreIdx])
		if looksNumeric(val) {
			if bestVal == "" || val > bestVal {
				bestVal = val
			}
		}
	}
	return bestVal
}

func parseCFScoresJSON(dir string) string {
	matches, _ := filepath.Glob(filepath.Join(dir, "*_scores.json"))
	if len(matches) == 0 {
		return ""
	}
	if len(matches) > 20 {
		matches = matches[:20]
	}
	bestAvg := -1.0
	for _, m := range matches {
		fi, err := os.Stat(m)
		if err != nil || fi.Size() > 10*1024*1024 {
			continue
		}
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		var obj map[string]interface{}
		if json.Unmarshal(data, &obj) != nil {
			continue
		}
		plddt, ok := obj["plddt"]
		if !ok {
			continue
		}
		arr, ok := plddt.([]interface{})
		if !ok || len(arr) == 0 {
			continue
		}
		sum := 0.0
		count := 0
		for _, v := range arr {
			if val, ok := v.(float64); ok {
				sum += val
				count++
			}
		}
		if count > 0 {
			avg := sum / float64(count)
			if avg > bestAvg {
				bestAvg = avg
			}
		}
	}
	if bestAvg < 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", bestAvg)
}

func afPhase(hasMSA, hasModels, hasRelaxed, completed, hasFeatures bool) string {
	if completed {
		return "completed"
	}
	if hasRelaxed {
		return "relaxation"
	}
	if hasModels {
		return "model prediction"
	}
	if hasFeatures {
		return "feature generation"
	}
	if hasMSA {
		return "MSA search"
	}
	return ""
}
