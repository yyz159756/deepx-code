package agent

import (
	"fmt"

	"deepx/tools"
)

// Failure Protocol:失败结果在模型上下文中的协议化表达。
// 目的:降低失败诊断文本被模型当作可执行模式复用的风险 —— 用明确的失败事件标记
// (<tool_failure>) 区分"失败事件"与"普通 observation"。表达层隔离:不改 ToolResult。
const (
	// failureSummaryMaxLen:summary 截断上限(旧工具 Error=Output 可能很长)。
	failureSummaryMaxLen = 200
	// failureDiagnosticMaxLen:diagnostic 截断上限(防编译日志/堆栈/stderr 污染上下文)。
	failureDiagnosticMaxLen = 4000
)

// RenderToolFailureProtocol 把失败 ToolResult 渲染为 <tool_failure> 协议。
// 字段:status(FAILED)/ category / summary(Error,≤200)/ recovery(FailureHint 或
// category 默认动作)/ diagnostic(Output,≤4000,截断带标记)。成功结果不走此协议。
func RenderToolFailureProtocol(r tools.ToolResult) string {
	summary := truncateUTF8(r.Error, failureSummaryMaxLen)
	recovery := r.FailureHint
	if recovery == "" {
		recovery = defaultFailureAction(r.FailureCategory)
	}
	diag := truncateUTF8(r.Output, failureDiagnosticMaxLen)
	truncated := ""
	if len(r.Output) > failureDiagnosticMaxLen {
		truncated = "\ndiagnostic truncated: true"
	}
	return fmt.Sprintf(`<tool_failure>

status:
FAILED

category:
%s

summary:
%s

recovery:
%s

diagnostic:
%s%s

</tool_failure>`, r.FailureCategory, summary, recovery, diag, truncated)
}

// truncateUTF8 按字节截断到 max,超限加 … 且不截断多字节字符中间。
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && s[cut]&0xC0 == 0x80 { // 落在 UTF-8 续字节,回退到字符边界
		cut--
	}
	return s[:cut] + "…"
}
