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

// failurePolicy 是单一失败类别的策略:是否可重试 + 结构化恢复方向。
type failurePolicy struct {
	retryable bool
	action    RecoveryAction
}

// failurePolicies 集中全部失败类别的策略映射(机制/策略分离):
// 策略(哪些类别可重试、恢复方向)只在此一张表维护,IsRetryable/GetRecoveryAction 查表取值,
// 改策略只动这一处,不散落 switch。表覆盖 AllFailureCategories 全部 7 个类别
// (有 TestAllFailureCategoriesHavePolicy 防漏:新增类别漏加策略会测试失败);
// unknown 显式入表 = {false, abort},语义与历史 default 分支一致。
var failurePolicies = map[tools.FailureCategory]failurePolicy{
	tools.FailureCategoryUnknown:          {retryable: false, action: RecoveryAbort},
	tools.FailureCategoryNotFound:         {retryable: false, action: RecoveryInspectBeforeRetry},
	tools.FailureCategoryInvalidArgument:  {retryable: false, action: RecoveryModifyArguments},
	tools.FailureCategoryPermissionDenied: {retryable: false, action: RecoveryRequestPermission},
	tools.FailureCategoryExecution:        {retryable: false, action: RecoveryInspectBeforeRetry},
	tools.FailureCategoryTimeout:          {retryable: true, action: RecoveryRetryWithBackoff},
	tools.FailureCategoryNetwork:          {retryable: true, action: RecoveryRetryWithBackoff},
}

// IsRetryable 判断该失败类别是否存在"继续尝试的可能"(通过调整条件/等待/改变恢复路径后重试)。
// 注意:retryable=true ≠ 允许原参数立即重试 —— 行为控制由 FailureTracker/nudge(Phase 1)负责,
// 这里只表达"是否存在后续恢复可能性"。保守默认:execution_error 这类杂项桶一律 false。
func IsRetryable(category tools.FailureCategory) bool {
	return failurePolicies[category].retryable
}

// GetRecoveryAction 返回失败类别的结构化恢复方向(纯函数,不依赖 tracker)。
func GetRecoveryAction(category tools.FailureCategory) RecoveryAction {
	if p, ok := failurePolicies[category]; ok {
		return p.action
	}
	return RecoveryAbort
}
