package tools

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Git 执行 git 命令,直调 git 可执行文件(不经 cmd/powershell —— 避免 Windows 下
// PowerShell 把 native stderr 当错误流、污染退出码导致的"git 成功却误报失败")。
//
// 参数:
//
//	args    ([]string, 必需) git 参数数组,如 ["status","--short"] / ["diff","HEAD"]
//	cwd     (string, 可选) 仓库目录;缺省用当前工作目录
//	timeout (int,   可选) 超时秒数,默认 60
//
// exit code 语义(git 约定,与通用 Bash 语义不同):
//
//	0  → Success=true,Output=stdout
//	1  → Success=true(如 diff 有差异 / 无匹配是正常结果),Output 带 "[exit] 1" 标记
//	≥2 → Success=false(真错误),Output=stdout+stderr(诊断保留),带 "[exit] N"
func Git(args map[string]any) ToolResult {
	argv, _ := args["args"].([]any)
	if len(argv) == 0 {
		return ToolResult{Output: "错误: args 参数为空", Success: false}
	}
	gitArgs := make([]string, 0, len(argv))
	for _, a := range argv {
		if s, ok := a.(string); ok && strings.TrimSpace(s) != "" {
			gitArgs = append(gitArgs, s)
		}
	}
	if len(gitArgs) == 0 {
		return ToolResult{Output: "错误: args 参数为空", Success: false}
	}
	cwd, _ := args["cwd"].(string)
	timeout := toInt(args["timeout"], 60)
	if timeout <= 0 {
		timeout = 60
	}

	cmd := exec.Command("git", gitArgs...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		return formatGitResult(stdout.String(), stderr.String(), err)
	case <-time.After(time.Duration(timeout) * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return ToolResult{
			Output:  fmt.Sprintf("%s\n%s\n[exit] 超时(%ds)", stdout.String(), stderr.String(), timeout),
			Success: false,
		}
	}
}

// formatGitResult 按 git exit code 语义格式化结果。
func formatGitResult(stdout, stderr string, err error) ToolResult {
	if err == nil {
		// exit 0:成功
		return ToolResult{Output: strings.TrimRight(stdout, "\n"), Success: true}
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		// git exit 1:正常结果(如 diff 有差异 / grep 无匹配),不是失败
		out := strings.TrimRight(stdout, "\n")
		if s := strings.TrimRight(stderr, "\n"); s != "" {
			if out != "" {
				out += "\n"
			}
			out += s
		}
		return ToolResult{Output: out + "\n[exit] 1", Success: true}
	}
	// exit ≥ 2:真错误。stdout+stderr 作为诊断完整保留(agent 层会截断/协议化)。
	out := strings.TrimRight(stdout, "\n")
	if s := strings.TrimRight(stderr, "\n"); s != "" {
		if out != "" {
			out += "\n"
		}
		out += s
	}
	exitCode := "?"
	if ee, ok := err.(*exec.ExitError); ok {
		exitCode = fmt.Sprintf("%d", ee.ExitCode())
	}
	return ToolResult{Output: out + fmt.Sprintf("\n[exit] %s", exitCode), Success: false}
}
