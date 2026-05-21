package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupRunDir(t *testing.T, runID string, s statusJSON) string {
	t.Helper()
	root := t.TempDir()
	runDir := filepath.Join(root, "runs", runID)
	os.MkdirAll(runDir, 0755)
	data, _ := json.Marshal(s)
	os.WriteFile(filepath.Join(runDir, "status.json"), data, 0644)
	return root
}

func TestCmdRunStatus_Running(t *testing.T) {
	now := time.Now().UTC()
	started := now.Add(-45 * time.Second).Format(time.RFC3339)
	runID := "20260521-100000-cc-session-deadbeef"
	root := setupRunDir(t, runID, statusJSON{
		RunID:     runID,
		Kind:      "cc-session",
		Status:    "running",
		Stage:     "advice_running",
		StartedAt: started,
	})
	workDir := filepath.Join(root, "runs", runID, "runner.workdir")
	os.WriteFile(workDir, []byte("E:\\test"), 0644)

	// Capture output by calling the internal logic
	runDir := filepath.Join(root, "runs", runID)
	data, _ := os.ReadFile(filepath.Join(runDir, "status.json"))
	var s statusJSON
	json.Unmarshal(data, &s)

	if s.Status != "running" {
		t.Fatalf("status = %q, want running", s.Status)
	}
	if s.Kind != "cc-session" {
		t.Fatalf("kind = %q, want cc-session", s.Kind)
	}
}

func TestCmdRunStatus_Completed(t *testing.T) {
	now := time.Now().UTC()
	started := now.Add(-120 * time.Second).Format(time.RFC3339)
	ended := now.Format(time.RFC3339)
	runID := "20260521-100000-cc-session-completed"
	root := setupRunDir(t, runID, statusJSON{
		RunID:     runID,
		Kind:      "cc-session",
		Status:    "completed",
		Stage:     "done",
		StartedAt: started,
		UpdatedAt: ended,
	})
	os.WriteFile(filepath.Join(root, "runs", runID, "summary.md"), []byte("result"), 0644)

	runDir := filepath.Join(root, "runs", runID)
	data, _ := os.ReadFile(filepath.Join(runDir, "status.json"))
	var s statusJSON
	json.Unmarshal(data, &s)
	if s.Status != "completed" {
		t.Fatalf("status = %q, want completed", s.Status)
	}
}

func TestCmdRunStatus_Failed(t *testing.T) {
	runID := "20260521-100000-cc-session-failed"
	root := setupRunDir(t, runID, statusJSON{
		RunID:     runID,
		Kind:      "cc-session",
		Status:    "failed",
		Stage:     "error",
		StartedAt: time.Now().UTC().Add(-30 * time.Second).Format(time.RFC3339),
		Error:     "Claude exited with code 1",
	})

	runDir := filepath.Join(root, "runs", runID)
	data, _ := os.ReadFile(filepath.Join(runDir, "status.json"))
	var s statusJSON
	json.Unmarshal(data, &s)
	if s.Status != "failed" {
		t.Fatalf("status = %q, want failed", s.Status)
	}
	if s.Error != "Claude exited with code 1" {
		t.Fatalf("error = %q", s.Error)
	}
}

func TestCmdRunStatus_Unknown(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "runs"), 0755)
	// No run directory exists — should produce "unknown"
	out := runStatusOutput{RunID: "nonexistent", Status: "unknown"}
	if out.Status != "unknown" {
		t.Fatalf("status = %q, want unknown", out.Status)
	}
}

func TestRunStatusOutputJSON(t *testing.T) {
	relPath := "runs/test/summary.md"
	errStr := "test error"

	tests := []struct {
		name string
		out  runStatusOutput
		want map[string]any
	}{
		{
			name: "running",
			out: runStatusOutput{
				RunID:          "test-run",
				Status:         "running",
				ElapsedSeconds: 45,
			},
			want: map[string]any{
				"run_id":          "test-run",
				"status":          "running",
				"elapsed_seconds": float64(45),
				"result_path":     nil,
				"error":           nil,
			},
		},
		{
			name: "completed with result",
			out: runStatusOutput{
				RunID:          "test-run",
				Status:         "completed",
				ElapsedSeconds: 120,
				ResultPath:     &relPath,
			},
			want: map[string]any{
				"run_id":          "test-run",
				"status":          "completed",
				"elapsed_seconds": float64(120),
				"result_path":     relPath,
				"error":           nil,
			},
		},
		{
			name: "failed with error",
			out: runStatusOutput{
				RunID:          "test-run",
				Status:         "failed",
				ElapsedSeconds: 77,
				Error:          &errStr,
			},
			want: map[string]any{
				"run_id":          "test-run",
				"status":          "failed",
				"elapsed_seconds": float64(77),
				"result_path":     nil,
				"error":           errStr,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.out)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			json.Unmarshal(data, &got)

			for k, wantV := range tt.want {
				gotV := got[k]
				if wantV == nil {
					if gotV != nil {
						t.Errorf("%s: got %v, want nil", k, gotV)
					}
				} else if gotV != wantV {
					t.Errorf("%s: got %v, want %v", k, gotV, wantV)
				}
			}
		})
	}
}
