package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- isNegatedFailure / filterFalseErrors ---

func TestIsNegatedFailure(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"6 of 6 job(s) succeeded; 0 job(s) failed.", true},
		{"0 job(s) failed", true},
		{"10 of 10 task(s) completed; 0 task(s) failed.", true},
		{"3 job(s) succeeded; 0 job(s) failed", true},
		{"ERROR: Failed to generate ring conformations.", false},
		{"FATAL: segmentation fault", false},
		{"2 job(s) succeeded; 1 job(s) failed.", false},
		{"5 task(s) failed", false},
		{"", false},
	}
	for _, tc := range cases {
		got := isNegatedFailure(tc.line)
		if got != tc.want {
			t.Errorf("isNegatedFailure(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestFilterFalseErrors(t *testing.T) {
	input := []string{
		"ERROR: 6 of 6 job(s) succeeded; 0 job(s) failed.",
		"ERROR: Failed to generate ring conformations.",
		"0 task(s) failed",
	}
	got := filterFalseErrors(input)
	if len(got) != 1 || got[0] != "ERROR: Failed to generate ring conformations." {
		t.Fatalf("filterFalseErrors: want 1 real error, got %v", got)
	}
}

// --- grepLines ---

func TestGrepLinesTruncation(t *testing.T) {
	long := make([]byte, 200)
	for i := range long {
		long[i] = 'x'
	}
	lines := []string{string(long)}
	hits := grepLines(lines, []string{"xxxx"})
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if len(hits[0]) > 154 { // 150 + "..."
		t.Errorf("hit not truncated: len=%d", len(hits[0]))
	}
}

// --- GROMACS Detector ---

func TestGromacsMatch_MinScore(t *testing.T) {
	dir := t.TempDir()
	d := &gromacsDetector{}

	// Only a .tpr — score=30, below threshold 40
	os.WriteFile(filepath.Join(dir, "test.tpr"), []byte{}, 0644)
	matched, _ := d.Match(dir)
	if matched {
		t.Error("should not match with only .tpr (score=30 < 40)")
	}

	// Add .cpt → score=50 ≥ 40
	os.WriteFile(filepath.Join(dir, "test.cpt"), []byte{}, 0644)
	matched, score := d.Match(dir)
	if !matched || score < 40 {
		t.Errorf("should match with .tpr+.cpt: matched=%v score=%d", matched, score)
	}
}

func TestGromacsInspect_CompletedByPostProcessing(t *testing.T) {
	dir := t.TempDir()
	// Minimal GROMACS dir with post-processing outputs but no md.log
	os.WriteFile(filepath.Join(dir, "md_0_1.tpr"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "md_0_1.cpt"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "md_fit.xtc"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "md_nojump.xtc"), []byte{}, 0644)

	d := &gromacsDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed (post-processing files present)", rs.State)
	}
	if rs.Confidence != "medium" {
		t.Errorf("confidence=%q, want medium (no md.log confirmation)", rs.Confidence)
	}
}

func TestGromacsInspect_CompletedByMdLog(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "md_0_1.tpr"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "md.log"), []byte("step 1000\nFinished mdrun on rank 0\n"), 0644)

	d := &gromacsDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high (md.log has Finished mdrun)", rs.Confidence)
	}
}

func TestGromacsInspect_FailedByLogError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "md_0_1.tpr"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "md.log"), []byte("step 500\nFatal error: nan in force\n"), 0644)

	d := &gromacsDetector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed", rs.State)
	}
}

func TestGromacsInspect_Running(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "md_0_1.tpr"), []byte{}, 0644)
	cptPath := filepath.Join(dir, "md_0_1.cpt")
	os.WriteFile(cptPath, []byte{}, 0644)
	// Touch to now
	os.Chtimes(cptPath, time.Now(), time.Now())

	d := &gromacsDetector{}
	rs := d.Inspect(dir)
	if rs.State != "running" {
		t.Errorf("state=%q, want running (fresh .cpt)", rs.State)
	}
}

func TestGromacsInspect_IdleNoOutput(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "topol.tpr"), []byte{}, 0644)
	// tpr exists but set mod time to ancient
	ancient := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	os.Chtimes(filepath.Join(dir, "topol.tpr"), ancient, ancient)

	d := &gromacsDetector{}
	rs := d.Inspect(dir)
	// tpr only, no output patterns matched → idle
	if rs.State != "idle" {
		t.Errorf("state=%q, want idle (tpr only, no output)", rs.State)
	}
}

// --- Generic CLI Detector ---

func TestGenericCLI_NoFalsePositiveOnSuccessLog(t *testing.T) {
	dir := t.TempDir()
	content := "Job started\n6 of 6 job(s) succeeded; 0 job(s) failed.\nDone.\n"
	os.WriteFile(filepath.Join(dir, "run.log"), []byte(content), 0644)

	d := &genericCLIDetector{}
	matched, _ := d.Match(dir)
	if !matched {
		t.Fatal("should match (has .log)")
	}
	rs := d.Inspect(dir)
	if rs.State == "failed" {
		t.Errorf("state=failed for log with '0 job(s) failed' — false positive not filtered")
	}
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed (log says 'Done')", rs.State)
	}
}

func TestGenericCLI_RealError(t *testing.T) {
	dir := t.TempDir()
	content := "Processing...\nERROR: segmentation fault at 0x0\n"
	os.WriteFile(filepath.Join(dir, "run.log"), []byte(content), 0644)

	d := &genericCLIDetector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed (real error in log)", rs.State)
	}
}

func TestGenericCLI_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	d := &genericCLIDetector{}
	matched, _ := d.Match(dir)
	if matched {
		t.Error("should not match empty directory")
	}
}

// --- Python Detector ---

func TestPythonMatch_ContextJsonBoost(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.py"), []byte("pass\n"), 0644)
	os.WriteFile(filepath.Join(dir, "context.json"), []byte(`{"phase":"running"}`), 0644)

	d := &pythonDetector{}
	matched, score := d.Match(dir)
	if !matched {
		t.Fatal("should match main.py + context.json")
	}
	if score < 40 {
		t.Errorf("score=%d, want ≥40 (main.py=20 + context.json=20)", score)
	}
}

func TestPythonMatch_BelowThreshold(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.py"), []byte("x=1\n"), 0644)

	d := &pythonDetector{}
	matched, _ := d.Match(dir)
	if matched {
		t.Error("config.py alone (score=15) should NOT match (threshold=20)")
	}
}

func TestPythonInspect_ContextPhase(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "run_pipeline.py"), []byte("pass\n"), 0644)
	ctx := `{"scripts":{"step1":{"status":"success"},"step2":{"status":"success"},"step3":{"status":"running"}}}`
	os.WriteFile(filepath.Join(dir, "pharmcell_context.json"), []byte(ctx), 0644)

	d := &pythonDetector{}
	rs := d.Inspect(dir)
	if rs.ContextPhase == "" {
		t.Error("expected non-empty ContextPhase from pharmcell context")
	}
	if rs.ContextPhase != "2/3 scripts done" {
		t.Errorf("ContextPhase=%q, want '2/3 scripts done'", rs.ContextPhase)
	}
}

// --- R Detector ---

func TestRMatch_MinScore(t *testing.T) {
	dir := t.TempDir()
	d := &rDetector{}

	// Single .R file with no pipeline markers
	os.WriteFile(filepath.Join(dir, "analysis.R"), []byte("1+1\n"), 0644)
	matched, _ := d.Match(dir)
	if !matched {
		// score = 10 for *.R < 20 threshold
		// should NOT match
	}
}

// --- extractContextPhase ---

func TestExtractContextPhase_Pharmcell(t *testing.T) {
	ctx := map[string]interface{}{
		"scripts": map[string]interface{}{
			"step1": map[string]interface{}{"status": "success"},
			"step2": map[string]interface{}{"status": "failed"},
			"step3": map[string]interface{}{"status": "success"},
		},
	}
	phase := extractContextPhase(ctx, "pharmcell_context.json")
	if phase != "2/3 scripts done, 1 failed" {
		t.Errorf("phase=%q", phase)
	}
}

func TestExtractContextPhase_TCGA(t *testing.T) {
	ctx := map[string]interface{}{
		"checkpoints": map[string]interface{}{
			"data_loaded":    true,
			"genes_filtered": true,
			"model_trained":  false,
			"report_done":    false,
		},
	}
	phase := extractContextPhase(ctx, "context.json")
	if phase != "2/4 checkpoints filled" {
		t.Errorf("phase=%q", phase)
	}
}

func TestExtractContextPhase_Generic(t *testing.T) {
	ctx := map[string]interface{}{
		"current_phase": "feature_selection",
	}
	phase := extractContextPhase(ctx, "context.json")
	if phase != "current_phase = feature_selection" {
		t.Errorf("phase=%q", phase)
	}
}

// --- scanProject ---

func TestScanProject_ExcludesDirs(t *testing.T) {
	dir := t.TempDir()
	// Create an excluded dir (.git) with a fake .log
	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "test.log"), []byte("error\n"), 0644)

	// Create a valid dir with a .log
	subDir := filepath.Join(dir, "experiment")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "run.log"), []byte("done\n"), 0644)

	results := scanProject(dir, 2, "")
	for _, rs := range results {
		if rs.WorkDir == gitDir {
			t.Error("scanProject should exclude .git directory")
		}
	}
}

func TestScanProject_DetectorFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "md_0_1.tpr"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "md_0_1.cpt"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "run.log"), []byte("step 1\n"), 0644)

	// With filter=gromacs, should only get gromacs results
	results := scanProject(dir, 1, "gromacs")
	for _, rs := range results {
		if rs.Detector != "gromacs" {
			t.Errorf("got detector=%q with filter=gromacs", rs.Detector)
		}
	}
}

// --- readTail ---

func TestReadTail_Cap(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.log")
	lines := make([]byte, 0, 500)
	for i := 0; i < 100; i++ {
		lines = append(lines, []byte("line "+itoa(i)+"\n")...)
	}
	os.WriteFile(p, lines, 0644)

	tail := readTail(p, 5)
	// readTail strips trailing empty lines, so last \n produces one empty
	// line that gets trimmed. Result: last 5 non-empty lines from 100.
	if len(tail) < 4 || len(tail) > 5 {
		t.Errorf("readTail(5) got %d lines, want 4-5", len(tail))
	}
	// Verify it contains the last real content lines
	last := tail[len(tail)-1]
	if last != "line 99" {
		t.Errorf("last line=%q, want 'line 99'", last)
	}
}

func TestReadTail_NonExistent(t *testing.T) {
	tail := readTail("/nonexistent/path.log", 10)
	if tail != nil {
		t.Error("readTail should return nil for nonexistent file")
	}
}

// --- Docker Detector ---

func TestIsScienceImage(t *testing.T) {
	cases := []struct {
		image string
		want  bool
	}{
		{"ghcr.io/haddocking/haddock3:latest", true},
		{"prosettac-local", true},
		{"colabfold:blackwell", true},
		{"rfdiffusion:cu124", true},
		{"rosettacommons/rosetta:latest", true},
		{"mlikiowa/napcat-docker:latest", false},
		{"ubuntu:22.04", false},
		{"nginx:latest", false},
		{"gromacs/gromacs:2024.1", true},
	}
	for _, tc := range cases {
		got := isScienceImage(tc.image)
		if got != tc.want {
			t.Errorf("isScienceImage(%q) = %v, want %v", tc.image, got, tc.want)
		}
	}
}

func TestExtractExitCode(t *testing.T) {
	cases := []struct {
		status string
		want   int
	}{
		{"Exited (0) 2 days ago", 0},
		{"Exited (137) 4 days ago", 137},
		{"Exited (1) 13 days ago", 1},
		{"Exited (255) 4 days ago", 255},
		{"Up 5 hours", -1},
	}
	for _, tc := range cases {
		got := extractExitCode(tc.status)
		if got != tc.want {
			t.Errorf("extractExitCode(%q) = %d, want %d", tc.status, got, tc.want)
		}
	}
}

func TestExtractImageShort(t *testing.T) {
	cases := []struct {
		image string
		want  string
	}{
		{"ghcr.io/haddocking/haddock3:latest", "haddock3"},
		{"prosettac-local", "prosettac-local"},
		{"colabfold:blackwell", "colabfold"},
		{"rosettacommons/rosetta:latest", "rosetta"},
	}
	for _, tc := range cases {
		got := extractImageShort(tc.image)
		if got != tc.want {
			t.Errorf("extractImageShort(%q) = %q, want %q", tc.image, got, tc.want)
		}
	}
}

func TestExtractBindMount(t *testing.T) {
	labels := "desktop.docker.io/binds/0/Source=G:\\proteinwork\\work-11\\prosettac\\test,desktop.docker.io/binds/0/SourceKind=hostFile,desktop.docker.io/binds/0/Target=/work"
	got := extractBindMount(labels)
	if got != "G:\\proteinwork\\work-11\\prosettac\\test" {
		t.Errorf("extractBindMount = %q", got)
	}

	// No bind mount
	got2 := extractBindMount("org.opencontainers.image.version=base")
	if got2 != "" {
		t.Errorf("expected empty for no bind mount, got %q", got2)
	}
}

func TestInspectDockerContainer_ExitZero(t *testing.T) {
	c := dockerContainer{
		ID:    "3e7577a5a892abcdef",
		Image: "prosettac-local",
		Names: "prosettac-8011",
		State: "exited",
		Status: "Exited (0) 2 days ago",
		Command: "python -u /work/run_pipeline.py",
	}
	rs := inspectDockerContainer(c)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed (exit 0)", rs.State)
	}
	if rs.Detector != "docker:prosettac-local" {
		t.Errorf("detector=%q", rs.Detector)
	}
}

func TestInspectDockerContainer_ExitNonZero(t *testing.T) {
	c := dockerContainer{
		ID:    "abc123def456abcdef",
		Image: "ghcr.io/haddocking/haddock3:latest",
		Names: "haddock3-worker1",
		State: "exited",
		Status: "Exited (137) 4 days ago",
		Command: "bash -c 'sleep 172800'",
	}
	rs := inspectDockerContainer(c)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed (exit 137)", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
}

func TestInspectDockerContainer_Running(t *testing.T) {
	c := dockerContainer{
		ID:    "ed9bec108d9babcdef",
		Image: "gromacs/gromacs:2024.1",
		Names: "gromacs-md",
		State: "running",
		Status: "Up 2 hours",
		Command: "gmx mdrun -deffnm md",
	}
	rs := inspectDockerContainer(c)
	if rs.State != "running" {
		t.Errorf("state=%q, want running", rs.State)
	}
}

func TestParseDockerAgeMins(t *testing.T) {
	cases := []struct {
		status string
		want   int
	}{
		{"Up 5 hours", 300},
		{"Up 30 minutes", 30},
		{"Exited (0) 2 days ago", 2880},
		{"Exited (255) 4 days ago", 5760},
		{"Exited (137) 1 hours ago", 60},
		{"Exited (1) 2 weeks ago", 20160},
		{"Up 3 days", 4320},
		{"Created", -1},
	}
	for _, tc := range cases {
		got := parseDockerAgeMins(tc.status)
		if got != tc.want {
			t.Errorf("parseDockerAgeMins(%q) = %d, want %d", tc.status, got, tc.want)
		}
	}
}

func TestClassifyBucket(t *testing.T) {
	cases := []struct {
		name   string
		state  string
		mins   int
		want   string
	}{
		{"running never archived", "running", 999999, ""},
		{"active stuck", "stuck", 5000, ""},
		{"archived stuck >7d", "stuck", 11520, "archived_stuck"},
		{"completed not bucketed", "completed", 100, ""},
		{"recent failed", "failed", 60, "active_failed"},
		{"unknown time failed", "failed", -1, "active_failed"},
		{"1-day failed", "failed", 2000, "historical_failed"},
		{"6-day failed", "failed", 8640, "historical_failed"},
		{"8-day failed", "failed", 11520, "archived_failed"},
		{"30-day failed", "failed", 43200, "archived_failed"},
	}
	for _, tc := range cases {
		rs := ResearchStatus{State: tc.state, LastUpdateMins: tc.mins}
		got := classifyBucket(rs)
		if got != tc.want {
			t.Errorf("%s: classifyBucket(state=%q, mins=%d) = %q, want %q",
				tc.name, tc.state, tc.mins, got, tc.want)
		}
	}
}

// --- Schrodinger parseSchrodingerLog ---

func TestParseSchrodingerLog_CleanCompletion(t *testing.T) {
	tail := []string{
		"Starting docking...",
		"Best docking score: -8.5",
		"Writing 10 poses to glide_sp_pv.maegz",
		"Exiting Glide.",
	}
	state, _ := parseSchrodingerLog(tail, sjGlideDock)
	if state != "completed" {
		t.Errorf("state=%q, want completed (clean exit)", state)
	}
}

func TestParseSchrodingerLog_ErrorBeforeExit(t *testing.T) {
	tail := []string{
		"Starting docking...",
		"FATAL: cannot read input file",
		"Exiting Glide.",
	}
	state, _ := parseSchrodingerLog(tail, sjGlideDock)
	if state != "failed" {
		t.Errorf("state=%q, want failed (error before Exiting Glide)", state)
	}
}

func TestParseSchrodingerLog_ErrorAfterCompletionMarker(t *testing.T) {
	tail := []string{
		"All jobs have completed.",
		"ERROR: post-processing failed",
	}
	state, _ := parseSchrodingerLog(tail, sjGlideDock)
	if state != "failed" {
		t.Errorf("state=%q, want failed (error after completion marker)", state)
	}
}

func TestParseSchrodingerLog_DefinitiveSuccessSummary(t *testing.T) {
	tail := []string{
		"Some processing...",
		"ERROR: non-fatal warning in error_handler",
		"6 of 6 job(s) succeeded; 0 job(s) failed.",
		"Total elapsed time: 12:34:56",
	}
	state, _ := parseSchrodingerLog(tail, sjGlideDock)
	if state != "completed" {
		t.Errorf("state=%q, want completed (definitive success summary)", state)
	}
}

func TestParseSchrodingerLog_PartialFailureSummary(t *testing.T) {
	tail := []string{
		"Processing...",
		"4 of 6 job(s) succeeded; 2 job(s) failed.",
	}
	state, _ := parseSchrodingerLog(tail, sjGlideDock)
	if state != "failed" {
		t.Errorf("state=%q, want failed (partial failure summary)", state)
	}
}

func TestParseSchrodingerLog_NoMarkers(t *testing.T) {
	tail := []string{
		"Processing ligand 42...",
		"Docking pose generated",
	}
	state, _ := parseSchrodingerLog(tail, sjGlideDock)
	if state != "running" {
		t.Errorf("state=%q, want running (no markers)", state)
	}
}

func TestParseSchrodingerLog_ErrorsOnly(t *testing.T) {
	tail := []string{
		"Starting...",
		"Fatal error: license checkout failed",
	}
	state, _ := parseSchrodingerLog(tail, sjLigPrep)
	if state != "failed" {
		t.Errorf("state=%q, want failed (error without completion)", state)
	}
}

// --- Schrodinger classifySchrodingerDir ---

func TestClassifySchrodingerDir(t *testing.T) {
	cases := []struct {
		dirName string
		want    schrodingerJobType
	}{
		{"glide-dock_SP", sjGlideDock},
		{"Glide_SP_1", sjGlideDock},
		{"glide_xp_binding", sjGlideDock},
		{"glide_htvs_screen", sjGlideDock},
		{"glide-grid_bigbox", sjGlideGrid},
		{"Glide_Grid_1", sjGlideGrid},
		{"ligprep_library", sjLigPrep},
		{"proteinprep_1abc", sjProteinPrep},
		{"prot-prot-docking_complex", sjProtProtDock},
		{"prot-prot_1", sjProtProtDock},
		{"some_random_dir", sjUnknown},
	}
	for _, tc := range cases {
		got := classifySchrodingerDir(tc.dirName)
		if got != tc.want {
			t.Errorf("classifySchrodingerDir(%q) = %d, want %d", tc.dirName, got, tc.want)
		}
	}
}

// --- Schrodinger Inspect: failed log + output files = still failed ---

func TestSchrodingerInspect_FailedLogWithOutputFiles(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "glide-dock_test")
	os.MkdirAll(sub, 0755)

	// Write a log with errors + completion marker
	logContent := "Starting Glide...\nFATAL: cannot allocate memory\nExiting Glide.\n"
	os.WriteFile(filepath.Join(sub, "glide-dock_test.log"), []byte(logContent), 0644)

	// Write output files that would normally suggest completion
	os.WriteFile(filepath.Join(sub, "glide-dock_test_pv.maegz"), []byte{}, 0644)
	os.WriteFile(filepath.Join(sub, "results.csv"), []byte("score\n-8.5\n"), 0644)

	d := &schrodingerDetector{}
	rs := d.Inspect(sub)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed (log has errors, output files should NOT override)", rs.State)
	}
}

// --- statePriority sorting ---

func TestStatePrioritySorting(t *testing.T) {
	if statePriority["failed"] >= statePriority["stuck"] {
		t.Error("failed should sort before stuck")
	}
	if statePriority["stuck"] >= statePriority["running"] {
		t.Error("stuck should sort before running")
	}
	if statePriority["running"] >= statePriority["completed"] {
		t.Error("running should sort before completed")
	}
	if statePriority["completed"] >= statePriority["unknown"] {
		t.Error("completed should sort before unknown")
	}
}
