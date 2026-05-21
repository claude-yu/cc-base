package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseFrontmatter(t *testing.T) {
	content := `---
name: test-memory
description: "A test memory file"
metadata:
  node_type: memory
  type: feedback
  originSessionId: abc-123
---

Some body content here.
`
	name, desc, memType := parseFrontmatter(content)
	if name != "test-memory" {
		t.Errorf("name = %q, want %q", name, "test-memory")
	}
	if desc != "A test memory file" {
		t.Errorf("desc = %q, want %q", desc, "A test memory file")
	}
	if memType != "feedback" {
		t.Errorf("memType = %q, want %q", memType, "feedback")
	}
}

func TestParseFrontmatterNoQuotes(t *testing.T) {
	content := `---
name: simple
description: no quotes here
metadata:
  type: project
---
`
	name, desc, memType := parseFrontmatter(content)
	if name != "simple" {
		t.Errorf("name = %q, want %q", name, "simple")
	}
	if desc != "no quotes here" {
		t.Errorf("desc = %q, want %q", desc, "no quotes here")
	}
	if memType != "project" {
		t.Errorf("memType = %q, want %q", memType, "project")
	}
}

func TestParseFrontmatterEmpty(t *testing.T) {
	name, desc, memType := parseFrontmatter("no frontmatter here")
	if name != "" || desc != "" || memType != "" {
		t.Errorf("expected empty, got name=%q desc=%q type=%q", name, desc, memType)
	}
}

func TestCalcStaleness(t *testing.T) {
	now := time.Now()
	tests := []struct {
		age  time.Duration
		want string
	}{
		{1 * time.Hour, "fresh"},
		{23 * time.Hour, "fresh"},
		{3 * 24 * time.Hour, "aging"},
		{6 * 24 * time.Hour, "aging"},
		{10 * 24 * time.Hour, "stale"},
		{29 * 24 * time.Hour, "stale"},
		{31 * 24 * time.Hour, "expired"},
		{90 * 24 * time.Hour, "expired"},
	}
	for _, tt := range tests {
		got := calcStaleness(now, now.Add(-tt.age))
		if got != tt.want {
			t.Errorf("age=%v: got %q, want %q", tt.age, got, tt.want)
		}
	}
}

func TestCalcNoise(t *testing.T) {
	tests := []struct {
		lines int
		want  string
	}{
		{5, "low"},
		{20, "low"},
		{25, "medium"},
		{35, "medium"},
		{36, "high"},
		{100, "high"},
	}
	for _, tt := range tests {
		got := calcNoise(tt.lines)
		if got != tt.want {
			t.Errorf("lines=%d: got %q, want %q", tt.lines, got, tt.want)
		}
	}
}

func TestCoreMemoryClassification(t *testing.T) {
	coreNames := []string{
		"proxy-isolation", "cc-codex-division", "codex-on-demand-only",
		"oral-words-no-alias", "verify-env-before-plan", "real-config-separate",
		"command-not-nlp", "skill-first-workflow",
	}
	for _, name := range coreNames {
		if !coreMemoryNames[name] {
			t.Errorf("%q should be core memory", name)
		}
	}

	nonCore := []string{"dlg1-virtual-ko", "cc-connect-fork-v3", "gromacs-n-flag"}
	for _, name := range nonCore {
		if coreMemoryNames[name] {
			t.Errorf("%q should NOT be core memory", name)
		}
	}

	if !governanceNames["memory-maintenance-policy"] {
		t.Error("memory-maintenance-policy should be governance")
	}
	if coreMemoryNames["memory-maintenance-policy"] {
		t.Error("memory-maintenance-policy should NOT be in coreMemoryNames")
	}
}

func TestParseIndexNames(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "MEMORY.md")
	content := `- [Proxy isolation](feedback_proxy-isolation.md) — Claude needs HTTP proxy
- [DLG1 virtual knockout](project_dlg1-virtual-ko.md) — Active research
`
	os.WriteFile(indexPath, []byte(content), 0644)

	names := parseIndexNames(indexPath)
	if !names["proxy-isolation"] {
		t.Error("expected proxy-isolation in index")
	}
	if !names["dlg1-virtual-ko"] {
		t.Error("expected dlg1-virtual-ko in index")
	}
	if names["nonexistent"] {
		t.Error("nonexistent should not be in index")
	}
}

func TestScanMemoryDir(t *testing.T) {
	dir := t.TempDir()

	// Create a memory file
	content := `---
name: test-entry
description: Test
metadata:
  type: feedback
---

Body text.
`
	os.WriteFile(filepath.Join(dir, "feedback_test-entry.md"), []byte(content), 0644)
	os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("- [Test](feedback_test-entry.md) — Test\n"), 0644)

	entries := scanMemoryDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "test-entry" {
		t.Errorf("name = %q, want %q", entries[0].Name, "test-entry")
	}
	if entries[0].MemType != "feedback" {
		t.Errorf("type = %q, want %q", entries[0].MemType, "feedback")
	}
	if entries[0].Staleness != "fresh" {
		t.Errorf("staleness = %q, want %q", entries[0].Staleness, "fresh")
	}
}

func TestIndexReconciliation(t *testing.T) {
	dir := t.TempDir()

	// File WITH frontmatter, indexed
	indexed := `---
name: proxy-isolation
description: Proxy rules
metadata:
  type: feedback
---

Body.
`
	// File WITH frontmatter, NOT indexed
	notIndexed := `---
name: new-finding
description: Something new
metadata:
  type: project
---

New content.
`
	// File WITHOUT frontmatter — name derived from filename with prefix stripping
	noFrontmatter := "Just plain text, no YAML frontmatter.\n"

	// File with governance layer
	govFile := `---
name: memory-maintenance-policy
description: Phase 0 governance
metadata:
  type: reference
  layer: governance
---

Policy content.
`

	os.WriteFile(filepath.Join(dir, "feedback_proxy-isolation.md"), []byte(indexed), 0644)
	os.WriteFile(filepath.Join(dir, "project_new-finding.md"), []byte(notIndexed), 0644)
	os.WriteFile(filepath.Join(dir, "feedback_bare-note.md"), []byte(noFrontmatter), 0644)
	os.WriteFile(filepath.Join(dir, "memory-maintenance-policy.md"), []byte(govFile), 0644)

	// Index references proxy-isolation (exists) and ghost-entry (does not exist)
	indexContent := `- [Proxy isolation](feedback_proxy-isolation.md) — Proxy rules
- [Ghost](project_ghost-entry.md) — This file does not exist
`
	os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte(indexContent), 0644)

	// Scan files
	entries := scanMemoryDir(dir)
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	// Parse index
	indexNames := parseIndexNames(filepath.Join(dir, "MEMORY.md"))

	// Run reconciliation (same logic as cmdMemoryHealth)
	for i := range entries {
		if _, ok := indexNames[entries[i].Name]; ok {
			entries[i].InIndex = true
			delete(indexNames, entries[i].Name)
		}
	}

	// Check: proxy-isolation should be indexed
	for _, e := range entries {
		if e.Name == "proxy-isolation" {
			if !e.InIndex {
				t.Error("proxy-isolation should be marked InIndex")
			}
			if e.Layer != "core" {
				t.Errorf("proxy-isolation layer = %q, want core", e.Layer)
			}
			if !e.HasFrontmatter {
				t.Error("proxy-isolation should have frontmatter")
			}
		}
		if e.Name == "new-finding" {
			if e.InIndex {
				t.Error("new-finding should NOT be in index")
			}
			if !e.HasFrontmatter {
				t.Error("new-finding should have frontmatter")
			}
		}
		if e.Name == "bare-note" {
			if e.InIndex {
				t.Error("bare-note should NOT be in index")
			}
			if e.MemType != "" {
				t.Errorf("bare-note type should be empty, got %q", e.MemType)
			}
			if e.HasFrontmatter {
				t.Error("bare-note should NOT have frontmatter")
			}
		}
	}

	// Check: governance layer
	for _, e := range entries {
		if e.Name == "memory-maintenance-policy" {
			if e.Layer != "governance" {
				t.Errorf("memory-maintenance-policy layer = %q, want governance", e.Layer)
			}
			if !e.HasFrontmatter {
				t.Error("memory-maintenance-policy should have frontmatter")
			}
		}
	}

	// Check orphan: ghost-entry should remain in indexNames (orphan)
	if !indexNames["ghost-entry"] {
		t.Error("ghost-entry should be detected as orphan index entry")
	}
	if indexNames["proxy-isolation"] {
		t.Error("proxy-isolation should have been removed from orphans")
	}
}

func TestStripTypePrefix(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"feedback_proxy-isolation", "proxy-isolation"},
		{"project_dlg1-virtual-ko", "dlg1-virtual-ko"},
		{"user_role", "role"},
		{"reference_docs", "docs"},
		{"no-prefix-here", "no-prefix-here"},
		{"memory-maintenance-policy", "memory-maintenance-policy"},
	}
	for _, tt := range tests {
		got := stripTypePrefix(tt.input)
		if got != tt.want {
			t.Errorf("stripTypePrefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPadRight(t *testing.T) {
	if got := padRight("abc", 6); got != "abc   " {
		t.Errorf("padRight(%q, 6) = %q", "abc", got)
	}
	if got := padRight("abcdef", 3); got != "abcdef" {
		t.Errorf("padRight(%q, 3) = %q, should not truncate", "abcdef", got)
	}
}
