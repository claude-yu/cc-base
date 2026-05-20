package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var projectContextTrigger = []string{
	"项目状态", "项目进度", "项目计划", "项目总结",
	"当前计划", "当前工作", "当前进展",
	"下一步", "接下来做",
	"工作重点", "工作进展",
	"总结状态", "总结进度", "总结项目", "总结工作",
	"分析进度", "分析状态",
}

func isProjectContextQuery(text string) bool {
	for _, s := range projectContextTrigger {
		if strings.Contains(text, s) {
			return true
		}
	}
	return false
}

const maxSectionLines = 30

const defaultProjectContext = `This is a mobile-controlled local Agent controller (cc-base).
Available infrastructure:
- controller_root: stores runs/, sessions/, state, plans, progress, handoff files
- work_dir: user/scientific workspace (may differ from controller_root)
- local handlers: status queries, result location, research monitor
- Agent modes: advice (default), readonly, execute (with confirmation)
If project memory files are missing, ask concise follow-up questions instead of guessing file paths.`

func buildProjectMemoryContext(root, workDir string) string {
	projectRoot := filepath.Dir(root)

	var sb strings.Builder
	sb.WriteString("\n\n--- Project Memory Context ---\n")
	sb.WriteString("controller_root: " + root + "\n")
	sb.WriteString("project_root: " + projectRoot + "\n")
	if workDir != "" && workDir != "." {
		sb.WriteString("work_dir: " + workDir + "\n")
	}

	found := false

	// Priority 1: Plan file (authoritative current priorities)
	if content := findAndRead("mobile-agent-platform-plan.md", projectRoot, root); content != "" {
		found = true
		if decision := extractSection(content, "Current Decision"); decision != "" {
			sb.WriteString("\n[Current Priorities | source: mobile-agent-platform-plan.md | weight: authoritative]\n")
			sb.WriteString(capLines(decision, 15))
			sb.WriteString("\n")
		}
		if roadmap := extractSection(content, "Near-Term Roadmap"); roadmap != "" {
			sb.WriteString("\n")
			sb.WriteString(capLines(roadmap, maxSectionLines))
			sb.WriteString("\n")
		}
	}

	// Priority 2: Progress file (backlog)
	if content := findAndRead("progress.md", projectRoot, root); content != "" {
		found = true
		if todo := extractSection(content, "TODO"); todo != "" {
			sb.WriteString("\n[Active TODO | source: progress.md | weight: current]\n")
			sb.WriteString(capLines(todo, maxSectionLines))
			sb.WriteString("\n")
		}
		if inProgress := extractSection(content, "In Progress"); inProgress != "" && inProgress != "（无）" {
			sb.WriteString("\n[In Progress]\n")
			sb.WriteString(inProgress)
			sb.WriteString("\n")
		}
	}

	// Priority 3: Latest handoff (session state only, next steps superseded by plan)
	handoff := findLatestHandoffDedup(projectRoot, root)
	if handoff != "" {
		if data, err := os.ReadFile(handoff); err == nil {
			content := string(data)
			found = true
			for _, heading := range []string{"当前状态", "Current Status"} {
				if state := extractSection(content, heading); state != "" {
					sb.WriteString("\n[Previous Session | source: " + filepath.Base(handoff) + " | weight: historical, may be stale]\n")
					sb.WriteString(capLines(state, 20))
					sb.WriteString("\n")
					break
				}
			}
		}
	}

	// Priority 4: Fallback — CLAUDE.md / AGENTS.md / README.md (first found)
	if !found {
		for _, name := range []string{"CLAUDE.md", "AGENTS.md", "README.md"} {
			if content := findAndRead(name, projectRoot, root, workDir); content != "" {
				found = true
				sb.WriteString("\n[Project Info | source: " + name + " | weight: fallback]\n")
				sb.WriteString(capLines(content, 20))
				sb.WriteString("\n")
				break
			}
		}
	}

	// Priority 5: Default context (no project files found at all)
	if !found {
		if custom := findAndRead("default-project-context.md",
			filepath.Join(root, "templates"),
			filepath.Join(projectRoot, "templates")); custom != "" {
			sb.WriteString("\n" + strings.TrimSpace(custom) + "\n")
		} else {
			sb.WriteString("\n" + defaultProjectContext + "\n")
		}
	}

	sb.WriteString("\n--- End Project Memory Context ---")
	return sb.String()
}

func findAndRead(name string, dirs ...string) string {
	for _, dir := range dirs {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			return string(data)
		}
	}
	return ""
}

func findLatestHandoff(dir string) string {
	matches, err := filepath.Glob(filepath.Join(dir, "handoff-*.md"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[len(matches)-1]
}

func findLatestHandoffDedup(dirs ...string) string {
	seen := map[string]bool{}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = dir
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		if h := findLatestHandoff(dir); h != "" {
			return h
		}
	}
	return ""
}

func extractSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inSection := false
	sectionLevel := 0
	for _, line := range lines {
		if lev := markdownHeadingLevel(line); lev > 0 {
			text := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if strings.Contains(text, heading) {
				inSection = true
				sectionLevel = lev
				continue
			} else if inSection && lev <= sectionLevel {
				break
			}
		}
		if inSection {
			result = append(result, line)
		}
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

func markdownHeadingLevel(line string) int {
	if !strings.HasPrefix(line, "#") {
		return 0
	}
	level := 0
	for _, c := range line {
		if c == '#' {
			level++
		} else {
			break
		}
	}
	if level > 0 && len(line) > level && line[level] == ' ' {
		return level
	}
	return 0
}

func capLines(s string, max int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= max {
		return s
	}
	return strings.Join(lines[:max], "\n") + "\n..."
}
