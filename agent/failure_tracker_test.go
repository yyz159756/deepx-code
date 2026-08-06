package agent

import (
	"strings"
	"testing"

	"deepx/tools"
)

// 工具失败恢复:handleToolFailure 分级升级 + 去重 + 成功清除。
// 不触发真实工具执行,纯逻辑单测。

func failureToolCall(name, args string) ToolCall {
	return ToolCall{ID: "c1", Function: ToolCallFunc{Name: name, Arguments: args}}
}

func TestHandleToolFailure_Escalation(t *testing.T) {
	ft := newFailureTracker()
	tc := failureToolCall("Update", `{"path":"a.go","old_string":"旧","new_string":"新"}`)
	res := tools.ToolResult{Output: "未找到", Success: false, FailureCategory: tools.FailureCategoryNotFound}

	// 第 1 次:标准引导
	n1, abort := handleToolFailure(tc, res, ft)
	if abort || !strings.Contains(n1, "不要原样重试") {
		t.Fatalf("第 1 次应给标准引导,got %q abort=%v", n1, abort)
	}
	// 第 2 次:soft
	n2, abort := handleToolFailure(tc, res, ft)
	if abort || !strings.Contains(n2, "连续失败 2 次") {
		t.Fatalf("第 2 次应 soft,got %q abort=%v", n2, abort)
	}
	// 第 3 次:hard
	n3, abort := handleToolFailure(tc, res, ft)
	if abort || !strings.Contains(n3, "禁止用相同参数") {
		t.Fatalf("第 3 次应 hard,got %q abort=%v", n3, abort)
	}
	// 第 4 次:级别已注入过(hard)→ 不重复注入,也不 abort
	n4, abort := handleToolFailure(tc, res, ft)
	if abort || n4 != "" {
		t.Fatalf("第 4 次不应重复注入,got %q abort=%v", n4, abort)
	}
	// 第 5 次:abort
	_, abort = handleToolFailure(tc, res, ft)
	if !abort {
		t.Fatal("第 5 次应 abort")
	}
}

func TestHandleToolFailure_SuccessClears(t *testing.T) {
	ft := newFailureTracker()
	tc := failureToolCall("Update", `{"path":"b.go","old_string":"旧","new_string":"新"}`)
	res := tools.ToolResult{Output: "未找到", Success: false, FailureCategory: tools.FailureCategoryNotFound}

	// 失败 2 次(soft 级)
	handleToolFailure(tc, res, ft)
	handleToolFailure(tc, res, ft)

	// 同工具同路径成功 → 清除
	ft.clearByTool(tc)
	if len(ft.counts) != 0 {
		t.Fatalf("成功后应清空计数,got %v", ft.counts)
	}

	// 再失败:应从第 1 级重新计(标准引导,非 soft)
	n, abort := handleToolFailure(tc, res, ft)
	if abort || !strings.Contains(n, "不要原样重试") || strings.Contains(n, "连续失败") {
		t.Fatalf("清除后应回到第 1 级,got %q abort=%v", n, abort)
	}
}

func TestHandleToolFailure_OldToolKeywordFallback(t *testing.T) {
	ft := newFailureTracker()
	// 无 FailureCategory 的旧工具(如 Explore 失败)→ 按关键词分类
	tc := failureToolCall("Explore", `{"task":"x"}`)
	res := tools.ToolResult{Output: "探索失败: connection refused", Success: false}
	n, abort := handleToolFailure(tc, res, ft)
	if abort {
		t.Fatal("不应 abort")
	}
	if !strings.Contains(n, "网络错误") {
		t.Fatalf("应按网络分类,got %q", n)
	}
}

func TestBashFingerprint_Normalize(t *testing.T) {
	ft := newFailureTracker()
	// 双空格与单空格应是同一指纹
	tc1 := failureToolCall("Bash", `{"command":"npm  install --x"}`)
	tc2 := failureToolCall("Bash", `{"command":"npm install --y"}`)
	fp1 := ft.fingerprint(tc1, tools.FailureCategoryExecution)
	fp2 := ft.fingerprint(tc2, tools.FailureCategoryExecution)
	if fp1 != fp2 {
		t.Fatalf("normalize 后应同指纹: %q vs %q", fp1, fp2)
	}
	// 不同子命令应是不同指纹
	tc3 := failureToolCall("Bash", `{"command":"python train.py"}`)
	fp3 := ft.fingerprint(tc3, tools.FailureCategoryExecution)
	if fp1 == fp3 {
		t.Fatal("不同命令不应同指纹")
	}
}
