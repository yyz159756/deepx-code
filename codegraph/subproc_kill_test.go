package codegraph

import (
	"os/exec"
	"testing"
	"time"
)

// killTree 的孤儿防护:构建子进程超时/失败后必须能清掉进程(及其树),否则孙进程
// (go list)残留堆积(实测 92+ 进程卡死系统)。两个轻量验证:未启动幂等 + 真能杀。

func TestKillTree_NoopWhenNotStarted(t *testing.T) {
	// Process==nil(未启动/已回收)→ 不应 panic、不应报错
	killTree(&exec.Cmd{})
}

func TestKillTree_KillsProcess(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "ping -t 127.0.0.1") // 无限挂起的子进程
	if err := cmd.Start(); err != nil {
		t.Skipf("无法启动子进程: %v", err)
	}
	time.Sleep(800 * time.Millisecond) // 确保进程真的在跑

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	killTree(cmd)
	select {
	case <-done:
		// 顶层进程被清,Wait 返回
	case <-time.After(5 * time.Second):
		t.Fatal("killTree 后顶层进程仍未退出")
	}
}
