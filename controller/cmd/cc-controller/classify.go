package main

import "strings"

type Mode int

const (
	ModeAdvice Mode = iota
	ModeReadonly
	ModeExecuteRequest
	ModeExecute
)

var modeNames = map[Mode]string{
	ModeAdvice:         "advice",
	ModeReadonly:       "readonly",
	ModeExecuteRequest: "execute_request",
	ModeExecute:        "execute",
}

func (m Mode) String() string {
	if s, ok := modeNames[m]; ok {
		return s
	}
	return "advice"
}

// ── Tier 1: Strong execute triggers ──────────────────────────
// These reliably indicate a code-execution intent.
var executeStrong = []string{
	"修改", "修复", "删除", "写入", "保存", "安装",
	"提交", "落地", "实施", "部署", "构建",
	"运行命令", "执行脚本", "帮我改", "改掉", "修了",
	"创建", "新建",
	"commit", "push", "deploy", "build",
}

// ── Tier 2: "生成" (special) ──────────────────────────────────
// "生成" is ambiguous in scientiﬁc contexts.
//   "生成文件在哪" → question about existing output → advice
//   "生成一个文件" → creation request → execute
// Decided by question-pattern detection in classifyMode.

// ── Tier 3: Weak execute words ────────────────────────────────
// Scientiﬁc-domain false-positive risk:
//   "运行" → "运行结果怎么看" (advice), "运行命令" → execute
//   "模拟" → "模拟结果分析" (advice)
//   "处理" → "数据处理流程" (advice)
//   "执行" → only triggers with action context (脚本/命令/测试/代码)
// These require a paired action context word to be classiﬁed as execute.
// When a question pattern is also present, weak-word execute is skipped.
var executeWeakActionWords = []string{
	"运行", "模拟", "处理", "执行",
}

var executeActionContext = []string{
	"命令", "脚本", "测试", "代码", "程序",
}

// adviceQuestionPatterns block weak-word execute classification.
// A question about the result/location/quality of something is advice,
// not a request to execute.
var adviceQuestionPatterns = []string{
	"怎么看", "在哪", "哪里", "质量如何", "是什么",
}

// ── Readonly keywords ────────────────────────────────────────
var readonlyKeywords = []string{
	"看一下", "查一下", "读取", "检查",
	"状态", "进度", "日志", "grep",
	"搜索", "列出", "读一下", "查看", "展示",
	"显示", "打开", "找了", "找到",
	"list", "show", "read", "check", "status", "log", "file",
}

func hasQuestionPattern(s string) bool {
	for _, q := range adviceQuestionPatterns {
		if strings.Contains(s, q) {
			return true
		}
	}
	return false
}

// classifyMode returns the most likely intent based on keyword matching.
// Priority: strong execute > "生成" special > weak execute > readonly > advice.
//
// Strong execute keywords win unconditionally.
// "生成" is treated as execute UNLESS the text asks about existing output
// (question pattern detected).
// Weak execute words only win when paired with an action context word AND
// no question pattern is present — this avoids false positives from
// scientiﬁc terminology (e.g. "运行结果怎么看", "模拟结果分析", "修饰蛋白").
func classifyMode(text string) Mode {
	lower := strings.ToLower(text)

	// Tier 1: strong execute keywords — immediate match
	for _, kw := range executeStrong {
		if strings.Contains(lower, kw) {
			return ModeExecuteRequest
		}
	}

	// Tier 2: "生成" — execute unless asking about existing output
	if strings.Contains(lower, "生成") && !hasQuestionPattern(lower) {
		return ModeExecuteRequest
	}

	// Tier 3: weak execute words require action context AND no question
	if !hasQuestionPattern(lower) {
		for _, weak := range executeWeakActionWords {
			if strings.Contains(lower, weak) {
				for _, ctx := range executeActionContext {
					if strings.Contains(lower, ctx) {
						return ModeExecuteRequest
					}
				}
			}
		}
	}

	// Readonly
	for _, kw := range readonlyKeywords {
		if strings.Contains(lower, kw) {
			return ModeReadonly
		}
	}

	return ModeAdvice
}
