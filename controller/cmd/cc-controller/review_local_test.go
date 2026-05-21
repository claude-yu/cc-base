package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatReviewMobile(t *testing.T) {
	tests := []struct {
		name    string
		report  *ReviewReport
		preset  string
		wantHas []string
		wantNot []string
	}{
		{
			name: "pass verdict",
			report: &ReviewReport{
				Verdict:  VerdictPass,
				Summary:  "No issues found",
				Findings: nil,
			},
			preset:  "security",
			wantHas: []string{"✅", "PASS", "security", "No issues found"},
		},
		{
			name: "warn with findings",
			report: &ReviewReport{
				Verdict: VerdictWarn,
				Summary: "One issue",
				Findings: []ReviewFinding{
					{Severity: "HIGH", Title: "hardcoded token", File: "main.go", Line: 42},
					{Severity: "LOW", Title: "unused var"},
				},
			},
			preset:  "general",
			wantHas: []string{"⚠️", "WARN", "[HIGH]", "hardcoded token", "main.go:42", "1 个 LOW"},
			wantNot: []string{"[LOW] unused var"},
		},
		{
			name: "fail with critical",
			report: &ReviewReport{
				Verdict: VerdictFail,
				Summary: "Critical issue",
				Findings: []ReviewFinding{
					{Severity: "CRITICAL", Title: "SQL injection", File: "db.go"},
					{Severity: "MEDIUM", Title: "missing error check"},
				},
			},
			preset:  "security",
			wantHas: []string{"❌", "FAIL", "[CRITICAL]", "SQL injection", "(db.go)", "[MEDIUM]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatReviewMobile(tt.report, tt.preset, "test-run-id")
			for _, want := range tt.wantHas {
				if !strings.Contains(result, want) {
					t.Errorf("want %q in output, got:\n%s", want, result)
				}
			}
			for _, not := range tt.wantNot {
				if strings.Contains(result, not) {
					t.Errorf("want %q NOT in output, got:\n%s", not, result)
				}
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("got %q", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("got %q", got)
	}
}

func TestGetLocalDiffReadsControllerRepoOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test")
	path := filepath.Join(repo, "tracked.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "tracked.txt")
	runGit("commit", "-m", "init")
	if err := os.WriteFile(path, []byte("after\n"), 0644); err != nil {
		t.Fatal(err)
	}

	diff := getLocalDiff(repo)
	if !strings.Contains(diff, "tracked.txt") || !strings.Contains(diff, "+after") {
		t.Fatalf("getLocalDiff did not read repo diff, got:\n%s", diff)
	}
}

func TestResolveReviewRepoRootPrefersEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CC_REVIEW_REPO_ROOT", tmp)
	got := resolveReviewRepoRoot(filepath.Join(t.TempDir(), "controller"))
	if got != tmp {
		t.Fatalf("resolveReviewRepoRoot env = %q, want %q", got, tmp)
	}
}

func TestResolveReviewRepoRootUsesParentGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	controller := filepath.Join(repo, "controller")
	if err := os.MkdirAll(controller, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	got := resolveReviewRepoRoot(controller)
	if got != repo {
		t.Fatalf("resolveReviewRepoRoot = %q, want parent repo %q", got, repo)
	}
}
