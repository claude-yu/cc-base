package main

import (
	"bytes"
	"testing"
)

func TestPatchQuietUIDisplayInsertsInCCProjectOnly(t *testing.T) {
	input := []byte("[[commands]]\r\nname = \"cc\"\r\nexec = \"echo wrong\"\r\n\r\n[[projects]]\r\nname = \"cc\"\r\nunknown_slash = \"error\"\r\n[projects.agent]\r\ntype = \"claudecode\"\r\n\r\n[[projects]]\r\nname = \"codex\"\r\nunknown_slash = \"error\"\r\n")
	got, changed, err := patchQuietUIDisplay(input)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected change")
	}
	commandSection := got[:bytes.Index(got, []byte("[[projects]]\r\nname = \"cc\""))]
	if bytes.Contains(commandSection, []byte("[projects.display]")) {
		t.Fatalf("command section should not be patched:\n%s", commandSection)
	}
	ccProject := got[bytes.Index(got, []byte("[[projects]]\r\nname = \"cc\"")):bytes.Index(got, []byte("[[projects]]\r\nname = \"codex\""))]
	if !bytes.Contains(ccProject, []byte("[projects.display]")) {
		t.Fatalf("cc project missing display block:\n%s", ccProject)
	}
	codexProject := got[bytes.Index(got, []byte("[[projects]]\r\nname = \"codex\"")):]
	if bytes.Contains(codexProject, []byte("[projects.display]")) {
		t.Fatalf("codex project should not be patched:\n%s", codexProject)
	}
}

func TestPatchQuietUIDisplayIsIdempotent(t *testing.T) {
	input := []byte("language = \"zh\"\r\n\r\n[[projects]]\r\nname = \"cc\"\r\nunknown_slash = \"error\"\r\n\r\n[projects.display]\r\ntool_messages = false\r\n")
	got, changed, err := patchQuietUIDisplay(input)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected no change")
	}
	if !bytes.Equal(got, input) {
		t.Fatal("idempotent patch changed bytes")
	}
}

func TestPatchQuietUIDisplayMissingMarker(t *testing.T) {
	input := []byte("[[projects]]\r\nname = \"cc\"\r\n")
	_, changed, err := patchQuietUIDisplay(input)
	if err == nil {
		t.Fatal("expected error")
	}
	if changed {
		t.Fatal("expected changed=false on error")
	}
}
