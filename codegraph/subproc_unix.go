//go:build !windows

package codegraph

import (
	"os/exec"
	"syscall"
)

// setChildProcessGroup 让子进程进入独立进程组(组 ID = 子进程 pid)。
// 孙进程(go list 等)继承该组,killTree 按 -pid 杀整组,保证超时/失败后不留孤儿
// (只杀顶层会残留孙进程:父进程杀掉子进程后,子进程内部的取消信号无法传导给孙进程)。
func setChildProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTreeUnix 杀整个进程组(含孙进程)。组不存在(未 Setpgid / 组已空)→ 回退杀顶层。
func killTreeUnix(cmd *exec.Cmd) {
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}
