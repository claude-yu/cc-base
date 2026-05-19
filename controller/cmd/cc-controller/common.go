package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func mustHaveArgs(args []string, n int, msg string) {
	if len(args) < n {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

func resolveControllerRoot() string {
	if dir := os.Getenv("CC_CONTROLLER_DIR"); dir != "" {
		return dir
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if filepath.Base(dir) == "bin" {
			return filepath.Dir(dir)
		}
		if fi, err := os.Stat(filepath.Join(dir, "runs")); err == nil && fi.IsDir() {
			return dir
		}
	}
	if root := os.Getenv("CONTROLLER_ROOT"); root != "" {
		return root
	}
	fmt.Fprintln(os.Stderr, "FATAL: cannot resolve controller root. Set CC_CONTROLLER_DIR or CONTROLLER_ROOT.")
	os.Exit(1)
	return ""
}

// resolveProjectWorkDir returns the per-research-project working directory.
// Switch projects by changing CC_WORK_DIR only — controller dir and sandbox
// stay fixed.
func resolveProjectWorkDir() string {
	if dir := os.Getenv("CC_WORK_DIR"); dir != "" {
		return dir
	}
	return "."
}

func writeFile(path, content string) {
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte(content), 0644)
}

func sendCallback(runDir, message string) {
	callbackPath := filepath.Join(runDir, "callback-msg.md")
	os.WriteFile(callbackPath, []byte(message), 0644)

	ccConnect, err := exec.LookPath("cc-connect")
	if err != nil {
		candidates := []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "cc-connect", "cc-connect.exe"),
			"cc-connect.exe",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				ccConnect = c
				break
			}
		}
		if ccConnect == "" {
			return
		}
	}
	cmd := exec.Command(ccConnect, "send", "--stdin", "-p", "cc")
	cmd.Stdin = strings.NewReader(message)
	cmd.Run()
}

func writeError(runDir string, err error) {
	msg := fmt.Sprintf("Error: %s", err.Error())
	os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(msg), 0644)
	os.WriteFile(filepath.Join(runDir, "runner.exitcode.txt"), []byte("1"), 0644)
	appendEvent(runDir, eventEntry{Ts: time.Now().UTC().Format(time.RFC3339), Type: "error", Message: err.Error()})
}

// trimToken strips a leading UTF-8 BOM (legacy PowerShell-written files have one)
// plus surrounding whitespace, so single-token files compare reliably.
func trimToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, string(rune(0xFEFF)))
	return strings.TrimSpace(s)
}

// readInput returns the first arg or stdin, trimming whitespace.
func readInput(args []string) string {
	if len(args) > 0 {
		return strings.Join(args, " ")
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		return ""
	}
	return strings.TrimSpace(string(data))
}
