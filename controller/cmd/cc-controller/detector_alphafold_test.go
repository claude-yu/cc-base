package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Match tests ---

func TestAlphaFoldMatch_AF2Complete(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ranking_debug.json"),
		[]byte(`{"order":["model_1","model_2"],"plddts":{"model_1":85.5,"model_2":82.3}}`), 0644)
	os.WriteFile(filepath.Join(dir, "features.pkl"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "ranked_0.pdb"), []byte("ATOM\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "msas"), 0755)

	d := &alphafoldDetector{}
	matched, score := d.Match(dir)
	if !matched || score < 60 {
		t.Errorf("AF2 complete: matched=%v score=%d, want matched+score>=60", matched, score)
	}
}

func TestAlphaFoldMatch_AF3Complete(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test_model.cif"), []byte("data_\n"), 0644)
	os.WriteFile(filepath.Join(dir, "ranking_scores.csv"),
		[]byte("rank,seed,sample,score\n1,0,0,0.85\n2,0,1,0.82\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "seed-0_sample-0"), 0755)

	d := &alphafoldDetector{}
	matched, score := d.Match(dir)
	if !matched || score < 60 {
		t.Errorf("AF3 complete: matched=%v score=%d, want matched+score>=60", matched, score)
	}
}

func TestAlphaFoldMatch_ColabFoldComplete(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.a3m"), []byte(">seq\nMKT\n"), 0644)
	os.WriteFile(filepath.Join(dir, "test_unrelaxed_rank_1_model_1.pdb"), []byte("ATOM\n"), 0644)
	os.WriteFile(filepath.Join(dir, "test_scores.json"),
		[]byte(`{"plddt":[85.5,90.2,78.1]}`), 0644)
	os.WriteFile(filepath.Join(dir, "test.done.txt"), []byte{}, 0644)

	d := &alphafoldDetector{}
	matched, score := d.Match(dir)
	if !matched || score < 50 {
		t.Errorf("ColabFold complete: matched=%v score=%d, want matched+score>=50", matched, score)
	}
}

func TestAlphaFoldMatch_AF2MSAOnly(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "msas"), 0755)
	os.WriteFile(filepath.Join(dir, "msas", "bfd_uniref_hits.a3m"), []byte(">hit\nMKT\n"), 0644)

	d := &alphafoldDetector{}
	matched, score := d.Match(dir)
	// msas/ alone gives 15 points, below threshold 40 → no match
	if matched {
		t.Errorf("AF2 MSA only: matched=%v score=%d, want no match (score<40)", matched, score)
	}
}

func TestAlphaFoldMatch_ColabFoldA3MOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.a3m"), []byte(">seq\nMKT\n"), 0644)

	d := &alphafoldDetector{}
	matched, _ := d.Match(dir)
	// A single .a3m is too generic to match
	if matched {
		t.Error("ColabFold a3m alone should not match (too generic)")
	}
}

func TestAlphaFoldMatch_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	d := &alphafoldDetector{}
	matched, _ := d.Match(dir)
	if matched {
		t.Error("empty dir should not match")
	}
}

func TestAlphaFoldMatch_PDBOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "output.pdb"), []byte("ATOM\n"), 0644)

	d := &alphafoldDetector{}
	matched, _ := d.Match(dir)
	if matched {
		t.Error("random PDB alone should not match")
	}
}

func TestAlphaFoldMatch_AF2Partial(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "features.pkl"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "unrelaxed_model_1_pred_0.pdb"), []byte("ATOM\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "msas"), 0755)

	d := &alphafoldDetector{}
	matched, score := d.Match(dir)
	// features.pkl(20) + unrelaxed_model(10) + msas/(15) = 45 ≥ 40
	if !matched {
		t.Errorf("AF2 partial (running): matched=%v score=%d, want matched", matched, score)
	}
}

// --- Inspect tests ---

func TestAlphaFoldInspect_AF2Completed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ranking_debug.json"),
		[]byte(`{"order":["model_1","model_2"],"plddts":{"model_1":85.5,"model_2":82.3}}`), 0644)
	os.WriteFile(filepath.Join(dir, "features.pkl"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "ranked_0.pdb"), []byte("ATOM\n"), 0644)
	os.WriteFile(filepath.Join(dir, "ranked_1.pdb"), []byte("ATOM\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "msas"), 0755)

	d := &alphafoldDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
	// Should report pLDDT in evidence
	hasPLDDT := false
	for _, e := range rs.Evidence {
		if strings.Contains(e, "85.5") || strings.Contains(e, "pLDDT") || strings.Contains(e, "plddt") {
			hasPLDDT = true
			break
		}
	}
	if !hasPLDDT {
		t.Error("should report best pLDDT (85.5) in evidence")
	}
}

func TestAlphaFoldInspect_AF3Completed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test_model.cif"), []byte("data_\n"), 0644)
	os.WriteFile(filepath.Join(dir, "ranking_scores.csv"),
		[]byte("rank,seed,sample,score\n1,0,0,0.85\n2,0,1,0.82\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "seed-0_sample-0"), 0755)

	d := &alphafoldDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed", rs.State)
	}
}

func TestAlphaFoldInspect_ColabFoldCompleted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.a3m"), []byte(">seq\nMKT\n"), 0644)
	os.WriteFile(filepath.Join(dir, "test_unrelaxed_rank_1_model_1.pdb"), []byte("ATOM\n"), 0644)
	os.WriteFile(filepath.Join(dir, "test_scores.json"),
		[]byte(`{"plddt":[85.5,90.2,78.1]}`), 0644)
	os.WriteFile(filepath.Join(dir, "test.done.txt"), []byte{}, 0644)

	d := &alphafoldDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed", rs.State)
	}
}

func TestAlphaFoldInspect_Running(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "features.pkl"), []byte{}, 0644)
	os.MkdirAll(filepath.Join(dir, "msas"), 0755)
	pdbPath := filepath.Join(dir, "unrelaxed_model_1_pred_0.pdb")
	os.WriteFile(pdbPath, []byte("ATOM\n"), 0644)
	// recently modified → running
	os.Chtimes(pdbPath, time.Now(), time.Now())

	d := &alphafoldDetector{}
	rs := d.Inspect(dir)
	if rs.State != "running" {
		t.Errorf("state=%q, want running (recent unrelaxed model)", rs.State)
	}
}

func TestAlphaFoldInspect_Stuck(t *testing.T) {
	dir := t.TempDir()
	staleTime := time.Now().Add(-90 * time.Minute)
	os.WriteFile(filepath.Join(dir, "features.pkl"), []byte{}, 0644)
	os.Chtimes(filepath.Join(dir, "features.pkl"), staleTime, staleTime)
	os.MkdirAll(filepath.Join(dir, "msas"), 0755)
	pdbPath := filepath.Join(dir, "unrelaxed_model_1_pred_0.pdb")
	os.WriteFile(pdbPath, []byte("ATOM\n"), 0644)
	os.Chtimes(pdbPath, staleTime, staleTime)

	d := &alphafoldDetector{}
	rs := d.Inspect(dir)
	if rs.State != "stuck" {
		t.Errorf("state=%q, want stuck (old unrelaxed model, >45min stale)", rs.State)
	}
}

func TestAlphaFoldInspect_Failed(t *testing.T) {
	dir := t.TempDir()
	staleTime := time.Now().Add(-2 * time.Hour)
	os.MkdirAll(filepath.Join(dir, "msas"), 0755)
	os.Chtimes(filepath.Join(dir, "msas"), staleTime, staleTime)
	// msas exist but no model output → failed
	os.WriteFile(filepath.Join(dir, "features.pkl"), []byte{}, 0644)
	os.Chtimes(filepath.Join(dir, "features.pkl"), staleTime, staleTime)
	// No unrelaxed/ranked PDB, no ranking_debug.json → failed
	// Add a log with error to trigger failed state
	logPath := filepath.Join(dir, "alphafold.log")
	os.WriteFile(logPath, []byte("RuntimeError: CUDA out of memory\n"), 0644)
	os.Chtimes(logPath, staleTime, staleTime)

	d := &alphafoldDetector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed (msas exist, no model output, error in log)", rs.State)
	}
}

func TestAlphaFoldInspect_StaleFeaturesNoOutput(t *testing.T) {
	dir := t.TempDir()
	staleTime := time.Now().Add(-90 * time.Minute)
	os.WriteFile(filepath.Join(dir, "features.pkl"), []byte{}, 0644)
	os.Chtimes(filepath.Join(dir, "features.pkl"), staleTime, staleTime)

	d := &alphafoldDetector{}
	rs := d.Inspect(dir)
	// features.pkl stale, no models, no MSA → failed
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed (stale features.pkl, no output)", rs.State)
	}
}

func TestAlphaFoldInspect_ColabFoldRunningNoDone(t *testing.T) {
	dir := t.TempDir()
	// ColabFold in progress: has a3m + ranked PDB but no done.txt → running
	os.WriteFile(filepath.Join(dir, "seq1.a3m"), []byte(">seq1\nMKT\n"), 0644)
	pdbPath := filepath.Join(dir, "seq1_unrelaxed_rank_1_model_1.pdb")
	os.WriteFile(pdbPath, []byte("ATOM\n"), 0644)
	os.WriteFile(filepath.Join(dir, "seq1_scores.json"),
		[]byte(`{"plddt":[80.0]}`), 0644)
	os.Chtimes(pdbPath, time.Now(), time.Now())

	d := &alphafoldDetector{}
	rs := d.Inspect(dir)
	if rs.State != "running" {
		t.Errorf("state=%q, want running (colabfold no done.txt yet)", rs.State)
	}
}

// --- Helper tests ---

func TestParseRankingDebug(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ranking_debug.json"),
		[]byte(`{"order":["model_1","model_2"],"plddts":{"model_1":85.5,"model_2":82.3}}`), 0644)

	best := parseRankingDebug(dir)
	if best != "85.50" {
		t.Errorf("parseRankingDebug best pLDDT = %q, want %q", best, "85.50")
	}
}

func TestParseRankingDebug_Missing(t *testing.T) {
	best := parseRankingDebug(t.TempDir())
	if best != "" {
		t.Errorf("parseRankingDebug missing file = %q, want empty", best)
	}
}

func TestParseRankingScoresCSV(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ranking_scores.csv"),
		[]byte("rank,seed,sample,score\n1,0,0,0.85\n2,0,1,0.82\n"), 0644)

	best := parseRankingScoresCSV(dir)
	if best != "0.85" {
		t.Errorf("parseRankingScoresCSV best score = %q, want %q", best, "0.85")
	}
}

func TestParseRankingScoresCSV_Missing(t *testing.T) {
	best := parseRankingScoresCSV(t.TempDir())
	if best != "" {
		t.Errorf("parseRankingScoresCSV missing file = %q, want empty", best)
	}
}

// --- JSON round-trip test ---

func TestAlphaFoldInspect_JSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ranking_debug.json"),
		[]byte(`{"order":["model_1","model_2"],"plddts":{"model_1":85.5,"model_2":82.3}}`), 0644)
	os.WriteFile(filepath.Join(dir, "features.pkl"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "ranked_0.pdb"), []byte("ATOM\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "msas"), 0755)

	d := &alphafoldDetector{}
	rs := d.Inspect(dir)

	data, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ResearchStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify required fields survive round-trip
	if decoded.Detector == "" {
		t.Error("detector field missing after JSON round-trip")
	}
	if decoded.State == "" {
		t.Error("state field missing after JSON round-trip")
	}
	if decoded.Confidence == "" {
		t.Error("confidence field missing after JSON round-trip")
	}
	if decoded.WorkDir == "" {
		t.Error("work_dir field missing after JSON round-trip")
	}
	if len(decoded.Evidence) == 0 {
		t.Error("evidence field missing/empty after JSON round-trip")
	}
	if decoded.Detector != "alphafold" {
		t.Errorf("detector=%q, want alphafold", decoded.Detector)
	}
}

func TestAlphaFoldStuckMinutes(t *testing.T) {
	d := &alphafoldDetector{}
	if d.StuckMinutes() != 45 {
		t.Errorf("StuckMinutes=%d, want 45", d.StuckMinutes())
	}
}

func TestAlphaFoldName(t *testing.T) {
	d := &alphafoldDetector{}
	if d.Name() != "alphafold" {
		t.Errorf("Name=%q, want alphafold", d.Name())
	}
}
