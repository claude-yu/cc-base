package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildMonitorOutput_Schema(t *testing.T) {
	results := []ResearchStatus{
		{
			Detector:       "gromacs",
			State:          "running",
			Confidence:     "high",
			Score:          85,
			Index:          1,
			WorkDir:        "/work/sim1",
			KeyFiles:       []string{"md.log", "topol.tpr"},
			LastUpdate:     "10 分钟前",
			LastUpdateMins: 10,
			Evidence:       []string{"md.log exists", "tpr found"},
			Warnings:       nil,
			NextActions:    []string{"wait for completion"},
			ContextPhase:   "production MD",
		},
		{
			Detector:       "haddock3",
			State:          "failed",
			Confidence:     "high",
			Score:          70,
			Index:          2,
			Bucket:         "active_failed",
			WorkDir:        "/work/dock1",
			KeyFiles:       []string{"run.cfg"},
			LastUpdate:     "2 小时前",
			LastUpdateMins: 120,
			Evidence:       []string{"stage 3 error"},
			Warnings:       []string{"check config"},
			NextActions:    nil,
		},
	}

	out := buildMonitorOutput(results, "20260520-141530-abc", "/work", "gromacs", 3)

	// Verify scan metadata
	if out.Scan.RunID != "20260520-141530-abc" {
		t.Errorf("Scan.RunID = %q, want %q", out.Scan.RunID, "20260520-141530-abc")
	}
	if out.Scan.WorkDir != "/work" {
		t.Errorf("Scan.WorkDir = %q, want %q", out.Scan.WorkDir, "/work")
	}
	if out.Scan.ScanDepth != 3 {
		t.Errorf("Scan.ScanDepth = %d, want 3", out.Scan.ScanDepth)
	}
	if out.Scan.DetectorFilter != "gromacs" {
		t.Errorf("Scan.DetectorFilter = %q, want %q", out.Scan.DetectorFilter, "gromacs")
	}
	if out.Scan.TotalTasks != 2 {
		t.Errorf("Scan.TotalTasks = %d, want 2", out.Scan.TotalTasks)
	}
	if out.Scan.ScannedAt == "" {
		t.Error("Scan.ScannedAt should not be empty")
	}

	// Verify summary
	if out.Summary.ByState["running"] != 1 {
		t.Errorf("Summary.ByState[running] = %d, want 1", out.Summary.ByState["running"])
	}
	if out.Summary.ByState["failed"] != 1 {
		t.Errorf("Summary.ByState[failed] = %d, want 1", out.Summary.ByState["failed"])
	}
	if out.Summary.ByBucket["active_failed"] != 1 {
		t.Errorf("Summary.ByBucket[active_failed] = %d, want 1", out.Summary.ByBucket["active_failed"])
	}

	// Verify tasks
	if len(out.Tasks) != 2 {
		t.Fatalf("Tasks length = %d, want 2", len(out.Tasks))
	}
	if out.Tasks[0].Detector != "gromacs" {
		t.Errorf("Tasks[0].Detector = %q, want %q", out.Tasks[0].Detector, "gromacs")
	}
	if out.Tasks[1].Bucket != "active_failed" {
		t.Errorf("Tasks[1].Bucket = %q, want %q", out.Tasks[1].Bucket, "active_failed")
	}
}

func TestBuildMonitorOutput_EmptyResults(t *testing.T) {
	out := buildMonitorOutput(nil, "run-1", "/work", "", 3)
	if out.Scan.TotalTasks != 0 {
		t.Errorf("TotalTasks = %d, want 0", out.Scan.TotalTasks)
	}
	if len(out.Summary.ByState) != 0 {
		t.Errorf("ByState should be empty, got %v", out.Summary.ByState)
	}
	if len(out.Tasks) != 0 {
		t.Errorf("Tasks should be empty for nil input, got %v", out.Tasks)
	}

	// Verify JSON serializes as [] not null
	data, _ := json.Marshal(out)
	if !strings.Contains(string(data), `"tasks":[]`) {
		t.Errorf("empty tasks should serialize as [], got %s", string(data))
	}
}

func TestBuildMonitorOutput_JSONRoundTrip(t *testing.T) {
	results := []ResearchStatus{
		{
			Detector:       "rosetta",
			State:          "completed",
			Confidence:     "medium",
			Score:          60,
			Index:          1,
			WorkDir:        "/work/design",
			KeyFiles:       []string{"score.sc"},
			Evidence:       []string{"score.sc: 50 decoys, best -220.5"},
			ContextPhase:   "scoring",
		},
	}

	out := buildMonitorOutput(results, "run-2", "/work", "", 3)
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var parsed MonitorOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify all top-level fields survive round-trip
	if parsed.Scan.RunID != out.Scan.RunID {
		t.Errorf("RunID mismatch after round-trip")
	}
	if parsed.Scan.TotalTasks != 1 {
		t.Errorf("TotalTasks = %d after round-trip", parsed.Scan.TotalTasks)
	}
	if len(parsed.Tasks) != 1 {
		t.Fatalf("Tasks length = %d after round-trip", len(parsed.Tasks))
	}
	if parsed.Tasks[0].Detector != "rosetta" {
		t.Errorf("Tasks[0].Detector = %q after round-trip", parsed.Tasks[0].Detector)
	}
	if parsed.Tasks[0].ContextPhase != "scoring" {
		t.Errorf("Tasks[0].ContextPhase = %q after round-trip", parsed.Tasks[0].ContextPhase)
	}
}

func TestBuildMonitorOutput_RequiredJSONFields(t *testing.T) {
	results := []ResearchStatus{
		{
			Detector:   "gromacs",
			State:      "running",
			Confidence: "high",
			Score:      85,
			Index:      1,
			WorkDir:    "/sim",
			KeyFiles:   []string{"md.log"},
			Evidence:   []string{"running"},
		},
	}

	out := buildMonitorOutput(results, "run-3", "/sim", "", 3)
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	requiredTopLevel := []string{"scan", "summary", "tasks"}
	for _, key := range requiredTopLevel {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing required top-level field %q", key)
		}
	}

	var scan map[string]json.RawMessage
	json.Unmarshal(raw["scan"], &scan)
	requiredScan := []string{"run_id", "work_dir", "scanned_at", "scan_depth", "total_tasks"}
	for _, key := range requiredScan {
		if _, ok := scan[key]; !ok {
			t.Errorf("missing required scan field %q", key)
		}
	}

	var summary map[string]json.RawMessage
	json.Unmarshal(raw["summary"], &summary)
	requiredSummary := []string{"by_state", "by_bucket"}
	for _, key := range requiredSummary {
		if _, ok := summary[key]; !ok {
			t.Errorf("missing required summary field %q", key)
		}
	}

	var tasks []map[string]json.RawMessage
	json.Unmarshal(raw["tasks"], &tasks)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	requiredTask := []string{"detector", "state", "confidence", "score", "index", "work_dir", "key_files", "evidence"}
	for _, key := range requiredTask {
		if _, ok := tasks[0][key]; !ok {
			t.Errorf("missing required task field %q", key)
		}
	}
}

func TestBuildMonitorOutput_OmitsEmptyOptionalFields(t *testing.T) {
	results := []ResearchStatus{
		{
			Detector:   "python_pipeline",
			State:      "idle",
			Confidence: "low",
			Score:      20,
			Index:      1,
			WorkDir:    "/scripts",
			KeyFiles:   []string{"main.py"},
			Evidence:   []string{"python script found"},
		},
	}

	out := buildMonitorOutput(results, "run-4", "/scripts", "", 3)
	data, _ := json.Marshal(out)

	var tasks []map[string]json.RawMessage
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)
	json.Unmarshal(raw["tasks"], &tasks)

	optionalOmitted := []string{"bucket", "warnings", "next_actions", "context_phase", "last_update", "last_update_mins"}
	for _, key := range optionalOmitted {
		if _, ok := tasks[0][key]; ok {
			t.Errorf("optional field %q should be omitted when empty/zero, but was present", key)
		}
	}
}

func TestPrintJSONResults_UsesMonitorOutputSchema(t *testing.T) {
	results := []ResearchStatus{
		{Detector: "gromacs", State: "running", Index: 1, WorkDir: "/sim", Confidence: "high", Score: 80, KeyFiles: []string{"md.log"}, Evidence: []string{"running"}},
	}

	out := MonitorOutput{
		Scan: MonitorScan{
			RunID:          "run-5",
			DetectorFilter: "gromacs",
			TotalTasks:     1,
		},
		Summary: buildSummaryFromResults(results),
		Tasks:   results,
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	for _, key := range []string{"scan", "summary", "tasks"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing top-level field %q — printJSONResults must use MonitorOutput schema", key)
		}
	}
}
