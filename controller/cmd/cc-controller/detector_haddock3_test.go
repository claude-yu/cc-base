package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHaddock3Match_ConfigOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.cfg"), []byte("[topoaa]\nmolecules = [...]\n"), 0644)

	d := &haddock3Detector{}
	matched, score := d.Match(dir)
	// config.cfg alone = 40 < threshold 50
	if matched {
		t.Errorf("config.cfg alone: matched=%v score=%d, want not matched (score<50)", matched, score)
	}
}

func TestHaddock3Match_ConfigPlusStages(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.cfg"), []byte("[topoaa]\nmolecules = [...]\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "0_topoaa"), 0755)

	d := &haddock3Detector{}
	matched, score := d.Match(dir)
	// 40 (config) + 30 + 1*5 = 75
	if !matched || score < 50 {
		t.Errorf("config + 1 stage: matched=%v score=%d, want matched+score>=50", matched, score)
	}
}

func TestHaddock3Match_NoHaddockConfig(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.cfg"), []byte("[general]\nmode = production\n"), 0644)

	d := &haddock3Detector{}
	matched, _ := d.Match(dir)
	if matched {
		t.Error("config.cfg without haddock keywords should not match")
	}
}

func TestHaddock3Match_StagesOnly(t *testing.T) {
	dir := t.TempDir()
	for _, stage := range []string{"0_topoaa", "1_rigidbody", "2_seletop"} {
		os.MkdirAll(filepath.Join(dir, stage), 0755)
	}

	d := &haddock3Detector{}
	matched, score := d.Match(dir)
	// 30 + 3*5 = 45, below 50
	if matched {
		t.Errorf("3 stages alone: matched=%v score=%d, want not matched (score<50)", matched, score)
	}
}

func TestHaddock3Match_StagesWithRunDir(t *testing.T) {
	dir := t.TempDir()
	for _, stage := range []string{"0_topoaa", "1_rigidbody", "2_seletop", "3_flexref"} {
		os.MkdirAll(filepath.Join(dir, stage), 0755)
	}
	os.MkdirAll(filepath.Join(dir, "run-1"), 0755)

	d := &haddock3Detector{}
	matched, score := d.Match(dir)
	// 30 + 4*5 + 10 = 60
	if !matched || score < 50 {
		t.Errorf("4 stages + run dir: matched=%v score=%d, want matched+score>=50", matched, score)
	}
}

func TestHaddock3Match_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	d := &haddock3Detector{}
	matched, _ := d.Match(dir)
	if matched {
		t.Error("empty dir should not match")
	}
}

func TestIsHaddock3Config(t *testing.T) {
	dir := t.TempDir()

	positive := filepath.Join(dir, "good.cfg")
	os.WriteFile(positive, []byte("# HADDOCK3 config\n[topoaa]\nmolecules = [...]\n"), 0644)
	if !isHaddock3Config(positive) {
		t.Error("should detect haddock3 config with topoaa keyword")
	}

	negative := filepath.Join(dir, "bad.cfg")
	os.WriteFile(negative, []byte("[general]\nmode = production\nworkers = 4\n"), 0644)
	if isHaddock3Config(negative) {
		t.Error("should not detect non-haddock config")
	}
}

func TestCountHaddock3Stages(t *testing.T) {
	dir := t.TempDir()
	for _, stage := range []string{"0_topoaa", "1_rigidbody", "5_rmsdmatrix"} {
		os.MkdirAll(filepath.Join(dir, stage), 0755)
	}
	os.WriteFile(filepath.Join(dir, "not_a_stage"), []byte{}, 0644)

	count := countHaddock3Stages(dir)
	if count != 3 {
		t.Errorf("countHaddock3Stages = %d, want 3", count)
	}
}

func TestClassifyHaddock3Stages(t *testing.T) {
	dir := t.TempDir()

	// 0_topoaa: completed (has >=2 files)
	s0 := filepath.Join(dir, "0_topoaa")
	os.MkdirAll(s0, 0755)
	os.WriteFile(filepath.Join(s0, "output1.pdb"), []byte{}, 0644)
	os.WriteFile(filepath.Join(s0, "output2.pdb"), []byte{}, 0644)

	// 1_rigidbody: completed
	s1 := filepath.Join(dir, "1_rigidbody")
	os.MkdirAll(s1, 0755)
	os.WriteFile(filepath.Join(s1, "result1.pdb"), []byte{}, 0644)
	os.WriteFile(filepath.Join(s1, "result2.pdb"), []byte{}, 0644)
	os.WriteFile(filepath.Join(s1, "result3.pdb"), []byte{}, 0644)

	// 2_seletop: current (only 1 file, last stage)
	s2 := filepath.Join(dir, "2_seletop")
	os.MkdirAll(s2, 0755)
	os.WriteFile(filepath.Join(s2, "partial.log"), []byte{}, 0644)

	completed, current := classifyHaddock3Stages(dir)
	if len(completed) != 2 {
		t.Errorf("completed = %v, want 2 stages", completed)
	}
	if current != "2_seletop" {
		t.Errorf("current = %q, want 2_seletop", current)
	}
}

func TestClassifyHaddock3Stages_AllCompleted(t *testing.T) {
	dir := t.TempDir()
	for _, stage := range []string{"0_topoaa", "1_rigidbody"} {
		sd := filepath.Join(dir, stage)
		os.MkdirAll(sd, 0755)
		os.WriteFile(filepath.Join(sd, "a.pdb"), []byte{}, 0644)
		os.WriteFile(filepath.Join(sd, "b.pdb"), []byte{}, 0644)
	}

	completed, current := classifyHaddock3Stages(dir)
	if len(completed) != 2 || current != "" {
		t.Errorf("completed=%v current=%q, want 2 completed + no current", completed, current)
	}
}

func TestClassifyHaddock3Stages_Empty(t *testing.T) {
	dir := t.TempDir()
	completed, current := classifyHaddock3Stages(dir)
	if len(completed) != 0 || current != "" {
		t.Error("empty dir should return no stages")
	}
}

func TestStageHasOutput(t *testing.T) {
	dir := t.TempDir()
	stage := "0_topoaa"
	sd := filepath.Join(dir, stage)
	os.MkdirAll(sd, 0755)

	if stageHasOutput(dir, stage) {
		t.Error("empty stage dir should not have output")
	}

	os.WriteFile(filepath.Join(sd, "a.pdb"), []byte{}, 0644)
	if stageHasOutput(dir, stage) {
		t.Error("1 file should not count as output (threshold=2)")
	}

	os.WriteFile(filepath.Join(sd, "b.pdb"), []byte{}, 0644)
	if !stageHasOutput(dir, stage) {
		t.Error("2 files should count as output")
	}
}

func TestFormatStageProgress(t *testing.T) {
	cases := []struct {
		completed []string
		current   string
		contains  []string
		empty     bool
	}{
		{nil, "", nil, true},
		{[]string{"0_topoaa"}, "", []string{"completed:", "0_topoaa"}, false},
		{nil, "1_rigidbody", []string{"current:", "1_rigidbody"}, false},
		{[]string{"0_topoaa", "1_rigidbody"}, "2_seletop", []string{"completed:", "current:", "2_seletop"}, false},
		{[]string{"0_topoaa", "1_rigidbody", "2_seletop", "3_flexref"}, "",
			[]string{"0_topoaa..3_flexref", "4 stages"}, false},
	}
	for _, tc := range cases {
		got := formatStageProgress(tc.completed, tc.current)
		if tc.empty && got != "" {
			t.Errorf("formatStageProgress(%v, %q) = %q, want empty", tc.completed, tc.current, got)
		}
		for _, s := range tc.contains {
			if !strings.Contains(got, s) {
				t.Errorf("formatStageProgress(%v, %q) = %q, missing %q", tc.completed, tc.current, got, s)
			}
		}
	}
}

func TestFindCaprievalResults(t *testing.T) {
	dir := t.TempDir()
	capri := filepath.Join(dir, "8_caprieval")
	os.MkdirAll(capri, 0755)
	os.WriteFile(filepath.Join(capri, "capri_ss.tsv"), []byte("header\nrow1\n"), 0644)
	os.WriteFile(filepath.Join(capri, "capri_clt.tsv"), []byte("header\nrow1\n"), 0644)

	clusterDir := filepath.Join(capri, "cluster")
	os.MkdirAll(clusterDir, 0755)
	os.WriteFile(filepath.Join(clusterDir, "cluster_1"), []byte{}, 0644)
	os.WriteFile(filepath.Join(clusterDir, "cluster_2"), []byte{}, 0644)

	evidence := findCaprievalResults(dir)
	if len(evidence) < 2 {
		t.Fatalf("findCaprievalResults = %v, want >=2 entries", evidence)
	}
	found := strings.Join(evidence, " ")
	if !strings.Contains(found, "capri_ss.tsv") {
		t.Error("missing capri_ss.tsv in evidence")
	}
	if !strings.Contains(found, "clusters") {
		t.Error("missing cluster count in evidence")
	}
}

func TestFindCaprievalResults_NoDir(t *testing.T) {
	dir := t.TempDir()
	evidence := findCaprievalResults(dir)
	if len(evidence) != 0 {
		t.Errorf("expected empty evidence for missing caprieval dir, got %v", evidence)
	}
}

// --- Inspect state tests ---

func TestHaddock3Inspect_Completed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.cfg"), []byte("[topoaa]\nmolecules = ...\n"), 0644)

	for _, stage := range haddock3Stages {
		sd := filepath.Join(dir, stage)
		os.MkdirAll(sd, 0755)
		os.WriteFile(filepath.Join(sd, "a.pdb"), []byte{}, 0644)
		os.WriteFile(filepath.Join(sd, "b.pdb"), []byte{}, 0644)
	}

	d := &haddock3Detector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed (all stages including 8_caprieval)", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
}

func TestHaddock3Inspect_Running(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.cfg"), []byte("[topoaa]\n"), 0644)

	s0 := filepath.Join(dir, "0_topoaa")
	os.MkdirAll(s0, 0755)
	os.WriteFile(filepath.Join(s0, "a.pdb"), []byte{}, 0644)
	os.WriteFile(filepath.Join(s0, "b.pdb"), []byte{}, 0644)

	// 1_rigidbody as current (1 file, recently touched)
	s1 := filepath.Join(dir, "1_rigidbody")
	os.MkdirAll(s1, 0755)
	recentFile := filepath.Join(s1, "partial.log")
	os.WriteFile(recentFile, []byte("running..."), 0644)
	os.Chtimes(recentFile, time.Now(), time.Now())

	d := &haddock3Detector{}
	rs := d.Inspect(dir)
	if rs.State != "running" {
		t.Errorf("state=%q, want running (current stage with recent activity)", rs.State)
	}
}

func TestHaddock3Inspect_Failed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.cfg"), []byte("[topoaa]\n"), 0644)

	s0 := filepath.Join(dir, "0_topoaa")
	os.MkdirAll(s0, 0755)
	os.WriteFile(filepath.Join(s0, "a.pdb"), []byte{}, 0644)
	os.WriteFile(filepath.Join(s0, "b.pdb"), []byte{}, 0644)

	os.WriteFile(filepath.Join(dir, "haddock3.log"),
		[]byte("Starting...\nTraceback (most recent call last):\n  File foo.py\nRuntimeError: bad input\n"), 0644)

	d := &haddock3Detector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed (log has Traceback)", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
}

func TestHaddock3Inspect_Stuck(t *testing.T) {
	dir := t.TempDir()
	staleTime := time.Now().Add(-3 * time.Hour)

	cfg := filepath.Join(dir, "config.cfg")
	os.WriteFile(cfg, []byte("[topoaa]\n"), 0644)
	os.Chtimes(cfg, staleTime, staleTime)

	s0 := filepath.Join(dir, "0_topoaa")
	os.MkdirAll(s0, 0755)
	for _, f := range []string{"a.pdb", "b.pdb"} {
		p := filepath.Join(s0, f)
		os.WriteFile(p, []byte{}, 0644)
		os.Chtimes(p, staleTime, staleTime)
	}

	s1 := filepath.Join(dir, "1_rigidbody")
	os.MkdirAll(s1, 0755)
	staleFile := filepath.Join(s1, "partial.log")
	os.WriteFile(staleFile, []byte("working..."), 0644)
	os.Chtimes(staleFile, staleTime, staleTime)

	d := &haddock3Detector{}
	rs := d.Inspect(dir)
	if rs.State != "stuck" {
		t.Errorf("state=%q, want stuck (current stage >120 min stale)", rs.State)
	}
}

func TestHaddock3Inspect_Idle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.cfg"), []byte("[topoaa]\n"), 0644)

	d := &haddock3Detector{}
	rs := d.Inspect(dir)
	if rs.State != "idle" {
		t.Errorf("state=%q, want idle (config exists but no stages)", rs.State)
	}
}

func TestHaddock3Inspect_IdleAfterPartialCompletion(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.cfg"), []byte("[topoaa]\n"), 0644)

	// Only 0_topoaa completed, no current stage
	s0 := filepath.Join(dir, "0_topoaa")
	os.MkdirAll(s0, 0755)
	os.WriteFile(filepath.Join(s0, "a.pdb"), []byte{}, 0644)
	os.WriteFile(filepath.Join(s0, "b.pdb"), []byte{}, 0644)

	d := &haddock3Detector{}
	rs := d.Inspect(dir)
	if rs.State != "idle" {
		t.Errorf("state=%q, want idle (partial completion, no current stage, not caprieval)", rs.State)
	}
}

func TestHaddock3Inspect_FailedOverridesOldCaprieval(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.cfg"), []byte("[topoaa]\n"), 0644)

	for _, stage := range haddock3Stages {
		sd := filepath.Join(dir, stage)
		os.MkdirAll(sd, 0755)
		os.WriteFile(filepath.Join(sd, "a.pdb"), []byte{}, 0644)
		os.WriteFile(filepath.Join(sd, "b.pdb"), []byte{}, 0644)
	}

	// Old caprieval exists, but log has recent error from a re-run attempt
	os.WriteFile(filepath.Join(dir, "haddock3.log"),
		[]byte("Restarting run...\nTraceback (most recent call last):\nFileNotFoundError: missing input\n"), 0644)

	d := &haddock3Detector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed (log error should override old caprieval completion)", rs.State)
	}
}

func TestHaddock3StuckMinutes(t *testing.T) {
	d := &haddock3Detector{}
	if d.StuckMinutes() != 120 {
		t.Errorf("StuckMinutes=%d, want 120", d.StuckMinutes())
	}
}
