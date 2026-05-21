package main

import (
	"strings"
	"testing"
)

func TestFormatReviewMobile(t *testing.T) {
	tests := []struct {
		name     string
		report   *ReviewReport
		preset   string
		wantHas  []string
		wantNot  []string
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
