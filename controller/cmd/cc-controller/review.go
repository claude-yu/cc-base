package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const retryPrompt = "Your previous response was not valid JSON. Please respond with ONLY a JSON object matching the ReviewReport schema. No markdown, no explanation."

func cmdReview(args []string) {
	var backend, file, format string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backend":
			i++
			if i < len(args) {
				backend = args[i]
			}
		case "--file":
			i++
			if i < len(args) {
				file = args[i]
			}
		case "--format":
			i++
			if i < len(args) {
				format = args[i]
			}
		}
	}
	if format == "" {
		format = "json"
	}

	if backend == "" {
		fmt.Fprintln(os.Stderr, "error: --backend is required (deepseek, glm, openai)")
		os.Exit(1)
	}

	be := CodexBackend(backend)
	switch be {
	case CodexBackendDeepSeek, CodexBackendGLM, CodexBackendOpenAI:
	default:
		fmt.Fprintf(os.Stderr, "error: unknown backend %q (valid: deepseek, glm, openai)\n", backend)
		os.Exit(1)
	}

	var diff []byte
	var err error
	if file != "" {
		diff, err = os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot read file %s: %v\n", file, err)
			os.Exit(1)
		}
	} else {
		diff, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot read stdin: %v\n", err)
			os.Exit(1)
		}
	}

	if len(diff) > 64*1024 {
		outputJSON(errorReport(fmt.Sprintf("input too large: %d bytes exceeds 64KB limit", len(diff))))
		os.Exit(1)
	}
	trimmed := strings.TrimSpace(string(diff))
	if trimmed == "" {
		outputJSON(errorReport("empty input: no diff provided"))
		os.Exit(1)
	}

	raw, err := callReviewBackend(trimmed, be)
	if err != nil {
		outputJSON(errorReport(fmt.Sprintf("backend call failed: %v", err)))
		os.Exit(1)
	}

	report, err := extractReviewJSON(raw)
	if err != nil {
		raw2, err2 := callReviewBackendRetry(trimmed, raw, be)
		if err2 != nil {
			outputJSON(errorReport(fmt.Sprintf("retry backend call failed: %v", err2)))
			os.Exit(1)
		}
		report, err = extractReviewJSON(raw2)
		if err != nil {
			outputJSON(errorReport(fmt.Sprintf("cannot parse model output after retry: %v", err)))
			os.Exit(1)
		}
	}
	outputJSON(*report)
}

const reviewSystemPrompt = `You are an independent code reviewer.

Rules:
- Do not trust any executor summary.
- Review only the provided diff.
- Return JSON only.
- Do not include markdown.
- Do not speculate beyond evidence.
- If evidence is insufficient, use WARN or BLOCKED.

Project-specific checks:
- Do not approve incompatible research-monitor JSON schema changes.
- Do not approve broad Chinese routing triggers such as 看看, 结果, 怎么样.
- Do not approve write operations in read-only scientific monitor paths.
- Treat token, credential, and private path leakage as security findings.
- Prefer false negatives over dangerous false positives in execute routing.

Response format (JSON only, no markdown fences):
{
  "verdict": "PASS|WARN|FAIL|BLOCKED|ERROR",
  "summary": "one-line summary",
  "findings": [
    {
      "severity": "CRITICAL|HIGH|MEDIUM|LOW",
      "file": "filename",
      "line": 0,
      "title": "short title",
      "evidence": "what you found",
      "recommendation": "what to do"
    }
  ]
}`

func callReviewBackend(diff string, backend CodexBackend) (string, error) {
	return doReviewCall([]chatMessage{
		{Role: "system", Content: reviewSystemPrompt},
		{Role: "user", Content: diff},
	}, backend)
}

func callReviewBackendRetry(diff, prevResponse string, backend CodexBackend) (string, error) {
	return doReviewCall([]chatMessage{
		{Role: "system", Content: reviewSystemPrompt},
		{Role: "user", Content: diff},
		{Role: "assistant", Content: prevResponse},
		{Role: "user", Content: retryPrompt},
	}, backend)
}

func doReviewCall(messages []chatMessage, backend CodexBackend) (string, error) {
	cfg := resolveAPIConfig(backend)
	if cfg.APIKey == "" {
		return "", fmt.Errorf("CC_CODEX_API_KEY not set for backend %q", backend)
	}

	body, err := json.Marshal(chatRequest{Model: cfg.Model, Messages: messages})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	const maxResponseBytes = 4 * 1024 * 1024
	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if chatResp.Error != nil && chatResp.Error.Message != "" {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("API returned empty choices")
	}
	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

func extractReviewJSON(raw string) (*ReviewReport, error) {
	s := strings.TrimSpace(raw)

	var report ReviewReport
	// 1. Direct unmarshal.
	if json.Unmarshal([]byte(s), &report) == nil {
		if report.Findings == nil {
			report.Findings = []ReviewFinding{}
		}
		return &report, nil
	}
	// 2. Extract from markdown code fence.
	if idx := strings.Index(s, "```"); idx >= 0 {
		after := s[idx+3:]
		// Skip optional language tag on same line.
		if nl := strings.Index(after, "\n"); nl >= 0 {
			after = after[nl+1:]
		}
		if end := strings.Index(after, "```"); end >= 0 {
			if json.Unmarshal([]byte(strings.TrimSpace(after[:end])), &report) == nil {
				if report.Findings == nil {
					report.Findings = []ReviewFinding{}
				}
				return &report, nil
			}
		}
	}
	// 3. First '{' to last '}'.
	first := strings.Index(s, "{")
	last := strings.LastIndex(s, "}")
	if first >= 0 && last > first {
		if json.Unmarshal([]byte(s[first:last+1]), &report) == nil {
			if report.Findings == nil {
				report.Findings = []ReviewFinding{}
			}
			return &report, nil
		}
	}
	return nil, fmt.Errorf("no valid JSON found in model output")
}

func outputJSON(report ReviewReport) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal output: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}
