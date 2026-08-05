package agent

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// mkTC 构造一个工具调用。
func mkTC(name, argsJSON string) ToolCall {
	return ToolCall{ID: "id1", Type: "function", Function: ToolCallFunc{Name: name, Arguments: argsJSON}}
}

// argsMap 把 arguments JSON 解回 map,断言仍是合法 JSON。
func argsMap(t *testing.T, argsJSON string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		t.Fatalf("结果不是合法 JSON: %v\n%s", err, argsJSON)
	}
	return m
}

func TestElideWriteContent_LargeReplacedWithReference(t *testing.T) {
	// 全中文大内容:老实现按字节切会切出半个 rune,这里应整体换成引用、不含乱码。
	content := strings.Repeat("这是一段中文内容。", 200) // 远超 512 字节
	in := mkTC("Write", `{"path":"a/b/中文.go","content":`+jsonStr(content)+`}`)

	out := rewriteToolCallArgsForHistory([]ToolCall{in})
	got := out[0].Function.Arguments

	if !utf8.ValidString(got) {
		t.Fatalf("结果含非法 UTF-8(切出了半个字符): %q", got)
	}
	m := argsMap(t, got)
	gotContent, _ := m["content"].(string)
	if strings.Contains(gotContent, "这是一段中文内容") == true && len(gotContent) > 200 {
		t.Fatalf("大 content 应被换成引用而非保留原文, got=%q", gotContent)
	}
	if !strings.Contains(gotContent, "已写入") || !strings.Contains(gotContent, "Read") {
		t.Fatalf("引用描述应含'已写入'和'Read'提示, got=%q", gotContent)
	}
	if !strings.Contains(gotContent, "a/b/中文.go") {
		t.Fatalf("引用描述应含文件路径, got=%q", gotContent)
	}
	if p, _ := m["path"].(string); p != "a/b/中文.go" {
		t.Fatalf("path 应保持不变, got=%q", p)
	}
}

func TestElideWriteContent_SmallKeptInline(t *testing.T) {
	in := mkTC("Write", `{"path":"x.txt","content":"小内容"}`)
	out := rewriteToolCallArgsForHistory([]ToolCall{in})
	if out[0].Function.Arguments != in.Function.Arguments {
		t.Fatalf("小 content 应原样保留\n want=%s\n got =%s", in.Function.Arguments, out[0].Function.Arguments)
	}
}

func TestElideWriteContent_NoHTMLEscape(t *testing.T) {
	// path 含 < > &,大 content 触发重编码;不应被转成 < 等。
	content := strings.Repeat("x", 600)
	in := mkTC("Write", `{"path":"a<b>&c.go","content":`+jsonStr(content)+`}`)
	got := rewriteToolCallArgsForHistory([]ToolCall{in})[0].Function.Arguments
	// 若 < > & 被 HTML 转义,原始 JSON 里 path 会变成 a<b>&c.go,
	// 就不再包含字面子串 "a<b>&c.go"。含字面子串即证明未转义。
	if !strings.Contains(got, "a<b>&c.go") {
		t.Fatalf("< > & 不应被 HTML 转义(path 应保持字面量), got=%s", got)
	}
	if p, _ := argsMap(t, got)["path"].(string); p != "a<b>&c.go" {
		t.Fatalf("path 应原样保留 < > &, got=%q", p)
	}
}

func TestUpdate_NeverTruncated(t *testing.T) {
	// Update 即使 old_string/new_string 巨大也一律原样保留。
	old := strings.Repeat("旧", 500)
	nw := strings.Repeat("新", 500)
	raw := `{"path":"f.go","old_string":` + jsonStr(old) + `,"new_string":` + jsonStr(nw) + `}`
	in := mkTC("Update", raw)
	out := rewriteToolCallArgsForHistory([]ToolCall{in})
	if out[0].Function.Arguments != raw {
		t.Fatalf("Update 应原样保留,不裁剪\n want=%s\n got =%s", raw, out[0].Function.Arguments)
	}
}

func TestOtherTools_Untouched(t *testing.T) {
	in := mkTC("Bash", `{"command":"`+strings.Repeat("echo ", 300)+`"}`)
	out := rewriteToolCallArgsForHistory([]ToolCall{in})
	if out[0].Function.Arguments != in.Function.Arguments {
		t.Fatalf("非 Write 工具不应被改动")
	}
}

func TestInvalidJSON_ReturnedAsIs(t *testing.T) {
	// 超过阈值但不是合法 JSON:原样返回,不 panic。
	broken := "{not json" + strings.Repeat("x", 600)
	if got := elideWriteContent(broken); got != broken {
		t.Fatalf("非法 JSON 应原样返回")
	}
}

func TestRewrite_DoesNotMutateOriginal(t *testing.T) {
	// 执行仍用原始 toolCalls:确认原始未被改动。
	content := strings.Repeat("y", 4000)
	raw := `{"path":"z.go","content":` + jsonStr(content) + `}`
	orig := []ToolCall{mkTC("Write", raw)}
	_ = rewriteToolCallArgsForHistory(orig)
	if orig[0].Function.Arguments != raw {
		t.Fatalf("原始 toolCalls 不应被改动(执行用的是它)")
	}
}

func TestRewrite_MixedBatch(t *testing.T) {
	bigWrite := `{"path":"big.go","content":` + jsonStr(strings.Repeat("a", 4000)) + `}`
	tcs := []ToolCall{
		mkTC("Write", bigWrite),
		mkTC("Update", `{"path":"u.go","old_string":"a","new_string":"b"}`),
		mkTC("Read", `{"path":"r.go"}`),
	}
	out := rewriteToolCallArgsForHistory(tcs)
	if c, _ := argsMap(t, out[0].Function.Arguments)["content"].(string); !strings.Contains(c, "已写入") {
		t.Fatalf("批次中的大 Write 应被换引用, got=%q", c)
	}
	if out[1].Function.Arguments != tcs[1].Function.Arguments {
		t.Fatalf("批次中的 Update 应原样")
	}
	if out[2].Function.Arguments != tcs[2].Function.Arguments {
		t.Fatalf("批次中的 Read 应原样")
	}
}

// jsonStr 把字符串编成合法 JSON 字面量(带引号、转义),供拼接测试用例。
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
