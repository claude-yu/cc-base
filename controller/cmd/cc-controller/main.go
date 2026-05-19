package main

import (
	"fmt"
	"os"
	"regexp"
)

const timeFormat = "20060102-150405"
const maxContextTurns = 10
const maxContextBytes = 20 * 1024

// runIDPattern matches genRunID output: yyyyMMdd-HHmmss-<suffix>[-hex].
var runIDPattern = regexp.MustCompile(`^\d{8}-\d{6}-`)

type statusJSON struct {
	RunID        string `json:"run_id"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	Stage        string `json:"stage"`
	SessionID    string `json:"session_id,omitempty"`
	SessionScope string `json:"session_scope,omitempty"`
	StartedAt    string `json:"started_at"`
	UpdatedAt    string `json:"updated_at"`
	PID          int    `json:"pid,omitempty"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	Error        string `json:"error,omitempty"`
}

type eventEntry struct {
	Ts         string `json:"ts"`
	RunID      string `json:"run_id"`
	Type       string `json:"type"`
	Stage      string `json:"stage,omitempty"`
	ElapsedSec int    `json:"elapsed_sec,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	Message    string `json:"message,omitempty"`
}

type transcriptEntry struct {
	Ts    string `json:"ts"`
	RunID string `json:"run_id"`
	Role  string `json:"role"`
	Text  string `json:"text"`
}

type sessionMeta struct {
	SessionID    string `json:"session_id"`
	SessionScope string `json:"session_scope"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	LastRunID    string `json:"last_run_id"`
	TurnCount    int    `json:"turn_count"`
	Mode         string `json:"mode"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	root := resolveControllerRoot()
	args := os.Args[2:]

	switch cmd {
	case "ask-cc":
		cmdAsk(root, "cc-ask", "run-cc", args)
	case "ask-codex":
		cmdAsk(root, "codex-ask", "run-codex", args)
	case "exec-cc":
		mustHaveArgs(args, 1, "usage: cc-controller exec-cc --text <msg> [--session <id>] [--auto]")
		cmdExecCC(root, args)
	case "run-cc":
		mustHaveArgs(args, 1, "usage: cc-controller run-cc <RunId> [--session <SessionId>] [--mode <mode>]")
		runID := args[0]
		sessionID := ""
		mode := "advice"
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--session":
				i++
				if i < len(args) {
					sessionID = args[i]
				}
			case "--mode":
				i++
				if i < len(args) {
					mode = args[i]
				}
			}
		}
		runCC(root, runID, sessionID, mode)
	case "run-codex":
		mustHaveArgs(args, 1, "usage: cc-controller run-codex <RunId>")
		runCodex(root, args[0])
	case "execute":
		mustHaveArgs(args, 1, "usage: cc-controller execute RunId")
		cmdExecute(root, args[0])
	case "show":
		runID := ""
		if len(args) >= 1 {
			if !runIDPattern.MatchString(args[0]) {
				showRun(root, "", args[0])
				return
			}
			runID = args[0]
		}
		showRun(root, runID, "")
	case "cancel":
		if len(args) >= 1 && args[0] != "" {
			cancelTask(root, args[0])
		} else {
			cancelLatest(root)
		}
	case "project":
		cmdProject(root)
	case "status":
		cmdStatus(root)
	case "switch":
		mustHaveArgs(args, 1, "usage: cc-controller switch <project-name|path>")
		cmdSwitchProject(root, args[0])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `cc-controller — async ask/callback pipeline

Commands:
  ask-cc <text>           Ask Claude Code asynchronously
  ask-codex <text>        Ask Codex asynchronously
  exec-cc --text <msg>    Session-aware CC execution (with heartbeat/callback)
           [--session <id>]
           [--auto]       Auto-classify mode (advice/readonly/execute_request)
  run-cc <RunId>          Background runner for ask-cc
           [--session <SessionId>]
           [--mode <mode>]  Mode: advice (default), readonly, execute
  run-codex <RunId>       Background runner for ask-codex
  execute RunId           Execute a confirmed task (full tool access)
  show [RunId|kind]       Show run result
  cancel [RunId]          Cancel a running task (omit RunId to cancel latest)
  project                 Show active project info
  switch <name|path>      Switch to another project`)
	os.Exit(1)
}
