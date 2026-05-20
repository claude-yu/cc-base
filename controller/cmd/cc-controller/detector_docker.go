package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var scienceImagePatterns = []string{
	"prosettac", "haddock", "colabfold", "rfdiffusion",
	"rosetta", "gromacs", "alphafold", "openmm", "schrodinger",
	"amber", "namd", "pymol", "chimera",
}

type dockerContainer struct {
	ID      string `json:"ID"`
	Image   string `json:"Image"`
	Names   string `json:"Names"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Command string `json:"Command"`
	Labels  string `json:"Labels"`
}

func isScienceImage(image string) bool {
	lower := strings.ToLower(image)
	for _, pat := range scienceImagePatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

func scanDockerContainers(root string) []ResearchStatus {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil
	}

	out, err := exec.Command("docker", "ps", "-a", "--format", "{{json .}}").Output()
	if err != nil {
		return nil
	}

	var containers []dockerContainer
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var c dockerContainer
		if json.Unmarshal([]byte(line), &c) == nil {
			containers = append(containers, c)
		}
	}

	var results []ResearchStatus
	for _, c := range containers {
		if !isScienceImage(c.Image) {
			continue
		}
		if root != "" {
			mount := extractBindMount(c.Labels)
			if mount == "" || !isUnderRoot(mount, root) {
				continue
			}
		}
		rs := inspectDockerContainer(c)
		results = append(results, rs)
	}

	return results
}

// isUnderRoot checks whether mount is a path under root (case-insensitive, Windows-aware).
func isUnderRoot(mount, root string) bool {
	if mount == "" || root == "" {
		return false
	}
	normMount := strings.ToLower(filepath.Clean(mount))
	normRoot := strings.ToLower(filepath.Clean(root))
	// Normalize forward slashes to backslashes (Windows paths)
	normMount = strings.ReplaceAll(normMount, "/", "\\")
	normRoot = strings.ReplaceAll(normRoot, "/", "\\")
	// Ensure root ends with separator so "work-9" doesn't match "work-91"
	if !strings.HasSuffix(normRoot, "\\") {
		normRoot += "\\"
	}
	return normMount == strings.TrimSuffix(normRoot, "\\") || strings.HasPrefix(normMount, normRoot)
}

func inspectDockerContainer(c dockerContainer) ResearchStatus {
	rs := ResearchStatus{
		Detector:   "docker:" + extractImageShort(c.Image),
		WorkDir:    extractBindMount(c.Labels),
		Confidence: "medium",
	}

	short := c.ID
	if len(short) > 12 {
		short = short[:12]
	}
	rs.Evidence = append(rs.Evidence, "容器: "+c.Names+" ("+short+")")
	rs.Evidence = append(rs.Evidence, "镜像: "+c.Image)

	cmd := strings.Trim(c.Command, "\"")
	if len(cmd) > 80 {
		cmd = cmd[:80] + "..."
	}
	rs.Evidence = append(rs.Evidence, "命令: "+cmd)

	switch c.State {
	case "running":
		rs.State = "running"
		rs.Evidence = append(rs.Evidence, "状态: "+c.Status)
	case "exited":
		exitCode := extractExitCode(c.Status)
		if exitCode == 0 {
			rs.State = "completed"
			rs.Evidence = append(rs.Evidence, "状态: 正常退出 (code 0), "+c.Status)
		} else {
			rs.State = "failed"
			rs.Confidence = "high"
			rs.Evidence = append(rs.Evidence, "状态: 异常退出 (code "+itoa(exitCode)+"), "+c.Status)
		}
	case "dead":
		rs.State = "failed"
		rs.Confidence = "high"
		rs.Evidence = append(rs.Evidence, "状态: dead — "+c.Status)
	default:
		rs.State = "unknown"
		rs.Confidence = "low"
		rs.Evidence = append(rs.Evidence, "状态: "+c.State+" — "+c.Status)
	}

	// Read container logs tail (only for running or recently exited)
	if c.State == "running" || c.State == "exited" {
		logTail := readDockerLogs(c.ID, 40)
		if len(logTail) > 0 {
			errHits := grepLines(logTail, []string{"error", "fatal", "traceback", "exception", "FAILED"})
			errHits = filterFalseErrors(errHits)
			for _, e := range errHits {
				if len(rs.Evidence) < 12 {
					rs.Evidence = append(rs.Evidence, "LOG-ERROR: "+e)
				}
			}
			if c.State == "running" {
				last := logTail[len(logTail)-1]
				if len(last) > 100 {
					last = last[:100] + "..."
				}
				rs.Evidence = append(rs.Evidence, "最新日志: "+last)
			}
		}
	}

	if rs.WorkDir != "" {
		rs.Evidence = append(rs.Evidence, "挂载目录: "+rs.WorkDir)
	}
	rs.KeyFiles = []string{c.Names}
	rs.LastUpdate = c.Status
	rs.LastUpdateMins = parseDockerAgeMins(c.Status)
	rs.Score = 50
	return rs
}

func parseDockerAgeMins(status string) int {
	// "Exited (255) 4 days ago" → strip "(255)" → parse "4 days" → 5760 min
	// "Up 5 hours" → 300 min
	cleaned := status
	for {
		start := strings.Index(cleaned, "(")
		if start < 0 {
			break
		}
		end := strings.Index(cleaned[start:], ")")
		if end < 0 {
			break
		}
		cleaned = cleaned[:start] + cleaned[start+end+1:]
	}
	lower := strings.ToLower(cleaned)
	n := 0
	for _, c := range lower {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else if n > 0 {
			break
		}
	}
	if n == 0 {
		return -1
	}
	if strings.Contains(lower, "minute") {
		return n
	}
	if strings.Contains(lower, "hour") {
		return n * 60
	}
	if strings.Contains(lower, "day") {
		return n * 24 * 60
	}
	if strings.Contains(lower, "week") {
		return n * 7 * 24 * 60
	}
	if strings.Contains(lower, "month") {
		return n * 30 * 24 * 60
	}
	return -1
}

func readDockerLogs(containerID string, lines int) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "logs", "--tail", itoa(lines), containerID).CombinedOutput()
	if err != nil {
		return nil
	}
	result := strings.Split(string(out), "\n")
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}
	return result
}

func extractImageShort(image string) string {
	// "ghcr.io/haddocking/haddock3:latest" → "haddock3"
	// "prosettac-local" → "prosettac-local"
	parts := strings.Split(image, "/")
	last := parts[len(parts)-1]
	if idx := strings.Index(last, ":"); idx > 0 {
		last = last[:idx]
	}
	return last
}

func extractBindMount(labels string) string {
	// Docker Desktop labels: desktop.docker.io/binds/0/Source=G:\proteinwork\...
	for _, label := range strings.Split(labels, ",") {
		if strings.Contains(label, "binds/") && strings.Contains(label, "/Source=") {
			if idx := strings.Index(label, "="); idx > 0 {
				return label[idx+1:]
			}
		}
	}
	return ""
}

func extractExitCode(status string) int {
	// "Exited (0) 2 days ago" → 0
	// "Exited (137) 4 days ago" → 137
	idx := strings.Index(status, "(")
	if idx < 0 {
		return -1
	}
	end := strings.Index(status[idx:], ")")
	if end < 0 {
		return -1
	}
	return atoiSafe(status[idx+1 : idx+end])
}

