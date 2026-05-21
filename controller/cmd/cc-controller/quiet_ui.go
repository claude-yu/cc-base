package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var quietUIDisplayBlock = []byte("\r\n\r\n[projects.display]\r\nthinking_messages = false\r\ntool_messages = false\r\nreply_footer = false\r\nshow_context_indicator = false\r\n")

func cmdQuietUI(root string, args []string) {
	paths := args
	if len(paths) == 0 {
		if home, err := os.UserHomeDir(); err == nil {
			paths = append(paths, filepath.Join(home, ".cc-connect", "config.toml"))
		}
		paths = append(paths, filepath.Join(filepath.Dir(root), "cc-connect", "config.toml"))
	}

	seen := map[string]bool{}
	for _, path := range paths {
		clean := filepath.Clean(path)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		changed, backup, err := ensureQuietUIDisplay(clean)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", clean, err)
			continue
		}
		if changed {
			fmt.Printf("updated: %s\nbackup: %s\n", clean, backup)
		} else {
			fmt.Printf("already quiet: %s\n", clean)
		}
	}
}

func ensureQuietUIDisplay(path string) (bool, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, "", err
	}
	updated, changed, err := patchQuietUIDisplay(data)
	if err != nil || !changed {
		return false, "", err
	}
	backup := path + ".bak-quiet-ui-" + time.Now().Format("20060102-150405")
	if err := os.WriteFile(backup, data, 0644); err != nil {
		return false, "", err
	}
	if err := os.WriteFile(path, updated, 0644); err != nil {
		return false, backup, err
	}
	return true, backup, nil
}

func patchQuietUIDisplay(data []byte) ([]byte, bool, error) {
	projectStart, projectEnd := findCCProjectSection(data)
	if projectStart < 0 {
		return nil, false, fmt.Errorf(`project name "cc" not found`)
	}
	section := data[projectStart:projectEnd]
	if bytes.Contains(section, []byte("[projects.display]")) {
		return data, false, nil
	}
	needle := []byte(`unknown_slash = "error"`)
	rel := bytes.Index(section, needle)
	if rel < 0 {
		return nil, false, fmt.Errorf(`unknown_slash marker not found in cc project`)
	}
	insertAt := projectStart + rel + len(needle)
	out := make([]byte, 0, len(data)+len(quietUIDisplayBlock))
	out = append(out, data[:insertAt]...)
	out = append(out, quietUIDisplayBlock...)
	out = append(out, data[insertAt:]...)
	return out, true, nil
}

func findCCProjectSection(data []byte) (int, int) {
	marker := []byte("[[projects]]")
	for start := bytes.Index(data, marker); start >= 0; {
		nextRel := bytes.Index(data[start+len(marker):], marker)
		end := len(data)
		if nextRel >= 0 {
			end = start + len(marker) + nextRel
		}
		section := data[start:end]
		if bytes.Contains(section, []byte(`name = "cc"`)) {
			return start, end
		}
		if nextRel < 0 {
			break
		}
		nextStart := start + len(marker) + nextRel
		start = nextStart
	}
	return -1, -1
}
