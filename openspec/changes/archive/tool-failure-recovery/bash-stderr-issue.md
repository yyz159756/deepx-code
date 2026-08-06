# Issue: Bash Tool 在 Windows PowerShell 下误报失败(stderr 被当错误流导致 exit 1)

> 面向 deepx 上游(itmisx/deepx-code)的独立 bug issue
> **与任何失败处理相关改动无关** —— 已在纯上游代码上实测复现

## 背景

在测试工具失败处理相关改动时,发现 Windows PowerShell 下存在一种**误报**:

```
实际执行:git checkout / git merge / git push —— 全部成功,git 自身 exit code = 0
```

但 deepx Bash Tool 收到的结果:

```
Success = false
Error   = command failed: exit status 1
Output  = git 的 stderr 输出(如 "Switched to branch 'dev'")
```

## 根因

PowerShell wrapper 中使用 `2>&1` 后,**native command(git)的 stderr 输出被 PowerShell 当作错误流处理**,最终导致整个 `powershell -Command` 返回 `exit status 1` —— 即使 native command 本身成功(exit 0)。

最小复现:

```
# git checkout 成功,但:
powershell -Command "git checkout dev 2>&1 | Select-Object -Last 1"
# → 返回 exit status 1(诊断里出现 PowerShell 的 NativeCommandError 包装)

# 同命令加显式 exit 0 → 正常:
powershell -Command "git checkout dev 2>&1 | Select-Object -Last 1; exit 0"
# → 成功
```

## 实测验证(纯上游代码,排除改动影响)

1. 用 **main 分支最新上游代码**(`ae8f16d`,不含任何失败处理相关改动)编译 exe 实测:

   ```
   powershell -Command "cd <repo>; git checkout dev 2>&1 | Select-Object -Last 1"
   ```

   → **同样误报**:git 成功("Switched to branch 'dev'")但 Bash Tool 收到 `exit status 1`

2. **上游源码判定逻辑**(`tools/command.go`):`err != nil → Success:false` —— 该判定与失败处理相关改动无关,是既有逻辑

3. **git 自身 `$LASTEXITCODE = 0`**(实测)—— 是 PowerShell 管道处理 stderr 时污染了外层退出码

结论:这是 **Bash Tool 的 Windows/PowerShell 兼容问题**,独立于任何失败处理改动存在。

## 可能的修复方向

- **(推荐)** Bash Tool 的 PowerShell wrapper 正确保留 native command 的 `$LASTEXITCODE`,不因 stderr 有输出而改变外层退出码
- 或针对 Windows native command 做 stdout/stderr 与 exit code 的分离处理
- 治标(调用侧):对已知噪音命令(如 git)吞 stderr(`2>$null`)或命令末尾显式 `exit 0`

## 附带观察(为何值得修复)

该问题原本不易察觉:失败信息混在文本里时,`git : Switched to branch 'dev'` 这类"成功但被标红"的输出容易被忽略。一旦失败状态被结构化呈现(协议化/字段化),这类执行层误判就非常显眼 —— 说明结构化失败状态对**诊断执行层问题**本身有独立价值,不依赖任何特定实现。

本问题可独立处理;失败处理相关改动(feat/tool-failure-recovery,审阅中)与它相互独立,可分别评估。
