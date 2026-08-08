package agent

import "testing"

// 钉死 failNudge(失败恢复引导,role=user)的注入位置:
// 必须追加在所有 tool 结果之后;若插在 tool 组中间,会被 sanitizeToolPairs 的
// user 分支拆散配对 → 后续 tool 结果变孤儿被丢弃 + 悬挂 tool_call → 严格后端 400
// "assistant tool_calls must be followed by tool messages"(write-elision 执行记录同款 bug,
// 2026-08-07 12:20 现场:同轮 Bash 失败 + Bash 成功,failNudge 插中间)。

// 修复后形态:failNudge 在所有 tool 结果之后 → 配对必须完好。
func TestFailNudge_AfterAllTools_KeepsPairing(t *testing.T) {
	convo := []ChatMessage{
		{Role: "user", Content: "改文件并查看页面"},
		{Role: "assistant", ToolCalls: []ToolCall{
			{ID: "call_update", Type: "function", Function: ToolCallFunc{Name: "Update", Arguments: `{"path":"a.go","old_string":"A","new_string":"B"}`}},
			{ID: "call_snapshot", Type: "function", Function: ToolCallFunc{Name: "Bash", Arguments: `{"command":"playwright-cli snapshot"}`}},
		}},
		{Role: "tool", ToolCallID: "call_update", Name: "Update", Content: "<tool_failure>\nstatus: failed\ncategory: not_found\n</tool_failure>"},
		{Role: "tool", ToolCallID: "call_snapshot", Name: "Bash", Content: "### Page\n- URL: https://example.com"},
		// failNudge:所有 tool 之后(修复后的正确位置)
		{Role: "user", Content: "(工具 Update 调用失败(原因:未找到)。不要原样重试——先读取文件确认目标内容)"},
	}

	dropped, dangling := pairingReport(convo)
	if dropped != 0 {
		t.Errorf("不应有 tool 结果被丢弃, got %d 条", dropped)
	}
	if len(dangling) != 0 {
		t.Errorf("不应有悬挂 tool_calls, got %v", dangling)
	}
}

// 修复前形态:nudge 插在 tool 组中间 → 后续 tool 被丢 + 悬挂 tool_call。
// 这条是测试判别力证据:证明 pairingReport 能抓到 12:20 那种 400 前兆。
func TestFailNudge_InMiddle_BreaksPairing(t *testing.T) {
	convo := []ChatMessage{
		{Role: "user", Content: "改文件并查看页面"},
		{Role: "assistant", ToolCalls: []ToolCall{
			{ID: "call_update", Type: "function", Function: ToolCallFunc{Name: "Update", Arguments: `{"path":"a.go","old_string":"A","new_string":"B"}`}},
			{ID: "call_snapshot", Type: "function", Function: ToolCallFunc{Name: "Bash", Arguments: `{"command":"playwright-cli snapshot"}`}},
		}},
		{Role: "tool", ToolCallID: "call_update", Name: "Update", Content: "<tool_failure>\nstatus: failed\ncategory: not_found\n</tool_failure>"},
		// failNudge 插在中间(修复前的错误位置,即 llm.go 即时注入形态)
		{Role: "user", Content: "(工具 Update 调用失败(原因:未找到)。不要原样重试——先读取文件确认目标内容)"},
		{Role: "tool", ToolCallID: "call_snapshot", Name: "Bash", Content: "### Page\n- URL: https://example.com"},
	}

	dropped, dangling := pairingReport(convo)
	if dropped != 1 {
		t.Errorf("中间形态应丢弃 1 条 tool 结果, got %d", dropped)
	}
	if len(dangling) != 1 || dangling[0] != "call_snapshot" {
		t.Errorf("中间形态应悬挂 call_snapshot, got %v", dangling)
	}
}
