# proposal: Git 工具 —— 避免 PowerShell 误报 + 结构化 git 操作

## Why

**问题 1(本 change 直接动因)**:Bash 工具在 Windows 用 cmd.exe 执行命令串;当命令串含 `powershell -Command "...; git ... 2>&1"` 时,PowerShell 5.1 把 native 命令的 stderr 当错误流,污染外层退出码 → git 成功却误报失败(`command failed: exit status 1`)。已在上游原版实测复现。

**问题 2**:git 操作拼命令串(引号/转义/路径)易错;在非仓库目录误跑报 "not a git repository"。

**解法**:新增 Git 工具——`exec.Command("git", args...)` 直接调 git 可执行文件,**不经 cmd/powershell** → 无 stderr 污染;args 数组传参无转义问题;cwd 参数指定仓库;失败结果走 Failure Protocol(Error 摘要 + stderr 诊断)。

## What Changes

1. 新增 `tools/git.go`:`Git` 工具
   - `args`([]string, 必需):git 参数数组
   - `cwd`(string, 可选):仓库目录(默认 workspace 根)
   - `timeout`(int, 可选):默认 60
2. exit code 语义:
   - `0` → Success=true,Output=stdout
   - `1` → Success=true(如 diff 有差异是正常结果),Output 带 `[exit] 1` 标记
   - `≥2` → Success=false,Error=摘要(stderr 首行)、Output=stdout+stderr(诊断保留)、FailureCategory=execution_error、FailureHint 引导
3. tools 注册 + system prompt 工具定义
4. tui:extractMainArg 加 Git case(显示 args 摘要)

## 设计要点(详见 design.md)

- exec 直调 git,不经 shell(Windows 上 CreateProcess,无 cmd/powershell 层)
- stderr 进 Output(诊断),Error 只存摘要 —— 落 Phase 2 边界,自动被 `<tool_failure>` 协议覆盖
- exit 1 视为成功(git 语义:diff 有差异/无匹配是正常结果),避免误判失败

## 不做的事

- ❌ 不做 git 操作白名单/安全过滤(agent 是可信执行者)
- ❌ 不做多命令事务(每次调用单命令,agent 编排)
- ❌ 不替代 Bash(通用 shell 仍需 Bash;Bash 的 PowerShell 使用注意另行文档化)
