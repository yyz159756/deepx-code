//go:build !windows

package codegraph

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// Unix 进程组树杀:子进程(sh)带孙进程(sleep),killTree 必须连孙进程一起杀
// (只杀顶层会残留孤儿 —— 对应生产路径 go list 孙进程,父杀子后取消信号无法传导)。
func TestKillTree_KillsProcessGroup(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "sleep 100 & wait") // sh + 孙进程 sleep
	setChildProcessGroup(cmd)                                // 与生产路径一致:独立进程组
	if err := cmd.Start(); err != nil {
		t.Skipf("无法启动子进程: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	killTree(cmd)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("killTree 后顶层进程仍未退出")
	}
	// 进程组应已清空:信号 0 探测组(-pgid),组内无存活进程 → ESRCH
	time.Sleep(300 * time.Millisecond) // 让 kill 信号送达孙进程
	if err := syscall.Kill(-cmd.Process.Pid, 0); err == nil {
		t.Fatalf("killTree 后进程组仍有存活进程(孙进程 sleep 未清),组=%d", cmd.Process.Pid)
	} else if !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("探测进程组意外错误: %v", err)
	}
}
