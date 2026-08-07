package tools

import "strings"

// FailureCategory 是工具失败时的机器可读类别,统一错误语义,供 agent 分类并生成恢复引导。
// Tool 在失败时设置;旧工具未设置时,agent 用 ClassifyFailure 按错误文本关键词回退。
// 常量名统一 FailureCategory* 前缀;值与常量名解耦,值保持小写 snake_case。
type FailureCategory string

const (
	FailureCategoryUnknown          FailureCategory = "unknown"
	FailureCategoryNotFound         FailureCategory = "not_found"
	FailureCategoryInvalidArgument  FailureCategory = "invalid_argument"
	FailureCategoryPermissionDenied FailureCategory = "permission_denied"
	FailureCategoryExecution        FailureCategory = "execution_error"
	FailureCategoryTimeout          FailureCategory = "timeout"
	FailureCategoryNetwork          FailureCategory = "network"
)

// AllFailureCategories 是 FailureCategory 全集,供策略表/UI/测试遍历
// (如"新增类别必须同步策略映射"的防漏保护,见 agent failure_metadata_test)。
// 新增常量时必须同步加入此列表。
var AllFailureCategories = []FailureCategory{
	FailureCategoryUnknown,
	FailureCategoryNotFound,
	FailureCategoryInvalidArgument,
	FailureCategoryPermissionDenied,
	FailureCategoryExecution,
	FailureCategoryTimeout,
	FailureCategoryNetwork,
}

// classifyRules 是错误文本关键词 → 类别的映射(agent 对未带 category 的旧工具做回退)。
// 同时覆盖中英文常见措辞;按更具体的类别优先(执行/网络/权限/超时在 generic 之前判断)。
var classifyRules = []struct {
	cat FailureCategory
	kw  []string
}{
	// execution 前置:含 "command not found" 这类精确短语,避免被 not_found 的 "not found" 抢走;
	// 不用泛化的 "失败/错误/无法/不能" 作独立关键词(会被任何中文错误文本命中,抢走 network 等更具体类别)。
	{FailureCategoryExecution, []string{"command not found", "exit", "failed", "error", "执行错误"}},
	{FailureCategoryPermissionDenied, []string{"permission denied", "access denied", "denied", "拒绝", "无权", "权限"}},
	{FailureCategoryTimeout, []string{"timeout", "timed out", "超时"}},
	{FailureCategoryNetwork, []string{"network", "connection", "dns", "refused", "unreachable", "http", "网络", "连接", "无法访问"}},
	{FailureCategoryNotFound, []string{"not found", "missing", "no such file", "不存在", "未找到", "找不到", "未启用"}},
	{FailureCategoryInvalidArgument, []string{"invalid", "错误:", "不能为空", "必须", "unknown op", "未知"}},
}

// ClassifyFailure 按错误输出文本关键词回退分类;无命中返回 unknown。
// 注意:这是兜底——正式接口是 ToolResult.FailureCategory,工具自带优先。
func ClassifyFailure(output string) FailureCategory {
	low := strings.ToLower(output)
	for _, r := range classifyRules {
		for _, kw := range r.kw {
			if strings.Contains(low, kw) {
				return r.cat
			}
		}
	}
	return FailureCategoryUnknown
}
