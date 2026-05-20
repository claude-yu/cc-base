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

func TestAmberMatch_FullAmberProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "min.mdin"), []byte("minimization\n"), 0644)
	os.WriteFile(filepath.Join(dir, "min.mdout"), []byte("NSTEP\n"), 0644)
	os.WriteFile(filepath.Join(dir, "min.rst7"), []byte{}, 0644)

	d := &amberOpenMMDetector{}
	matched, score := d.Match(dir)
	// prmtop(30) + mdin(20) + mdout(15) + rst7(10) = 75
	if !matched || score < 60 {
		t.Errorf("full amber: matched=%v score=%d, want matched+score>=60", matched, score)
	}
}

func TestAmberMatch_LeapOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tleap.in"), []byte("source leaprc\n"), 0644)
	os.WriteFile(filepath.Join(dir, "leap.log"), []byte("Welcome to LEaP\n"), 0644)
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "system.inpcrd"), []byte{}, 0644)

	d := &amberOpenMMDetector{}
	matched, score := d.Match(dir)
	// prmtop(30) + leap.log(15) + tleap.in(10) + inpcrd(10) = 65
	if !matched || score < 50 {
		t.Errorf("leap prep: matched=%v score=%d, want matched+score>=50", matched, score)
	}
}

func TestAmberMatch_OpenMMOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "run_sim.py"),
		[]byte("from openmm import app\nsimulation = app.Simulation()\n"), 0644)

	d := &amberOpenMMDetector{}
	matched, _ := d.Match(dir)
	// openmm script(35) < 40 threshold
	if matched {
		t.Error("OpenMM script alone should not match (score 35 < 40)")
	}
}

func TestAmberMatch_OpenMMWithTopology(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "run_sim.py"),
		[]byte("from openmm import app\n"), 0644)
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)

	d := &amberOpenMMDetector{}
	matched, score := d.Match(dir)
	// prmtop(30) + openmm(35) = 65
	if !matched || score < 60 {
		t.Errorf("openmm+topology: matched=%v score=%d, want matched+score>=60", matched, score)
	}
}

func TestAmberMatch_EmptyDir(t *testing.T) {
	d := &amberOpenMMDetector{}
	matched, _ := d.Match(t.TempDir())
	if matched {
		t.Error("empty dir should not match")
	}
}

func TestAmberMatch_PrmtopAlone(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)

	d := &amberOpenMMDetector{}
	matched, _ := d.Match(dir)
	// prmtop(30) < 40
	if matched {
		t.Error("prmtop alone should not match (score 30 < 40)")
	}
}

func TestAmberMatch_MdcrdAndTopology(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "system.parm7"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "prod.mdcrd"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "prod.mdin"), []byte("production\n"), 0644)

	d := &amberOpenMMDetector{}
	matched, score := d.Match(dir)
	// parm7(30) + mdcrd(10) + mdin(20) = 60
	if !matched || score < 50 {
		t.Errorf("parm7+mdcrd+mdin: matched=%v score=%d, want matched+score>=50", matched, score)
	}
}

func TestAmberMatch_ScoreCap100(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "min.mdin"), []byte("min\n"), 0644)
	os.WriteFile(filepath.Join(dir, "min.mdout"), []byte("NSTEP\n"), 0644)
	os.WriteFile(filepath.Join(dir, "min.rst7"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "system.inpcrd"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "leap.log"), []byte("LEaP\n"), 0644)
	os.WriteFile(filepath.Join(dir, "tleap.in"), []byte("source\n"), 0644)
	os.WriteFile(filepath.Join(dir, "prod.mdcrd"), []byte{}, 0644)

	d := &amberOpenMMDetector{}
	_, score := d.Match(dir)
	if score > 100 {
		t.Errorf("score=%d, should be capped at 100", score)
	}
}

// --- Inspect tests ---

func TestAmberInspect_Completed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "prod.mdin"), []byte("production\n"), 0644)
	mdout := filepath.Join(dir, "prod.mdout")
	os.WriteFile(mdout, []byte(strings.Join([]string{
		"   NSTEP       ENERGY",
		"   50000     -12345.678",
		"",
		"|  Master Total wall time:    123456 seconds",
		"| Final Performance Info:",
		"|     ns/day =    45.67   seconds/ns =  1891.23",
	}, "\n")), 0644)

	d := &amberOpenMMDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
	hasPerf := false
	for _, e := range rs.Evidence {
		if strings.Contains(e, "Final Performance") || strings.Contains(e, "wallclock") {
			hasPerf = true
			break
		}
	}
	if !hasPerf {
		t.Error("should report completion marker in evidence")
	}
}

func TestAmberInspect_Running(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "heat.mdin"), []byte("heating\n"), 0644)
	mdout := filepath.Join(dir, "heat.mdout")
	os.WriteFile(mdout, []byte("   NSTEP       ENERGY\n   5000     -9876.543\n"), 0644)
	os.Chtimes(mdout, time.Now(), time.Now())

	d := &amberOpenMMDetector{}
	rs := d.Inspect(dir)
	if rs.State != "running" {
		t.Errorf("state=%q, want running", rs.State)
	}
}

func TestAmberInspect_Stuck(t *testing.T) {
	dir := t.TempDir()
	stale := time.Now().Add(-120 * time.Minute)
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	mdout := filepath.Join(dir, "equil.mdout")
	os.WriteFile(mdout, []byte("NSTEP\n 1000\n"), 0644)
	os.Chtimes(mdout, stale, stale)
	os.WriteFile(filepath.Join(dir, "equil.mdin"), []byte("equil\n"), 0644)
	os.Chtimes(filepath.Join(dir, "equil.mdin"), stale, stale)
	prmtop := filepath.Join(dir, "system.prmtop")
	os.Chtimes(prmtop, stale, stale)

	d := &amberOpenMMDetector{}
	rs := d.Inspect(dir)
	if rs.State != "stuck" {
		t.Errorf("state=%q, want stuck (>60min stale)", rs.State)
	}
}

func TestAmberInspect_Failed(t *testing.T) {
	dir := t.TempDir()
	stale := time.Now().Add(-2 * time.Hour)
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	mdout := filepath.Join(dir, "prod.mdout")
	os.WriteFile(mdout, []byte("FATAL: Could not open topology file\n"), 0644)
	os.Chtimes(mdout, stale, stale)

	d := &amberOpenMMDetector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
}

func TestAmberInspect_Idle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "system.inpcrd"), []byte{}, 0644)

	d := &amberOpenMMDetector{}
	rs := d.Inspect(dir)
	if rs.State != "idle" {
		t.Errorf("state=%q, want idle (topology+inpcrd but no output)", rs.State)
	}
}

func TestAmberInspect_OpenMMCompleted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "run_sim.py"),
		[]byte("from openmm import app\nsimulation.step(500000)\n"), 0644)
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	logFile := filepath.Join(dir, "simulation.log")
	os.WriteFile(logFile, []byte("Step 500000\nSimulation complete\n"), 0644)

	d := &amberOpenMMDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed (OpenMM Simulation complete)", rs.State)
	}
}

func TestAmberInspect_OpenMMIdle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "run_sim.py"),
		[]byte("import openmm\n"), 0644)

	d := &amberOpenMMDetector{}
	rs := d.Inspect(dir)
	if rs.State != "idle" {
		t.Errorf("state=%q, want idle (OpenMM script, no output)", rs.State)
	}
}

// --- Phase detection tests ---

func TestDetectAmberPhase_Production(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "min.mdout"), []byte("min\n"), 0644)
	os.WriteFile(filepath.Join(dir, "heat.mdout"), []byte("heat\n"), 0644)
	os.WriteFile(filepath.Join(dir, "prod.mdout"), []byte("prod\n"), 0644)

	phase := detectAmberPhase(dir)
	if phase != "production" {
		t.Errorf("phase=%q, want production", phase)
	}
}

func TestDetectAmberPhase_Minimization(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "min.mdout"), []byte("min\n"), 0644)

	phase := detectAmberPhase(dir)
	if phase != "minimization" {
		t.Errorf("phase=%q, want minimization", phase)
	}
}

func TestDetectAmberPhase_Preparation(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "leap.log"), []byte("Welcome to LEaP\n"), 0644)

	phase := detectAmberPhase(dir)
	if phase != "preparation" {
		t.Errorf("phase=%q, want preparation", phase)
	}
}

func TestDetectAmberPhase_OpenMM(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "run.py"),
		[]byte("from openmm import app\n"), 0644)
	os.WriteFile(filepath.Join(dir, "traj.dcd"), []byte{}, 0644)

	phase := detectAmberPhase(dir)
	if phase != "production (OpenMM)" {
		t.Errorf("phase=%q, want 'production (OpenMM)'", phase)
	}
}

func TestDetectAmberPhase_Equilibration(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "min.mdout"), []byte("min\n"), 0644)
	os.WriteFile(filepath.Join(dir, "npt.mdout"), []byte("npt\n"), 0644)

	phase := detectAmberPhase(dir)
	if phase != "equilibration" {
		t.Errorf("phase=%q, want equilibration", phase)
	}
}

// --- Variant detection tests ---

func TestDetectAmberVariant_Amber(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "min.mdin"), []byte("min\n"), 0644)

	v := detectAmberVariant(dir)
	if v != "amber" {
		t.Errorf("variant=%q, want amber", v)
	}
}

func TestDetectAmberVariant_OpenMM(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "run.py"),
		[]byte("from openmm import app\n"), 0644)

	v := detectAmberVariant(dir)
	if v != "openmm" {
		t.Errorf("variant=%q, want openmm", v)
	}
}

func TestDetectAmberVariant_Both(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "run.py"),
		[]byte("from openmm import app\n"), 0644)

	v := detectAmberVariant(dir)
	if v != "amber+openmm" {
		t.Errorf("variant=%q, want amber+openmm", v)
	}
}

// --- Helper tests ---

func TestHasOpenMMScript_Positive(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sim.py"),
		[]byte("import openmm\nfrom openmm import app\n"), 0644)

	if !hasOpenMMScript(dir) {
		t.Error("should detect openmm import")
	}
}

func TestHasOpenMMScript_SimtkLegacy(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "old_sim.py"),
		[]byte("from simtk.openmm import *\n"), 0644)

	if !hasOpenMMScript(dir) {
		t.Error("should detect simtk.openmm import")
	}
}

func TestHasOpenMMScript_NoPython(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.csv"), []byte("a,b\n1,2\n"), 0644)

	if hasOpenMMScript(dir) {
		t.Error("should not detect openmm in non-python files")
	}
}

func TestHasOpenMMScript_PythonNoOpenMM(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "analysis.py"),
		[]byte("import numpy as np\nimport pandas as pd\n"), 0644)

	if hasOpenMMScript(dir) {
		t.Error("generic python script should not match openmm")
	}
}

func TestMdoutHasCompletion_WallClock(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "prod.mdout"), []byte(
		"Run on 05/20/2026\n"+
			"|  Master Total wall time:    12345 seconds\n"+
			"| Final Performance Info:\n",
	), 0644)

	if !mdoutHasCompletion(dir, "amber") {
		t.Error("should detect wallclock completion")
	}
}

func TestMdoutHasCompletion_None(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "prod.mdout"), []byte(
		"   NSTEP       ENERGY\n   5000     -9876.543\n",
	), 0644)

	if mdoutHasCompletion(dir, "amber") {
		t.Error("should not detect completion in running mdout")
	}
}

func TestMdoutHasCompletion_OpenMMOnlyWhenVariant(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sim.log"), []byte("Simulation complete\n"), 0644)

	// Without openmm variant, log should be ignored
	if mdoutHasCompletion(dir, "amber") {
		t.Error("should not check OpenMM logs when variant is amber")
	}
	// With openmm variant, should detect
	if !mdoutHasCompletion(dir, "openmm") {
		t.Error("should detect OpenMM completion when variant is openmm")
	}
}

func TestParseLastNSTEP(t *testing.T) {
	lines := []string{
		"   NSTEP       ENERGY          RMS            GMAX         NAME    NUMBER    TIME(PS)",
		"  50000      -12345.678       1.234          5.678         C1       123    100.000",
		"",
		"more output",
	}
	nstep := parseLastNSTEP(lines)
	if nstep != "50000" {
		t.Errorf("parseLastNSTEP=%q, want 50000", nstep)
	}
}

func TestParseLastNSTEP_NoNSTEP(t *testing.T) {
	lines := []string{"some random output", "no nstep here"}
	nstep := parseLastNSTEP(lines)
	if nstep != "" {
		t.Errorf("parseLastNSTEP=%q, want empty", nstep)
	}
}

// --- JSON round-trip test ---

func TestAmberInspect_JSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "prod.mdin"), []byte("prod\n"), 0644)
	os.WriteFile(filepath.Join(dir, "prod.mdout"), []byte(
		"| Final Performance Info:\n|     ns/day = 45.67\n",
	), 0644)

	d := &amberOpenMMDetector{}
	rs := d.Inspect(dir)

	data, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ResearchStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Detector != "amber_openmm" {
		t.Errorf("detector=%q, want amber_openmm", decoded.Detector)
	}
	if decoded.State == "" {
		t.Error("state field missing after JSON round-trip")
	}
	if decoded.WorkDir == "" {
		t.Error("work_dir field missing after JSON round-trip")
	}
	if len(decoded.Evidence) == 0 {
		t.Error("evidence field missing/empty after JSON round-trip")
	}
}

// --- Meta tests ---

func TestAmberName(t *testing.T) {
	d := &amberOpenMMDetector{}
	if d.Name() != "amber_openmm" {
		t.Errorf("Name=%q, want amber_openmm", d.Name())
	}
}

func TestAmberStuckMinutes(t *testing.T) {
	d := &amberOpenMMDetector{}
	if d.StuckMinutes() != 60 {
		t.Errorf("StuckMinutes=%d, want 60", d.StuckMinutes())
	}
}

// --- Inspect with analysis (completed via analysis output) ---

func TestAmberInspect_CompletedViaAnalysis(t *testing.T) {
	dir := t.TempDir()
	stale := time.Now().Add(-2 * time.Hour)
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	os.Chtimes(filepath.Join(dir, "system.prmtop"), stale, stale)
	os.WriteFile(filepath.Join(dir, "prod.mdin"), []byte("prod\n"), 0644)
	os.Chtimes(filepath.Join(dir, "prod.mdin"), stale, stale)
	// mdout without completion marker, also stale
	mdout := filepath.Join(dir, "prod.mdout")
	os.WriteFile(mdout, []byte("   NSTEP       ENERGY\n   50000     -12345\n"), 0644)
	os.Chtimes(mdout, stale, stale)
	// cpptraj analysis directory with actual products drives completed state
	os.MkdirAll(filepath.Join(dir, "cpptraj"), 0755)
	os.WriteFile(filepath.Join(dir, "cpptraj", "rmsd.dat"), []byte("0 0.1\n1 0.15\n"), 0644)

	d := &amberOpenMMDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed (cpptraj/ with products)", rs.State)
	}
	if rs.Confidence != "medium" {
		t.Errorf("confidence=%q, want medium (inferred from analysis)", rs.Confidence)
	}
}

func TestAmberInspect_EmptyCpptrajNotCompleted(t *testing.T) {
	dir := t.TempDir()
	stale := time.Now().Add(-2 * time.Hour)
	os.WriteFile(filepath.Join(dir, "system.prmtop"), []byte{}, 0644)
	os.Chtimes(filepath.Join(dir, "system.prmtop"), stale, stale)
	mdout := filepath.Join(dir, "prod.mdout")
	os.WriteFile(mdout, []byte("   NSTEP       ENERGY\n   50000     -12345\n"), 0644)
	os.Chtimes(mdout, stale, stale)
	// Empty cpptraj dir — should NOT trigger completed
	os.MkdirAll(filepath.Join(dir, "cpptraj"), 0755)

	d := &amberOpenMMDetector{}
	rs := d.Inspect(dir)
	if rs.State == "completed" {
		t.Errorf("state=%q, want non-completed (empty cpptraj/ dir)", rs.State)
	}
}
