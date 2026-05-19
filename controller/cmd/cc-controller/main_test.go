package main

import (
	"io"
	"os"
	"path/filepath"
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
// (cc-connect → cc-controller.exe ask-codex {{args}} → os.Args → readInput).
// This is the empirical verification of Codex round-2 mandatory fix #2.
func TestReadInputArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"plain ascii", []string{"hello", "world"}, "hello world"},
		{"chinese", []string{"这是中文测试"}, "这是中文测试"},
		{"double quotes inside one arg", []string{`他说"你好"`}, `他说"你好"`},
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
