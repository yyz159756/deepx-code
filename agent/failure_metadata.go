package agent

import "deepx/tools"

// RecoveryAction 失败后的结构化恢复方向(机器可读,供协议/planner/自动策略)。
// 与 FailureHint(自然语言)双轨:RecoveryAction 是枚举策略,FailureHint 是人/模型可读的引导。
type RecoveryAction string

const (
	RecoveryInspectBeforeRetry RecoveryAction = "inspect_before_retry"
	RecoveryModifyArguments    RecoveryAction = "modify_arguments"
	RecoveryRetryWithBackoff   RecoveryAction = "retry_with_backoff"
	RecoveryRequestPermission  RecoveryAction = "request_permission"
	RecoveryAbort              RecoveryAction = "abort"
)

// IsRetryable 判断该失败类别是否存在"继续尝试的可能"(通过调整条件/等待/改变恢复路径后重试)。
// 注意:retryable=true ≠ 允许原参数立即重试 —— 行为控制由 FailureTracker/nudge(Phase 1)负责,
// 这里只表达"是否存在后续恢复可能性"。保守默认:execution_error 这类杂项桶一律 false。
func IsRetryable(category tools.FailureCategory) bool {
	switch category {
	case tools.FailureCategoryTimeout, tools.FailureCategoryNetwork:
		return true
	default:
		return false
	}
}

// GetRecoveryAction 返回失败类别的结构化恢复方向(纯函数,不依赖 tracker)。
func GetRecoveryAction(category tools.FailureCategory) RecoveryAction {
	switch category {
	case tools.FailureCategoryNotFound:
		return RecoveryInspectBeforeRetry
	case tools.FailureCategoryInvalidArgument:
		return RecoveryModifyArguments
	case tools.FailureCategoryPermissionDenied:
		return RecoveryRequestPermission
	case tools.FailureCategoryExecution:
		return RecoveryInspectBeforeRetry
	case tools.FailureCategoryTimeout, tools.FailureCategoryNetwork:
		return RecoveryRetryWithBackoff
	default:
		return RecoveryAbort
	}
}
