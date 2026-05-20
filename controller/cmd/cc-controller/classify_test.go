package main

import "testing"

func TestClassifyMode(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  Mode
	}{
		// Tier 1: strong execute
		{"strong:帮我改", "帮我改这个文件", ModeExecuteRequest},
		{"strong:修复bug", "修复bug", ModeExecuteRequest},
		{"strong:修了", "修了", ModeExecuteRequest},
		{"strong:删除", "删除临时文件", ModeExecuteRequest},
		{"strong:写入", "写入配置文件", ModeExecuteRequest},
		{"strong:保存", "保存修改", ModeExecuteRequest},
		{"strong:提交", "提交代码", ModeExecuteRequest},
		{"strong:安装", "安装依赖", ModeExecuteRequest},
		{"strong:落地", "落地B方案", ModeExecuteRequest},
		{"strong:部署", "部署到服务器", ModeExecuteRequest},
		{"strong:创建文件夹", "创建一个工作文件夹", ModeExecuteRequest},
		{"strong:新建项目", "新建一个项目", ModeExecuteRequest},

		// "生成" special: execute unless question
		{"生成:文件创建", "生成一个文件", ModeExecuteRequest},
		{"生成:报告", "生成报告", ModeExecuteRequest},
		{"生成:脚本", "生成脚本", ModeExecuteRequest},
		{"生成:文件在哪(question)", "生成文件在哪", ModeAdvice},
		{"生成:结果怎么看(question)", "生成结果怎么看", ModeAdvice},
		{"生成:火山图怎么看(question)", "生成的火山图怎么看", ModeAdvice},

		// Tier 3: weak words + action context = execute
		{"weak+ctx:运行命令", "运行命令", ModeExecuteRequest},
		{"weak+ctx:运行测试", "运行测试", ModeExecuteRequest},
		{"weak+ctx:执行脚本", "执行脚本", ModeExecuteRequest},
		{"weak+ctx:执行命令", "执行命令", ModeExecuteRequest},
		{"weak+ctx:运行代码", "运行代码", ModeExecuteRequest},

		// Tier 3: weak words alone = advice (scientific safety)
		{"weak+sci:运行GSEA结果", "运行GSEA结果怎么看", ModeAdvice},
		{"weak+sci:模拟结果", "模拟结果分析", ModeAdvice},
		{"weak+sci:处理单细胞", "处理后的单细胞对象质量如何", ModeAdvice},
		{"weak+sci:运行结果", "运行结果怎么看", ModeAdvice},
		{"weak+sci:模拟结果2", "模拟结果", ModeAdvice},
		{"weak+sci:处理流程", "数据处理流程", ModeAdvice},

		// Scientific protection: 修/修饰 not execute
		{"sci:修饰蛋白", "修饰蛋白", ModeAdvice},
		{"sci:修饰位点", "修饰位点", ModeAdvice},
		{"sci:表达上调", "表达上调", ModeAdvice},
		{"sci:表达下调", "表达下调", ModeAdvice},

		// Readonly
		{"ro:查看进度", "查看进度", ModeReadonly},
		{"ro:读日志", "读一下日志", ModeReadonly},
		{"ro:检查状态", "检查状态", ModeReadonly},
		{"ro:grep", "grep 错误", ModeReadonly},
		{"ro:list", "list files", ModeReadonly},
		{"ro:show", "show results", ModeReadonly},
		{"ro:读取", "读取文件", ModeReadonly},

		// New readonly compound phrases
		{"ro:看看结果", "看看结果", ModeReadonly},
		{"ro:看看进度", "看看进度", ModeReadonly},
		{"ro:运行情况", "运行情况", ModeReadonly},
		{"ro:运行状态", "运行状态", ModeReadonly},
		{"ro:任务状态", "任务状态", ModeReadonly},
		{"ro:查看结果", "查看结果", ModeReadonly},
		{"ro:result", "show me result", ModeReadonly},

		// Compound phrases still work after broad-word removal
		{"ro:运行状态2", "看看运行状态", ModeReadonly},
		{"ro:任务进度2", "当前任务进度", ModeReadonly},
		{"ro:看看状态2", "看看状态怎么样", ModeReadonly},

		// Safety: broad words alone should NOT trigger readonly
		{"advice:看看方案", "看看这个方案怎么样", ModeAdvice},
		{"advice:GSEA结果", "GSEA结果怎么看", ModeAdvice},
		{"advice:分析结果", "分析结果", ModeAdvice},

		// P1 tightening: "状态"/"进度" removed from readonlyKeywords
		{"advice:状态机", "状态机是什么", ModeAdvice},
		{"advice:进度条", "进度条怎么写", ModeAdvice},
		{"advice:状态转移", "状态转移图怎么画", ModeAdvice},
		{"advice:进度安排", "进度安排建议", ModeAdvice},

		// Negative: casual chat / non-science context
		{"advice:颜色怎样", "这个颜色怎么样", ModeAdvice},
		{"advice:名字怎样", "这个名字怎么样", ModeAdvice},
		{"advice:闲聊你好", "你好", ModeAdvice},
		{"advice:解释概念", "什么是分子动力学", ModeAdvice},
		{"advice:PROTAC方案", "PROTAC方案怎么设计", ModeAdvice},
		{"advice:markdown", "帮我写个md文件", ModeAdvice},
		{"advice:模拟解释", "模拟退火是什么意思", ModeAdvice},

		// Negative: md/系统/科研 broad words should not trigger readonly
		{"advice:md格式", "md格式怎么写", ModeAdvice},
		{"advice:markdown表格", "markdown表格怎么写", ModeAdvice},
		{"advice:系统性狼疮", "系统性红斑狼疮是什么", ModeAdvice},
		{"advice:科研概念", "科研是什么意思", ModeAdvice},
		{"advice:电影", "这个电影怎么样", ModeAdvice},
		{"advice:结果没文件", "这个结果怎么看没贴文件", ModeAdvice},

		// Negative: result location queries → advice (handled by isResultLocationQuery, not classifyMode)
		{"advice:结果在哪", "结果在哪", ModeAdvice},
		{"advice:文件在哪", "文件在哪", ModeAdvice},
		{"advice:输出在哪", "输出在哪", ModeAdvice},

		// Negative: text editing ≠ code execution
		{"advice:改句话", "帮我改一下这句话", ModeAdvice},
		{"advice:修改句子", "修改这个句子", ModeAdvice},
		{"advice:改掉措辞", "改掉这个措辞", ModeAdvice},
		{"advice:改文章", "帮我改一下这篇文章", ModeAdvice},

		// Negative: pasted terminal output ≠ execute
		{"advice:cmd-error-cn", "'Get-Content' 不是内部或外部命令，也不是可运行的程序或批处理文件。", ModeAdvice},
		{"advice:cmd-error-en", "bash: gromacs: command not found", ModeAdvice},
		{"advice:cmd-not-recognized", "'gmx' is not recognized as an internal or external command", ModeAdvice},

		// Regression: execute intent preserved
		{"exec:生成结果不变", "生成结果", ModeExecuteRequest},
		{"exec:帮我改文件", "帮我改这个文件", ModeExecuteRequest},
		{"exec:修改代码", "修改代码逻辑", ModeExecuteRequest},
		{"exec:改掉bug", "改掉这个bug", ModeExecuteRequest},
		{"exec:运行命令保留", "运行命令", ModeExecuteRequest},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyMode(c.input); got != c.want {
				t.Errorf("classifyMode(%q) = %s, want %s", c.input, got, c.want)
			}
		})
	}
}

func TestIsStatusQuery(t *testing.T) {
	positive := []string{
		"查看状态", "看看结果", "结果如何", "任务怎么样了",
		"运行情况", "跑到哪了", "看看进度", "看看状态",
		"进度如何", "现在怎么样了", "进展怎么样了",
		"md进度", "模拟进度", "gromacs状态",
		"科研任务", "科研监控", "研究进度", "任务监控",
		"系统状态", "当前状态",
		"模拟结果", "任务结果",
		// oral / natural language
		"md跑完了吗", "刚才那个任务完成了吗", "还在跑吗",
		"动力学跑到哪了", "轨迹出来了吗", "项目状态",
		"gromacs现在怎么样", "最近那个md任务怎么样",
		"系统还在跑吗",
		// ultra-short (≤8 runes)
		"状态", "进度", "现在状态",
	}
	negative := []string{
		"分析结果", "帮我看看这个文件", "运行这个脚本",
		"这个方案怎么样", "GSEA结果怎么看",
		"你好", "什么是分子动力学", "帮我写个md文件",
		"这个颜色怎么样", "PROTAC方案怎么设计",
		"运行结果怎么看", "模拟结果怎么看", "科研结果在哪",
		"科研是什么意思", "md格式怎么写", "系统性红斑狼疮是什么",
		// ultra-short safety: question patterns still exclude
		"状态机是什么", "进度条怎么看",
		// advice override: long text with "状态/进度" should go to Claude
		"帮我总结一下当前项目状态", "分析一下现在项目进度",
		"帮我分析当前状态", "总结一下这个项目的进度",
		"解释一下当前系统状态",
	}
	for _, s := range positive {
		if !isStatusQuery(s) {
			t.Errorf("isStatusQuery(%q) = false, want true", s)
		}
	}
	for _, s := range negative {
		if isStatusQuery(s) {
			t.Errorf("isStatusQuery(%q) = true, want false", s)
		}
	}
}

func TestIsResearchQuery(t *testing.T) {
	positive := []string{
		"看看科研任务", "科研监控", "任务监控",
		"研究进度", "科研结果", "模拟结果", "任务结果",
		"科研监控 haddock", "haddock状态", "haddock进度",
		"schrodinger状态", "rosetta进度",
		"alphafold状态", "alphafold进度",
		"colabfold状态", "colabfold进度",
		"蛋白折叠状态", "蛋白折叠进度",
		"amber状态", "amber进度",
		"openmm状态", "openmm进度",
		"gaussian状态", "gaussian进度",
		"g16状态", "g09进度",
	}
	negative := []string{
		"查看状态", "系统状态", "cc状态", "看看结果",
	}
	for _, s := range positive {
		if !isResearchQuery(s) {
			t.Errorf("isResearchQuery(%q) = false, want true", s)
		}
	}
	for _, s := range negative {
		if isResearchQuery(s) {
			t.Errorf("isResearchQuery(%q) = true, want false", s)
		}
	}
}

func TestIsMDQuery(t *testing.T) {
	positive := []string{
		"md进度", "模拟进度", "md状态", "md跑到哪了",
		"模拟跑到哪了", "gromacs进度", "gromacs状态",
		"MD状态", "MD进度",
		// oral
		"md跑完了吗", "最近那个md任务怎么样",
		"动力学跑到哪了", "轨迹出来了吗",
		"gromacs现在怎么样",
		// #3 新增
		"md还在跑吗", "模拟结束了吗", "动力学完成没",
		"gromacs还在跑吗", "轨迹出来没",
		"模拟完成了吗", "gromacs还在动吗",
	}
	negative := []string{
		"科研任务", "系统状态", "看看结果", "模拟结果",
		"帮我写个md文件", "分子动力学是什么", "模拟退火",
		"md格式怎么写", "markdown表格怎么写",
		"模拟退火算法", "gromacs怎么安装",
	}
	for _, s := range positive {
		if !isMDQuery(s) {
			t.Errorf("isMDQuery(%q) = false, want true", s)
		}
	}
	for _, s := range negative {
		if isMDQuery(s) {
			t.Errorf("isMDQuery(%q) = true, want false", s)
		}
	}
}

func TestExtractDetectorKeyword(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"科研监控 haddock", "haddock3"},
		{"haddock状态", "haddock3"},
		{"haddock进度", "haddock3"},
		{"HADDOCK状态", "haddock3"},
		{"schrodinger进度", "schrodinger"},
		{"maestro状态", "schrodinger"},
		{"glide监控", "schrodinger"},
		{"rosetta状态", "rosetta"},
		{"pyrosetta进度", "rosetta"},
		{"gromacs状态", "gromacs"},
		{"vina状态", "autodock_vina"},
		{"autodock进度", "autodock_vina"},
		{"autogrid监控", "autodock_vina"},
		{"科研监控 vina", "autodock_vina"},
		{"python进度", "python_pipeline"},
		{"alphafold", "alphafold"},
		{"colabfold", "alphafold"},
		{"蛋白折叠预测状态", "alphafold"},
		{"科研监控 alphafold", "alphafold"},
		{"amber状态", "amber_openmm"},
		{"openmm进度", "amber_openmm"},
		{"pmemd运行状态", "amber_openmm"},
		{"sander怎么样了", "amber_openmm"},
		{"科研监控 amber", "amber_openmm"},
		{"gaussian状态", "gaussian"},
		{"g16进度", "gaussian"},
		{"g09状态", "gaussian"},
		{"gjf文件检查", "gaussian"},
		{"科研监控 gaussian", "gaussian"},
		{"科研监控", ""},
		{"科研状态", ""},
		{"看看结果", ""},
		{"md进度", ""},
	}
	for _, tc := range cases {
		got := extractDetectorKeyword(tc.text)
		if got != tc.want {
			t.Errorf("extractDetectorKeyword(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestToolSpecificStatusRouting(t *testing.T) {
	positive := []string{
		"haddock状态", "haddock进度", "haddock监控",
		"schrodinger状态", "schrodinger进度",
		"rosetta状态", "rosetta进度",
		"vina状态", "vina进度",
		"autodock状态", "autodock进度",
		"alphafold状态", "colabfold进度",
		"amber状态", "amber进度",
		"openmm状态", "openmm进度",
		"gaussian状态", "gaussian进度",
		"g16状态", "g09进度",
	}
	for _, s := range positive {
		if !isStatusQuery(s) {
			t.Errorf("isStatusQuery(%q) = false, want true (tool-specific trigger)", s)
		}
		if !isResearchQuery(s) {
			t.Errorf("isResearchQuery(%q) = false, want true (tool-specific trigger)", s)
		}
	}

	// Tool name alone without status intent → advice, not status
	negative := []string{
		"haddock是什么意思",
		"rosetta原理",
		"schrodinger软件介绍",
		"haddock怎么安装",
		"gromacs和rosetta的区别",
		"gromacs命令怎么用",
		"vina原理",
		"autodock安装教程",
		"alphafold安装教程",
		"colabfold怎么用",
		"alphafold原理",
		"colabfold原理",
		"蛋白折叠原理",
		"amber原理",
		"openmm怎么安装",
		"amber和gromacs区别",
		"gaussian原理",
		"g16怎么安装",
		"gaussian和orca的区别",
	}
	for _, s := range negative {
		if isStatusQuery(s) {
			t.Errorf("isStatusQuery(%q) = true, want false (tool name without status intent)", s)
		}
	}
}

func TestSlashNoSlashDetectorEquivalence(t *testing.T) {
	// All three input forms should extract the same detector keyword
	groups := []struct {
		inputs   []string
		detector string
	}{
		{[]string{"科研监控 haddock", "haddock状态", "haddock进度"}, "haddock3"},
		{[]string{"科研监控 schrodinger", "schrodinger状态", "maestro状态"}, "schrodinger"},
		{[]string{"科研监控 rosetta", "rosetta状态", "rosetta进度"}, "rosetta"},
		{[]string{"科研监控 gromacs", "gromacs状态", "gromacs进度"}, "gromacs"},
		{[]string{"科研监控 vina", "vina状态", "vina进度"}, "autodock_vina"},
		{[]string{"alphafold状态", "alphafold进度", "colabfold状态", "colabfold进度", "蛋白折叠状态", "蛋白折叠进度"}, "alphafold"},
		{[]string{"amber状态", "amber进度", "openmm状态", "openmm进度", "科研监控 pmemd"}, "amber_openmm"},
		{[]string{"gaussian状态", "gaussian进度", "g16状态", "g09进度", "科研监控 gaussian"}, "gaussian"},
	}
	for _, g := range groups {
		for _, input := range g.inputs {
			got := extractDetectorKeyword(input)
			if got != g.detector {
				t.Errorf("extractDetectorKeyword(%q) = %q, want %q", input, got, g.detector)
			}
			if !isResearchQuery(input) {
				t.Errorf("isResearchQuery(%q) = false, want true", input)
			}
		}
	}
}

func TestIsSystemStatusQuery(t *testing.T) {
	positive := []string{"系统状态", "controller状态", "当前状态"}
	negative := []string{"科研任务", "看看结果", "md进度"}
	for _, s := range positive {
		if !isSystemStatusQuery(s) {
			t.Errorf("isSystemStatusQuery(%q) = false, want true", s)
		}
	}
	for _, s := range negative {
		if isSystemStatusQuery(s) {
			t.Errorf("isSystemStatusQuery(%q) = true, want false", s)
		}
	}
}

func TestIsResultLocationQuery(t *testing.T) {
	positive := []string{
		"结果在哪", "文件在哪", "输出在哪",
		"保存到哪", "保存在哪", "生成了什么文件",
		"产出在哪", "报告在哪",
		"结果放在哪", "文件放在哪", "输出放在哪",
		"刚才的结果在哪", "模拟结果在哪",
	}
	negative := []string{
		"看看结果", "结果如何", "查看结果",
		"运行结果怎么看", "GSEA结果怎么看",
		"帮我生成文件", "生成报告", "删除文件",
		"进度", "状态",
	}
	for _, s := range positive {
		if !isResultLocationQuery(s) {
			t.Errorf("isResultLocationQuery(%q) = false, want true", s)
		}
	}
	for _, s := range negative {
		if isResultLocationQuery(s) {
			t.Errorf("isResultLocationQuery(%q) = true, want false", s)
		}
	}
}
