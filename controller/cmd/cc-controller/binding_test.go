package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBindingKey(t *testing.T) {
	if got := bindingKey("wechat", "user1"); got != "wechat|user1" {
		t.Errorf("bindingKey(wechat, user1) = %q, want wechat|user1", got)
	}
	if got := bindingKey("", "user1"); got != "user1" {
		t.Errorf("bindingKey('', user1) = %q, want user1", got)
	}
}

func TestSanitizeChatID(t *testing.T) {
	cases := []struct {
		name, input, want string
	}{
		{"normal", "user123", "user123"},
		{"slash", "../../etc/passwd", "____etc_passwd"},
		{"backslash", "a\\b", "a_b"},
		{"pipe", "a|b", "a_b"},
		{"long", "aaaaaaaaaa", "aaaaa"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			maxLen := 128
			if c.name == "long" {
				maxLen = 5
			}
			if got := sanitizeChatID(c.input, maxLen); got != c.want {
				t.Errorf("sanitizeChatID(%q, %d) = %q, want %q", c.input, maxLen, got, c.want)
			}
		})
	}
}

func TestLoadBindings_Empty(t *testing.T) {
	dir := t.TempDir()
	store := loadBindings(dir)
	if len(store) != 0 {
		t.Errorf("expected empty store, got %d entries", len(store))
	}
}

func TestSaveAndLoadBindings(t *testing.T) {
	dir := t.TempDir()
	store := BindingStore{
		"wechat|u1": {ProjectID: "proj-a", WorkDir: dir, SessionID: "proj-a-default", UpdatedAt: time.Now().UTC().Format(time.RFC3339)},
	}
	saveBindings(dir, store)

	loaded := loadBindings(dir)
	if len(loaded) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded))
	}
	if loaded["wechat|u1"].ProjectID != "proj-a" {
		t.Errorf("ProjectID = %q, want proj-a", loaded["wechat|u1"].ProjectID)
	}
}

func TestGetBinding_Found(t *testing.T) {
	dir := t.TempDir()
	updateBinding(dir, "wechat", "u1", ChatBinding{ProjectID: "proj-a", WorkDir: dir, SessionID: "proj-a-default"})

	store := loadBindings(dir)
	b, ok := getBinding(store, "wechat", "u1")
	if !ok {
		t.Fatal("expected binding to be found")
	}
	if b.ProjectID != "proj-a" {
		t.Errorf("ProjectID = %q, want proj-a", b.ProjectID)
	}
}

func TestGetBinding_NotFound(t *testing.T) {
	store := BindingStore{}
	_, ok := getBinding(store, "wechat", "unknown")
	if ok {
		t.Error("expected binding not found")
	}
}

func TestGetBinding_EmptyChatID(t *testing.T) {
	store := BindingStore{}
	_, ok := getBinding(store, "wechat", "")
	if ok {
		t.Error("expected false for empty chatID")
	}
}

func TestTouchBinding(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	store := BindingStore{
		"wechat|u1": {ProjectID: "p", WorkDir: dir, SessionID: "s", UpdatedAt: old},
	}
	saveBindings(dir, store)

	touchBinding(dir, "wechat", "u1")

	loaded := loadBindings(dir)
	b, ok := getBinding(loaded, "wechat", "u1")
	if !ok {
		t.Fatal("expected binding")
	}
	updated, _ := time.Parse(time.RFC3339, b.UpdatedAt)
	if time.Since(updated) > 5*time.Second {
		t.Errorf("UpdatedAt not refreshed: %s", b.UpdatedAt)
	}
}

func TestPruneBindings_RemovesExpired(t *testing.T) {
	fresh := time.Now().UTC().Format(time.RFC3339)
	stale := time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339)
	store := BindingStore{
		"fresh": {ProjectID: "a", UpdatedAt: fresh},
		"stale": {ProjectID: "b", UpdatedAt: stale},
	}
	pruned := pruneBindings(store)
	if len(pruned) != 1 {
		t.Errorf("expected 1 entry after prune, got %d", len(pruned))
	}
	if _, ok := pruned["fresh"]; !ok {
		t.Error("expected fresh entry to survive prune")
	}
}

func TestPruneBindings_RemovesBadTimestamp(t *testing.T) {
	store := BindingStore{
		"good":  {ProjectID: "a", UpdatedAt: time.Now().UTC().Format(time.RFC3339)},
		"empty": {ProjectID: "b", UpdatedAt: ""},
		"bad":   {ProjectID: "c", UpdatedAt: "not-a-date"},
	}
	pruned := pruneBindings(store)
	if len(pruned) != 1 {
		t.Errorf("expected 1 entry after prune, got %d", len(pruned))
	}
}

func TestResolveProjectForChat_NoBinding(t *testing.T) {
	dir := t.TempDir()
	wd := filepath.Join(dir, "work")
	os.MkdirAll(wd, 0755)
	writeActiveProject(dir, ActiveProject{Name: "global", WorkDir: wd, ProjectID: "global"})

	p := resolveProjectForChat(dir, "wechat", "nobody")
	if p.ProjectID != "global" {
		t.Errorf("expected global fallback, got %q", p.ProjectID)
	}
}

func TestResolveProjectForChat_WithBinding(t *testing.T) {
	dir := t.TempDir()
	globalWd := filepath.Join(dir, "global")
	boundWd := filepath.Join(dir, "bound")
	os.MkdirAll(globalWd, 0755)
	os.MkdirAll(boundWd, 0755)
	writeActiveProject(dir, ActiveProject{Name: "global", WorkDir: globalWd, ProjectID: "global"})

	updateBinding(dir, "wechat", "u1", ChatBinding{
		ProjectID: "bound-proj",
		WorkDir:   boundWd,
		SessionID: "bound-proj-default",
	})

	p := resolveProjectForChat(dir, "wechat", "u1")
	if p.ProjectID != "bound-proj" {
		t.Errorf("expected bound-proj, got %q", p.ProjectID)
	}
	if p.WorkDir != boundWd {
		t.Errorf("expected WorkDir=%q, got %q", boundWd, p.WorkDir)
	}
}

func TestResolveProjectForChat_StaleBinding(t *testing.T) {
	dir := t.TempDir()
	globalWd := filepath.Join(dir, "global")
	os.MkdirAll(globalWd, 0755)
	writeActiveProject(dir, ActiveProject{Name: "global", WorkDir: globalWd, ProjectID: "global"})

	updateBinding(dir, "wechat", "u1", ChatBinding{
		ProjectID: "gone-proj",
		WorkDir:   filepath.Join(dir, "nonexistent"),
		SessionID: "gone-proj-default",
	})

	p := resolveProjectForChat(dir, "wechat", "u1")
	if p.ProjectID != "global" {
		t.Errorf("expected global fallback for stale binding, got %q", p.ProjectID)
	}
	store := loadBindings(dir)
	if _, ok := store[bindingKey("wechat", "u1")]; ok {
		t.Error("expected stale binding to be removed")
	}
}

func TestParseExecFlags_ChatID(t *testing.T) {
	f := parseExecFlags([]string{"--text", "hello", "--chat-id", "wx123", "--platform", "wechat", "--auto"})
	if f.text != "hello" {
		t.Errorf("text = %q", f.text)
	}
	if f.chatID != "wx123" {
		t.Errorf("chatID = %q", f.chatID)
	}
	if f.platform != "wechat" {
		t.Errorf("platform = %q", f.platform)
	}
	if !f.auto {
		t.Error("expected auto=true")
	}
}

func TestParseExecFlags_NoChatID(t *testing.T) {
	f := parseExecFlags([]string{"--text", "hello", "--auto"})
	if f.chatID != "" {
		t.Errorf("chatID should be empty, got %q", f.chatID)
	}
	if f.platform != "" {
		t.Errorf("platform should be empty, got %q", f.platform)
	}
}
