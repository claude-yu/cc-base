package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractReviewJSON(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantVerdict     ReviewVerdict
		wantSummary     string
		wantFindingsLen int
		wantSeverity    string // first finding severity, if any
		wantErr         bool
	}{
		{
			name:            "bare_json",
			input:           `{"verdict":"PASS","summary":"clean diff","findings":[]}`,
			wantVerdict:     VerdictPass,
			wantSummary:     "clean diff",
			wantFindingsLen: 0,
		},
		{
			name: "json_code_fence",
			input: "```json\n" +
				`{"verdict":"WARN","summary":"minor issue","findings":[{"severity":"LOW","title":"style","evidence":"tabs","recommendation":"use spaces"}]}` +
				"\n```",
			wantVerdict:     VerdictWarn,
			wantFindingsLen: 1,
			wantSeverity:    "LOW",
		},
		{
			name: "generic_code_fence",
			input: "Here is the review:\n```\n" +
				`{"verdict":"FAIL","summary":"security issue","findings":[{"severity":"CRITICAL","title":"leak","evidence":"token in diff","recommendation":"remove"}]}` +
				"\n```",
			wantVerdict:     VerdictFail,
			wantFindingsLen: 1,
			wantSeverity:    "CRITICAL",
		},
		{
			name:            "json_with_prefix",
			input:           "Here is my review:\n" + `{"verdict":"PASS","summary":"looks good","findings":[]}`,
			wantVerdict:     VerdictPass,
			wantSummary:     "looks good",
			wantFindingsLen: 0,
		},
		{
			name:            "null_findings_becomes_empty",
			input:           `{"verdict":"PASS","summary":"ok","findings":null}`,
			wantVerdict:     VerdictPass,
			wantSummary:     "ok",
			wantFindingsLen: 0,
		},
		{
			name:    "invalid_json",
			input:   "not json at all",
			wantErr: true,
		},
		{
			name:    "empty_input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractReviewJSON(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("[%s] expected error, got nil", tc.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("[%s] unexpected error: %v", tc.name, err)
			}
			if got.Verdict != tc.wantVerdict {
				t.Errorf("[%s] verdict = %q, want %q", tc.name, got.Verdict, tc.wantVerdict)
			}
			if tc.wantSummary != "" && got.Summary != tc.wantSummary {
				t.Errorf("[%s] summary = %q, want %q", tc.name, got.Summary, tc.wantSummary)
			}
			if got.Findings == nil {
				t.Fatalf("[%s] findings is nil, want non-nil slice", tc.name)
			}
			if len(got.Findings) != tc.wantFindingsLen {
				t.Errorf("[%s] len(findings) = %d, want %d", tc.name, len(got.Findings), tc.wantFindingsLen)
			}
			if tc.wantSeverity != "" && len(got.Findings) > 0 {
				if got.Findings[0].Severity != tc.wantSeverity {
					t.Errorf("[%s] findings[0].severity = %q, want %q", tc.name, got.Findings[0].Severity, tc.wantSeverity)
				}
			}
		})
	}
}

func TestErrorReport(t *testing.T) {
	r := errorReport("test error")
	if r.Verdict != VerdictError {
		t.Errorf("verdict = %q, want %q", r.Verdict, VerdictError)
	}
	if r.Error != "test error" {
		t.Errorf("error = %q, want %q", r.Error, "test error")
	}
	if r.Summary != "test error" {
		t.Errorf("summary = %q, want %q", r.Summary, "test error")
	}
	if r.Findings == nil {
		t.Fatal("findings is nil, want empty slice")
	}
	if len(r.Findings) != 0 {
		t.Errorf("len(findings) = %d, want 0", len(r.Findings))
	}
}

func TestReviewReportJSON(t *testing.T) {
	original := ReviewReport{
		Verdict: VerdictWarn,
		Summary: "one issue found",
		Findings: []ReviewFinding{
			{
				Severity:       "HIGH",
				File:           "main.go",
				Line:           42,
				Title:          "hardcoded secret",
				Evidence:       "token = \"abc123\"",
				Recommendation: "use environment variable",
			},
		},
		TokensUsed: 500,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Verify JSON contains expected keys
	s := string(data)
	for _, key := range []string{"verdict", "summary", "findings", "tokens_used"} {
		if !strings.Contains(s, key) {
			t.Errorf("marshaled JSON missing key %q", key)
		}
	}
	// error field should be omitted (empty)
	if strings.Contains(s, `"error"`) {
		t.Error("marshaled JSON contains error key, expected omitempty to drop it")
	}

	var roundtrip ReviewReport
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if roundtrip.Verdict != original.Verdict {
		t.Errorf("verdict = %q, want %q", roundtrip.Verdict, original.Verdict)
	}
	if roundtrip.Summary != original.Summary {
		t.Errorf("summary = %q, want %q", roundtrip.Summary, original.Summary)
	}
	if len(roundtrip.Findings) != 1 {
		t.Fatalf("len(findings) = %d, want 1", len(roundtrip.Findings))
	}
	f := roundtrip.Findings[0]
	if f.Severity != "HIGH" || f.File != "main.go" || f.Line != 42 {
		t.Errorf("finding mismatch: got severity=%q file=%q line=%d", f.Severity, f.File, f.Line)
	}
	if f.Title != "hardcoded secret" {
		t.Errorf("finding title = %q, want %q", f.Title, "hardcoded secret")
	}
	if roundtrip.TokensUsed != 500 {
		t.Errorf("tokens_used = %d, want 500", roundtrip.TokensUsed)
	}
}
