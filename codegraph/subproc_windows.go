//go:build windows

package codegraph

import "os/exec"

// setChildProcessGroup Windows no-op:树杀由 killTree 的 taskkill /T 实现。
func setChildProcessGroup(cmd *exec.Cmd) {}

// killTreeUnix Windows 不调用(走 taskkill 分支)。
func killTreeUnix(cmd *exec.Cmd) {}
