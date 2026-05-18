# PowerShell 特殊字符安全规则

## cc-connect `{{args}}` 模板替换

cc-connect 将用户消息通过 `{{args}}` 插入到命令行。以下字符会导致 PowerShell 解析错误：

| 字符 | 问题 | 替换方案 |
|------|------|---------|
| `{ }` | 脚本块 | 用中文描述替换 |
| `( )` | 分组表达式 | 用逗号分隔 |
| `[ ]` | 类型/索引 | 用中文替换 |
| `;` | 语句分隔符 | 用 `，` 逗号 |
| `$` | 变量引用 | 避免或转义 |
| `\|` | 管道 | 避免 |
| `>` `<` | 重定向 | 避免 |
| `"` | 字符串界定 | 用单引号或避免 |

## Null-Guard 模板

所有接收 `{{args}}` 的脚本必须用此模式：

```powershell
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ArgsRest
)

$Task = ""
if ($null -ne $ArgsRest -and $ArgsRest.Count -gt 0) {
    $Task = ($ArgsRest | ForEach-Object { $_.ToString() }) -join " "
}
```

## 任务文本清洁

通过微信发送的任务文本，必须：
1. 去除所有 PowerShell 特殊字符
2. 压成单行
3. 保留技术关键词

示例：
```
❌ 三体系在 D:\projects\my-project 下的 {system_a, system_b}
✅ 三体系在 D:\projects\my-project 下的 system_a、system_b
```
