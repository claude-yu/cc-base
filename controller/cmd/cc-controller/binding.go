package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	bindingExpiry    = 7 * 24 * time.Hour
	maxChatIDLen     = 128
	maxPlatformLen   = 32
	bindingDelimiter = "|"
)

type ChatBinding struct {
	ProjectID string `json:"project_id"`
	WorkDir   string `json:"work_dir"`
	SessionID string `json:"session_id"`
	UpdatedAt string `json:"updated_at"`
}

type BindingStore map[string]ChatBinding

func bindingPath(root string) string {
	return filepath.Join(root, "bindings.json")
}

func loadBindings(root string) BindingStore {
	data, err := os.ReadFile(bindingPath(root))
	if err != nil {
		return BindingStore{}
	}
	var store BindingStore
	if json.Unmarshal(data, &store) != nil {
		return BindingStore{}
	}
	return pruneBindings(store)
}

func saveBindings(root string, store BindingStore) {
	writeJSON(bindingPath(root), store)
}

func sanitizeChatID(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	s = strings.ReplaceAll(s, "|", "_")
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

func bindingKey(platform, chatID string) string {
	platform = sanitizeChatID(platform, maxPlatformLen)
	chatID = sanitizeChatID(chatID, maxChatIDLen)
	if platform != "" {
		return platform + bindingDelimiter + chatID
	}
	return chatID
}

func getBinding(store BindingStore, platform, chatID string) (ChatBinding, bool) {
	if chatID == "" {
		return ChatBinding{}, false
	}
	b, ok := store[bindingKey(platform, chatID)]
	return b, ok
}

func updateBinding(root, platform, chatID string, b ChatBinding) {
	if chatID == "" {
		return
	}
	store := loadBindings(root)
	b.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	store[bindingKey(platform, chatID)] = b
	saveBindings(root, store)
}

func touchBinding(root, platform, chatID string) {
	if chatID == "" {
		return
	}
	store := loadBindings(root)
	key := bindingKey(platform, chatID)
	if b, ok := store[key]; ok {
		b.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		store[key] = b
		saveBindings(root, store)
	}
}

func pruneBindings(store BindingStore) BindingStore {
	now := time.Now()
	for k, b := range store {
		t, err := time.Parse(time.RFC3339, b.UpdatedAt)
		if err != nil || now.Sub(t) > bindingExpiry {
			delete(store, k)
		}
	}
	return store
}

func resolveProjectForChat(root, platform, chatID string) ActiveProject {
	if chatID == "" {
		return readActiveProject(root)
	}
	store := loadBindings(root)
	b, ok := getBinding(store, platform, chatID)
	if !ok {
		return readActiveProject(root)
	}
	if b.WorkDir == "" {
		return readActiveProject(root)
	}
	if fi, err := os.Stat(b.WorkDir); err != nil || !fi.IsDir() {
		delete(store, bindingKey(platform, chatID))
		saveBindings(root, store)
		return readActiveProject(root)
	}
	return ActiveProject{
		Name:      filepath.Base(b.WorkDir),
		WorkDir:   b.WorkDir,
		ProjectID: b.ProjectID,
	}
}
