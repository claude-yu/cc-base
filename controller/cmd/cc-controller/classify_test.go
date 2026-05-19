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
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyMode(c.input); got != c.want {
				t.Errorf("classifyMode(%q) = %s, want %s", c.input, got, c.want)
			}
		})
	}
}
