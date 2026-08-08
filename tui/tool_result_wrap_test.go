package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// 工具结果段(kindTools)超长行自动换行:失败提示/长命令在窄终端折行而非截断。

// 失败提示超长行 → 折行成多行(而非视口截断)。
func TestKindTools_WrapsLongFailureLine(t *testing.T) {
	m := &model{chatContent: newChatLog(0)}
	long := "✗ Bash 失败: warning: " + strings.Repeat("x", 300)
	m.chatContent.Open(kindTools, long+"\n")

	rendered := ansi.Strip(m.renderChatBaseContent(40)) // 窄宽 → 必然 wrap
	lines := strings.Count(rendered, "\n")
	if lines < 2 {
		t.Fatalf("超长失败行应折成多行,got %d 行:\n%s", lines, rendered)
	}
	// 内容完整:折行后不应丢字符(截断是 … 结尾,wrap 是完整保留)
	if !strings.Contains(rendered, "warning:") || strings.Contains(rendered, "…") {
		t.Fatalf("wrap 应保留完整内容,不应截断:\n%s", rendered)
	}
}

// 多行工具列表:单 \n 语义保留(不合并成一行 —— 跳过 markdown 的原因)。
func TestKindTools_PreservesLineBreaks(t *testing.T) {
	m := &model{chatContent: newChatLog(0)}
	m.chatContent.Open(kindTools, "📄 Read (a.go)\n🔍 Grep (b.go)\n")

	rendered := ansi.Strip(m.renderChatBaseContent(40))
	if !strings.Contains(rendered, "Read (a.go)") || !strings.Contains(rendered, "Grep (b.go)") {
		t.Fatalf("两行工具应各自保留:\n%s", rendered)
	}
	// 两行之间不应被合并(各自独立存在,中间允许 wrap 出的额外行)
	if !strings.Contains(rendered, "Read (a.go)\n") && !strings.Contains(rendered, "Read (a.go)\n  ") {
		t.Fatalf("工具行应保留行边界:\n%s", rendered)
	}
}

// diff 块染色行 wrap 后颜色延续(ANSI 序列安全)。
func TestKindTools_WrapPreservesDiffColor(t *testing.T) {
	m := &model{chatContent: newChatLog(0)}
	long := strings.Repeat("y", 200)
	m.chatContent.Open(kindTools, "~~~diff\n+"+long+"\n"+long+"\n~~~\n")

	rendered := m.renderChatBaseContent(40) // 带 ANSI
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatal("diff 染色应产生 ANSI 颜色")
	}
	// wrap 后内容完整(颜色序列存在 + y 总数不丢;wrap 会断开连续串,故按总数校验)
	stripped := ansi.Strip(rendered)
	if got := strings.Count(stripped, "y"); got != 400 {
		t.Fatalf("diff 两行各 200 y,wrap 后应完整(400 个),got %d", got)
	}
}
