# 编码规则

> **适用范围**：当前 controller / cc-connect 链路，环境 Windows 11 + Windows PowerShell 5.1。
> 实证日期 2026-05-18（用户实跑 `/编码测试` + 审计 22 个 controller `.ps1` + work-9 实测）。
> 若更换 cc-connect 版本、替换 stdout 捕获/桥接实现、或切换 PowerShell 版本，
> **必须先跑编码探针复测**，再据结果更新本规则——不要把以下结论当成无条件永真。

本规则区分三件**互不相同**的事，不要混为一谈：
1. `.ps1` **源码文件**被 PowerShell 解码（→ BOM 问题，见 §2）
2. PowerShell **pipe stdout 输出**编码（→ UTF-8 块，见 §1）
3. **外部程序** stdin/stdout 编码（→ §5）

---

## 1. cc-connect 按 UTF-8 解码脚本 stdout

**实证**：脚本顶部设 `[Console]::OutputEncoding = UTF8` → 微信端中文显示正确；
设 `GetEncoding(936)`(GBK) → 乱码。PowerShell 的 pipe writer 编码在**首次写 stdout 时锁定**，
之后再改 `OutputEncoding` 无效——所以必须在任何输出之前设好。

**UTF-8 块** — 被 cc-connect 调用且向 stdout 写中文的脚本，在 `param(...)` 之后、
任何 stdout 输出之前加（无 `param()` 则放文件首段可执行语句）：

```powershell
[Console]::InputEncoding  = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
```

> 写法遵循现有 controller 脚本约定（`[System.Text.UTF8Encoding]::new($false)`）。
> 不要引入 `New-Object`/`GetEncoding` 等第二种写法，一致性优先。

## 2. 含中文的 .ps1 必须存为 UTF-8 with BOM（致命）

**根因不是 stdout，是源码读取**：Windows PowerShell 5.1 读取**无 BOM** 的非 ASCII `.ps1`
时倾向按系统 ANSI 代码页（中文环境=GBK/936）解码，中文 UTF-8 字节被误解码 →
轻则字符串乱码，重则引号/终止符/语法被破坏 → **parser error，脚本根本跑不起来**。

- 含非 ASCII（中文，**包括注释里的中文**）的 `.ps1` → **必须 UTF-8 with BOM**
- 机器可验证标准：文件前 3 字节为 `EF BB BF`
- 工具（如 Write）写出脚本后必须复验 BOM；多数编辑器/工具默认写无 BOM

**例外（不需要 BOM）**：
- `.ps1` 全 ASCII（无任何中文字面量与中文注释）
- `.ps1` 不含中文字面量/注释，中文只从外部 UTF-8 文件、JSON、环境变量、参数读入
  （脚本本身不需 BOM；外部文件编码另算）
- 通过 `-Command`、stdin、临时文件、生成器传入的脚本内容：编码规则取决于传入方式，
  不等同磁盘 `.ps1`
- 目标为 PowerShell 7 的专用脚本可另按 pwsh 规则；但本项目以 PS 5.1 为准，
  共享库若需兼容 5.1 仍建议 BOM

## 3. 判断标准（按场景查表）

| 场景 | stdout UTF-8 块 | 文件 UTF-8 BOM |
|---|---|---|
| cc-connect 调用，stdout 输出中文 | 需要 | 脚本含中文则需要 |
| cc-connect 调用，stdout 纯 ASCII | 可不需要 | 脚本含中文仍需要 |
| 库脚本/函数文件（dot-source/import，无 stdout） | 不需要 | 文件含中文（含注释）则需要 |
| 仅后台写 UTF-8 文件，不给微信 stdout 中文 | 不需要（除非子进程依赖 `$OutputEncoding`） | 文件含中文则需要 |
| 调 Claude CLI，中文走 stdin | 见 §5，**不靠** stdout 块解决 | 脚本含中文则需要 |
| 调外部 exe 并经管道传中文 | 通常需 `$OutputEncoding=UTF8`，并确认该 exe 期望编码 | 脚本含中文则需要 |
| PowerShell 7 专用脚本 | 按 pwsh 规则另写 | 共享兼容 5.1 仍建议 BOM |

## 4. 禁止

- 禁止 `[Console]::OutputEncoding = ...GetEncoding(936)` 处理 cc-connect 中文 stdout
- 禁止"输出前切回 GBK"这类反向操作
- 禁止把旧版 GBK 修复模板作为"可选方案"并列保留（见附录）

## 5. Claude CLI 中文输入（stdin）

> 此条解决的是 **stdin 传参/提示词编码**，与 §1 的 cc-connect stdout 显示乱码是不同问题。

`claude -p $ChineseText` 在 Windows PowerShell 5.1 上会丢失中文字符。修复模板：

```powershell
$tmpFile = [System.IO.Path]::GetTempFileName()
$prompt | Set-Content -Encoding UTF8 -LiteralPath $tmpFile
Get-Content -Raw -Encoding UTF8 $tmpFile | & claude -p --output-format text
Remove-Item -LiteralPath $tmpFile -Force
```

---

## 附录：已废弃的旧结论（历史误区，勿回归）

旧版本 `rules/encoding.md`（2026-05-18 21:15 提交）曾给出**相反且错误**的指导：

- 旧称："根因：cc-connect 用 `GetACP()=936` 按 GBK 解码 stdout"
- 旧修复模板（**错误**）："脚本顶部加 `[Console]::OutputEncoding = GetEncoding(936)`"
- 旧版完全没有 BOM 相关内容

**废弃原因**：该结论很可能把三件事混淆——`.ps1` 源码按 ANSI/GBK 读取（真问题，§2）、
pipe stdout 输出编码（§1）、以及历史/其它桥接实现——误归因成"stdout 被 cc-connect 按 GBK 解码"。
本会话同一链路实测：`OutputEncoding=UTF8` 中文正确、`936` 乱码；照旧模板设 936
会**重新引入最初的乱码 bug**。controller 中曾存在的 GBK 派代码（fix-controller /
auto-callback-toggle 的 `GetEncoding(936)`）即此误区产物，已移除。

**复测要求**：若日后怀疑旧结论在某环境成立，不要直接改规则——先用 `/编码测试`
（或等效探针）在目标环境复现，记录 cc-connect 版本/PowerShell 版本/系统代码页，
据实证更新本文件并保留废弃说明，防止后人考古把 GBK 结论捡回。
