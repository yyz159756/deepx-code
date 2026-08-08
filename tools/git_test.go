package tools

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// execGit 测试辅助:在 dir 里跑 git。
func execGit(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd
}

// Git 工具测试:exec 直调,exit code 语义(git 约定)。

// 临时 git 仓库辅助:初始化 + 一次提交。
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := execGit(dir, args...)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("commit", "-q", "-m", "init")
	return dir
}

func TestGit_StatusSuccess(t *testing.T) {
	dir := newTestRepo(t)
	r := Git(map[string]any{"args": []any{"status", "--short"}, "cwd": dir})
	if !r.Success {
		t.Fatalf("status 应成功,got Success=%v Output=%q", r.Success, r.Output)
	}
	// 干净仓库 status --short 输出为空 —— 这是合法成功结果,不是错误。
	// 该空输出入历史后,agent 层序列化必须保证 tool 消息仍带 content 字段,
	// 否则严格后端 400 "missing field `content`"(见 agent/marshal_test.go 的
	// TestMarshalToolEmptyContentEmitsEmptyString)。
	if r.Output != "" {
		t.Fatalf("干净仓库 status --short 输出应为空,got %q", r.Output)
	}
}

func TestGit_LogOutput(t *testing.T) {
	dir := newTestRepo(t)
	r := Git(map[string]any{"args": []any{"log", "--oneline"}, "cwd": dir})
	if !r.Success {
		t.Fatalf("log 应成功,got %v %q", r.Success, r.Output)
	}
	if !strings.Contains(r.Output, "init") {
		t.Fatalf("log 应含提交信息,got %q", r.Output)
	}
}

// git diff 有差异时是正常结果(非失败)。注意:Windows git 2.45 默认 diff 有差异也 exit 0;
// 显式 --exit-code 才返回 1 —— 两者都应视为成功(有差异是观察结果,不是失败)。
func TestGit_DiffExit1IsSuccess(t *testing.T) {
	dir := newTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// exit 0 场景:默认 diff --stat
	r := Git(map[string]any{"args": []any{"diff", "--stat"}, "cwd": dir})
	if !r.Success {
		t.Fatalf("diff 有差异应视为成功,got Success=%v Output=%q", r.Success, r.Output)
	}
	// exit 1 场景:--exit-code 有差异 → 成功 + [exit] 1 标记
	r2 := Git(map[string]any{"args": []any{"diff", "--exit-code"}, "cwd": dir})
	if !r2.Success {
		t.Fatalf("diff --exit-code 有差异(exit 1)应视为成功,got Success=%v Output=%q", r2.Success, r2.Output)
	}
	if !strings.Contains(r2.Output, "[exit] 1") {
		t.Fatalf("exit 1 应带标记,got %q", r2.Output)
	}
}

// 操作类命令(git checkout 不存在的分支)exit 1 = 操作失败,必须 Success=false
// (git 对"目标不存在"也返回 1,统一当成功会吞掉真实错误)。
func TestGit_CheckoutExit1IsFailure(t *testing.T) {
	dir := newTestRepo(t)
	r := Git(map[string]any{"args": []any{"checkout", "nonexistent-branch"}, "cwd": dir})
	if r.Success {
		t.Fatalf("checkout 不存在的分支(exit 1)应失败,got Success=%v Output=%q", r.Success, r.Output)
	}
	if !strings.Contains(r.Output, "did not match") && !strings.Contains(r.Output, "error: pathspec") {
		t.Fatalf("checkout 失败应带诊断,got %q", r.Output)
	}
}

// 查询类 exit 1 = 正常;操作类 exit 1 = 失败(纯函数单测)。
func TestGitExit1IsNormal(t *testing.T) {
	cases := []struct {
		argv []string
		want bool
	}{
		{[]string{"diff", "--exit-code"}, true},
		{[]string{"grep", "x"}, true},
		{[]string{"log", "--oneline"}, true},
		{[]string{"status", "--short"}, true},
		{[]string{"-C", "/tmp", "diff", "--exit-code"}, true},  // 跳过全局选项
		{[]string{"checkout", "foo"}, false},
		{[]string{"merge", "foo"}, false},
		{[]string{"reset", "--hard"}, false},
		{[]string{"apply", "p.patch"}, false},
	}
	for _, c := range cases {
		if got := gitExit1IsNormal(c.argv); got != c.want {
			t.Errorf("gitExit1IsNormal(%v) = %v, want %v", c.argv, got, c.want)
		}
	}
}

// 非 git 目录 → git exit 128 → 失败 + 诊断保留。
func TestGit_NotARepositoryFails(t *testing.T) {
	dir := t.TempDir() // 非 git 仓库
	r := Git(map[string]any{"args": []any{"status"}, "cwd": dir})
	if r.Success {
		t.Fatalf("非仓库应失败,got Success=%v", r.Success)
	}
	if !strings.Contains(r.Output, "not a git repository") && !strings.Contains(r.Output, "128") {
		t.Fatalf("失败应带诊断(exit 128),got %q", r.Output)
	}
}

// args 空 → 拒绝。
func TestGit_EmptyArgs(t *testing.T) {
	r := Git(map[string]any{"args": []any{}})
	if r.Success {
		t.Fatal("空 args 应失败")
	}
}

// git 可执行文件不存在(exec.ErrNotFound,非 ExitError):真实错误必须进诊断,
// 否则模型只看到 "[exit] ?" 无从判断原因 —— 失败恢复协议失去诊断基础。
func TestGit_ExecutableNotFound(t *testing.T) {
	r := formatGitResult("", "", &exec.Error{Name: "git", Err: errors.New("executable file not found in %PATH%")}, nil)
	if r.Success {
		t.Fatal("git 缺失应失败")
	}
	if !strings.Contains(r.Output, "not found") {
		t.Errorf("诊断应保留真实错误,got Output=%q", r.Output)
	}
	if !strings.Contains(r.Error, "not found") {
		t.Errorf("Error 摘要应含真实原因,got %q", r.Error)
	}
	if r.FailureCategory != FailureCategoryExecution {
		t.Errorf("应为 execution,got %q", r.FailureCategory)
	}
}
