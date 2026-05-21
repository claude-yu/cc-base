package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseLatestSessionProjectPlatform(t *testing.T) {
	input := `#  Project         Platform  User
1  cc_561e18ff     weixin    user-a
2  cc_561e18ff     qq        user-b
3  codex_ad800c6a  qq        user-c`

	gotProject, gotPlatform := parseLatestSessionProjectPlatform(input, "cc")
	if gotProject != "cc_561e18ff" || gotPlatform != "weixin" {
		t.Fatalf("got (%q, %q), want latest cc weixin session", gotProject, gotPlatform)
	}
}

func TestExtractActiveSessionKeys(t *testing.T) {
	input := `{
  "sessions": {},
  "active_session": {
    "qq:949330422": "s2",
    "weixin:dm:user@im.wechat": "s3"
  }
}`

	got := extractActiveSessionKeys(input)
	want := []string{"qq:949330422", "weixin:dm:user@im.wechat"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("active keys = %#v, want %#v", got, want)
	}
}

func TestChooseActiveSessionKeyPrefersPlatform(t *testing.T) {
	keys := []string{"qq:949330422", "weixin:dm:user@im.wechat"}
	got := chooseActiveSessionKey(keys, "weixin")
	if got != "weixin:dm:user@im.wechat" {
		t.Fatalf("chosen key = %q, want weixin key", got)
	}
}

func TestChooseActiveSessionKeyFallsBackToFirst(t *testing.T) {
	keys := []string{"qq:949330422", "weixin:dm:user@im.wechat"}
	got := chooseActiveSessionKey(keys, "telegram")
	if got != "qq:949330422" {
		t.Fatalf("chosen key = %q, want first key fallback", got)
	}
}

func TestSessionKeyFromBindingKey(t *testing.T) {
	got := sessionKeyFromBindingKey("weixin|dm:user@im.wechat")
	if got != "weixin:dm:user@im.wechat" {
		t.Fatalf("session key = %q, want active cc-connect key", got)
	}
}

func TestResolveCallbackSessionKeyPrefersEnv(t *testing.T) {
	t.Setenv("CC_SESSION_KEY", "weixin:dm:from-env")
	got := resolveCallbackSessionKey(t.TempDir(), "missing-cc-connect", "cc")
	if got != "weixin:dm:from-env" {
		t.Fatalf("session key = %q, want env key", got)
	}
}

func TestResolveCallbackSessionKeyUsesRunnerChatID(t *testing.T) {
	t.Setenv("CC_SESSION_KEY", "")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "runner.chat-id"), []byte("qq|949330422"), 0644); err != nil {
		t.Fatal(err)
	}
	got := resolveCallbackSessionKey(dir, "missing-cc-connect", "cc")
	if got != "qq:949330422" {
		t.Fatalf("session key = %q, want runner chat-id key", got)
	}
}

func TestResolveCallbackSessionKeyUsesPersistedSessionKey(t *testing.T) {
	t.Setenv("CC_SESSION_KEY", "")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "runner.cc-session-key"), []byte("weixin:dm:stored"), 0644); err != nil {
		t.Fatal(err)
	}
	got := resolveCallbackSessionKey(dir, "missing-cc-connect", "cc")
	if got != "weixin:dm:stored" {
		t.Fatalf("session key = %q, want persisted key", got)
	}
}
