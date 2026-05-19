package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ActiveProject struct {
	Name      string `json:"name"`
	WorkDir   string `json:"work_dir"`
	ProjectID string `json:"project_id"`
}

func activeProjectPath(root string) string {
	return filepath.Join(root, "active_project.json")
}

// readActiveProject returns the configured project from active_project.json.
// Falls back to CC_WORK_DIR env (or ".").
func readActiveProject(root string) ActiveProject {
	path := activeProjectPath(root)
	data, err := os.ReadFile(path)
	if err == nil {
		var p ActiveProject
		if json.Unmarshal(data, &p) == nil && p.WorkDir != "" {
			return p
		}
	}
	workDir := "."
	if wd := os.Getenv("CC_WORK_DIR"); wd != "" {
		workDir = wd
	}
	return ActiveProject{
		Name:      filepath.Base(workDir),
		WorkDir:   workDir,
		ProjectID: sanitizeProjectID(filepath.Base(workDir)),
	}
}

func writeActiveProject(root string, p ActiveProject) error {
	os.Setenv("CC_WORK_DIR", p.WorkDir)
	path := activeProjectPath(root)
	data, _ := json.MarshalIndent(p, "", "  ")
	return os.WriteFile(path, data, 0644)
}

// resolveSessionID prepends the project ID to the session name,
// giving each project independent session context.
func resolveSessionID(root, sessionID string) string {
	project := readActiveProject(root)
	base := sessionID
	if base == "" || base == "default" {
		base = "default"
	}
	return project.ProjectID + "-" + base
}

// sanitizeProjectID replaces characters outside [a-zA-Z0-9._-] with _.
func sanitizeProjectID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// cmdProject shows the active project info.
func cmdProject(root string) {
	p := readActiveProject(root)
	fmt.Printf("当前项目: %s\n", p.Name)
	fmt.Printf("项目 ID:  %s\n", p.ProjectID)
	fmt.Printf("工作目录: %s\n", p.WorkDir)
}

// cmdSwitchProject switches to a project by name or path.
//   - If the arg is a full path (contains \ or /), use it directly.
//   - If it's a name, resolve relative to the parent of the current work_dir.
//   - If no active_project.json exists, relative names are rejected (require full path).
func cmdSwitchProject(root, target string) {
	current := readActiveProject(root)

	var workDir string
	if strings.ContainsAny(target, "\\/") {
		workDir = target
	} else {
		if _, err := os.Stat(activeProjectPath(root)); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "当前未设定活动项目，无法解析相对名称。请使用完整路径:\n")
			fmt.Fprintf(os.Stderr, "  切项目 完整路径\\%s\n", target)
			os.Exit(1)
		}
		parent := filepath.Dir(current.WorkDir)
		workDir = filepath.Join(parent, target)
	}

	abs, err := filepath.Abs(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法解析路径: %s\n", err)
		os.Exit(1)
	}
	workDir = abs

	fi, err := os.Stat(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "目录不存在: %s\n", workDir)
		fmt.Fprintf(os.Stderr, "请先创建该目录\n")
		os.Exit(1)
	}
	if !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "不是目录: %s\n", workDir)
		os.Exit(1)
	}

	p := ActiveProject{
		Name:      filepath.Base(workDir),
		WorkDir:   workDir,
		ProjectID: sanitizeProjectID(filepath.Base(workDir)),
	}
	if err := writeActiveProject(root, p); err != nil {
		fmt.Fprintf(os.Stderr, "写入项目配置失败: %s\n", err)
		os.Exit(1)
	}
	session := p.ProjectID + "-default"
	fmt.Printf("已切换项目\n")
	fmt.Printf("Name: %s\n", p.Name)
	fmt.Printf("WorkDir: %s\n", p.WorkDir)
	fmt.Printf("Session: %s\n", session)
}
