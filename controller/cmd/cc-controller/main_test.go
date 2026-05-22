package main

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestFindLatestRunSkipsSidecarDirs is a regression for the e2e-caught bug:
// sidecar dirs like verify2-032548 / cc-restart lexically sort after digit-
// prefixed run IDs, so an unfiltered desc sort wrongly returned them.
func TestFindLatestRunSkipsSidecarDirs(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"20260519-022800-codex-ask",
		"20260519-103418-codex-ask-b52bd396", // newest real run
		"verify2-032548",                     // 'v' > '2' lexically
		"cc-restart",
		"not-a-dir-file",
	} {
		if name == "not-a-dir-file" {
			os.WriteFile(filepath.Join(root, name), []byte("x"), 0644)
			continue
		}
		os.MkdirAll(filepath.Join(root, name), 0755)
	}
	if got := findLatestRun(root); got != "20260519-103418-codex-ask-b52bd396" {
		t.Fatalf("findLatestRun = %q, want newest timestamped run", got)
	}
}

// TestReadInputArgs pins the fidelity of the {{args}} path that config.toml uses
// (cc-connect 鈫?cc-controller.exe ask-codex {{args}} 鈫?os.Args 鈫?readInput).
// This is the empirical verification of Codex round-2 mandatory fix #2.
func TestReadInputArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"plain ascii", []string{"hello", "world"}, "hello world"},
		{"chinese", []string{"杩欐槸涓枃娴嬭瘯"}, "杩欐槸涓枃娴嬭瘯"},
		{"double quotes inside one arg", []string{`浠栬"浣犲ソ"`}, `浠栬"浣犲ソ"`},
		{"apostrophe", []string{"it's", "fine"}, "it's fine"},
		{"shell metachars preserved", []string{"a & b | c $x"}, "a & b | c $x"},
		{"newline survives inside a single arg", []string{"line1\nline2"}, "line1\nline2"},
		// FIDELITY LOSS, asserted on purpose: multi-arg join collapses any
		// run of whitespace to a single space. If cc-connect word-splits the
		// user message, "a  b" (double space) arrives as "a b".
		{"interior whitespace collapsed", []string{"a", "", "b"}, "a  b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := readInput(c.args); got != c.want {
				t.Fatalf("readInput(%q) = %q, want %q", c.args, got, c.want)
			}
		})
	}
}

// TestReadInputStdinTrims documents the stdin-fallback fidelity gap:
// readInput does bytes.TrimSpace, so intentional leading/trailing newlines
// or padding sent via stdin are stripped (Codex fix #2 residual).
func TestReadInputStdinTrims(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig }()

	go func() {
		io.WriteString(w, "\n\n  padded body\nkept line  \n\n")
		w.Close()
	}()

	got := readInput(nil)
	want := "padded body\nkept line" // leading/trailing whitespace stripped
	if got != want {
		t.Fatalf("stdin readInput = %q, want %q (TrimSpace strips edges)", got, want)
	}
}

func TestSanitizeProjectID_ASCII(t *testing.T) {
	got := sanitizeProjectID(`E:\projects\my-app`)
	if got != "my-app" {
		t.Fatalf("ASCII dir: got %q, want %q", got, "my-app")
	}
}

func TestSanitizeProjectID_ChineseDir(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("backslash path test only meaningful on Windows")
	}
	got := sanitizeProjectID(`D:\research-work\work_12\铏氭嫙鏁查櫎`)
	if got == "____" || got == "" {
		t.Fatalf("Chinese dir produced degenerate ID: %q", got)
	}
	if !strings.Contains(got, "work_12") {
		t.Fatalf("expected parent slug in ID, got %q", got)
	}
}

func TestSanitizeProjectID_SamePathSameID(t *testing.T) {
	a := sanitizeProjectID(`D:\research-work\work_12\铏氭嫙鏁查櫎`)
	b := sanitizeProjectID(`D:\research-work\work_12\铏氭嫙鏁查櫎`)
	if a != b {
		t.Fatalf("same path produced different IDs: %q vs %q", a, b)
	}
}

func TestSanitizeProjectID_DifferentPathsDifferentIDs(t *testing.T) {
	a := sanitizeProjectID(`D:\research-work\work_12\铏氭嫙鏁查櫎`)
	b := sanitizeProjectID(`D:\research-work\work_13\铏氭嫙鏁查櫎`)
	if a == b {
		t.Fatalf("different paths produced same ID: %q", a)
	}
}

func TestSanitizeProjectID_MixedName(t *testing.T) {
	got := sanitizeProjectID(`E:\projects\vko_铏氭嫙鏁查櫎`)
	if got != "vko___" && !strings.HasPrefix(got, "vko_") {
		t.Fatalf("mixed name: expected ASCII prefix preserved, got %q", got)
	}
	if strings.TrimRight(got, "_") == "" {
		t.Fatalf("mixed name produced all underscores: %q", got)
	}
}

func TestResolveExecuteRequestWorkDirPrefersActiveProject(t *testing.T) {
	projectDir := t.TempDir()
	envDir := t.TempDir()
	t.Setenv("CC_EXECUTE_WORK_DIR", envDir)

	got, label, err := resolveExecuteRequestWorkDir(ActiveProject{WorkDir: projectDir})
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(projectDir) {
		t.Fatalf("workdir = %q, want active project %q", got, projectDir)
	}
	if label != "当前项目工作目录" {
		t.Fatalf("label = %q, want active project label", label)
	}
}

func TestResolveExecuteRequestWorkDirFallsBackToEnv(t *testing.T) {
	envDir := t.TempDir()
	t.Setenv("CC_EXECUTE_WORK_DIR", envDir)

	got, label, err := resolveExecuteRequestWorkDir(ActiveProject{WorkDir: filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(envDir) {
		t.Fatalf("workdir = %q, want env dir %q", got, envDir)
	}
	if label != "CC_EXECUTE_WORK_DIR" {
		t.Fatalf("label = %q, want env label", label)
	}
}

func writeTestRunStatus(t *testing.T, runsRoot, runID, sessionID, status, stage string) {
	t.Helper()
	runDir := filepath.Join(runsRoot, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeStatusJSON(runDir, statusJSON{
		RunID:        runID,
		Kind:         "cc-session",
		Status:       status,
		Stage:        stage,
		SessionID:    sessionID,
		SessionScope: "project_default",
	})
}

func TestFindLatestMeaningfulRunForSessionRequiresExactSession(t *testing.T) {
	runsRoot := t.TempDir()
	writeTestRunStatus(t, runsRoot, "20260521-100000-cc-session-old", "", "completed", "done")
	writeTestRunStatus(t, runsRoot, "20260521-100100-cc-session-other", "other-default", "completed", "done")
	writeTestRunStatus(t, runsRoot, "20260521-100200-cc-session-current", "work_12-76504f6a-default", "completed", "done")

	got := findLatestMeaningfulRunForSession(runsRoot, "", "work_12-76504f6a-default")
	if got != "20260521-100200-cc-session-current" {
		t.Fatalf("current session latest = %q, want current session run", got)
	}
}

func TestFindLatestRunByKindForSessionIgnoresLegacyNoSessionRuns(t *testing.T) {
	runsRoot := t.TempDir()
	writeTestRunStatus(t, runsRoot, "20260521-100000-cc-session-current", "work_12-76504f6a-default", "completed", "done")
	writeTestRunStatus(t, runsRoot, "20260521-100100-cc-session-legacy", "", "completed", "done")

	got := findLatestRunByKindForSession(runsRoot, "cc-session", "work_12-76504f6a-default")
	if got != "20260521-100000-cc-session-current" {
		t.Fatalf("kind current session latest = %q, want current session run", got)
	}
}

func TestCleanCodexOutputTaskkill(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			"english taskkill",
			"SUCCESS: The process with PID 1234 has been terminated.\nActual answer",
			"Actual answer",
		},
		{
			"multiple taskkill lines",
			"SUCCESS: PID 7260\nSUCCESS: PID 7261\nAnswer starts here",
			"Answer starts here",
		},
		{
			"no taskkill passthrough",
			"This is a normal Codex answer\nwithout process noise",
			"This is a normal Codex answer\nwithout process noise",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanCodexOutput(tc.input)
			if got != tc.want {
				t.Fatalf("cleanCodexOutput() = %q, want %q", got, tc.want)
			}
		})
	}
}



