package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

// hostPythonOrSkip 环境没有 python 解释器就跳过(精简环境不应硬失败)。
func hostPythonOrSkip(t *testing.T) {
	t.Helper()
	if _, err := hostPython(); err != nil {
		t.Skip("环境无 python 解释器,跳过 Python 工具测试")
	}
}

func TestRunPython_Print(t *testing.T) {
	hostPythonOrSkip(t)
	res := RunPython(map[string]any{"code": `print("hello-deepx")`})
	if !res.Success {
		t.Fatalf("应成功,got: %s", res.Output)
	}
	if !strings.Contains(res.Output, "hello-deepx") {
		t.Fatalf("输出缺 hello-deepx,got: %q", res.Output)
	}
}

func TestRunPython_QuotesUnscathed(t *testing.T) {
	// 核心卖点:代码含单/双引号、$、反引号,不经 shell 应原样执行。
	hostPythonOrSkip(t)
	code := "print(\"a'b\")\nprint('c\"d')\nprint(\"$HOME `echo`\")\n"
	res := RunPython(map[string]any{"code": code})
	if !res.Success {
		t.Fatalf("应成功,got: %s", res.Output)
	}
	for _, want := range []string{"a'b", `c"d`, "$HOME `echo`"} {
		if !strings.Contains(res.Output, want) {
			t.Fatalf("输出缺 %q,got: %q", want, res.Output)
		}
	}
}

func TestRunPython_Error(t *testing.T) {
	hostPythonOrSkip(t)
	res := RunPython(map[string]any{"code": `1/0`})
	if res.Success {
		t.Fatalf("除零应失败,got: %s", res.Output)
	}
	if !strings.Contains(res.Output, "ZeroDivisionError") {
		t.Fatalf("应含 traceback,got: %q", res.Output)
	}
}

func TestRunPython_EmptyCode(t *testing.T) {
	res := RunPython(map[string]any{"code": "  \n "})
	if res.Success {
		t.Fatal("空 code 应失败")
	}
	if res.FailureCategory != FailureCategoryInvalidArgument {
		t.Errorf("空 code 应为 invalid_argument,got %q", res.FailureCategory)
	}
	if res.Error == "" {
		t.Error("空 code 应带 Error 摘要")
	}
}

// TestRunPython_TimeoutResultCategory 快速测超时"状态转换逻辑"(类别/Error/Hint 填充):
// 纯函数 pythonTimeoutResult,不真实等 timeout 秒 —— 敏捷开发下本地秒级跑完。
func TestRunPython_TimeoutResultCategory(t *testing.T) {
	res := pythonTimeoutResult("已有部分输出", 30)
	if res.Success {
		t.Fatal("超时应失败")
	}
	if !strings.Contains(res.Output, "超时") {
		t.Errorf("输出应含超时标记,got: %q", res.Output)
	}
	if res.FailureCategory != FailureCategoryTimeout {
		t.Errorf("超时结果应为 timeout,got %q", res.FailureCategory)
	}
	if res.Error == "" {
		t.Error("超时应带 Error 摘要")
	}
	if res.FailureHint == "" {
		t.Error("超时应带 FailureHint")
	}
}

// TestRunPython_Timeout 真实超时属系统行为验证:进程确实被杀、输出被保留。
// 本地敏捷开发用 go test -short 跳过(状态转换逻辑已由上方纯函数测试覆盖),CI 全量验证。
func TestRunPython_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("真实超时行为留 CI 全量验证,本地 -short 跳过")
	}
	hostPythonOrSkip(t)
	res := RunPython(map[string]any{"code": `import time; time.sleep(5)`, "timeout": 1})
	if res.Success {
		t.Fatal("超时应失败")
	}
	if !strings.Contains(res.Output, "超时") {
		t.Fatalf("应含超时标记,got: %q", res.Output)
	}
	if res.FailureCategory != FailureCategoryTimeout {
		t.Errorf("超时应为 timeout,got %q", res.FailureCategory)
	}
	if res.Error == "" {
		t.Error("超时应带 Error 摘要")
	}
}

func TestRunPython_Cwd(t *testing.T) {
	hostPythonOrSkip(t)
	dir := t.TempDir()
	res := RunPython(map[string]any{
		"code": `import os; print(os.getcwd())`,
		"cwd":  dir,
	})
	if !res.Success {
		t.Fatalf("应成功,got: %s", res.Output)
	}
	abs, _ := filepath.Abs(dir)
	if !strings.Contains(res.Output, abs) {
		t.Fatalf("cwd 未生效,got: %q want: %s", res.Output, abs)
	}
}
