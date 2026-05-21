package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindDuplicateRunWithQuestionFile(t *testing.T) {
	tmp := t.TempDir()
	runName := time.Now().Format(timeFormat) + "-ask-deadbeef"
	runDir := filepath.Join(tmp, runName)
	os.MkdirAll(runDir, 0755)
	os.WriteFile(filepath.Join(runDir, "incoming-question.md"), []byte("hello world"), 0644)

	got := findDuplicateRun(tmp, "ask", "hello world")
	if got != runName {
		t.Errorf("findDuplicateRun = %q, want %q", got, runName)
	}

	got = findDuplicateRun(tmp, "ask", "different text")
	if got != "" {
		t.Errorf("findDuplicateRun for different text = %q, want empty", got)
	}
}

func TestFindDuplicateRunWithMessageFile(t *testing.T) {
	tmp := t.TempDir()
	runName := time.Now().Format(timeFormat) + "-cc-session-deadbeef"
	runDir := filepath.Join(tmp, runName)
	os.MkdirAll(runDir, 0755)
	os.WriteFile(filepath.Join(runDir, "incoming-message.txt"), []byte("我们现在的工作区是"), 0644)

	got := findDuplicateRun(tmp, "cc-session", "我们现在的工作区是")
	if got != runName {
		t.Errorf("findDuplicateRun = %q, want %q", got, runName)
	}
}

func TestFindDuplicateRunExpiredWindow(t *testing.T) {
	tmp := t.TempDir()
	oldTime := time.Now().Add(-60 * time.Second)
	runName := oldTime.Format(timeFormat) + "-cc-session-deadbeef"
	runDir := filepath.Join(tmp, runName)
	os.MkdirAll(runDir, 0755)
	os.WriteFile(filepath.Join(runDir, "incoming-message.txt"), []byte("same text"), 0644)

	got := findDuplicateRun(tmp, "cc-session", "same text")
	if got != "" {
		t.Errorf("findDuplicateRun for expired run = %q, want empty", got)
	}
}

func TestFindDuplicateRunWrongSuffix(t *testing.T) {
	tmp := t.TempDir()
	runName := time.Now().Format(timeFormat) + "-ask-deadbeef"
	runDir := filepath.Join(tmp, runName)
	os.MkdirAll(runDir, 0755)
	os.WriteFile(filepath.Join(runDir, "incoming-question.md"), []byte("hello"), 0644)

	got := findDuplicateRun(tmp, "cc-session", "hello")
	if got != "" {
		t.Errorf("findDuplicateRun with wrong suffix = %q, want empty", got)
	}
}
