package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type runStatusOutput struct {
	RunID          string  `json:"run_id"`
	Status         string  `json:"status"`
	Stage          string  `json:"stage,omitempty"`
	Kind           string  `json:"kind,omitempty"`
	WorkDir        string  `json:"work_dir,omitempty"`
	ElapsedSeconds int     `json:"elapsed_seconds"`
	StartedAt      string  `json:"started_at,omitempty"`
	ResultPath     *string `json:"result_path"`
	Error          *string `json:"error"`
}

func cmdRunStatus(root string, args []string) {
	if len(args) == 0 || args[0] == "" {
		fmt.Fprintln(os.Stderr, "usage: cc-controller run-status <run_id> [--format json]")
		os.Exit(1)
	}
	runID := args[0]

	runDir := filepath.Join(root, "runs", runID)
	statusPath := filepath.Join(runDir, "status.json")

	data, err := os.ReadFile(statusPath)
	if err != nil {
		outputUnknown(runID)
		return
	}

	var s statusJSON
	if json.Unmarshal(data, &s) != nil {
		outputUnknown(runID)
		return
	}

	out := runStatusOutput{
		RunID:     runID,
		Status:    s.Status,
		Stage:     s.Stage,
		Kind:      s.Kind,
		StartedAt: s.StartedAt,
	}

	if data, err := os.ReadFile(filepath.Join(runDir, "runner.workdir")); err == nil {
		out.WorkDir = strings.TrimSpace(string(data))
	}

	if s.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, s.StartedAt); err == nil {
			out.ElapsedSeconds = int(time.Since(t).Seconds())
		}
	}

	if s.Status == "completed" || s.Status == "failed" {
		if s.UpdatedAt != "" {
			if tStart, err1 := time.Parse(time.RFC3339, s.StartedAt); err1 == nil {
				if tEnd, err2 := time.Parse(time.RFC3339, s.UpdatedAt); err2 == nil {
					out.ElapsedSeconds = int(tEnd.Sub(tStart).Seconds())
				}
			}
		}
	}

	switch s.Status {
	case "completed":
		for _, name := range []string{"summary.md", "cc-answer.md", "result.md"} {
			p := filepath.Join(runDir, name)
			if _, err := os.Stat(p); err == nil {
				relPath := filepath.Join("runs", runID, name)
				out.ResultPath = &relPath
				break
			}
		}
	case "failed":
		if s.Error != "" {
			out.Error = &s.Error
		} else if data, err := os.ReadFile(filepath.Join(runDir, "error.txt")); err == nil {
			errStr := strings.TrimSpace(string(data))
			out.Error = &errStr
		}
	}

	writeStatusOutput(out)
}

func outputUnknown(runID string) {
	writeStatusOutput(runStatusOutput{
		RunID:  runID,
		Status: "unknown",
	})
}

func writeStatusOutput(v runStatusOutput) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}
