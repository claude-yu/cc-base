package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var coreMemoryNames = map[string]bool{
	"proxy-isolation":       true,
	"cc-codex-division":     true,
	"codex-on-demand-only":  true,
	"oral-words-no-alias":   true,
	"verify-env-before-plan": true,
	"real-config-separate":  true,
	"command-not-nlp":       true,
	"skill-first-workflow":  true,
}

var governanceNames = map[string]bool{
	"memory-maintenance-policy": true,
}

var scopeMap = map[string]string{
	"cc-connect-admin-from":        "controller",
	"cc-connect-gbk-encoding":      "controller",
	"cc-connect-fork-v3":           "controller",
	"cc-connect-fork-v4c":          "controller",
	"classifier-no-broad-keywords": "controller",
	"verify-config-wired":          "controller",
	"backend-abstraction-v1":       "controller",
	"multi-model-qq":               "controller",
	"gromacs-n-flag":               "scientific",
	"wsl-windows-path-separation":  "scientific",
	"pid-detection-windows-wsl":    "scientific",
	"ndx-group-parsing":            "scientific",
	"dlg1-virtual-ko":              "scientific",
	"cc-stdin-chinese":             "shared",
	"codex-review-convergence":     "shared",
	"garbled-output-kill-bg":       "shared",
	"deepseek-chinese-diff":        "shared",
}

type memEntry struct {
	FileName       string
	Name           string
	Description    string
	MemType        string
	Scope          string
	Layer          string // core, governance, or active
	Lines          int
	ModTime        time.Time
	Staleness      string
	Noise          string
	InIndex        bool
	HasFrontmatter bool
}

func cmdMemoryHealth(args []string) {
	memDir := os.Getenv("CC_MEMORY_STORE")
	if len(args) > 0 && args[0] != "" {
		memDir = args[0]
	}
	if memDir == "" {
		fmt.Fprintln(os.Stderr, "用法: cc-controller memory-health <memory-dir>")
		fmt.Fprintln(os.Stderr, "  或设置 CC_MEMORY_STORE 环境变量")
		os.Exit(1)
	}

	fi, err := os.Stat(memDir)
	if err != nil || !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "目录不存在: %s\n", memDir)
		os.Exit(1)
	}

	indexPath := filepath.Join(memDir, "MEMORY.md")
	indexNames := parseIndexNames(indexPath)
	entries := scanMemoryDir(memDir)

	for i := range entries {
		e := &entries[i]
		if _, ok := indexNames[e.Name]; ok {
			e.InIndex = true
			delete(indexNames, e.Name)
		}
	}

	printHealthReport(entries, indexNames, memDir)
}

func parseIndexNames(path string) map[string]bool {
	names := make(map[string]bool)
	f, err := os.Open(path)
	if err != nil {
		return names
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		// Format: - [Title](filename.md) — description
		if idx := strings.Index(line, "]("); idx >= 0 {
			rest := line[idx+2:]
			if end := strings.Index(rest, ")"); end > 0 {
				fname := rest[:end]
				name := strings.TrimSuffix(fname, ".md")
				// Strip type prefix (feedback_, project_, etc.)
				for _, prefix := range []string{"feedback_", "project_", "user_", "reference_"} {
					name = strings.TrimPrefix(name, prefix)
				}
				names[name] = true
			}
		}
	}
	return names
}

func scanMemoryDir(dir string) []memEntry {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var entries []memEntry
	now := time.Now()

	for _, de := range dirEntries {
		fname := de.Name()
		if !strings.HasSuffix(fname, ".md") || fname == "MEMORY.md" {
			continue
		}

		fullPath := filepath.Join(dir, fname)
		info, err := de.Info()
		if err != nil {
			continue
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		content := string(data)
		lines := strings.Count(content, "\n") + 1
		modTime := info.ModTime()

		name, desc, memType := parseFrontmatter(content)
		hasFM := name != ""
		if !hasFM {
			name = stripTypePrefix(strings.TrimSuffix(fname, ".md"))
		}

		layer := "active"
		if coreMemoryNames[name] {
			layer = "core"
		} else if governanceNames[name] {
			layer = "governance"
		}

		scope := "unknown"
		if s, ok := scopeMap[name]; ok {
			scope = s
		} else if coreMemoryNames[name] {
			scope = "shared"
		}

		staleness := calcStaleness(now, modTime)
		noise := calcNoise(lines)

		entries = append(entries, memEntry{
			FileName:       fname,
			Name:           name,
			Description:    desc,
			MemType:        memType,
			Scope:          scope,
			Layer:          layer,
			Lines:          lines,
			ModTime:        modTime,
			Staleness:      staleness,
			Noise:          noise,
			InIndex:        false,
			HasFrontmatter: hasFM,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Layer != entries[j].Layer {
			return layerOrder(entries[i].Layer) < layerOrder(entries[j].Layer)
		}
		return entries[i].Name < entries[j].Name
	})

	return entries
}

func parseFrontmatter(content string) (name, desc, memType string) {
	if !strings.HasPrefix(content, "---") {
		return
	}

	end := strings.Index(content[3:], "---")
	if end < 0 {
		return
	}
	fm := content[3 : 3+end]

	inMetadata := false
	for _, line := range strings.Split(fm, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "name:") {
			name = stripQuotes(strings.TrimSpace(strings.TrimPrefix(trimmed, "name:")))
		} else if strings.HasPrefix(trimmed, "description:") {
			desc = stripQuotes(strings.TrimSpace(strings.TrimPrefix(trimmed, "description:")))
		} else if trimmed == "metadata:" {
			inMetadata = true
		} else if inMetadata && strings.HasPrefix(trimmed, "type:") {
			memType = stripQuotes(strings.TrimSpace(strings.TrimPrefix(trimmed, "type:")))
		} else if inMetadata && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
			inMetadata = false
		}
	}
	return
}

func stripTypePrefix(name string) string {
	for _, prefix := range []string{"feedback_", "project_", "user_", "reference_"} {
		if strings.HasPrefix(name, prefix) {
			return strings.TrimPrefix(name, prefix)
		}
	}
	return name
}

func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func calcStaleness(now, modTime time.Time) string {
	age := now.Sub(modTime)
	switch {
	case age < 24*time.Hour:
		return "fresh"
	case age < 7*24*time.Hour:
		return "aging"
	case age < 30*24*time.Hour:
		return "stale"
	default:
		return "expired"
	}
}

func calcNoise(lines int) string {
	switch {
	case lines <= 20:
		return "low"
	case lines <= 35:
		return "medium"
	default:
		return "high"
	}
}

func printHealthReport(entries []memEntry, orphanIndex map[string]bool, memDir string) {
	var sb strings.Builder

	sb.WriteString("📋 Memory Health Report\n")
	sb.WriteString(fmt.Sprintf("目录: %s\n", memDir))
	sb.WriteString(fmt.Sprintf("文件数: %d\n\n", len(entries)))

	// Summary counts
	coreCount := 0
	govCount := 0
	activeCount := 0
	freshCount := 0
	agingCount := 0
	staleCount := 0
	expiredCount := 0
	notIndexed := 0
	noiseHigh := 0
	missingFM := 0

	for _, e := range entries {
		switch e.Layer {
		case "core":
			coreCount++
		case "governance":
			govCount++
		default:
			activeCount++
		}
		if !e.HasFrontmatter {
			missingFM++
		}
		switch e.Staleness {
		case "fresh":
			freshCount++
		case "aging":
			agingCount++
		case "stale":
			staleCount++
		case "expired":
			expiredCount++
		}
		if !e.InIndex {
			notIndexed++
		}
		if e.Noise == "high" && e.Layer == "active" {
			noiseHigh++
		}
	}

	sb.WriteString("── 总览 ──\n")
	sb.WriteString(fmt.Sprintf("Core: %d  Governance: %d  Active: %d\n", coreCount, govCount, activeCount))
	sb.WriteString(fmt.Sprintf("Fresh: %d  Aging: %d  Stale: %d  Expired: %d\n", freshCount, agingCount, staleCount, expiredCount))
	if notIndexed > 0 {
		sb.WriteString(fmt.Sprintf("⚠ 未索引: %d\n", notIndexed))
	}
	if noiseHigh > 0 {
		sb.WriteString(fmt.Sprintf("⚠ 高噪声: %d\n", noiseHigh))
	}
	if len(orphanIndex) > 0 {
		sb.WriteString(fmt.Sprintf("⚠ 孤儿索引: %d\n", len(orphanIndex)))
	}
	if missingFM > 0 {
		sb.WriteString(fmt.Sprintf("⚠ 缺 frontmatter: %d\n", missingFM))
	}

	// Core Memory section
	sb.WriteString("\n── Core Memory (受保护) ──\n")
	for _, e := range entries {
		if e.Layer != "core" {
			continue
		}
		icon := stalenessIcon(e.Staleness)
		idx := "✓"
		if !e.InIndex {
			idx = "✗"
		}
		sb.WriteString(fmt.Sprintf("  %s %-28s %s %2d行 idx:%s\n",
			icon, e.Name, padRight(e.Staleness, 7), e.Lines, idx))
	}

	// Governance section
	hasGov := false
	for _, e := range entries {
		if e.Layer != "governance" {
			continue
		}
		if !hasGov {
			sb.WriteString("\n── Governance (受保护) ──\n")
			hasGov = true
		}
		icon := stalenessIcon(e.Staleness)
		idx := "✓"
		if !e.InIndex {
			idx = "✗"
		}
		fmTag := ""
		if !e.HasFrontmatter {
			fmTag = " ⚠无frontmatter"
		}
		sb.WriteString(fmt.Sprintf("  %s %-28s %s %2d行 idx:%s%s\n",
			icon, e.Name, padRight(e.Staleness, 7), e.Lines, idx, fmTag))
	}

	// Active Memory by scope
	for _, scope := range []string{"controller", "scientific", "shared", "unknown"} {
		var scopeEntries []memEntry
		for _, e := range entries {
			if e.Layer == "active" && e.Scope == scope {
				scopeEntries = append(scopeEntries, e)
			}
		}
		if len(scopeEntries) == 0 {
			continue
		}

		label := scope
		if scope == "unknown" {
			label = "未分类"
		}
		sb.WriteString(fmt.Sprintf("\n── Active: %s ──\n", label))

		for _, e := range scopeEntries {
			icon := stalenessIcon(e.Staleness)
			tags := ""
			if e.Noise == "high" {
				tags += " ⚠噪声"
			}
			if !e.HasFrontmatter {
				tags += " ⚠无FM"
			}
			idx := "✓"
			if !e.InIndex {
				idx = "✗"
			}
			sb.WriteString(fmt.Sprintf("  %s %-28s %-8s %s %2d行 idx:%s%s\n",
				icon, e.Name, e.MemType, padRight(e.Staleness, 7), e.Lines, idx, tags))
		}
	}

	// Orphan index entries
	if len(orphanIndex) > 0 {
		sb.WriteString("\n── 孤儿索引条目 (MEMORY.md 引用但文件不存在) ──\n")
		for name := range orphanIndex {
			sb.WriteString(fmt.Sprintf("  ✗ %s\n", name))
		}
	}

	// Recommendations
	sb.WriteString("\n── 建议 ──\n")
	if expiredCount > 0 {
		sb.WriteString(fmt.Sprintf("• %d 个文件已过期(>30天)，建议验证准确性或归档\n", expiredCount))
	}
	if staleCount > 0 {
		sb.WriteString(fmt.Sprintf("• %d 个文件变陈旧(7-30天)，建议审查\n", staleCount))
	}
	if noiseHigh > 0 {
		sb.WriteString(fmt.Sprintf("• %d 个文件噪声高(>35行)，考虑精简\n", noiseHigh))
	}
	if notIndexed > 0 {
		sb.WriteString(fmt.Sprintf("• %d 个文件未在 MEMORY.md 索引\n", notIndexed))
	}
	if len(orphanIndex) > 0 {
		sb.WriteString(fmt.Sprintf("• MEMORY.md 有 %d 个无对应文件的引用\n", len(orphanIndex)))
	}
	if missingFM > 0 {
		sb.WriteString(fmt.Sprintf("• %d 个文件缺少 YAML frontmatter (name/type/description)\n", missingFM))
	}
	if expiredCount == 0 && staleCount == 0 && noiseHigh == 0 && notIndexed == 0 && len(orphanIndex) == 0 && missingFM == 0 {
		sb.WriteString("• 状态良好，无需立即行动\n")
	}

	fmt.Print(sb.String())
}

func stalenessIcon(s string) string {
	switch s {
	case "fresh":
		return "🟢"
	case "aging":
		return "🟡"
	case "stale":
		return "🟠"
	case "expired":
		return "🔴"
	default:
		return "⚪"
	}
}

func layerOrder(layer string) int {
	switch layer {
	case "core":
		return 0
	case "governance":
		return 1
	case "active":
		return 2
	default:
		return 3
	}
}

func padRight(s string, width int) string {
	for len(s) < width {
		s += " "
	}
	return s
}
