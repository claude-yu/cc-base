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

func TestGaussianMatch_EmptyDir(t *testing.T) {
	d := &gaussianDetector{}
	matched, _ := d.Match(t.TempDir())
	if matched {
		t.Error("empty dir should not match")
	}
}

func TestGaussianMatch_GjfAlone(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "water.gjf"), []byte("%mem=1GB\n#p HF/6-31G* opt\n\nwater\n\n0 1\nO\nH 1 0.96\nH 1 0.96 2 104.5\n\n"), 0644)

	d := &gaussianDetector{}
	matched, score := d.Match(dir)
	// gjf alone = 25 < threshold 40
	if matched {
		t.Errorf("gjf alone: matched=%v score=%d, want no match (score<40)", matched, score)
	}
	if score != 0 {
		// Match returns 0 when below threshold
		t.Errorf("gjf alone: score=%d, want 0 (below threshold returns 0)", score)
	}
}

func TestGaussianMatch_GjfPlusGaussianLog(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "water.gjf"), []byte("#p HF/6-31G*\n"), 0644)
	os.WriteFile(filepath.Join(dir, "water.log"), []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Initial command:\n"+
			" /opt/gaussian/g16/l1.exe\n"+
			" Copyright (c) 1988-2019, Gaussian, Inc.  All Rights Reserved.\n"+
			" SCF Done:  E(RHF) =  -76.123456789     A.U. after    8 cycles\n"+
			" Normal termination of Gaussian 16 at Wed May 21 10:30:00 2026.\n",
	), 0644)

	d := &gaussianDetector{}
	matched, score := d.Match(dir)
	// gjf(25) + gaussianLog(40) = 65
	if !matched || score < 65 {
		t.Errorf("gjf+gaussianLog: matched=%v score=%d, want matched+score>=65", matched, score)
	}
}

func TestGaussianMatch_GenericLogOnly(t *testing.T) {
	dir := t.TempDir()
	// A .log file that is NOT a Gaussian log
	os.WriteFile(filepath.Join(dir, "app.log"), []byte("INFO: Starting application\nINFO: Done\n"), 0644)

	d := &gaussianDetector{}
	matched, _ := d.Match(dir)
	if matched {
		t.Error("generic log without Gaussian content should not match")
	}
}

func TestGaussianMatch_CompletedProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "opt.gjf"), []byte("#p B3LYP/6-31G* opt freq\n"), 0644)
	os.WriteFile(filepath.Join(dir, "opt.com"), []byte("#p B3LYP/6-31G* opt freq\n"), 0644)
	os.WriteFile(filepath.Join(dir, "opt.chk"), []byte{0x00}, 0644)
	os.WriteFile(filepath.Join(dir, "opt.fchk"), []byte("Number of atoms\n"), 0644)
	os.WriteFile(filepath.Join(dir, "opt.log"), []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Gaussian, Inc.  All Rights Reserved.\n"+
			" SCF Done:  E(RB3LYP) =  -230.712345678     A.U. after   12 cycles\n"+
			" Optimization completed.\n"+
			"    -- Stationary point found.\n"+
			" Frequencies --    45.6789                78.1234               120.4567\n"+
			" Normal termination of Gaussian 16 at Wed May 21 10:30:00 2026.\n",
	), 0644)

	d := &gaussianDetector{}
	matched, score := d.Match(dir)
	// gjf(25) + com(25) + chk(15) + fchk(15) + gaussianLog(40) + scfOrOpt(10) = 130 → capped 100
	if !matched {
		t.Errorf("completed project: matched=%v score=%d, want matched", matched, score)
	}
	if score < 80 {
		t.Errorf("completed project: score=%d, want high score (>=80)", score)
	}
}

func TestGaussianMatch_ScoreCap(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "calc.gjf"), []byte("#p HF/STO-3G\n"), 0644)
	os.WriteFile(filepath.Join(dir, "calc.com"), []byte("#p HF/STO-3G\n"), 0644)
	os.WriteFile(filepath.Join(dir, "calc.chk"), []byte{0x00}, 0644)
	os.WriteFile(filepath.Join(dir, "calc.fchk"), []byte("Number of atoms\n"), 0644)
	os.WriteFile(filepath.Join(dir, "calc.log"), []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Gaussian, Inc.  All Rights Reserved.\n"+
			" SCF Done:  E(RHF) =  -74.965901234     A.U. after    6 cycles\n"+
			" Normal termination of Gaussian 16 at Wed May 21 10:30:00 2026.\n",
	), 0644)

	d := &gaussianDetector{}
	_, score := d.Match(dir)
	if score > 100 {
		t.Errorf("score=%d, should be capped at 100", score)
	}
}

// --- Inspect tests ---

func TestGaussianInspect_Completed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "calc.gjf"), []byte("#p HF/6-31G*\n"), 0644)
	os.WriteFile(filepath.Join(dir, "calc.log"), []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Gaussian, Inc.  All Rights Reserved.\n"+
			" SCF Done:  E(RHF) =  -76.123456789     A.U. after    8 cycles\n"+
			" Normal termination of Gaussian 16 at Wed May 21 10:30:00 2026.\n",
	), 0644)

	d := &gaussianDetector{}
	rs := d.Inspect(dir)
	if rs.State != "completed" {
		t.Errorf("state=%q, want completed", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
}

func TestGaussianInspect_Failed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "calc.gjf"), []byte("#p HF/6-31G*\n"), 0644)
	os.WriteFile(filepath.Join(dir, "calc.log"), []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Gaussian, Inc.  All Rights Reserved.\n"+
			" Error termination via Lnk1e in /opt/gaussian/g16/l502.exe at Wed May 21 10:30:00 2026.\n"+
			" Job cpu time:       0 days  0 hours  5 minutes 30.0 seconds.\n",
	), 0644)

	d := &gaussianDetector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed", rs.State)
	}
	if rs.Confidence != "high" {
		t.Errorf("confidence=%q, want high", rs.Confidence)
	}
}

func TestGaussianInspect_FailedConvergence(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "calc.gjf"), []byte("#p B3LYP/6-31G* opt\n"), 0644)
	os.WriteFile(filepath.Join(dir, "calc.log"), []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Gaussian, Inc.  All Rights Reserved.\n"+
			" >>>>>>>>>> Convergence criterion not met.\n"+
			" SCF Done:  E(RB3LYP) =  -230.712345678     A.U. after  128 cycles\n"+
			" Convergence failure -- run terminated.\n",
	), 0644)

	d := &gaussianDetector{}
	rs := d.Inspect(dir)
	if rs.State != "failed" {
		t.Errorf("state=%q, want failed", rs.State)
	}
}

func TestGaussianInspect_Running(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "calc.gjf"), []byte("#p B3LYP/6-31G* opt\n"), 0644)
	logPath := filepath.Join(dir, "calc.log")
	os.WriteFile(logPath, []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Gaussian, Inc.  All Rights Reserved.\n"+
			" SCF Done:  E(RB3LYP) =  -230.712345678     A.U. after   12 cycles\n",
	), 0644)
	// Ensure log is recently modified (t.TempDir files are already recent, but be explicit)
	os.Chtimes(logPath, time.Now(), time.Now())

	d := &gaussianDetector{}
	rs := d.Inspect(dir)
	if rs.State != "running" {
		t.Errorf("state=%q, want running (recently modified log)", rs.State)
	}
}

func TestGaussianInspect_Stuck(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Now().Add(-3 * time.Hour)
	os.WriteFile(filepath.Join(dir, "calc.gjf"), []byte("#p B3LYP/6-31G* opt\n"), 0644)
	os.Chtimes(filepath.Join(dir, "calc.gjf"), oldTime, oldTime)
	logPath := filepath.Join(dir, "calc.log")
	os.WriteFile(logPath, []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Gaussian, Inc.  All Rights Reserved.\n"+
			" SCF Done:  E(RB3LYP) =  -230.712345678     A.U. after   12 cycles\n",
	), 0644)
	os.Chtimes(logPath, oldTime, oldTime)

	d := &gaussianDetector{}
	rs := d.Inspect(dir)
	if rs.State != "stuck" {
		t.Errorf("state=%q, want stuck (log >120min old)", rs.State)
	}
}

func TestGaussianInspect_Idle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "calc.gjf"), []byte("#p HF/6-31G*\n"), 0644)
	os.WriteFile(filepath.Join(dir, "calc.com"), []byte("#p HF/6-31G*\n"), 0644)

	d := &gaussianDetector{}
	rs := d.Inspect(dir)
	if rs.State != "idle" {
		t.Errorf("state=%q, want idle (input files only, no logs)", rs.State)
	}
}

func TestGaussianInspect_PhaseSCF(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "calc.gjf"), []byte("#p HF/6-31G*\n"), 0644)
	logPath := filepath.Join(dir, "calc.log")
	os.WriteFile(logPath, []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Gaussian, Inc.  All Rights Reserved.\n"+
			" SCF Done:  E(RB3LYP) =  -230.712345678     A.U. after   12 cycles\n",
	), 0644)

	d := &gaussianDetector{}
	rs := d.Inspect(dir)
	if !strings.Contains(rs.ContextPhase, "scf") {
		t.Errorf("ContextPhase=%q, want to contain 'scf'", rs.ContextPhase)
	}
}

func TestGaussianInspect_PhaseOptimization(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "calc.gjf"), []byte("#p B3LYP/6-31G* opt\n"), 0644)
	logPath := filepath.Join(dir, "calc.log")
	os.WriteFile(logPath, []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Gaussian, Inc.  All Rights Reserved.\n"+
			" SCF Done:  E(RB3LYP) =  -230.712345678     A.U. after   12 cycles\n"+
			"          Item               Value     Threshold  Converged?\n"+
			" Maximum Force            0.000012     0.000450     YES\n"+
			" RMS     Force            0.000003     0.000300     YES\n"+
			" Maximum Displacement     0.000543     0.001800     YES\n"+
			" RMS     Displacement     0.000123     0.001200     YES\n"+
			" Predicted change in Energy=-1.234567D-08\n"+
			" Optimization completed.\n"+
			"    -- Stationary point found.\n",
	), 0644)

	d := &gaussianDetector{}
	rs := d.Inspect(dir)
	if !strings.Contains(rs.ContextPhase, "optimization") {
		t.Errorf("ContextPhase=%q, want to contain 'optimization'", rs.ContextPhase)
	}
}

func TestGaussianInspect_PhaseFrequency(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "calc.gjf"), []byte("#p B3LYP/6-31G* opt freq\n"), 0644)
	logPath := filepath.Join(dir, "calc.log")
	os.WriteFile(logPath, []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Gaussian, Inc.  All Rights Reserved.\n"+
			" SCF Done:  E(RB3LYP) =  -230.712345678     A.U. after   12 cycles\n"+
			" Optimization completed.\n"+
			"    -- Stationary point found.\n"+
			" Frequencies --    45.6789                78.1234               120.4567\n"+
			" Red. masses --     3.1234                 5.4567                 2.7890\n"+
			" Frc consts  --     0.0045                 0.0123                 0.0234\n"+
			" IR Inten    --     2.3456                 0.1234                 5.6789\n",
	), 0644)

	d := &gaussianDetector{}
	rs := d.Inspect(dir)
	if !strings.Contains(rs.ContextPhase, "frequency") {
		t.Errorf("ContextPhase=%q, want to contain 'frequency'", rs.ContextPhase)
	}
}

// --- Meta tests ---

func TestGaussianDetector_Meta(t *testing.T) {
	d := &gaussianDetector{}
	if d.Name() != "gaussian" {
		t.Errorf("Name=%q, want gaussian", d.Name())
	}
	if d.StuckMinutes() != 120 {
		t.Errorf("StuckMinutes=%d, want 120", d.StuckMinutes())
	}
}

// --- JSON round-trip test ---

func TestGaussianInspect_JSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "calc.gjf"), []byte("#p HF/6-31G*\n"), 0644)
	os.WriteFile(filepath.Join(dir, "calc.log"), []byte(
		" Entering Gaussian System, Link 0=g16\n"+
			" Gaussian, Inc.  All Rights Reserved.\n"+
			" SCF Done:  E(RHF) =  -76.123456789     A.U. after    8 cycles\n"+
			" Normal termination of Gaussian 16 at Wed May 21 10:30:00 2026.\n",
	), 0644)

	d := &gaussianDetector{}
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
	if decoded.Detector != "gaussian" {
		t.Errorf("detector=%q, want gaussian", decoded.Detector)
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
	if decoded.ContextPhase == "" {
		t.Error("context_phase field missing after JSON round-trip")
	}
}
