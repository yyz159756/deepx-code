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
}

func TestRunPython_Timeout(t *testing.T) {
	hostPythonOrSkip(t)
	res := RunPython(map[string]any{"code": `import time; time.sleep(5)`, "timeout": 1})
	if res.Success {
		t.Fatal("超时应失败")
	}
	if !strings.Contains(res.Output, "超时") {
		t.Fatalf("应含超时标记,got: %q", res.Output)
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
