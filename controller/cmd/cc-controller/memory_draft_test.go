package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModeLabel(t *testing.T) {
	tests := map[string]string{
		"summary": "摘要",
		"record":  "进度记录",
		"other":   "other",
	}
	for mode, want := range tests {
		if got := modeLabel(mode); got != want {
			t.Errorf("modeLabel(%q) = %q, want %q", mode, got, want)
		}
	}
}

func TestExtractDraftSummary(t *testing.T) {
	draft := "# Summary\nLine 1\nLine 2\nLine 3"
	result := extractDraftSummary(draft, "summary", "test-123")
	if !strings.Contains(result, "摘要草稿已生成") {
		t.Error("missing label")
	}
	if !strings.Contains(result, "test-123") {
		t.Error("missing run ID")
	}
	if !strings.Contains(result, "这是草稿") {
		t.Error("missing warning")
	}
}

func TestMemoryDraftSystemPrompt(t *testing.T) {
	s := memoryDraftSystemPrompt("summary")
	if !strings.Contains(s, "阶段摘要") {
		t.Error("summary prompt missing expected content")
	}

	r := memoryDraftSystemPrompt("record")
	if !strings.Contains(r, "progress.md") {
		t.Error("record prompt missing expected content")
	}
}

func TestGatherMemoryContext(t *testing.T) {
	tmp := t.TempDir()
	runsDir := filepath.Join(tmp, "runs")
	os.MkdirAll(runsDir, 0755)

	// No files → empty
	result := gatherMemoryContext(tmp, tmp)
	if result != "" {
		t.Errorf("expected empty context, got %q", result)
	}

	// With progress.md
	os.WriteFile(filepath.Join(tmp, "progress.md"), []byte("# Progress\ntest"), 0644)
	result = gatherMemoryContext(tmp, tmp)
	if !strings.Contains(result, "progress.md") {
		t.Error("missing progress.md section")
	}
	if !strings.Contains(result, "test") {
		t.Error("missing progress content")
	}
}

func TestCountRecentRuns(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "run-1"), 0755)
	os.MkdirAll(filepath.Join(tmp, "run-2"), 0755)

	count := countRecentRuns(tmp, 1<<63-1) // huge window
	if count != 2 {
		t.Errorf("got %d, want 2", count)
	}
}
