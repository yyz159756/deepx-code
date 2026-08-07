# design: Git 工具

## 上下文

Bash 工具在 Windows 用 cmd.exe 执行命令串,命令串含 `powershell -Command` + `2>&1` 时 PowerShell 污染退出码,git 成功却误报失败。Git 工具绕过 shell 层,直调 git。

## 决策记录

### D1. 执行方式:exec 直调
`exec.Command("git", args...)`:Go 直接 CreateProcess 调 git,不经 cmd/powershell。
- Windows 无 stderr 污染(Go 直接拿 exit code,stdout/stderr 分离捕获)
- args 数组传参,无字符串转义/引号地狱

### D2. exit code 语义(git 约定)
| exit | 语义 | Success |
|---|---|---|
| 0 | 成功 | true |
| 1 | 正常结果(diff 有差异 / 无匹配等) | true,Output 带 `[exit] 1` 标记 |
| ≥2 | 错误(如 128 无仓库 / 致命错误) | false |

git 的 exit 1 ≠ 通用失败(与 Bash 语义不同),必须区分,否则 `git diff` 有差异会被误判失败。

### D3. 失败结果结构化(落 Failure Protocol 边界)
```
Success=false
Error:  "git: <stderr 首行>"(摘要)
Output: stdout + stderr(诊断完整保留)
FailureCategory: execution_error
FailureHint: "git 命令执行失败。检查输出中的错误信息,对症修正后再试,不要原样复读。"
```
→ agent 渲染 `<tool_failure>` 协议时自动覆盖(失败恢复协议的"未来工具适配"样例)。

### D4. cwd 默认 workspace 根
`cwd` 缺省用 workspace 根(agent 的 Git 工具一般就在当前项目操作);显式传则用传入目录。避免"在非仓库目录误跑"。

### D5. 输出截断
stdout 超过 16KB 截断(与 Bash clampToolOutput 同级),防大 diff 撑爆上下文;截断带标记。

## 数据流

```
agent Git(args, cwd) → exec.Command("git", args...) → stdout/stderr/exit
    ├─ exit 0/1 → ToolResult{Success:true, Output}
    └─ exit ≥2  → ToolResult{Success:false, Error:摘要, Output:stdout+stderr, Category:execution}
        → RenderToolFailureProtocol(<tool_failure>) → 模型
        → FailureTracker(nudge 引导)
```
