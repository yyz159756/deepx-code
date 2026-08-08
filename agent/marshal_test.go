package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMarshalAssistantEmptyContentEmitsEmptyString 防止回归:
// 模型只输出 reasoning_content 时,assistant 消息序列化必须含 content 字段(哪怕空字符串),
// 否则 DeepSeek API 会 400 "Invalid assistant message: content or tool_calls must be set"。
func TestMarshalAssistantEmptyContentEmitsEmptyString(t *testing.T) {
	m := ChatMessage{
		Role:             "assistant",
		Content:          "",
		ReasoningContent: "internal thoughts...",
		ToolCalls:        nil,
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal err: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"content":""`) {
		t.Errorf("expected content field present (empty string), got: %s", s)
	}
}

// TestMarshalAssistantWithToolCallsOmitsContentOK:
// 有 tool_calls 时不需要 content,空 content 仍可省略(omitempty 生效)。
func TestMarshalAssistantWithToolCallsOmitsContentOK(t *testing.T) {
	m := ChatMessage{
		Role:    "assistant",
		Content: "",
		ToolCalls: []ToolCall{
			{ID: "call_1", Type: "function", Function: ToolCallFunc{Name: "Read"}},
		},
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal err: %v", err)
	}
	s := string(b)
	if strings.Contains(s, `"content"`) {
		t.Errorf("assistant with tool_calls and empty content shouldn't emit content, got: %s", s)
	}
	if !strings.Contains(s, `"tool_calls"`) {
		t.Errorf("expected tool_calls present, got: %s", s)
	}
}

// TestMarshalUserMessageStillOmits:
// 非 assistant 角色 + 空 content,应该按原逻辑 omitempty 省略 content。
func TestMarshalUserMessageStillOmits(t *testing.T) {
	m := ChatMessage{Role: "system", Content: ""}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal err: %v", err)
	}
	if strings.Contains(string(b), `"content"`) {
		t.Errorf("system with empty content should still be omitted, got: %s", string(b))
	}
}

// TestMarshalToolEmptyContentEmitsEmptyString 防止回归:
// tool 消息 content 为空(如 git status --short 在干净工作区输出为空、exit 0)时,序列化必须
// 含 content 字段(空串)——OpenAI 规范 tool 消息 content 必填,缺字段会被严格后端 400
// "messages[N]: missing field `content`"(issue #94 家族)。
func TestMarshalToolEmptyContentEmitsEmptyString(t *testing.T) {
	m := ChatMessage{Role: "tool", ToolCallID: "call_1", Content: ""}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal err: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"content":""`) {
		t.Errorf("expected tool message to emit content field (empty string), got: %s", s)
	}
	if !strings.Contains(s, `"tool_call_id":"call_1"`) {
		t.Errorf("expected tool_call_id present, got: %s", s)
	}
}
