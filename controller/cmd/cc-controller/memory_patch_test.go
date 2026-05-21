package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindMemoryFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "feedback_proxy-isolation.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(dir, "project_dlg1-virtual-ko.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(dir, "memory-maintenance-policy.md"), []byte("test"), 0644)

	tests := []struct {
		name    string
		wantHit bool
	}{
		{"proxy-isolation", true},
		{"dlg1-virtual-ko", true},
		{"memory-maintenance-policy", true},
		{"nonexistent", false},
	}
	for _, tt := range tests {
		got := findMemoryFile(dir, tt.name)
		if tt.wantHit && got == "" {
			t.Errorf("findMemoryFile(%q) = empty, want hit", tt.name)
		}
		if !tt.wantHit && got != "" {
			t.Errorf("findMemoryFile(%q) = %q, want empty", tt.name, got)
		}
	}
}

func TestExtractPatchContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"markdown block",
			"Some intro\n```markdown\n---\nname: test\n---\nBody\n```\nDone.",
			"---\nname: test\n---\nBody",
		},
		{
			"md block",
			"```md\n---\nname: x\n---\nContent\n```",
			"---\nname: x\n---\nContent",
		},
		{
			"plain code block",
			"```\n---\nname: y\n---\nStuff\n```",
			"---\nname: y\n---\nStuff",
		},
		{
			"raw frontmatter",
			"---\nname: z\n---\nDirect content",
			"---\nname: z\n---\nDirect content",
		},
		{
			"no content",
			"Just a chat response with no code block.",
			"",
		},
	}
	for _, tt := range tests {
		got := extractPatchContent(tt.input)
		if got != tt.want {
			t.Errorf("extractPatchContent(%s) =\n%q\nwant\n%q", tt.name, got, tt.want)
		}
	}
}

func TestExtractVerdict(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"判定: APPROVE\n理由: 保留完整", "approved"},
		{"判定: REJECT\n理由: 丢失了关键约束", "rejected"},
		{"Verdict: NEEDS_REVISION\nReason: needs work", "needs_revision"},
		{"Decision: 通过\n信息完整", "approved"},
		{"判定: 拒绝\n缺少 Why 段落", "rejected"},
		{"判定: 不通过", "rejected"},
		// Ambiguous: no structured verdict line → needs_revision
		{"This patch looks good, I approve of it", "needs_revision"},
		{"do not approve this change", "needs_revision"},
		{"Some ambiguous response", "needs_revision"},
	}
	for _, tt := range tests {
		got := extractVerdict(tt.input)
		if got != tt.want {
			t.Errorf("extractVerdict(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCoreGovernanceGuard(t *testing.T) {
	for _, name := range []string{"proxy-isolation", "skill-first-workflow"} {
		if !coreMemoryNames[name] {
			t.Errorf("%q should be core", name)
		}
	}
	if !governanceNames["memory-maintenance-policy"] {
		t.Error("memory-maintenance-policy should be governance")
	}
}

func TestPatchPrompts(t *testing.T) {
	sys := patchSystemPrompt()
	if !strings.Contains(sys, "frontmatter") {
		t.Error("system prompt should mention frontmatter preservation")
	}
	if !strings.Contains(sys, "Why") {
		t.Error("system prompt should mention Why preservation")
	}

	user := patchUserPrompt("test", "feedback", "controller", 45, "content here")
	if !strings.Contains(user, "test") {
		t.Error("user prompt should contain target name")
	}
	if !strings.Contains(user, "45") {
		t.Error("user prompt should contain line count")
	}
}

func TestReviewPrompts(t *testing.T) {
	sys := reviewPatchSystemPrompt()
	if !strings.Contains(sys, "APPROVE") {
		t.Error("review prompt should mention APPROVE")
	}
	if !strings.Contains(sys, "判定:") {
		t.Error("review prompt should specify output format with 判定:")
	}

	user := reviewPatchUserPrompt("target", "old content", "new content")
	if !strings.Contains(user, "old content") || !strings.Contains(user, "new content") {
		t.Error("review prompt should contain both old and new content")
	}
}

// --- validatePatchFrontmatter tests ---

func TestValidateFrontmatter_OK(t *testing.T) {
	original := "---\nname: test-target\ndescription: X\nmetadata:\n  type: project\n---\nOld body.\n"
	proposed := "---\nname: test-target\ndescription: X updated\nmetadata:\n  type: project\n---\nNew body.\n"
	if err := validatePatchFrontmatter(original, proposed, "test-target"); err != nil {
		t.Errorf("expected ok, got: %v", err)
	}
}

func TestValidateFrontmatter_NameChanged(t *testing.T) {
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\n"
	proposed := "---\nname: different-name\nmetadata:\n  type: project\n---\n"
	err := validatePatchFrontmatter(original, proposed, "test-target")
	if err == nil {
		t.Fatal("expected error for name change")
	}
	if !strings.Contains(err.Error(), "name 被修改") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestValidateFrontmatter_NameMismatchTarget(t *testing.T) {
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\n"
	proposed := "---\nname: test-target\nmetadata:\n  type: project\n---\n"
	err := validatePatchFrontmatter(original, proposed, "wrong-target")
	if err == nil {
		t.Fatal("expected error for target mismatch")
	}
	if !strings.Contains(err.Error(), "name 与目标不匹配") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestValidateFrontmatter_TypeChanged(t *testing.T) {
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\n"
	proposed := "---\nname: test-target\nmetadata:\n  type: feedback\n---\n"
	err := validatePatchFrontmatter(original, proposed, "test-target")
	if err == nil {
		t.Fatal("expected error for type change")
	}
	if !strings.Contains(err.Error(), "metadata.type 被修改") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestValidateFrontmatter_TypeDeleted(t *testing.T) {
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\n"
	proposed := "---\nname: test-target\nmetadata:\n  node_type: memory\n---\n"
	err := validatePatchFrontmatter(original, proposed, "test-target")
	if err == nil {
		t.Fatal("expected error for type deletion")
	}
	if !strings.Contains(err.Error(), "metadata.type") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestValidateFrontmatter_MissingFrontmatter(t *testing.T) {
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\n"
	proposed := "No frontmatter here."
	err := validatePatchFrontmatter(original, proposed, "test-target")
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestValidateFrontmatter_LayerEscalation(t *testing.T) {
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\n"
	proposed := "---\nname: test-target\nmetadata:\n  type: project\n  layer: core\n---\n"
	err := validatePatchFrontmatter(original, proposed, "test-target")
	if err == nil {
		t.Fatal("expected error for layer escalation")
	}
	if !strings.Contains(err.Error(), "layer 提升") {
		t.Errorf("wrong error: %v", err)
	}
}

// --- applyMemoryPatch integration tests ---

func setupApplyRun(t *testing.T, target, memDir, originalContent, newContent, status string) string {
	t.Helper()
	runDir := filepath.Join(t.TempDir(), "run-test")
	os.MkdirAll(runDir, 0755)

	os.WriteFile(filepath.Join(runDir, "target-name.txt"), []byte(target), 0644)
	os.WriteFile(filepath.Join(runDir, "memory-dir.txt"), []byte(memDir), 0644)
	os.WriteFile(filepath.Join(runDir, "patch-new.md"), []byte(newContent), 0600)
	os.WriteFile(filepath.Join(runDir, "target-original.md"), []byte(originalContent), 0600)

	st := statusJSON{RunID: "test", Kind: "memory-patch", Status: "reviewed", Stage: status}
	data, _ := json.Marshal(st)
	os.WriteFile(filepath.Join(runDir, "status.json"), data, 0644)

	return runDir
}

func TestApply_RequiresApproval(t *testing.T) {
	memDir := t.TempDir()
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\nOld.\n"
	proposed := "---\nname: test-target\nmetadata:\n  type: project\n---\nNew.\n"
	os.WriteFile(filepath.Join(memDir, "project_test-target.md"), []byte(original), 0644)

	runDir := setupApplyRun(t, "test-target", memDir, original, proposed, "needs_revision")
	err := applyMemoryPatch(runDir)
	if err == nil {
		t.Fatal("expected error: patch not approved")
	}
	if !strings.Contains(err.Error(), "未通过审查") {
		t.Errorf("wrong error: %v", err)
	}

	// File should be unchanged
	data, _ := os.ReadFile(filepath.Join(memDir, "project_test-target.md"))
	if string(data) != original {
		t.Error("file was modified despite unapproved patch")
	}
}

func TestApply_RejectsUnapproved(t *testing.T) {
	memDir := t.TempDir()
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\nOld.\n"
	proposed := "---\nname: test-target\nmetadata:\n  type: project\n---\nNew.\n"
	os.WriteFile(filepath.Join(memDir, "project_test-target.md"), []byte(original), 0644)

	for _, stage := range []string{"rejected", "pending_review", "awaiting review", "generating"} {
		runDir := setupApplyRun(t, "test-target", memDir, original, proposed, stage)
		err := applyMemoryPatch(runDir)
		if err == nil {
			t.Fatalf("expected error for stage=%q", stage)
		}
		if !strings.Contains(err.Error(), "未通过审查") {
			t.Errorf("stage=%q: wrong error: %v", stage, err)
		}
	}
}

func TestApply_SuccessWithApproval(t *testing.T) {
	memDir := t.TempDir()
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\nOld content.\n"
	proposed := "---\nname: test-target\nmetadata:\n  type: project\n---\nCleaner content.\n"
	os.WriteFile(filepath.Join(memDir, "project_test-target.md"), []byte(original), 0644)

	runDir := setupApplyRun(t, "test-target", memDir, original, proposed, "approved")
	err := applyMemoryPatch(runDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should be updated
	data, _ := os.ReadFile(filepath.Join(memDir, "project_test-target.md"))
	if string(data) != proposed {
		t.Error("file was not updated")
	}

	// Backup should exist
	backups, _ := os.ReadDir(filepath.Join(memDir, ".backups"))
	if len(backups) == 0 {
		t.Error("no backup created")
	}

	// Audit log should exist
	auditData, _ := os.ReadFile(filepath.Join(memDir, ".audit-log.jsonl"))
	if !strings.Contains(string(auditData), "test-target") {
		t.Error("audit log missing entry")
	}

	// Lock should be cleaned up
	if _, err := os.Stat(filepath.Join(memDir, ".memory.lock")); err == nil {
		t.Error("lock file not removed")
	}
}

func TestApply_RejectsCoreMemory(t *testing.T) {
	memDir := t.TempDir()
	original := "---\nname: proxy-isolation\nmetadata:\n  type: feedback\n---\nContent.\n"
	os.WriteFile(filepath.Join(memDir, "feedback_proxy-isolation.md"), []byte(original), 0644)

	runDir := setupApplyRun(t, "proxy-isolation", memDir, original, original, "approved")
	err := applyMemoryPatch(runDir)
	if err == nil {
		t.Fatal("expected error for core memory")
	}
	if !strings.Contains(err.Error(), "受保护") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestApply_RejectsFrontmatterTamper(t *testing.T) {
	memDir := t.TempDir()
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\nOld.\n"
	tampered := "---\nname: hijacked\nmetadata:\n  type: project\n---\nNew.\n"
	os.WriteFile(filepath.Join(memDir, "project_test-target.md"), []byte(original), 0644)

	runDir := setupApplyRun(t, "test-target", memDir, original, tampered, "approved")
	err := applyMemoryPatch(runDir)
	if err == nil {
		t.Fatal("expected error for frontmatter tamper")
	}
	if !strings.Contains(err.Error(), "name 被修改") {
		t.Errorf("wrong error: %v", err)
	}

	// File should be unchanged
	data, _ := os.ReadFile(filepath.Join(memDir, "project_test-target.md"))
	if string(data) != original {
		t.Error("file was modified despite frontmatter tamper")
	}
}

func TestApply_RejectsStalePatchWhenTargetChanged(t *testing.T) {
	memDir := t.TempDir()
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\nOld content.\n"
	proposed := "---\nname: test-target\nmetadata:\n  type: project\n---\nCleaner content.\n"
	os.WriteFile(filepath.Join(memDir, "project_test-target.md"), []byte(original), 0644)

	runDir := setupApplyRun(t, "test-target", memDir, original, proposed, "approved")

	// Simulate someone editing the file after patch was generated
	modified := "---\nname: test-target\nmetadata:\n  type: project\n---\nUser edited this after patch generation.\n"
	os.WriteFile(filepath.Join(memDir, "project_test-target.md"), []byte(modified), 0644)

	err := applyMemoryPatch(runDir)
	if err == nil {
		t.Fatal("expected error: target changed since patch generation")
	}
	if !strings.Contains(err.Error(), "已被修改") {
		t.Errorf("wrong error: %v", err)
	}

	// File should still have the user's edit, not the stale patch
	data, _ := os.ReadFile(filepath.Join(memDir, "project_test-target.md"))
	if string(data) != modified {
		t.Error("user's edit was overwritten by stale patch")
	}
}

func TestApply_AtomicLock(t *testing.T) {
	memDir := t.TempDir()
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\nOld.\n"
	proposed := "---\nname: test-target\nmetadata:\n  type: project\n---\nNew.\n"
	os.WriteFile(filepath.Join(memDir, "project_test-target.md"), []byte(original), 0644)

	// Create active lock
	lockJSON, _ := json.Marshal(map[string]interface{}{
		"agent":       "other-writer",
		"operation":   "apply",
		"ttl_seconds": 300,
		"created":     "2099-01-01T00:00:00Z",
	})
	os.WriteFile(filepath.Join(memDir, ".memory.lock"), lockJSON, 0600)

	runDir := setupApplyRun(t, "test-target", memDir, original, proposed, "approved")
	err := applyMemoryPatch(runDir)
	if err == nil {
		t.Fatal("expected error: lock held by other agent")
	}
	if !strings.Contains(err.Error(), "锁定中") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestApply_ExpiredLockIsReplaced(t *testing.T) {
	memDir := t.TempDir()
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\nOld.\n"
	proposed := "---\nname: test-target\nmetadata:\n  type: project\n---\nNew.\n"
	os.WriteFile(filepath.Join(memDir, "project_test-target.md"), []byte(original), 0644)

	// Create expired lock
	lockJSON, _ := json.Marshal(map[string]interface{}{
		"agent":       "old-writer",
		"operation":   "apply",
		"ttl_seconds": 1,
		"created":     "2020-01-01T00:00:00Z",
	})
	os.WriteFile(filepath.Join(memDir, ".memory.lock"), lockJSON, 0600)

	runDir := setupApplyRun(t, "test-target", memDir, original, proposed, "approved")
	err := applyMemoryPatch(runDir)
	if err != nil {
		t.Fatalf("expired lock should not block: %v", err)
	}
}

func TestApply_AuditFailureMarksStatus(t *testing.T) {
	memDir := t.TempDir()
	original := "---\nname: test-target\nmetadata:\n  type: project\n---\nOld.\n"
	proposed := "---\nname: test-target\nmetadata:\n  type: project\n---\nNew.\n"
	os.WriteFile(filepath.Join(memDir, "project_test-target.md"), []byte(original), 0644)

	// Make .audit-log.jsonl a directory so writes fail
	os.MkdirAll(filepath.Join(memDir, ".audit-log.jsonl"), 0755)

	runDir := setupApplyRun(t, "test-target", memDir, original, proposed, "approved")
	err := applyMemoryPatch(runDir)
	if err != nil {
		t.Fatalf("audit failure should not block apply: %v", err)
	}

	// File should still be updated
	data, _ := os.ReadFile(filepath.Join(memDir, "project_test-target.md"))
	if string(data) != proposed {
		t.Error("file was not updated despite audit failure")
	}

	// Status should reflect audit failure
	statusData, _ := os.ReadFile(filepath.Join(runDir, "status.json"))
	if !strings.Contains(string(statusData), "applied_audit_failed") {
		t.Errorf("status should be applied_audit_failed, got: %s", string(statusData))
	}
}
