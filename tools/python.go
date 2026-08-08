package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// RunPython 执行 Python 代码并返回输出。
// 参数:
//
//	code    (string, 必需) 要执行的 Python 源码(完整代码,可含任意引号/换行/特殊字符)
//	cwd     (string, 可选) 工作目录
//	timeout (int,   可选) 超时秒数,默认 60
//
// 实现:代码经 stdin 传给 `python -`,**不经 shell** —— 完全绕开 cmd/bash 对引号、
// 反引号、$、\ 的解析,代码原样进解释器(这是与 Bash 里拼 `python.exe xxx.py` 的本质区别)。
// 沙箱:docker 模式在容器里跑;native 模式有 OS 隔离(bwrap/Landlock/Seatbelt)时套隔离,
// 无 OS 隔离的平台(Windows)直接 exec python。
func RunPython(args map[string]any) ToolResult {
	code, _ := args["code"].(string)
	if strings.TrimSpace(code) == "" {
		return ToolResult{
			Output:          "错误: code 参数为空",
			Success:         false,
			Error:           "code 参数为空",
			FailureCategory: FailureCategoryInvalidArgument,
		}
	}
	// 沙箱预检,与 RunCommand 同一入口:docker/OS 隔离放行;Windows 等无 OS 隔离平台跑软黑名单。
	if err := SandboxCheck(code); err != nil {
		return ToolResult{
			Output:          "🛡️ " + err.Error(),
			Success:         false,
			Error:           "沙箱拒绝执行该 Python 代码",
			FailureCategory: FailureCategoryPermissionDenied,
			FailureHint:     "沙箱策略拒绝该代码(黑名单命中)。如确需运行,请切换到 docker 沙箱(/sandbox docker)或修改代码规避被禁模式。",
		}
	}
	cwd, _ := args["cwd"].(string)
	timeout := toInt(args["timeout"], 60)
	if timeout <= 0 {
		timeout = 60
	}

	cmd, err := pythonCmd(code, cwd)
	if err != nil {
		return ToolResult{
			Output:          "🛡️ 沙箱启动失败: " + err.Error(),
			Success:         false,
			Error:           "沙箱/解释器启动失败",
			FailureCategory: FailureCategoryExecution,
			FailureHint:     "沙箱或 Python 解释器无法启动。检查 docker 容器状态(/sandbox docker)或 Python 是否在 PATH,对症处理后重试。",
		}
	}
	setPgid(cmd) // 进程组化:超时路径能整组杀,不留孤儿

	buf := &lockedBuffer{}
	readerDone, err := startWithPipe(cmd, buf)
	if err != nil {
		return ToolResult{
			Output:          fmt.Sprintf("启动失败: %v", err),
			Success:         false,
			Error:           "进程启动失败",
			FailureCategory: FailureCategoryExecution,
		}
	}

	waitErrCh := make(chan error, 1)
	go func() { waitErrCh <- cmd.Wait() }()

	select {
	case werr := <-waitErrCh:
		// 进程已退出:等输出管道收尾抓全残余输出(有后台子进程占管道时最多等 readerDrainGrace)。
		select {
		case <-readerDone:
		case <-time.After(readerDrainGrace):
		}
		return formatForegroundResult(buf.drain(), werr)
	case <-time.After(time.Duration(timeout) * time.Second):
		// 硬超时:杀进程组,等 wait 收尾(避免 goroutine 泄漏),返回已有输出 + 超时标记。
		_ = killProc(cmd)
		<-waitErrCh
		return pythonTimeoutResult(buf.drain(), timeout)
	}
}

// containerHasPython 检查沙箱容器内是否有 python3/python(默认镜像 ubuntu:24.04 无)。
func containerHasPython(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "exec", name, "sh", "-c",
		"command -v python3 || command -v python").CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// pythonTimeoutResult 构造超时失败结果(纯函数,无副作用)。
// 单独提取:状态转换逻辑(类别/Error/Hint)可快速单测,不必真实等 timeout 秒
// (真实超时属系统行为,留 CI 全量验证,本地敏捷开发跑 go test -short)。
func pythonTimeoutResult(out string, timeout int) ToolResult {
	return ToolResult{
		Output:          out + fmt.Sprintf("\n超时(%ds)", timeout),
		Success:         false,
		Error:           fmt.Sprintf("python 执行超时(%ds)", timeout),
		FailureCategory: FailureCategoryTimeout,
		FailureHint:     "Python 执行超时。检查代码是否死循环 / 等待输入,或适当加大 timeout,不要原样重试。",
	}
}

// pythonCmd 按当前沙箱模式构造"stdin 喂 python 代码"的 *exec.Cmd。
func pythonCmd(code, cwd string) (*exec.Cmd, error) {
	switch CurrentSandboxMode() {
	case SandboxDocker:
		// 容器内执行:sh 负责挑解释器(python3 优先,退回 python),stdin 经 docker exec -i 传进去。
		name, err := EnsureDockerContainer()
		if err != nil {
			return nil, err
		}
		// 预检容器内解释器:默认镜像 ubuntu:24.04 不含 python3/python,不预检会把失败
		// (2>/dev/null 吞掉 stderr)变成"空输出",用户无从诊断。
		if !containerHasPython(name) {
			return nil, fmt.Errorf("沙箱容器缺少 Python 解释器(镜像 %s 无 python3/python),请用 /sandbox image 换带 python 的镜像(如 python:3.12-slim)",
				SandboxDockerImage())
		}
		c := exec.Command("docker", "exec", "-i", "-w", containerWorkdir(cwd), name, "sh", "-c",
			"exec python3 - 2>/dev/null || exec python -")
		c.Stdin = strings.NewReader(code)
		return c, nil
	case SandboxNative:
		if nativeIsolationAvailable() {
			// OS 隔离(bwrap/Landlock/Seatbelt):经 nativeShellCmd 套隔离,命令串里插解释器绝对路径,
			// stdin 由 sh 原样转发给 python。
			py, err := hostPython()
			if err != nil {
				return nil, err
			}
			c := nativeShellCmd(py+" -", cwd)
			c.Stdin = strings.NewReader(code)
			return c, nil
		}
		return directPythonCmd(code, cwd)
	default: // SandboxOff
		return directPythonCmd(code, cwd)
	}
}

// directPythonCmd 不经 shell 直接 exec python:绕开 cmd/bash 的引号解析,代码原样进 stdin。
// 用于 off 模式,以及 native 模式无 OS 隔离的平台(Windows 等)。
func directPythonCmd(code, cwd string) (*exec.Cmd, error) {
	py, err := hostPython()
	if err != nil {
		return nil, err
	}
	c := exec.Command(py, "-")
	c.Dir = cwd
	c.Stdin = strings.NewReader(code)
	return c, nil
}

// hostPython 依次查找宿主上的 python / python3,返回绝对路径。
func hostPython() (string, error) {
	for _, name := range []string{"python", "python3"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到 Python 解释器(已尝试 python / python3),请安装并加入 PATH")
}
