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

// pastedOutputPatterns indicate the text is terminal/error output pasted
// by the user, not an execute intent. E.g. "不是可运行的程序" from Windows.
var pastedOutputPatterns = []string{
	"不是内部或外部命令", "不是可运行的程序",
	"command not found", "is not recognized",
}

// ── Text-edit context ────────────────────────────────────────
// When an ambiguous execute keyword ("帮我改", "修改", "改掉") appears
// alongside text-editing words, it's a writing request, not code execution.
var textEditContextWords = []string{
	"句话", "句子", "这句", "文字", "文章", "翻译", "措辞",
}

func hasTextEditContext(s string) bool {
	for _, w := range textEditContextWords {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// ── Readonly keywords ────────────────────────────────────────
var readonlyKeywords = []string{
	"看一下", "查一下", "读取", "检查",
	"日志", "grep",
	"搜索", "列出", "读一下", "查看", "展示",
	"显示", "打开", "找了", "找到",
	"运行情况", "运行状态", "任务状态", "任务进度",
	"查看结果", "查看进度", "查看日志",
	"看看结果", "看看进度", "看看状态", "看看日志",
	"list", "show", "read", "check", "status", "log", "file", "result",
}

func hasQuestionPattern(s string) bool {
	for _, q := range adviceQuestionPatterns {
		if strings.Contains(s, q) {
			return true
		}
	}
	return false
}

func hasPastedOutput(s string) bool {
	for _, p := range pastedOutputPatterns {
		if strings.Contains(s, p) {
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
	// Ambiguous keywords ("帮我改", "修改", "改掉") are skipped when
	// the text contains text-editing context (句子, 文章, etc.).
	for _, kw := range executeStrong {
		if strings.Contains(lower, kw) {
			if (kw == "帮我改" || kw == "修改" || kw == "改掉") && hasTextEditContext(lower) {
				continue
			}
			return ModeExecuteRequest
		}
	}

	// Tier 2: "生成" — execute unless asking about existing output
	if strings.Contains(lower, "生成") && !hasQuestionPattern(lower) {
		return ModeExecuteRequest
	}

	// Tier 3: weak execute words require action context AND no question
	// Also skip when text looks like pasted terminal output.
	if !hasQuestionPattern(lower) && !hasPastedOutput(lower) {
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
