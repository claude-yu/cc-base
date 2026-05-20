package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAutodockVinaMatch_VinaConfigPlusPDBQT(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.txt"),
		[]byte("receptor = receptor.pdbqt\nligand = ligand.pdbqt\nexhaustiveness = 8\ncenter_x = 10.0\nsize_x = 20.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte("ATOM\n"), 0644)

	d := &autodockVinaDetector{}
	matched, score := d.Match(dir)
	// 35 (config) + 15 (pdbqt) = 50
	if !matched || score < 40 {
		t.Errorf("vina config+pdbqt: matched=%v score=%d", matched, score)
	}
}

func TestAutodockVinaMatch_VinaLogPlusPDBQT(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "vina_run.log"),
		[]byte("Detecting 8 CPUs\nReading input ... done.\n"), 0644)
	os.WriteFile(filepath.Join(dir, "ligand.pdbqt"), []byte("ATOM\n"), 0644)

	d := &autodockVinaDetector{}
	matched, score := d.Match(dir)
	// 30 (vina log) + 15 (pdbqt) = 45
	if !matched || score < 40 {
		t.Errorf("vina log+pdbqt: matched=%v score=%d", matched, score)
	}
}

func TestAutodockVinaMatch_DLGPlusDPF(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "dock.dlg"), []byte("AutoDock results\n"), 0644)
	os.WriteFile(filepath.Join(dir, "dock.dpf"), []byte("autodock4 parameters\n"), 0644)

	d := &autodockVinaDetector{}
	matched, score := d.Match(dir)
	// 35 (dlg) + 20 (dpf) = 55
	if !matched || score < 40 {
		t.Errorf("dlg+dpf: matched=%v score=%d", matched, score)
	}
}

func TestAutodockVinaMatch_PDBQTOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte("ATOM\n"), 0644)
	os.WriteFile(filepath.Join(dir, "ligand.pdbqt"), []byte("ATOM\n"), 0644)

	d := &autodockVinaDetector{}
	matched, _ := d.Match(dir)
	// 15 < 40
	if matched {
		t.Error("pdbqt alone should not match")
	}
}

func TestAutodockVinaMatch_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	d := &autodockVinaDetector{}
	matched, _ := d.Match(dir)
	if matched {
		t.Error("empty dir should not match")
	}
}

func TestAutodockVinaMatch_ConfigNoKeywords(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.txt"), []byte("some_option = value\nother = 123\n"), 0644)
	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte("ATOM\n"), 0644)

	d := &autodockVinaDetector{}
	matched, _ := d.Match(dir)
	// 0 (config without keywords) + 15 (pdbqt) = 15 < 40
	if matched {
		t.Error("config.txt without vina keywords should not match")
	}
}

func TestAutodockVinaMatch_VinaOutput(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.txt"),
		[]byte("exhaustiveness = 8\ncenter_x = 10.0\nsize_x = 20.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "ligand.pdbqt"), []byte("ATOM\n"), 0644)
	os.WriteFile(filepath.Join(dir, "ligand_out.pdbqt"), []byte("MODEL 1\n"), 0644)

	d := &autodockVinaDetector{}
	matched, score := d.Match(dir)
	// 35 (config) + 15 (pdbqt) + 20 (output pdbqt) = 70
	if !matched || score < 40 {
		t.Errorf("config+pdbqt+output: matched=%v score=%d", matched, score)
	}
}

func TestAutodockVinaMatch_AutoDock4Full(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "dock.dlg"), []byte("AutoDock results\n"), 0644)
	os.WriteFile(filepath.Join(dir, "dock.dpf"), []byte("parameters\n"), 0644)
	os.WriteFile(filepath.Join(dir, "grid.gpf"), []byte("grid params\n"), 0644)
	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte("ATOM\n"), 0644)

	d := &autodockVinaDetector{}
	matched, score := d.Match(dir)
	// 35 + 20 + 15 + 15 = 85
	if !matched || score < 40 {
		t.Errorf("full autodock4: matched=%v score=%d", matched, score)
	}
}

// --- Helper tests ---

func TestHasVinaConfig(t *testing.T) {
	dir := t.TempDir()

	if hasVinaConfig(dir) {
		t.Error("empty dir should not have vina config")
	}

	os.WriteFile(filepath.Join(dir, "config.txt"), []byte("some_option = value\n"), 0644)
	if hasVinaConfig(dir) {
		t.Error("config without vina keywords should not match")
	}

	os.WriteFile(filepath.Join(dir, "config.txt"),
		[]byte("exhaustiveness = 8\ncenter_x = 10.0\n"), 0644)
	if !hasVinaConfig(dir) {
		t.Error("config with exhaustiveness + center_x should match")
	}
}

func TestHasVinaConfig_AlternateNames(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "vina_config.txt"),
		[]byte("size_x = 20\nsize_y = 20\nsize_z = 20\n"), 0644)
	if !hasVinaConfig(dir) {
		t.Error("vina_config.txt with size keywords should match")
	}
}

func TestIsVinaOutputPDBQT(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"out_ligand.pdbqt", true},
		{"out.pdbqt", true},
		{"ligand_out.pdbqt", true},
		{"output.pdbqt", true},
		{"receptor.pdbqt", false},
		{"ligand.pdbqt", false},
		{"some_output.pdbqt", false},
		{"results.txt", false},
	}
	for _, tc := range cases {
		got := isVinaOutputPDBQT(tc.name)
		if got != tc.want {
			t.Errorf("isVinaOutputPDBQT(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestParseVinaLogs(t *testing.T) {
	dir := t.TempDir()

	// Empty
	r := parseVinaLogs(dir)
	if r.modes != 0 || r.completed {
		t.Error("empty dir should have no results")
	}

	// Typical Vina output
	content := `Detecting 8 CPUs
Reading input ... done.
Setting up the scoring function ... done.
Analyzing the binding site ... done.
Performing search ... done.
Refine time: 0.123s
mode |   affinity | dist from best mode
                        rmsd l.b.| rmsd u.b.
-----+------------+----------+----------
   1         -8.7      0.000      0.000
   2         -8.3      1.982      3.271
   3         -7.9      1.847      2.987
Writing output ... done.
`
	os.WriteFile(filepath.Join(dir, "vina_run.log"), []byte(content), 0644)

	r = parseVinaLogs(dir)
	if r.modes != 3 {
		t.Errorf("modes=%d, want 3", r.modes)
	}
	if r.bestAffinity != "-8.7" {
		t.Errorf("bestAffinity=%q, want -8.7", r.bestAffinity)
	}
	if !r.completed {
		t.Error("should be completed (Writing output)")
	}
}

func TestParseDLG(t *testing.T) {
	dir := t.TempDir()

	// Empty
	r := parseDLG(dir)
	if r.runs != 0 || r.completed {
		t.Error("empty dir should have no results")
	}

	// Typical DLG
	content := `AutoDock 4.2.6
DOCKED: USER    Run = 1
DOCKED: USER    Estimated Free Energy of Binding    =  -7.56 kcal/mol
DOCKED: USER    Run = 2
DOCKED: USER    Estimated Free Energy of Binding    =  -6.89 kcal/mol
DOCKED: USER    Run = 3
DOCKED: USER    Estimated Free Energy of Binding    =  -8.12 kcal/mol
Total number of runs = 3
Final Docked State
`
	os.WriteFile(filepath.Join(dir, "dock.dlg"), []byte(content), 0644)

	r = parseDLG(dir)
	if r.runs != 3 {
		t.Errorf("runs=%d, want 3", r.runs)
	}
	if r.bestEnergy != "-8.12" {
		t.Errorf("bestEnergy=%q, want -8.12", r.bestEnergy)
	}
	if !r.completed {
		t.Error("should be completed (Final Docked State)")
	}
}

func TestCountVinaOutputPDBQT(t *testing.T) {
	dir := t.TempDir()

	if countVinaOutputPDBQT(dir) != 0 {
		t.Error("empty dir should have 0")
	}

	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "ligand.pdbqt"), []byte{}, 0644)
	if countVinaOutputPDBQT(dir) != 0 {
		t.Error("input pdbqt should not count")
	}

	os.WriteFile(filepath.Join(dir, "ligand_out.pdbqt"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "out_ligand2.pdbqt"), []byte{}, 0644)
	if got := countVinaOutputPDBQT(dir); got != 2 {
		t.Errorf("countVinaOutputPDBQT=%d, want 2", got)
	}
}

// --- Inspect state tests ---

func TestAutodockVinaInspect_VinaCompleted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.txt"),
		[]byte("exhaustiveness = 8\ncenter_x = 10\nsize_x = 20\n"), 0644)
	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte("ATOM\n"), 0644)
	vinaLog := `Detecting 8 CPUs
Reading input ... done.
Refine time: 0.5s
mode |   affinity | dist from best mode
                        rmsd l.b.| rmsd u.b.
-----+------------+----------+----------
   1         -9.2      0.000      0.000
   2         -8.8      2.100      3.500
Writing output ... done.
`
	os.WriteFile(filepath.Join(dir, "vina_run.log"), []byte(vinaLog), 0644)
	os.WriteFile(filepath.Join(dir, "ligand_out.pdbqt"), []byte("MODEL 1\n"), 0644)

	d := &autodockVinaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
	hasAffinity := false
	for _, e := range rs.Evidence {
		if strings.Contains(e, "-9.2") {
			hasAffinity = true
		}
	}
	if !hasAffinity {
		t.Error("should report best affinity -9.2")
	}
}

func TestAutodockVinaInspect_DLGCompleted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "dock.dpf"), []byte("autodock4 params\n"), 0644)
	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte("ATOM\n"), 0644)
	dlgContent := `AutoDock 4.2.6
DOCKED: USER    Estimated Free Energy of Binding    =  -7.56 kcal/mol
Total number of runs = 1
Final Docked State
`
	os.WriteFile(filepath.Join(dir, "dock.dlg"), []byte(dlgContent), 0644)

	d := &autodockVinaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
}

func TestAutodockVinaInspect_Failed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.txt"),
		[]byte("exhaustiveness = 8\ncenter_x = 10\n"), 0644)
	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte("ATOM\n"), 0644)
	os.WriteFile(filepath.Join(dir, "vina_run.log"),
		[]byte("Reading input ... done.\nERROR: could not open receptor.pdbqt\nParse error in receptor\n"), 0644)

	d := &autodockVinaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
}

func TestAutodockVinaInspect_Running(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.txt"),
		[]byte("exhaustiveness = 32\ncenter_x = 10\n"), 0644)
	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte("ATOM\n"), 0644)
	logPath := filepath.Join(dir, "vina_run.log")
	os.WriteFile(logPath, []byte("Detecting 8 CPUs\nPerforming search ...\n"), 0644)
	os.Chtimes(logPath, time.Now(), time.Now())

	d := &autodockVinaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "running" {
		t.Errorf("state=%q, want running", rs.State)
	}
}

func TestAutodockVinaInspect_Stuck(t *testing.T) {
	dir := t.TempDir()
	staleTime := time.Now().Add(-2 * time.Hour)
	cfg := filepath.Join(dir, "config.txt")
	os.WriteFile(cfg, []byte("exhaustiveness = 8\ncenter_x = 10\n"), 0644)
	os.Chtimes(cfg, staleTime, staleTime)
	pdbqt := filepath.Join(dir, "receptor.pdbqt")
	os.WriteFile(pdbqt, []byte("ATOM\n"), 0644)
	os.Chtimes(pdbqt, staleTime, staleTime)
	logPath := filepath.Join(dir, "vina_run.log")
	os.WriteFile(logPath, []byte("Performing search ...\n"), 0644)
	os.Chtimes(logPath, staleTime, staleTime)

	d := &autodockVinaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "stuck" {
		t.Errorf("state=%q, want stuck", rs.State)
	}
}

func TestAutodockVinaInspect_Idle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.txt"),
		[]byte("exhaustiveness = 8\ncenter_x = 10\n"), 0644)
	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte("ATOM\n"), 0644)
	os.WriteFile(filepath.Join(dir, "ligand.pdbqt"), []byte("ATOM\n"), 0644)

	d := &autodockVinaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "idle" {
		t.Errorf("state=%q, want idle (input files only)", rs.State)
	}
}

func TestAutodockVinaInspect_CompletedMediumFromOutput(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.txt"),
		[]byte("exhaustiveness = 8\ncenter_x = 10\n"), 0644)
	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte("ATOM\n"), 0644)
	os.WriteFile(filepath.Join(dir, "ligand_out.pdbqt"), []byte("MODEL 1\n"), 0644)

	d := &autodockVinaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed (output pdbqt exists)", rs.State)
	}
	if rs.Confidence != "medium" {
		t.Errorf("confidence=%q, want medium (no log confirmation)", rs.Confidence)
	}
}

func TestAutodockVinaInspect_FailedOverridesCompleted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.txt"),
		[]byte("exhaustiveness = 8\ncenter_x = 10\n"), 0644)
	os.WriteFile(filepath.Join(dir, "receptor.pdbqt"), []byte("ATOM\n"), 0644)
	os.WriteFile(filepath.Join(dir, "ligand_out.pdbqt"), []byte("MODEL 1\n"), 0644)
	os.WriteFile(filepath.Join(dir, "vina_run.log"),
		[]byte("Segmentation fault (core dumped)\n"), 0644)

	d := &autodockVinaDetector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed (error overrides output pdbqt)", rs.State)
	}
}

func TestAutodockVinaStuckMinutes(t *testing.T) {
	d := &autodockVinaDetector{}
	if d.StuckMinutes() != 45 {
		t.Errorf("StuckMinutes=%d, want 45", d.StuckMinutes())
	}
}
