package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsProjectContextQuery(t *testing.T) {
	positive := []string{
		"帮我总结一下当前项目状态",
		"分析一下现在项目进度",
		"当前计划是什么",
		"下一步做什么",
		"项目总结",
		"工作重点是什么",
		"总结一下工作进展",
		"接下来做什么",
	}
	negative := []string{
		"状态", "进度",
		"帮我改这个文件",
		"这个bug怎么修",
		"运行命令",
		"看看结果",
		"科研任务",
		"系统状态",
		"你好",
		"什么是分子动力学",
		"GSEA结果怎么看",
		"帮我写个md文件",
	}
	for _, s := range positive {
		if !isProjectContextQuery(s) {
			t.Errorf("isProjectContextQuery(%q) = false, want true", s)
		}
	}
	for _, s := range negative {
		if isProjectContextQuery(s) {
			t.Errorf("isProjectContextQuery(%q) = true, want false", s)
		}
	}
}

func TestExtractSection(t *testing.T) {
	content := `# Project

## TODO
- [P1] Task A
- [P2] Task B

## In Progress
Nothing right now.

## Done
- Task C completed
`
	todo := extractSection(content, "TODO")
	if !strings.Contains(todo, "Task A") || !strings.Contains(todo, "Task B") {
		t.Errorf("extractSection TODO = %q, expected Task A and B", todo)
	}
	if strings.Contains(todo, "Nothing") {
		t.Error("extractSection TODO leaked into In Progress")
	}

	inProgress := extractSection(content, "In Progress")
	if !strings.Contains(inProgress, "Nothing right now") {
		t.Errorf("extractSection In Progress = %q", inProgress)
	}

	missing := extractSection(content, "Nonexistent")
	if missing != "" {
		t.Errorf("extractSection Nonexistent = %q, want empty", missing)
	}
}

func TestExtractSection_Nested(t *testing.T) {
	content := `## Near-Term Roadmap

### P0: Binding
Add bindings.

### P1: Callback
Add callback.

## Deferred
Not now.
`
	roadmap := extractSection(content, "Near-Term Roadmap")
	if !strings.Contains(roadmap, "Add bindings") {
		t.Error("missing P0 content")
	}
	if !strings.Contains(roadmap, "Add callback") {
		t.Error("missing P1 content")
	}
	if strings.Contains(roadmap, "Not now") {
		t.Error("leaked Deferred section")
	}
}

func TestMarkdownHeadingLevel(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"# H1", 1},
		{"## H2", 2},
		{"### H3", 3},
		{"not a heading", 0},
		{"#nospace", 0},
		{"", 0},
	}
	for _, c := range cases {
		if got := markdownHeadingLevel(c.line); got != c.want {
			t.Errorf("markdownHeadingLevel(%q) = %d, want %d", c.line, got, c.want)
		}
	}
}

func TestFindLatestHandoff(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "handoff-2026-05-18.md"), []byte("old"), 0644)
	os.WriteFile(filepath.Join(dir, "handoff-2026-05-20.md"), []byte("new"), 0644)

	got := findLatestHandoff(dir)
	if filepath.Base(got) != "handoff-2026-05-20.md" {
		t.Errorf("findLatestHandoff = %q, want handoff-2026-05-20.md", filepath.Base(got))
	}
}

func TestFindLatestHandoff_None(t *testing.T) {
	dir := t.TempDir()
	if got := findLatestHandoff(dir); got != "" {
		t.Errorf("findLatestHandoff empty = %q, want empty", got)
	}
}

func TestFindAndRead(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir2, "test.md"), []byte("found in dir2"), 0644)

	got := findAndRead("test.md", dir1, dir2)
	if got != "found in dir2" {
		t.Errorf("findAndRead = %q, want 'found in dir2'", got)
	}

	got2 := findAndRead("missing.md", dir1, dir2)
	if got2 != "" {
		t.Errorf("findAndRead missing = %q, want empty", got2)
	}
}

func TestFindAndRead_FirstWins(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "test.md"), []byte("from dir1"), 0644)
	os.WriteFile(filepath.Join(dir2, "test.md"), []byte("from dir2"), 0644)

	got := findAndRead("test.md", dir1, dir2)
	if got != "from dir1" {
		t.Errorf("findAndRead should prefer first dir, got %q", got)
	}
}

func TestCapLines(t *testing.T) {
	s := "line1\nline2\nline3\nline4\nline5"
	got := capLines(s, 3)
	if got != "line1\nline2\nline3\n..." {
		t.Errorf("capLines(5, 3) = %q", got)
	}
	if capLines(s, 10) != s {
		t.Error("capLines should return original when under limit")
	}
}

// Full priority chain: plan + progress + handoff all present
func TestBuildProjectMemoryContext_FullChain(t *testing.T) {
	dir := t.TempDir()
	controllerDir := filepath.Join(dir, "controller")
	os.MkdirAll(controllerDir, 0755)

	os.WriteFile(filepath.Join(dir, "mobile-agent-platform-plan.md"), []byte(`# Plan

## Current Decision
Do not migrate to Hermes immediately. Keep cc-base stable.

## Near-Term Roadmap

### P0: Binding
Add bindings.

### P1: Callback
Add callback.

## Deferred
Not now.
`), 0644)

	os.WriteFile(filepath.Join(dir, "progress.md"), []byte(`# Project

## TODO
- [P1][OPEN] Task A
- [P2][OPEN] Task B

## In Progress
（无）

## Done
- Task C
`), 0644)

	os.WriteFile(filepath.Join(dir, "handoff-2026-05-20.md"), []byte(`# Handoff

## 当前状态
- research-monitor v1.1 done
- cc-connect fork v3 deployed

## 下一步建议
1. Old suggestion from handoff
`), 0644)

	result := buildProjectMemoryContext(controllerDir, `G:\test\work`)

	if !strings.Contains(result, "controller_root: "+controllerDir) {
		t.Error("missing controller_root")
	}
	if !strings.Contains(result, `work_dir: G:\test\work`) {
		t.Error("missing work_dir")
	}

	// Plan priorities first and authoritative with freshness label
	if !strings.Contains(result, "weight: authoritative") {
		t.Error("plan should have authoritative weight label")
	}
	if !strings.Contains(result, "Do not migrate to Hermes") {
		t.Error("missing platform decision")
	}
	if !strings.Contains(result, "Add bindings") {
		t.Error("missing roadmap P0")
	}
	if !strings.Contains(result, "Add callback") {
		t.Error("missing roadmap P1")
	}
	if strings.Contains(result, "Not now") {
		t.Error("Deferred should not leak")
	}

	// TODO from progress with freshness label
	if !strings.Contains(result, "weight: current") {
		t.Error("TODO should have current weight label")
	}
	if !strings.Contains(result, "Task A") {
		t.Error("missing TODO")
	}

	// Handoff state (historical) with freshness label
	if !strings.Contains(result, "weight: historical") {
		t.Error("handoff should have historical weight label")
	}
	if !strings.Contains(result, "research-monitor v1.1 done") {
		t.Error("missing handoff state")
	}
	if !strings.Contains(result, "Previous Session") {
		t.Error("handoff should be labeled as previous session")
	}

	// Handoff next steps excluded
	if strings.Contains(result, "Old suggestion from handoff") {
		t.Error("handoff next steps should be excluded")
	}

	// Empty In Progress skipped
	if strings.Contains(result, "（无）") {
		t.Error("should skip empty In Progress")
	}

	// Verify ordering: plan → TODO → handoff
	planIdx := strings.Index(result, "Current Priorities")
	todoIdx := strings.Index(result, "Active TODO")
	handoffIdx := strings.Index(result, "Previous Session")
	if planIdx >= todoIdx {
		t.Error("plan should appear before TODO")
	}
	if todoIdx >= handoffIdx {
		t.Error("TODO should appear before handoff")
	}
}

// Priority 4: Only CLAUDE.md/README.md when no plan/progress/handoff
func TestBuildProjectMemoryContext_FallbackDocs(t *testing.T) {
	dir := t.TempDir()
	controllerDir := filepath.Join(dir, "controller")
	os.MkdirAll(controllerDir, 0755)

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# My Project\nA scientific workflow controller."), 0644)

	result := buildProjectMemoryContext(controllerDir, ".")
	if !strings.Contains(result, "My Project") {
		t.Error("should fall back to README.md")
	}
	if !strings.Contains(result, "weight: fallback") {
		t.Error("README fallback should have fallback weight label")
	}
	// Should NOT contain default context since README was found
	if strings.Contains(result, "mobile-controlled local Agent") {
		t.Error("should not inject default context when README exists")
	}
}

// Priority 5: Default context when nothing exists
func TestBuildProjectMemoryContext_DefaultContext(t *testing.T) {
	dir := t.TempDir()
	controllerDir := filepath.Join(dir, "controller")
	os.MkdirAll(controllerDir, 0755)

	result := buildProjectMemoryContext(controllerDir, ".")
	if !strings.Contains(result, "mobile-controlled local Agent") {
		t.Error("should inject default context when no files found")
	}
	if !strings.Contains(result, "controller_root:") {
		t.Error("should still have path references")
	}
}

// Priority 5 override: custom template takes precedence over built-in default
func TestBuildProjectMemoryContext_CustomTemplate(t *testing.T) {
	dir := t.TempDir()
	controllerDir := filepath.Join(dir, "controller")
	templatesDir := filepath.Join(controllerDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	os.WriteFile(filepath.Join(templatesDir, "default-project-context.md"),
		[]byte("Custom context for this deployment."), 0644)

	result := buildProjectMemoryContext(controllerDir, ".")
	if !strings.Contains(result, "Custom context for this deployment") {
		t.Error("should use custom template")
	}
	if strings.Contains(result, "mobile-controlled local Agent") {
		t.Error("should not use built-in default when custom exists")
	}
}

// Handoff searched in both projectRoot and root
func TestBuildProjectMemoryContext_HandoffInControllerDir(t *testing.T) {
	dir := t.TempDir()
	controllerDir := filepath.Join(dir, "controller")
	os.MkdirAll(controllerDir, 0755)

	// No handoff in projectRoot, but one in controllerDir
	os.WriteFile(filepath.Join(controllerDir, "handoff-2026-05-20.md"), []byte(`# Handoff

## Current Status
- Pipeline v2 deployed
`), 0644)

	result := buildProjectMemoryContext(controllerDir, ".")
	if !strings.Contains(result, "Pipeline v2 deployed") {
		t.Error("should find handoff in controller dir")
	}
}

// Dedup: same dir passed twice should not cause double search
func TestFindLatestHandoffDedup(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "handoff-2026-05-20.md"), []byte("content"), 0644)

	got := findLatestHandoffDedup(dir, dir)
	if filepath.Base(got) != "handoff-2026-05-20.md" {
		t.Errorf("dedup = %q, want handoff-2026-05-20.md", filepath.Base(got))
	}
}

func TestFindLatestHandoffDedup_SecondDir(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir2, "handoff-2026-05-19.md"), []byte("in dir2"), 0644)

	got := findLatestHandoffDedup(dir1, dir2)
	if filepath.Base(got) != "handoff-2026-05-19.md" {
		t.Errorf("dedup fallback = %q", filepath.Base(got))
	}
}

// English heading fallback for handoff
func TestBuildProjectMemoryContext_EnglishHandoff(t *testing.T) {
	dir := t.TempDir()
	controllerDir := filepath.Join(dir, "controller")
	os.MkdirAll(controllerDir, 0755)

	os.WriteFile(filepath.Join(dir, "handoff-2026-05-20.md"), []byte(`# Handoff

## Current Status
- API v3 launched
`), 0644)

	result := buildProjectMemoryContext(controllerDir, ".")
	if !strings.Contains(result, "API v3 launched") {
		t.Error("should find English 'Current Status' heading")
	}
}
