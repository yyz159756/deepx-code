package agent

import (
	"strings"
	"testing"

	"deepx/tools"
)

// Phase 3.3:IsRetryable / GetRecoveryAction 映射(纯函数)。

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		cat  tools.FailureCategory
		want bool
	}{
		{tools.FailureCategoryTimeout, true},
		{tools.FailureCategoryNetwork, true},
		{tools.FailureCategoryNotFound, false},
		{tools.FailureCategoryInvalidArgument, false},
		{tools.FailureCategoryPermissionDenied, false},
		{tools.FailureCategoryExecution, false}, // 杂项桶,保守 false
		{tools.FailureCategoryUnknown, false},
	}
	for _, c := range cases {
		if got := IsRetryable(c.cat); got != c.want {
			t.Errorf("IsRetryable(%q) = %v, want %v", c.cat, got, c.want)
		}
	}
}

func TestGetRecoveryAction(t *testing.T) {
	cases := []struct {
		cat  tools.FailureCategory
		want RecoveryAction
	}{
		{tools.FailureCategoryNotFound, RecoveryInspectBeforeRetry},
		{tools.FailureCategoryInvalidArgument, RecoveryModifyArguments},
		{tools.FailureCategoryPermissionDenied, RecoveryRequestPermission},
		{tools.FailureCategoryExecution, RecoveryInspectBeforeRetry},
		{tools.FailureCategoryTimeout, RecoveryRetryWithBackoff},
		{tools.FailureCategoryNetwork, RecoveryRetryWithBackoff},
		{tools.FailureCategoryUnknown, RecoveryAbort},
	}
	for _, c := range cases {
		if got := GetRecoveryAction(c.cat); got != c.want {
			t.Errorf("GetRecoveryAction(%q) = %q, want %q", c.cat, got, c.want)
		}
	}
}

// 渲染字段顺序稳定:protocol_version/status/category/retryable/recovery_action 在前,summary/diagnostic 在后。
func TestRenderToolFailureProtocol_FieldOrder(t *testing.T) {
	r := tools.ToolResult{Success: false, Error: "boom", Output: "diag", FailureCategory: tools.FailureCategoryNetwork}
	got := RenderToolFailureProtocol(r)
	expect := `<tool_failure>
protocol_version: 1
status: failed
category: network
retryable: true
recovery_action: retry_with_backoff`
	if !strings.HasPrefix(got, expect) {
		t.Fatalf("字段顺序/值不对,got:\n%s", got)
	}
}
