package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRosettaMatch_ScoreSC(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "score.sc"), []byte("SCORE: total_score\nSCORE: -150.3 model_1\n"), 0644)

	d := &rosettaDetector{}
	matched, score := d.Match(dir)
	if !matched || score < 40 {
		t.Errorf("score.sc alone: matched=%v score=%d, want matched+score>=40", matched, score)
	}
}

func TestRosettaMatch_SilentFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "output.silent"), []byte("REMARK BINARY SILENTFILE\n"), 0644)
	os.WriteFile(filepath.Join(dir, "flags"), []byte("-in:file:s input.pdb\n"), 0644)

	d := &rosettaDetector{}
	matched, score := d.Match(dir)
	// 25 (silent) + 20 (flags) = 45
	if !matched || score < 40 {
		t.Errorf("silent+flags: matched=%v score=%d", matched, score)
	}
}

func TestRosettaMatch_PyRosetta(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "design.py"), []byte("import pyrosetta\npyrosetta.init()\n"), 0644)

	d := &rosettaDetector{}
	matched, score := d.Match(dir)
	// 30 (pyrosetta script)
	if matched {
		t.Errorf("pyrosetta alone: matched=%v score=%d, want not matched (score<40)", matched, score)
	}
}

func TestRosettaMatch_PyRosettaPlusScore(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "design.py"), []byte("import pyrosetta\npyrosetta.init()\n"), 0644)
	os.WriteFile(filepath.Join(dir, "score.sc"), []byte("SCORE: total_score\nSCORE: -100.0 model\n"), 0644)

	d := &rosettaDetector{}
	matched, score := d.Match(dir)
	// 30 + 35 = 65
	if !matched || score < 40 {
		t.Errorf("pyrosetta+score.sc: matched=%v score=%d", matched, score)
	}
}

func TestRosettaMatch_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	d := &rosettaDetector{}
	matched, _ := d.Match(dir)
	if matched {
		t.Error("empty dir should not match")
	}
}

func TestRosettaMatch_FlagsOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "flags"), []byte("-nstruct 100\n"), 0644)

	d := &rosettaDetector{}
	matched, _ := d.Match(dir)
	// 20 < 40
	if matched {
		t.Error("flags alone should not match (score=20 < 40)")
	}
}

func TestRosettaMatch_ScriptXML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "rosetta_scripts.xml"), []byte("<ROSETTASCRIPTS>\n</ROSETTASCRIPTS>\n"), 0644)
	os.WriteFile(filepath.Join(dir, "flags"), []byte("-parser:protocol rosetta_scripts.xml\n"), 0644)

	d := &rosettaDetector{}
	matched, score := d.Match(dir)
	// 20 (xml) + 20 (flags) = 40
	if !matched || score < 40 {
		t.Errorf("xml+flags: matched=%v score=%d", matched, score)
	}
}

// --- Helper tests ---

func TestHasRosettaLog(t *testing.T) {
	dir := t.TempDir()

	if hasRosettaLog(dir) {
		t.Error("empty dir should not have rosetta log")
	}

	os.WriteFile(filepath.Join(dir, "run.log"),
		[]byte("core.init: Rosetta version...\nprotocols.jd2.JobDistributor: Running\n"), 0644)

	if !hasRosettaLog(dir) {
		t.Error("log with core.init should be detected as rosetta log")
	}
}

func TestHasPyRosettaScript(t *testing.T) {
	dir := t.TempDir()

	if hasPyRosettaScript(dir) {
		t.Error("empty dir should not have pyrosetta script")
	}

	os.WriteFile(filepath.Join(dir, "run.py"), []byte("import numpy\nprint('hello')\n"), 0644)
	if hasPyRosettaScript(dir) {
		t.Error("regular python should not match")
	}

	os.WriteFile(filepath.Join(dir, "design.py"), []byte("import pyrosetta\npyrosetta.init()\n"), 0644)
	if !hasPyRosettaScript(dir) {
		t.Error("pyrosetta script should be detected")
	}
}

func TestParseScoreSC(t *testing.T) {
	dir := t.TempDir()

	// Empty
	info := parseScoreSC(dir)
	if info.rows != 0 {
		t.Errorf("empty dir: rows=%d", info.rows)
	}

	// Typical score.sc
	content := `SEQUENCE:
SCORE: total_score description
SCORE:    -150.300 model_0001
SCORE:    -145.200 model_0002
SCORE:    -160.500 model_0003
`
	os.WriteFile(filepath.Join(dir, "score.sc"), []byte(content), 0644)
	info = parseScoreSC(dir)
	if info.rows != 3 {
		t.Errorf("rows=%d, want 3", info.rows)
	}
	if info.bestScore != "-160.500" {
		t.Errorf("bestScore=%q, want -160.500", info.bestScore)
	}
}

func TestParseScoreSC_Empty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "score.sc"), []byte("SEQUENCE:\nSCORE: total_score description\n"), 0644)
	info := parseScoreSC(dir)
	if info.rows != 0 {
		t.Errorf("header-only: rows=%d, want 0", info.rows)
	}
}

func TestLooksNumeric(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"-150.3", true},
		{"42", true},
		{"+0.5", true},
		{"abc", false},
		{"", false},
		{"12.3.4", false},
		{"-", false},
	}
	for _, tc := range cases {
		got := looksNumeric(tc.s)
		if got != tc.want {
			t.Errorf("looksNumeric(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestParseFloat(t *testing.T) {
	cases := []struct {
		s    string
		want float64
	}{
		{"150.3", 150.3},
		{"-42.0", -42.0},
		{"0", 0},
		{"100", 100},
	}
	for _, tc := range cases {
		got := parseFloat(tc.s)
		diff := got - tc.want
		if diff < -0.01 || diff > 0.01 {
			t.Errorf("parseFloat(%q) = %f, want %f", tc.s, got, tc.want)
		}
	}
}

func TestCountOutputPDBs(t *testing.T) {
	dir := t.TempDir()

	// Input PDB should not count
	os.WriteFile(filepath.Join(dir, "input.pdb"), []byte{}, 0644)
	if countOutputPDBs(dir) != 0 {
		t.Error("input.pdb should not count")
	}

	os.WriteFile(filepath.Join(dir, "model_0001.pdb"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "model_0002.pdb"), []byte{}, 0644)
	if got := countOutputPDBs(dir); got != 2 {
		t.Errorf("countOutputPDBs=%d, want 2", got)
	}
}

func TestHasRosettaCompletion(t *testing.T) {
	dir := t.TempDir()

	if hasRosettaCompletion(dir) {
		t.Error("empty dir should not show completion")
	}

	os.WriteFile(filepath.Join(dir, "run.log"),
		[]byte("Running job...\nprotocols.jd2.JobDistributor: no more batches to process\nreported success in 45 seconds\n"), 0644)

	if !hasRosettaCompletion(dir) {
		t.Error("log with 'reported success' should show completion")
	}
}

// --- Inspect state tests ---

func TestRosettaInspect_Completed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "score.sc"),
		[]byte("SEQUENCE:\nSCORE: total_score description\nSCORE: -150.3 model_1\nSCORE: -145.2 model_2\n"), 0644)
	os.WriteFile(filepath.Join(dir, "run.log"),
		[]byte("core.init: Rosetta 2024\nreported success in 120 seconds\n"), 0644)

	d := &rosettaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed (score.sc + completion log)", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
}

func TestRosettaInspect_CompletedMedium(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "score.sc"),
		[]byte("SEQUENCE:\nSCORE: total_score description\nSCORE: -150.3 model_1\n"), 0644)

	d := &rosettaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed (score.sc exists)", rs.State)
	}
	if rs.Confidence != "medium" {
		t.Errorf("confidence=%q, want medium (no log confirmation)", rs.Confidence)
	}
}

func TestRosettaInspect_Failed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "flags"), []byte("-nstruct 10\n"), 0644)
	os.WriteFile(filepath.Join(dir, "run.log"),
		[]byte("core.init: starting...\nERROR: Cannot read input PDB\nFATAL: exit\n"), 0644)

	d := &rosettaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed (log errors)", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
}

func TestRosettaInspect_Running(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "flags"), []byte("-nstruct 10\n"), 0644)
	logPath := filepath.Join(dir, "run.log")
	os.WriteFile(logPath, []byte("core.init: Rosetta 2024\nRunning job 5/10\n"), 0644)
	os.Chtimes(logPath, time.Now(), time.Now())

	d := &rosettaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "running" {
		t.Errorf("state=%q, want running (recent log activity)", rs.State)
	}
}

func TestRosettaInspect_Stuck(t *testing.T) {
	dir := t.TempDir()
	staleTime := time.Now().Add(-2 * time.Hour)
	flags := filepath.Join(dir, "flags")
	os.WriteFile(flags, []byte("-nstruct 10\n"), 0644)
	os.Chtimes(flags, staleTime, staleTime)

	logPath := filepath.Join(dir, "run.log")
	os.WriteFile(logPath, []byte("core.init: starting\nRunning job 3/10\n"), 0644)
	os.Chtimes(logPath, staleTime, staleTime)

	d := &rosettaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "stuck" {
		t.Errorf("state=%q, want stuck (log >90 min stale)", rs.State)
	}
}

func TestRosettaInspect_Idle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "flags"), []byte("-nstruct 10\n"), 0644)
	os.WriteFile(filepath.Join(dir, "rosetta_scripts.xml"), []byte("<ROSETTASCRIPTS/>\n"), 0644)

	d := &rosettaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "idle" {
		t.Errorf("state=%q, want idle (input files but no output)", rs.State)
	}
}

func TestRosettaInspect_PyRosettaEvidence(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "design.py"),
		[]byte("import pyrosetta\npyrosetta.init()\npose = pyrosetta.pose_from_pdb('input.pdb')\n"), 0644)
	os.WriteFile(filepath.Join(dir, "score.sc"),
		[]byte("SEQUENCE:\nSCORE: total_score description\nSCORE: -200.5 design_1\n"), 0644)

	d := &rosettaDetector{}
	rs := d.Inspect(dir)
	found := false
	for _, e := range rs.Evidence {
		if strings.Contains(e, "PyRosetta") {
			found = true
			break
		}
	}
	if !found {
		t.Error("should have PyRosetta evidence")
	}
}

func TestRosettaInspect_FailedOverridesScoreSC(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "score.sc"),
		[]byte("SEQUENCE:\nSCORE: total_score description\nSCORE: -100.0 model_1\n"), 0644)
	os.WriteFile(filepath.Join(dir, "run.log"),
		[]byte("core.init: starting\nTraceback (most recent call last):\nRuntimeError: bad input\n"), 0644)

	d := &rosettaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed (log error overrides score.sc)", rs.State)
	}
}

func TestRosettaStuckMinutes(t *testing.T) {
	d := &rosettaDetector{}
	if d.StuckMinutes() != 90 {
		t.Errorf("StuckMinutes=%d, want 90", d.StuckMinutes())
	}
}
