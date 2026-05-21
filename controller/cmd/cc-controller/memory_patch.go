package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func resolveMemoryStore(args []string) string {
	if dir := os.Getenv("CC_MEMORY_STORE"); dir != "" {
		return dir
	}
	for i, a := range args {
		if a == "--dir" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func cmdMemoryPatch(root string, args []string) {
	memDir := resolveMemoryStore(args)
	if memDir == "" {
		fmt.Fprintln(os.Stderr, "用法: cc-controller memory-draft patch <target-name> --dir <memory-dir>")
		fmt.Fprintln(os.Stderr, "  或设置 CC_MEMORY_STORE 环境变量")
		os.Exit(1)
	}

	target := ""
	for _, a := range args {
		if a != "--dir" && !strings.HasPrefix(a, "-") && a != memDir {
			target = a
			break
		}
	}
	if target == "" {
		fmt.Fprintln(os.Stderr, "缺少 target-name 参数")
		os.Exit(1)
	}

	if coreMemoryNames[target] {
		fmt.Fprintf(os.Stderr, "拒绝: %q 是 Core Memory，不允许自动修改\n", target)
		os.Exit(1)
	}
	if governanceNames[target] {
		fmt.Fprintf(os.Stderr, "拒绝: %q 是 Governance 文件，不允许自动修改\n", target)
		os.Exit(1)
	}

	targetFile := findMemoryFile(memDir, target)
	if targetFile == "" {
		fmt.Fprintf(os.Stderr, "找不到 memory 文件: %q\n", target)
		os.Exit(1)
	}

	content, err := os.ReadFile(targetFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取失败: %v\n", err)
		os.Exit(1)
	}

	name, _, memType := parseFrontmatter(string(content))
	if name == "" {
		name = target
	}
	scope := "unknown"
	if s, ok := scopeMap[name]; ok {
		scope = s
	}
	lines := strings.Count(string(content), "\n") + 1

	runID := genRunID("memory-patch")
	runDir := filepath.Join(root, "runs", runID)
	os.MkdirAll(runDir, 0755)

	now := time.Now().UTC().Format(time.RFC3339)
	writeStatusJSON(runDir, statusJSON{
		RunID: runID, Kind: "memory-patch", Status: "running",
		Stage: "generating", StartedAt: now, UpdatedAt: now,
	})

	sendCallback(runDir, fmt.Sprintf("⏳ 生成 %s 的 memory patch\nRun ID: %s", target, runID))

	systemPrompt := patchSystemPrompt()
	userPrompt := patchUserPrompt(name, memType, scope, lines, string(content))

	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		start := time.Now()
		for {
			select {
			case <-ticker.C:
				elapsed := int(time.Since(start).Seconds())
				sendCallback(runDir, heartbeatMsg("patch 生成", runID, elapsed, ""))
			case <-done:
				return
			}
		}
	}()

	resp, err := doReviewCall([]chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, CodexBackendGLM)

	if err != nil {
		writeError(runDir, err)
		sendCallback(runDir, fmt.Sprintf("❌ Patch 生成失败: %s", truncate(err.Error(), 200)))
		fmt.Fprintf(os.Stderr, "❌ Patch 生成失败: %s\n", err.Error())
		os.Exit(1)
	}

	os.WriteFile(filepath.Join(runDir, "patch-raw.md"), []byte(resp), 0600)
	os.WriteFile(filepath.Join(runDir, "target-name.txt"), []byte(target), 0644)
	os.WriteFile(filepath.Join(runDir, "target-original.md"), content, 0600)
	os.WriteFile(filepath.Join(runDir, "memory-dir.txt"), []byte(memDir), 0644)

	newContent := extractPatchContent(resp)
	if newContent == "" {
		msg := "GLM 未返回有效的新内容块"
		writeError(runDir, fmt.Errorf("%s", msg))
		sendCallback(runDir, fmt.Sprintf("❌ %s\n查看原始输出: runs/%s/patch-raw.md", msg, runID))
		fmt.Fprintf(os.Stderr, "❌ %s\n", msg)
		os.Exit(1)
	}
	os.WriteFile(filepath.Join(runDir, "patch-new.md"), []byte(newContent), 0600)

	summary := patchSummary(target, lines, strings.Count(newContent, "\n")+1, runID)
	os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(summary), 0644)

	writeStatusJSON(runDir, statusJSON{
		RunID: runID, Kind: "memory-patch", Status: "pending_review",
		Stage: "awaiting review", StartedAt: now, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	sendCallback(runDir, summary)
	fmt.Println(summary)
}

func cmdMemoryPatchReview(root string, args []string) {
	if len(args) == 0 || args[0] == "" {
		fmt.Fprintln(os.Stderr, "用法: cc-controller memory-draft review-patch <runID>")
		os.Exit(1)
	}
	runID := args[0]
	runDir := filepath.Join(root, "runs", runID)

	original, err := os.ReadFile(filepath.Join(runDir, "target-original.md"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "找不到 run: %s\n", runID)
		os.Exit(1)
	}
	proposed, err := os.ReadFile(filepath.Join(runDir, "patch-new.md"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "找不到 patch: %s\n", runID)
		os.Exit(1)
	}
	targetBytes, _ := os.ReadFile(filepath.Join(runDir, "target-name.txt"))
	target := strings.TrimSpace(string(targetBytes))

	sendCallback(runDir, fmt.Sprintf("🔍 DeepSeek 审查 %s patch 中...\nRun ID: %s", target, runID))

	systemPrompt := reviewPatchSystemPrompt()
	userPrompt := reviewPatchUserPrompt(target, string(original), string(proposed))

	resp, err := doReviewCall([]chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, CodexBackendDeepSeek)

	if err != nil {
		writeError(runDir, err)
		sendCallback(runDir, fmt.Sprintf("❌ 审查失败: %s", truncate(err.Error(), 200)))
		fmt.Fprintf(os.Stderr, "❌ 审查失败: %s\n", err.Error())
		os.Exit(1)
	}

	os.WriteFile(filepath.Join(runDir, "review-result.md"), []byte(resp), 0644)

	verdict := extractVerdict(resp)
	writeStatusJSON(runDir, statusJSON{
		RunID: runID, Kind: "memory-patch", Status: "reviewed",
		Stage: verdict, StartedAt: "", UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	summary := fmt.Sprintf("🔍 审查完成: %s\n目标: %s\n判定: %s\n\n%s\n\n下一步: cc-controller memory-draft apply %s",
		runID, target, verdict, truncate(resp, 500), runID)
	sendCallback(runDir, summary)
	fmt.Println(summary)
}

func cmdMemoryApply(root string, args []string) {
	if len(args) == 0 || args[0] == "" {
		fmt.Fprintln(os.Stderr, "用法: cc-controller memory-draft apply <runID>")
		os.Exit(1)
	}
	runID := args[0]
	runDir := filepath.Join(root, "runs", runID)

	if err := applyMemoryPatch(runDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func applyMemoryPatch(runDir string) error {
	targetBytes, err := os.ReadFile(filepath.Join(runDir, "target-name.txt"))
	if err != nil {
		return fmt.Errorf("找不到 run: %s", filepath.Base(runDir))
	}
	target := strings.TrimSpace(string(targetBytes))

	if coreMemoryNames[target] || governanceNames[target] {
		return fmt.Errorf("拒绝: %q 是受保护文件", target)
	}

	// ── Gate 1: require approved review ──
	statusPath := filepath.Join(runDir, "status.json")
	statusData, err := os.ReadFile(statusPath)
	if err != nil {
		return fmt.Errorf("无法读取 status.json: %v", err)
	}
	var st statusJSON
	if err := json.Unmarshal(statusData, &st); err != nil {
		return fmt.Errorf("status.json 格式错误: %v", err)
	}
	if st.Stage != "approved" {
		return fmt.Errorf("拒绝: 补丁未通过审查 (当前状态: %s)。需先运行 review-patch 且判定为 approved", st.Stage)
	}

	memDirBytes, _ := os.ReadFile(filepath.Join(runDir, "memory-dir.txt"))
	memDir := strings.TrimSpace(string(memDirBytes))
	if memDir == "" {
		return fmt.Errorf("无法确定 memory 目录")
	}

	newContent, err := os.ReadFile(filepath.Join(runDir, "patch-new.md"))
	if err != nil {
		return fmt.Errorf("找不到 patch 内容: %s", filepath.Base(runDir))
	}

	targetFile := findMemoryFile(memDir, target)
	if targetFile == "" {
		return fmt.Errorf("找不到目标文件: %q", target)
	}

	originalContent, err := os.ReadFile(targetFile)
	if err != nil {
		return fmt.Errorf("读取原文件失败: %v", err)
	}

	// ── Gate 2: stale patch check ──
	snapshotContent, err := os.ReadFile(filepath.Join(runDir, "target-original.md"))
	if err != nil {
		return fmt.Errorf("找不到原始快照 target-original.md: %v", err)
	}
	if string(originalContent) != string(snapshotContent) {
		return fmt.Errorf("目标文件在 patch 生成后已被修改，请重新生成 patch")
	}

	// ── Gate 3: frontmatter validation ──
	if err := validatePatchFrontmatter(string(originalContent), string(newContent), target); err != nil {
		return fmt.Errorf("frontmatter 校验失败: %v", err)
	}

	// ── Atomic lock ──
	lockPath := filepath.Join(memDir, ".memory.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			lockData, _ := os.ReadFile(lockPath)
			var lock struct {
				Agent   string `json:"agent"`
				Created string `json:"created"`
				TTL     int    `json:"ttl_seconds"`
			}
			if json.Unmarshal(lockData, &lock) == nil {
				if created, e := time.Parse(time.RFC3339, lock.Created); e == nil {
					if time.Since(created) < time.Duration(lock.TTL)*time.Second {
						return fmt.Errorf("锁定中: agent=%s, created=%s", lock.Agent, lock.Created)
					}
				}
			}
			os.Remove(lockPath)
			lockFile, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return fmt.Errorf("获取锁失败: %v", err)
			}
		} else {
			return fmt.Errorf("获取锁失败: %v", err)
		}
	}
	lockJSON, _ := json.Marshal(map[string]interface{}{
		"agent":       "writer",
		"operation":   "apply",
		"ttl_seconds": 300,
		"created":     time.Now().UTC().Format(time.RFC3339),
	})
	lockFile.Write(lockJSON)
	lockFile.Close()
	defer os.Remove(lockPath)

	// ── Backup ──
	backupDir := filepath.Join(memDir, ".backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("创建备份目录失败: %v", err)
	}
	backupName := fmt.Sprintf("%s.%s.md", filepath.Base(targetFile), time.Now().Format("20060102-150405"))
	if err := os.WriteFile(filepath.Join(backupDir, backupName), originalContent, 0600); err != nil {
		return fmt.Errorf("备份失败: %v", err)
	}

	// ── Apply ──
	if err := os.WriteFile(targetFile, newContent, 0644); err != nil {
		return fmt.Errorf("写入失败: %v", err)
	}

	// ── Audit ──
	auditErr := writeAuditEntry(filepath.Join(memDir, ".audit-log.jsonl"), map[string]interface{}{
		"ts":        time.Now().UTC().Format(time.RFC3339),
		"agent":     "writer",
		"op":        "apply-patch",
		"target":    target,
		"run_id":    filepath.Base(runDir),
		"old_lines": strings.Count(string(originalContent), "\n") + 1,
		"new_lines": strings.Count(string(newContent), "\n") + 1,
		"backup":    backupName,
	})

	stage := "applied"
	if auditErr != nil {
		stage = "applied_audit_failed"
		fmt.Fprintf(os.Stderr, "⚠ audit 写入失败: %v\n", auditErr)
	}

	writeStatusJSON(runDir, statusJSON{
		RunID:  filepath.Base(runDir),
		Kind:   "memory-patch",
		Status: "completed",
		Stage:  stage,
	})
	setExitCode(runDir, 0)

	oldLines := strings.Count(string(originalContent), "\n") + 1
	newLines := strings.Count(string(newContent), "\n") + 1
	msg := fmt.Sprintf("✅ Patch 已应用\n目标: %s (%d行 → %d行)\nBackup: .backups/%s\nRun ID: %s",
		target, oldLines, newLines, backupName, filepath.Base(runDir))
	sendCallback(runDir, msg)
	fmt.Println(msg)
	return nil
}

// validatePatchFrontmatter ensures the proposed content preserves critical frontmatter fields.
func validatePatchFrontmatter(original, proposed, expectedName string) error {
	origName, _, origType := parseFrontmatter(original)
	newName, _, newType := parseFrontmatter(proposed)

	if newName == "" {
		return fmt.Errorf("proposed 内容缺少 name 字段")
	}

	if origName != "" && newName != origName {
		return fmt.Errorf("name 被修改: %q → %q", origName, newName)
	}

	if newName != expectedName {
		return fmt.Errorf("name 与目标不匹配: proposed=%q, target=%q", newName, expectedName)
	}

	if origType != "" && newType != origType {
		return fmt.Errorf("metadata.type 被修改: %q → %q", origType, newType)
	}

	if newType == "" && origType != "" {
		return fmt.Errorf("metadata.type 被删除 (原值: %q)", origType)
	}

	if !strings.HasPrefix(strings.TrimSpace(proposed), "---") {
		return fmt.Errorf("proposed 内容缺少 YAML frontmatter")
	}

	// Block layer escalation: proposed content must not claim core/governance
	proposedLower := strings.ToLower(proposed)
	if strings.Contains(proposedLower, "layer: core") || strings.Contains(proposedLower, "layer: governance") {
		if !strings.Contains(strings.ToLower(original), "layer: core") && !strings.Contains(strings.ToLower(original), "layer: governance") {
			return fmt.Errorf("proposed 尝试将 layer 提升为 core/governance")
		}
	}

	return nil
}

func writeAuditEntry(auditPath string, entry map[string]interface{}) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal: %v", err)
	}
	f, err := os.OpenFile(auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open: %v", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("write: %v", err)
	}
	return f.Close()
}

func findMemoryFile(memDir, name string) string {
	for _, prefix := range []string{"feedback_", "project_", "user_", "reference_", ""} {
		candidate := filepath.Join(memDir, prefix+name+".md")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func extractPatchContent(resp string) string {
	markers := []string{"```markdown", "```md", "```"}
	for _, marker := range markers {
		idx := strings.Index(resp, marker)
		if idx < 0 {
			continue
		}
		start := idx + len(marker)
		if start < len(resp) && resp[start] == '\n' {
			start++
		}
		end := strings.Index(resp[start:], "```")
		if end < 0 {
			continue
		}
		return strings.TrimSpace(resp[start : start+end])
	}

	if strings.HasPrefix(strings.TrimSpace(resp), "---") {
		return strings.TrimSpace(resp)
	}
	return ""
}

var verdictPattern = regexp.MustCompile(`(?i)(?:判定|verdict|decision)\s*[:：]\s*(APPROVE|REJECT|NEEDS_REVISION|通过|拒绝|不通过)`)

func extractVerdict(resp string) string {
	m := verdictPattern.FindStringSubmatch(resp)
	if m == nil {
		return "needs_revision"
	}
	v := strings.ToLower(m[1])
	switch {
	case v == "approve" || v == "通过":
		return "approved"
	case v == "reject" || v == "拒绝" || v == "不通过":
		return "rejected"
	default:
		return "needs_revision"
	}
}

func patchSummary(target string, oldLines, newLines int, runID string) string {
	delta := oldLines - newLines
	return fmt.Sprintf("📝 Patch 已生成\n目标: %s (%d行 → %d行, -%d)\nRun ID: %s\n\n下一步:\n1. 查看: runs/%s/patch-new.md\n2. 审查: cc-controller memory-draft review-patch %s\n3. 应用: cc-controller memory-draft apply %s",
		target, oldLines, newLines, delta, runID, runID, runID, runID)
}

func patchSystemPrompt() string {
	return `你是 Memory Maintenance 系统的补丁生成器。你的任务是精简 memory 文件，降低噪声，同时保留所有关键信息。

保留规则（必须保留）:
- YAML frontmatter (name, description, metadata) 保持不变
- 核心约束和决策（Why / How to apply 段落）
- 当前活跃状态描述
- 关联引用 [[name]]

移除/压缩规则:
- 可从代码或 git 推导的信息（完整文件路径列表、部署命令步骤）
- 历史演进细节（旧版本描述、时间线，压缩为一句话）
- 重复信息（同一约束出现多次）
- 过于具体的实现细节（行号引用、函数签名）

输出格式:
直接输出精简后的完整文件内容，用 markdown 代码块包裹:

` + "```markdown\n---\nname: ...\n---\n精简后的内容\n```" + `

不要解释你做了什么改动，只输出精简后的文件。`
}

func patchUserPrompt(name, memType, scope string, lines int, content string) string {
	return fmt.Sprintf(`目标文件: %s
类型: %s
范围: %s
当前行数: %d

当前内容:
---
%s
---

请精简此文件。目标: 保留关键决策和约束，移除可推导的细节，行数降低 30-50%%。`, name, memType, scope, lines, content)
}

func reviewPatchSystemPrompt() string {
	return `你是 Memory Maintenance 系统的审查员。你的任务是审查一个 memory 文件的修改补丁。

审查标准:
1. 信息完整性: 关键约束、决策理由（Why）、应用指导（How to apply）是否保留？
2. Frontmatter: name/description/metadata 是否完整且未被修改？
3. 安全性: 是否误删了安全相关约束？
4. 关联: [[name]] 引用是否保留？
5. 精简质量: 移除的内容确实是可推导的或历史性的吗？

输出格式（必须严格遵循）:
第一行必须是:
判定: APPROVE
或
判定: REJECT
或
判定: NEEDS_REVISION

然后:
理由: 1-3 句话
如果 REJECT/NEEDS_REVISION，列出具体问题`
}

func reviewPatchUserPrompt(target, original, proposed string) string {
	return fmt.Sprintf(`目标: %s

原始内容 (%d 行):
---
%s
---

修改后内容 (%d 行):
---
%s
---

请审查此修改。`, target,
		strings.Count(original, "\n")+1, original,
		strings.Count(proposed, "\n")+1, proposed)
}
