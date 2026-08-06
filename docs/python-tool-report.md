# Python 工具(Python Tool)方案实现报告

- 分支:`feat/python-tool`(基于 `dev`)
- 涉及改动:`tools/python.go`(新增)、`tools/tools.go`(注册)、`tools/python_test.go`(新增测试)、`README.md`(工具表同步)

---

## 1. 背景与动机

deepx 是纯 Go 二进制,执行外部命令的唯一入口是 `Bash` 工具(`tools/command.go`)。因此此前任何需要 Python 的任务(数据计算、文本处理、一次性脚本)都必须经由 shell 拼出 `python.exe xxx.py` 来执行,存在两处不便:

1. **调用链啰嗦**:模型要记住 shell 前缀(`python.exe`/`python3`)+ 处理引号转义,心智负担大;
2. **代码易被 shell 破坏**:Windows 的 `cmd.exe` 与类 Unix 的 `sh` 都会对命令串里的引号、反引号、`$`、反斜杠做解析,多行代码/复杂字符串拼进 `python -c "..."` 极易出错。

**目标**:新增一个独立的 `Python` 工具,模型直接传源码(`code` 参数)即可执行,调用体验干净,同时不破坏 deepx 既有的沙箱安全边界。

## 2. 需求与成功标准

需求:在 `tools/` 下仿 `command.go` 包一层,新增 Python 执行工具,对模型暴露独立工具。

可验证的成功标准:

| # | 标准 | 验证方式 |
|---|------|---------|
| 1 | 代码含单/双引号、`$`、反引号、换行时原样执行,不被 shell 破坏 | 单测 `TestRunPython_QuotesUnscathed` |
| 2 | 正常输出、异常 traceback、超时、cwd、空 code 五种行为正确 | 单测 `TestRunPython_*`(共 6 个) |
| 3 | 不绕过沙箱:docker 在容器内跑;native 有 OS 隔离则套隔离;入口过 `SandboxCheck` | 代码走查 + 与 Bash 行为对照 |
| 4 | `go build ./...`、`go vet`、`go test ./tools/` 全绿 | 命令输出 |

## 3. 方案设计

### 3.1 核心决策:代码经 stdin 传给 `python -`,不经 shell

对比过的备选:

| 方案 | 结论 |
|------|------|
| `python -c "<code>"` 拼进 shell 命令串 | ❌ cmd/bash 的引号/转义解析会破坏代码,Windows 上尤其严重 |
| 代码写临时文件再执行 | ❌ docker 模式下宿主 temp 与容器隔离,路径换算复杂;留垃圾文件 |
| 代码经 base64 塞进命令串 | ❌ 回到 shell 解析问题,Windows 无 `base64 -d` |
| **stdin 传代码 + `python -`** | ✅ 完全绕开 shell 引号解析,代码原样进解释器,跨平台行为一致 |

`python -` 从 stdin 读取脚本,是 Python 解释器的标准行为,Windows / Unix 均支持。

### 3.2 沙箱分派(不成为绕过 Bash 的后门)

`RunPython` 与 `RunCommand` 共用同一套沙箱架构,按当前模式分派构造 `*exec.Cmd`:

| 沙箱模式 | 执行方式 |
|---------|---------|
| `docker` | `docker exec -i -w <dir> <container> sh -c "exec python3 - 2>/dev/null \|\| exec python -"`,stdin 经 `-i` 传入容器 |
| `native` + OS 隔离(bwrap/Landlock/Seatbelt) | 经 `nativeShellCmd(py+" -", cwd)` 套隔离,stdin 由 shell 原样转发 |
| `native` 无 OS 隔离(Windows 等)/ `off` | 直接 `exec.Command(py, "-")` + `cmd.Dir`,不经 shell(绕开 cmd 引号解析) |

入口处对源码过 `SandboxCheck(code)` 预检,与 Bash 同一套软黑名单(仅 Windows 等无 OS 隔离平台生效;docker/OS 隔离放行)。

### 3.3 超时与进程组

- 默认超时 60s,`timeout` 参数可调;
- `setPgid(cmd)` 进程组化,超时后 `killProc` 整组杀,不留孤儿进程;
- 输出经 `startWithPipe` + `lockedBuffer` 收集,复用 `formatForegroundResult` 统一格式化(无输出兜底、16KB 截断)。

### 3.4 解释器发现

`hostPython()` 依次 `exec.LookPath("python")` → `"python3"`(Windows 上 `python.exe`、类 Unix 上 `python3` 都能命中),都找不到时返回明确错误。

### 3.5 明确不做(简单优先)

- **不接 auto-bg / 后台句柄**:长跑/常驻任务应走 `Bash`(`run_in_background`),Python 工具定位是"干净执行代码片段";
- **不引任何第三方 Go 库**:全部用标准库 `os/exec`。

## 4. 实现细节

### 4.1 `tools/python.go`(新增,~110 行)

| 符号 | 职责 |
|------|------|
| `RunPython(args) ToolResult` | 工具入口:参数校验 → 沙箱预检 → 构造 cmd → 启动 → select 等结果/超时 |
| `pythonCmd(code, cwd) (*exec.Cmd, error)` | 按 `CurrentSandboxMode()` 分派沙箱构造 |
| `directPythonCmd(code, cwd)` | 不经 shell 直接 exec python(off / 无 OS 隔离平台) |
| `hostPython() (string, error)` | 查找 python / python3 绝对路径 |

### 4.2 `tools/tools.go`(+20 行)

在 `Bash` 与 `BashOutput` 之间注册:

```go
{
    Name:        "Python",
    Description: "执行 Python 代码(子进程方式,代码经 stdin 传给 `python -`,**不经 shell** ...)",
    Parameters: ToolParam{
        Type: "object",
        Properties: map[string]PropDef{
            "code":    {Type: "string", Description: "要执行的 Python 源码..."},
            "cwd":     {Type: "string", Description: "工作目录(可选)"},
            "timeout": {Type: "integer", Description: "超时秒数,默认 60"},
        },
        Required: []string{"code"},
    },
    Executor: RunPython,
    ReadOnly: false,
},
```

### 4.3 `tools/python_test.go`(新增,6 个单测)

`Print`(正常输出)、`QuotesUnscathed`(引号/`$`/反引号原样,核心卖点)、`Error`(除零 traceback)、`EmptyCode`(空 code 拒绝)、`Timeout`(超时标记)、`Cwd`(工作目录生效)。环境无 Python 时 `hostPythonOrSkip` 跳过,不硬性失败。

### 4.4 `README.md`(+1 行)

工具集表格新增 `Python` 行(plan ✗ / auto ✓ / review ⏳,与 Bash 同权限)。

## 5. 验证结果

```
go build ./...             ✅
go vet ./tools/...         ✅
go test ./tools/ -run Python -v   → 6/6 PASS
go test ./tools/           ✅(无回归)
```

测试输出节选:

```
=== RUN   TestRunPython_Print
--- PASS: TestRunPython_Print (0.04s)
=== RUN   TestRunPython_QuotesUnscathed
--- PASS: TestRunPython_QuotesUnscathed (0.04s)
=== RUN   TestRunPython_Error
--- PASS: TestRunPython_Error (0.04s)
=== RUN   TestRunPython_EmptyCode
--- PASS: TestRunPython_EmptyCode (0.00s)
=== RUN   TestRunPython_Timeout
--- PASS: TestRunPython_Timeout (1.08s)
=== RUN   TestRunPython_Cwd
--- PASS: TestRunPython_Cwd (0.05s)
```

## 6. 权衡与限制

1. **黑名单匹配对象是源码字符串**:Windows native 无 OS 隔离,`SandboxCheck(code)` 的正则对 Python 源码做匹配,如 `os.system("rm -rf /")` 能命中 `rm -rf /` 规则;更隐蔽的绕过依赖 docker 模式兜底——与 Bash 现状一致,非本改动引入。
2. **docker 容器内解释器**:容器内由 `sh` 依次尝试 `python3` → `python`,自定义镜像若两者皆无会启动失败(错误信息会体现)。
3. **超时语义**:Python 工具超时即杀(无 Bash 的 15s auto-bg 预留在先),适合短任务;长任务用 Bash。
4. **无 stdin 输入参数**:当前 `code` 单向传入;需要交互式 stdin 的场景暂不支持(可用 Bash 替代)。

## 7. 后续可选方向(未实现)

- 加 `args` 参数(向 `python -` 传递命令行参数);
- 支持 `run_in_background`,复用 `BashOutput` / `KillBash` 句柄管理;
- 超时前返回部分输出(auto-bg 语义);
- 与 README 工具矩阵、MCP 工具列表的进一步联动。
