package agent

import (
	"strings"
	"testing"
)

// TestClassifyStreamResult 覆盖 issue #169 的两类异常响应分类:
// tool_call 被长度截断 / 空响应,以及正常情形不被误判。
func TestClassifyStreamResult(t *testing.T) {
	oneTool := []ToolCall{{ID: "x", Function: ToolCallFunc{Name: "Write", Arguments: `{"path":"a"`}}}

	cases := []struct {
		name      string
		content   string
		reasoning string
		toolCalls []ToolCall
		truncated bool
		want      streamOutcome
	}{
		// 会话 A:超大 Write 撞输出上限,tool_call arguments 残缺。
		{"truncated tool call", "", "", oneTool, true, outcomeTruncatedTool},
		// 有 content 的截断也归为截断工具(只要还带着工具调用)。
		{"truncated with content and tool", "思考中", "", oneTool, true, outcomeTruncatedTool},
		// 会话 B:供应商返回完全空。
		{"empty response", "", "", nil, false, outcomeEmpty},
		// finish_reason=length 但什么都没生成 → 也当空响应,催重试。
		{"truncated but nothing generated", "", "", nil, true, outcomeEmpty},
		// 正常:有文本、无工具。
		{"normal text", "结果如下", "", nil, false, outcomeNormal},
		// 正常:只有 reasoning(thinking 模型),非空。
		{"reasoning only", "", "让我想想", nil, false, outcomeNormal},
		// 正常:完整工具调用,未截断。
		{"complete tool call", "", "", oneTool, false, outcomeNormal},
		// 纯文本被截断(无工具)→ 走正常路径,交给 completionGate 催继续,不算截断工具。
		{"truncated plain text", "写到一半就断了", "", nil, true, outcomeNormal},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyStreamResult(c.content, c.reasoning, c.toolCalls, c.truncated)
			if got != c.want {
				t.Fatalf("classifyStreamResult(%q,%q,tools=%d,trunc=%v) = %v, want %v",
					c.content, c.reasoning, len(c.toolCalls), c.truncated, got, c.want)
			}
		})
	}
}

// TestCompletionGateCap 验证连续催继续不会无限循环:达到 maxGateNudges 后放行(返回空)。
func TestCompletionGateCap(t *testing.T) {
	nudges := 0
	got := 0
	for range maxGateNudges + 3 {
		if completionGate(true, nil, &nudges, "") != "" {
			got++
		}
	}
	if got != maxGateNudges {
		t.Fatalf("completionGate 应最多催 %d 次,实际 %d 次", maxGateNudges, got)
	}
}

// TestHasCommitment 验证"承诺未执行"检测:执行承诺 → true;完成性收尾 / 普通陈述 → false。
func TestHasCommitment(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"承诺:先写", "好的,先写 config.py 的内容", true},
		{"承诺:将创建", "我将创建 10 个文件", true},
		{"承诺:接下来调用", "接下来调用 Write 写入", true},
		{"承诺:准备执行", "我准备执行验证命令", true},
		{"承诺:英文 will write", "Now I will write batch 3 files", true},
		{"承诺:英文 let me create", "Let me create the remaining files", true},
		{"承诺:英文 next run", "Next, run the verification command", true},
		{"承诺:英文 gonna call", "I'm gonna call Write for each file", true},
		{"收尾:总结", "接下来我将总结一下结果", false},
		{"收尾:已完成", "任务已完成,总结如下", false},
		{"收尾:完毕", "就这些,完毕", false},
		{"收尾:英文 done", "All done, that's it", false},
		{"收尾:英文 summary", "Here is the summary of results", false},
		{"收尾:英文 finished", "Task finished, wrapping up", false},
		{"普通陈述", "这个文件的配置项有三个", false},
		{"普通英文陈述", "The file contains three config items", false},
		{"空文本", "", false},
		{"承诺+收尾(收尾优先)", "先写 config.py,最后总结", false},
	}
	for _, c := range cases {
		if got := hasCommitment(c.text); got != c.want {
			t.Errorf("[%s] hasCommitment(%q) = %v, want %v", c.name, c.text, got, c.want)
		}
	}
}

// TestCompletionGateCommitment 验证 completionGate 对"承诺未执行"返回催继续提示。
func TestCompletionGateCommitment(t *testing.T) {
	var nudges int
	got := completionGate(false, nil, &nudges, "我将先写 config.py")
	if got == "" {
		t.Fatalf("承诺未执行应催继续")
	}
	if !strings.Contains(got, "工具调用") {
		t.Fatalf("提示应点明未调用工具, got=%q", got)
	}
	// 完成性收尾不催。
	if got := completionGate(false, nil, &nudges, "任务已完成,总结如下"); got != "" {
		t.Fatalf("完成性收尾不应催继续, got=%q", got)
	}
}
