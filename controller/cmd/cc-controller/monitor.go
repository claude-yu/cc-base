package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	monitorAlertCooldown = 5 * time.Minute
	stuckWarnThreshold   = 5 * time.Minute
	stuckCritThreshold   = 10 * time.Minute
)

// cmdMonitor scans running tasks and detects stuck/zombie state.
// - Zombie: process dead but status still "running" → auto-mark failed + callback
// - Stuck: alive but no event update in 5+ min → send warning callback
// - Long-running: alive 10+ min → send critical callback with cancel hint
func cmdMonitor(root string) {
	runsRoot := filepath.Join(root, "runs")
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		fmt.Println("没有 runs 目录")
		return
	}

	now := time.Now()
	var alerts []string
	zombieCount, stuckCount := 0, 0

	for _, e := range entries {
		if !e.IsDir() || !runIDPattern.MatchString(e.Name()) {
			continue
		}
		runDir := filepath.Join(runsRoot, e.Name())
		s := readStatusJSON(runDir)

		if s.Status != "running" && s.Status != "accepted" {
			continue
		}

		pidAlive := false
		if s.PID > 0 {
			pidAlive = isProcessRunning(s.PID)
		}

		ts := s.UpdatedAt
		if ts == "" {
			ts = s.StartedAt
		}
		if ts == "" {
			continue
		}
		updated, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		elapsed := now.Sub(updated)

		if !pidAlive && s.PID > 0 {
			cleanupZombie(runDir, e.Name(), s, elapsed)
			zombieCount++
			alerts = append(alerts, fmt.Sprintf("☠️ %s: 进程 PID %d 已死，已标记失败 (闲置 %s)", e.Name(), s.PID, fmtDuration(elapsed)))
			continue
		}

		if pidAlive && elapsed > stuckCritThreshold {
			if !recentMonitorAlert(runDir) {
				writeMonitorAlert(runDir)
				msg := fmt.Sprintf("⚠️ 任务运行超 %d 分钟 (RunId: %s)\n可能卡住，建议取消:\n/取消任务 %s",
					int(elapsed.Minutes()), e.Name(), e.Name())
				sendCallback(runDir, msg)
				stuckCount++
				alerts = append(alerts, fmt.Sprintf("⚠️ %s: 运行 %d 分钟，已发送告警", e.Name(), int(elapsed.Minutes())))
			} else {
				alerts = append(alerts, fmt.Sprintf("⏳ %s: 运行 %d 分钟 (告警已发，冷却中)", e.Name(), int(elapsed.Minutes())))
			}
		} else if pidAlive && elapsed > stuckWarnThreshold {
			if !recentMonitorAlert(runDir) {
				writeMonitorAlert(runDir)
				msg := fmt.Sprintf("⏳ 任务已运行 %d 分钟 (RunId: %s)\n如需取消: /取消任务 %s",
					int(elapsed.Minutes()), e.Name(), e.Name())
				sendCallback(runDir, msg)
				stuckCount++
				alerts = append(alerts, fmt.Sprintf("⏳ %s: 运行 %d 分钟，已发送提醒", e.Name(), int(elapsed.Minutes())))
			}
		} else if pidAlive {
			alerts = append(alerts, fmt.Sprintf("✓ %s: 正常运行中 (%s)", e.Name(), fmtDuration(elapsed)))
		}
	}

	if len(alerts) == 0 {
		fmt.Println("没有运行中的任务")
	} else {
		fmt.Printf("监控结果: %d 僵尸, %d 告警, %d 总检查\n\n", zombieCount, stuckCount, len(alerts))
		for _, a := range alerts {
			fmt.Println(a)
		}
	}
}

func cleanupZombie(runDir, runID string, s statusJSON, elapsed time.Duration) {
	errMsg := fmt.Sprintf("进程 PID %d 已退出但状态仍为 %s (闲置 %s)", s.PID, s.Status, fmtDuration(elapsed))
	os.WriteFile(filepath.Join(runDir, "summary.md"), []byte("[Monitor] "+errMsg), 0644)
	updateStatusJSON(runDir, "failed", "zombie_cleaned", 0)
	setExitCode(runDir, 1)
	appendEvent(runDir, eventEntry{
		Ts:      time.Now().UTC().Format(time.RFC3339),
		RunID:   runID,
		Type:    "monitor_cleanup",
		Message: errMsg,
	})
	callbackMsg := fmt.Sprintf("[Monitor] 任务异常终止 (RunId: %s)\n%s\n可能原因: 进程崩溃/被杀/OOM\n/cc结果 %s 查看最后状态", runID, errMsg, runID)
	sendCallback(runDir, callbackMsg)
}

func recentMonitorAlert(runDir string) bool {
	data, err := os.ReadFile(filepath.Join(runDir, "last-monitor-alert.txt"))
	if err != nil {
		return false
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}
	return time.Since(t) < monitorAlertCooldown
}

func writeMonitorAlert(runDir string) {
	os.WriteFile(filepath.Join(runDir, "last-monitor-alert.txt"),
		[]byte(time.Now().UTC().Format(time.RFC3339)), 0644)
}

func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
