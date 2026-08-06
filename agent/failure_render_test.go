package agent

import (
	"strings"
	"testing"

	"deepx/tools"
)

// Phase 2:NormalizeToolResult 兼容 + RenderToolResultContent 语义隔离。

func TestNormalizeToolResult_LegacyFallback(t *testing.T) {
	// 旧工具:失败只有 Output → Error=Output,Output 保留(不清空)
	r := NormalizeToolResult(tools.ToolResult{Success: false, Output: "gcc error: undefined"})
	if r.Error != r.Output || r.Output == "" {
		t.Fatalf("legacy 应 Error=Output 且 Output 保留,got Error=%q Output=%q", r.Error, r.Output)
	}
	// 已迁移工具:Error 已有 → 不覆盖
	orig := tools.ToolResult{Success: false, Output: "stack trace", Error: "command failed: exit status 1"}
	r2 := NormalizeToolResult(orig)
	if r2.Error != orig.Error {
		t.Fatalf("已迁移工具 Error 不应被覆盖,got %q", r2.Error)
	}
	// 成功结果:不动
	r3 := NormalizeToolResult(tools.ToolResult{Success: true, Output: "ok"})
	if r3.Error != "" {
		t.Fatalf("成功结果不应有 Error,got %q", r3.Error)
	}
}

func TestRenderToolResultContent(t *testing.T) {
	cases := []struct {
		name string
		r    tools.ToolResult
		want string
	}{
		{"成功=Output", tools.ToolResult{Success: true, Output: "已写入 a.go"}, "已写入 a.go"},
		{"失败 Error+Output=拼接", tools.ToolResult{Success: false, Error: "exit status 1", Output: "stack trace"}, "exit status 1\nstack trace"},
		{"失败只 Error", tools.ToolResult{Success: false, Error: "超时"}, "超时"},
		{"失败只 Output(兜底)", tools.ToolResult{Success: false, Output: "boom"}, "boom"},
	}
	for _, c := range cases {
		if got := RenderToolResultContent(c.r); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// 不回归保证:失败时 Error(原因)与 Output(诊断)都不丢失。
func TestRenderToolResultContent_NoInfoLoss(t *testing.T) {
	r := tools.ToolResult{Success: false, Error: "exit status 1", Output: "stack trace"}
	got := RenderToolResultContent(r)
	if !strings.Contains(got, "exit status 1") || !strings.Contains(got, "stack trace") {
		t.Fatalf("失败渲染必须含原因与诊断,got %q", got)
	}
}
