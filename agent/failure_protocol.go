package agent

import (
	"fmt"

	"deepx/tools"
)

// Failure Protocol v2:失败结果在模型上下文中的协议化表达(模型侧)。
// 目的:降低失败诊断文本被模型当作可执行模式复用的风险,并让模型明确
// 失败状态(status)、类别(category)、是否存在后续恢复可能性(retryable)、
// 恢复方向(recovery_action)。纯渲染层:不做业务判断(无 if category 分支),
// 不修改 ToolResult(截断只在渲染副本上),不依赖 tracker。
const (
	failureProtocolVersion = 1
	// failureSummaryMaxLen:summary 截断上限(旧工具 Error=Output 可能很长)。
	failureSummaryMaxLen = 200
	// failureDiagnosticMaxLen:diagnostic 截断上限(防编译日志/堆栈/stderr 污染上下文)。
	failureDiagnosticMaxLen = 4000
)

// RenderToolFailureProtocol 把失败 ToolResult 渲染为 <tool_failure> 协议文本。
// 字段顺序固定(protocol_version/status/category/retryable/recovery_action/summary/diagnostic):
// token 稳定、缓存友好、测试易写、后续 diff 明确。成功结果不走此协议。
func RenderToolFailureProtocol(r tools.ToolResult) string {
	summary := truncateUTF8(r.Error, failureSummaryMaxLen)
	// 旧工具(NormalizeToolResult 复制 Error=Output)summary 与 diagnostic 完全重复——
	// 占位避免 200 字 token 浪费;Error 非空且与 Output 不同才显示摘要。
	if r.Error != "" && r.Error == r.Output {
		summary = "—(同诊断,见下)"
	}
	action := string(GetRecoveryAction(r.FailureCategory))
	if action == "" {
		action = string(RecoveryAbort)
	}
	diag := truncateUTF8(r.Output, failureDiagnosticMaxLen)
	truncated := ""
	if len(r.Output) > failureDiagnosticMaxLen {
		truncated = "\ndiagnostic truncated: true"
	}
	return fmt.Sprintf(`<tool_failure>
protocol_version: %d
status: failed
category: %s
retryable: %t
recovery_action: %s

summary:
%s

diagnostic:
%s%s

</tool_failure>`,
		failureProtocolVersion,
		r.FailureCategory,
		IsRetryable(r.FailureCategory),
		action,
		summary,
		diag,
		truncated,
	)
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
