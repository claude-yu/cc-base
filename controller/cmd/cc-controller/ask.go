package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func genRunID(suffix string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%s-%s-%s", time.Now().Format(timeFormat), suffix, hex.EncodeToString(b))
}

func cmdAsk(root, suffix, runnerName string, args []string) {
	text := readInput(args)
	if text == "" {
		fmt.Fprintln(os.Stderr, "no input provided")
		os.Exit(1)
	}

	runID := genRunID(suffix)
	runDir := filepath.Join(root, "runs", runID)
	os.MkdirAll(runDir, 0755)
	os.WriteFile(filepath.Join(runDir, "incoming-question.md"), []byte(strings.TrimSpace(text)), 0644)

	writeStatusJSON(runDir, statusJSON{
		RunID:     runID,
		Kind:      suffix,
		Status:    "accepted",
		Stage:     "accepted",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	})
	appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), RunID: runID, Type: "accepted"})

	exe, _ := os.Executable()
	runner := exec.Command(exe, runnerName, runID)
	runner.Dir = root
	runner.Stdin = nil
	runner.Stdout = nil
	runner.Stderr = nil
	runner.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := runner.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start runner: %s\n", err)
		os.Exit(1)
	}
	writeFile(filepath.Join(runDir, "runner.pid"), fmt.Sprintf("%d", runner.Process.Pid))
	updateStatusJSON(runDir, "running", "prepare", runner.Process.Pid)

	fmt.Println(runID)
}
