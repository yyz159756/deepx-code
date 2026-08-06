package tui

import (
	"strings"
	"testing"
)

// Python 工具调用行应显示 code 摘要(修复:原只显示 "Python",看不到执行内容)。

func TestFormatToolCallLine_PythonShowsCode(t *testing.T) {
	// 单行 code
	line := formatToolCallLine("Python", `{"code":"print(sum(range(100)))"}`)
	if line != "Python (print(sum(range(100))))" {
		t.Fatalf("单行 code 应显示在括号里,got %q", line)
	}
	// 多行 code:首行 + 行数提示
	multi := `{"code":"import json\nprint('hi')"}`
	line = formatToolCallLine("Python", multi)
	if !strings.HasPrefix(line, "Python (import json") || !strings.Contains(line, "(2 lines)") {
		t.Fatalf("多行 code 应取首行+行数,got %q", line)
	}
	// 超长 code:80 字符截断(由 formatToolCallLine 兜底,"Python (" + 80 + ")" = 89)
	long := `{"code":"` + strings.Repeat("x", 120) + `"}`
	line = formatToolCallLine("Python", long)
	if len(line) > 89 {
		t.Fatalf("超长 code 应截断到 80+括号,got len=%d %q", len(line), line)
	}
}

func TestFirstLineSummary(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"single", "single"},
		{"a\nb", "a …(2 lines)"},
		{"a\nb\nc", "a …(3 lines)"},
	}
	for _, c := range cases {
		if got := firstLineSummary(c.in); got != c.want {
			t.Errorf("firstLineSummary(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
