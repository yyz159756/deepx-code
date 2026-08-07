package tui

import "testing"

// Git 工具调用行显示 args 摘要。
func TestFormatToolCallLine_GitShowsArgs(t *testing.T) {
	line := formatToolCallLine("Git", `{"args":["status","--short"]}`)
	if line != "Git (status --short)" {
		t.Fatalf("Git 调用行应显示 args,got %q", line)
	}
	// 多参数 + cwd
	line = formatToolCallLine("Git", `{"args":["log","--oneline","-5"],"cwd":"/repo"}`)
	if line != "Git (log --oneline -5)" {
		t.Fatalf("多参数应 join 显示,got %q", line)
	}
}
