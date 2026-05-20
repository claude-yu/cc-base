package main

// ReviewVerdict represents the outcome of a code review.
type ReviewVerdict string

const (
	VerdictPass    ReviewVerdict = "PASS"
	VerdictWarn    ReviewVerdict = "WARN"
	VerdictFail    ReviewVerdict = "FAIL"
	VerdictBlocked ReviewVerdict = "BLOCKED"
	VerdictError   ReviewVerdict = "ERROR"
)

// ReviewReport is the top-level structure returned by the review command.
type ReviewReport struct {
	Verdict    ReviewVerdict   `json:"verdict"`
	Summary    string          `json:"summary"`
	Findings   []ReviewFinding `json:"findings"`
	TokensUsed int             `json:"tokens_used,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// ReviewFinding describes a single issue found during review.
type ReviewFinding struct {
	Severity       string `json:"severity"`       // CRITICAL / HIGH / MEDIUM / LOW
	File           string `json:"file,omitempty"`
	Line           int    `json:"line,omitempty"`
	Title          string `json:"title"`
	Evidence       string `json:"evidence"`
	Recommendation string `json:"recommendation"`
}

// errorReport creates a ReviewReport representing an error condition.
// Findings is initialized to an empty slice so JSON output renders [] not null.
func errorReport(msg string) ReviewReport {
	return ReviewReport{
		Verdict:  VerdictError,
		Summary:  msg,
		Findings: []ReviewFinding{},
		Error:    msg,
	}
}
