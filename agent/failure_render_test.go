package agent

import (
	"strings"
	"testing"

	"deepx/tools"
)

// Phase 2:NormalizeToolResult 兼容 + RenderToolResultContent 语义隔离。
// Phase 3:失败分支渲染为 Failure Protocol(<tool_failure>)。

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
	// 成功 = Output 原样
	ok := tools.ToolResult{Success: true, Output: "已写入 a.go"}
	if got := RenderToolResultContent(ok); got != "已写入 a.go" {
		t.Fatalf("成功应原样 Output,got %q", got)
	}
	// 失败 = Failure Protocol
	fail := tools.ToolResult{
		Success:         false,
		Error:           "在文件中未找到 old_string",
		Output:          "错误: 该文本不在文件中",
		FailureCategory: tools.FailureCategoryNotFound,
		FailureHint:     "请先 Read 文件确认实际内容",
	}
	got := RenderToolResultContent(fail)
	for _, want := range []string{
		"<tool_failure>", "protocol_version: 1", "status: failed", "category:", "not_found",
		"retryable: false", "recovery_action: inspect_before_retry",
		"summary:", "在文件中未找到 old_string", "diagnostic:", "错误: 该文本不在文件中", "</tool_failure>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("失败渲染应含 %q,got:\n%s", want, got)
		}
	}
	// FailureHint 不进协议(进 nudge),协议里不应有自由文本 hint
	if strings.Contains(got, "请先 Read 文件确认实际内容") {
		t.Fatalf("FailureHint 不应进协议(进 nudge),got:\n%s", got)
	}
}

// 旧工具(NormalizeToolResult 复制 Error=Output):summary 占位,避免与 diagnostic 重复。
func TestRenderToolFailureProtocol_SummaryDedup(t *testing.T) {
	r := tools.ToolResult{Success: false, Error: strings.Repeat("e", 300), Output: strings.Repeat("e", 300)}
	got := RenderToolFailureProtocol(r)
	if !strings.Contains(got, "—(同诊断,见下)") {
		t.Fatalf("Error==Output 时 summary 应占位去重,got:\n%s", got)
	}
	// 不同时正常显示摘要
	r2 := tools.ToolResult{Success: false, Error: "boom", Output: "stack trace"}
	if got2 := RenderToolFailureProtocol(r2); !strings.Contains(got2, "summary:\nboom") {
		t.Fatalf("Error≠Output 时 summary 应正常显示,got:\n%s", got2)
	}
}

// 不回归保证:失败时 Error(原因)与 Output(诊断)都不丢失(在协议字段内)。
func TestRenderToolResultContent_NoInfoLoss(t *testing.T) {
	r := tools.ToolResult{Success: false, Error: "exit status 1", Output: "stack trace"}
	got := RenderToolResultContent(r)
	if !strings.Contains(got, "exit status 1") || !strings.Contains(got, "stack trace") {
		t.Fatalf("失败渲染必须含原因与诊断,got %q", got)
	}
}

// Phase 3:summary 截断 200。
func TestRenderToolFailureProtocol_SummaryTruncated(t *testing.T) {
	long := strings.Repeat("e", 500)
	r := tools.ToolResult{Success: false, Error: long, Output: "d"}
	got := RenderToolFailureProtocol(r)
	// summary 行内容截断(找到 "summary:\n" 后到下一字段)
	idx := strings.Index(got, "summary:\n") + len("summary:\n")
	end := strings.Index(got[idx:], "\n\n")
	sum := got[idx : idx+end]
	body := strings.TrimSuffix(sum, "…") // 截断标记不计入内容长度
	if len(body) > failureSummaryMaxLen {
		t.Fatalf("summary 内容应 ≤%d,got %d", failureSummaryMaxLen, len(body))
	}
	if !strings.Contains(sum, "…") {
		t.Fatal("summary 截断应带 …")
	}
}

// Phase 3:diagnostic 截断 4000 + truncated 标记。
func TestRenderToolFailureProtocol_DiagnosticTruncated(t *testing.T) {
	long := strings.Repeat("x", 5000)
	r := tools.ToolResult{Success: false, Error: "boom", Output: long}
	got := RenderToolFailureProtocol(r)
	if !strings.Contains(got, "diagnostic truncated: true") {
		t.Fatal("diagnostic 超限应带 truncated 标记")
	}
	if strings.Count(got, "x") != failureDiagnosticMaxLen {
		t.Fatalf("diagnostic 应截断到 %d,got %d 个 x", failureDiagnosticMaxLen, strings.Count(got, "x"))
	}
}