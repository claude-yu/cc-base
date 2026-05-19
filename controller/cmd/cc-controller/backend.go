package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CodexBackend represents which implementation backs the Codex role.
type CodexBackend string

const (
	CodexBackendNative   CodexBackend = "native_codex"
	CodexBackendOpenAI   CodexBackend = "openai"
	CodexBackendDeepSeek CodexBackend = "deepseek"
	CodexBackendGLM      CodexBackend = "glm"
)

// apiConfig holds resolved API parameters for non-native backends.
type apiConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// resolveCodexBackend reads CC_CODEX_BACKEND and returns the selected backend.
// Empty or "native_codex" → CodexBackendNative. Unknown value → fatal exit.
func resolveCodexBackend() CodexBackend {
	v := strings.TrimSpace(os.Getenv("CC_CODEX_BACKEND"))
	if v == "" || v == string(CodexBackendNative) {
		return CodexBackendNative
	}
	switch CodexBackend(v) {
	case CodexBackendOpenAI, CodexBackendDeepSeek, CodexBackendGLM:
		return CodexBackend(v)
	default:
		fmt.Fprintf(os.Stderr, "FATAL: unknown CC_CODEX_BACKEND=%q (valid: native_codex, openai, deepseek, glm)\n", v)
		os.Exit(1)
		return "" // unreachable
	}
}

// resolveAPIConfig returns base URL, API key, and model for the given backend.
// Env vars CC_CODEX_API_BASE / CC_CODEX_API_KEY / CC_CODEX_MODEL override defaults.
func resolveAPIConfig(backend CodexBackend) apiConfig {
	var defaultBase, defaultModel string
	switch backend {
	case CodexBackendOpenAI:
		defaultBase = "https://api.openai.com/v1"
		defaultModel = "gpt-4o"
	case CodexBackendDeepSeek:
		defaultBase = "https://api.deepseek.com/v1"
		defaultModel = "deepseek-chat"
	case CodexBackendGLM:
		defaultBase = "https://open.bigmodel.cn/api/paas/v4"
		defaultModel = "glm-4"
	}

	base := os.Getenv("CC_CODEX_API_BASE")
	if base == "" {
		base = defaultBase
	}
	key := os.Getenv("CC_CODEX_API_KEY")
	model := os.Getenv("CC_CODEX_MODEL")
	if model == "" {
		model = defaultModel
	}
	return apiConfig{BaseURL: base, APIKey: key, Model: model}
}

// chatRequest / chatResponse mirror the minimal OpenAI-compatible chat schema.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// runAPICodex calls an OpenAI-compatible chat completions endpoint and writes
// the same file structure that the native codex path produces.
func runAPICodex(runDir, runID, question string, cfg apiConfig, backend CodexBackend) {
	// Loud fail: missing API key.
	if cfg.APIKey == "" {
		errMsg := fmt.Sprintf("CC_CODEX_API_KEY not set for backend '%s'", backend)
		os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(errMsg), 0644)
		updateStatusJSON(runDir, "failed", "failed", 0)
		setExitCode(runDir, 1)
		appendEvent(runDir, eventEntry{
			Ts:    time.Now().UTC().Format(time.RFC3339),
			RunID: runID, Type: "failed", ExitCode: 1,
			Message: errMsg,
		})
		sendCallback(runDir, fmt.Sprintf("[Codex role: %s] 调用失败 (RunId: %s)\n%s", backend, runID, errMsg))
		return
	}

	// Build system prompt (same as native codex).
	systemPrompt := `Reply in the same language as the user's question. If the question contains Chinese, use Simplified Chinese.

You are acting as an independent technical advisor (Codex role).
Answer the user's question directly based on your knowledge.

Core rules:
- 拒绝废话：直接给结论，不解释环境或定义
- 简洁优先：用最少的输出解决问题，不要堆砌
- 目标驱动：先确定要回答什么，再组织输出
- 大声失败：遇到不确定的就说不知道，不编造

At the end of your answer, include a "建议下一步" section:

## 建议下一步
- P1 ...
- P2 ...
- P3 ...

If no action is needed, write:
- 无需后续操作。`

	reqBody := chatRequest{
		Model: cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: question},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		writeAPIError(runDir, runID, backend, fmt.Sprintf("JSON marshal error: %v", err))
		return
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequest("POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		writeAPIError(runDir, runID, backend, fmt.Sprintf("HTTP request build error: %v", err))
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	// Heartbeat goroutine.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		start := time.Now()
		for {
			select {
			case <-ticker.C:
				elapsed := int(time.Since(start).Seconds())
				appendEvent(runDir, eventEntry{
					Ts:         time.Now().UTC().Format(time.RFC3339),
					RunID:      runID,
					Type:       "heartbeat",
					Stage:      "api_codex_running",
					ElapsedSec: elapsed,
				})
				sendCallback(runDir, heartbeatMsg(fmt.Sprintf("Codex(%s)", backend), runID, elapsed, ""))
			case <-done:
				return
			}
		}
	}()

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(httpReq)
	close(done)

	if err != nil {
		writeAPIError(runDir, runID, backend, fmt.Sprintf("HTTP request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		writeAPIError(runDir, runID, backend, fmt.Sprintf("read response body failed: %v", err))
		return
	}

	// Write raw response for debugging.
	os.WriteFile(filepath.Join(runDir, "codex-answer.raw.md"), respBytes, 0644)

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("API returned HTTP %d: %s", resp.StatusCode, string(respBytes))
		writeAPIError(runDir, runID, backend, errMsg)
		return
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		writeAPIError(runDir, runID, backend, fmt.Sprintf("JSON parse error: %v\nBody: %s", err, string(respBytes)))
		return
	}

	if chatResp.Error != nil && chatResp.Error.Message != "" {
		writeAPIError(runDir, runID, backend, fmt.Sprintf("API error: %s", chatResp.Error.Message))
		return
	}

	if len(chatResp.Choices) == 0 {
		writeAPIError(runDir, runID, backend, "API returned empty choices")
		return
	}

	answer := strings.TrimSpace(chatResp.Choices[0].Message.Content)

	// Write the same file set as the native codex path.
	os.WriteFile(filepath.Join(runDir, "codex-answer.md"), []byte(answer), 0644)
	os.WriteFile(filepath.Join(runDir, "codex-answer.exitcode.txt"), []byte("0"), 0644)
	os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(answer), 0644)
	updateStatusJSON(runDir, "completed", "done", 0)
	setExitCode(runDir, 0)
	appendEvent(runDir, eventEntry{
		Ts:    time.Now().UTC().Format(time.RFC3339),
		RunID: runID, Type: "completed", ExitCode: 0,
	})
	sendCallback(runDir, fmt.Sprintf("[Codex role: %s] 已回复 (RunId: %s)\n%s", backend, runID, answer))
}

// writeAPIError writes a failure to runDir and sends callback — used by runAPICodex.
func writeAPIError(runDir, runID string, backend CodexBackend, errMsg string) {
	os.WriteFile(filepath.Join(runDir, "codex-answer.md"), []byte(errMsg), 0644)
	os.WriteFile(filepath.Join(runDir, "codex-answer.exitcode.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(errMsg), 0644)
	updateStatusJSON(runDir, "failed", "failed", 0)
	setExitCode(runDir, 1)
	appendEvent(runDir, eventEntry{
		Ts:    time.Now().UTC().Format(time.RFC3339),
		RunID: runID, Type: "failed", ExitCode: 1,
		Message: errMsg,
	})
	sendCallback(runDir, fmt.Sprintf("[Codex role: %s] 调用失败 (RunId: %s)\n%s", backend, runID, errMsg))
}
