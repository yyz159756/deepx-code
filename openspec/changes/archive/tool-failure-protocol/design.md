# design: Failure Protocol 渲染

## 上下文

Phase 1/2 已落地(基线 `openspec/specs/agent/spec.md`):FailureCategory/Hint、恢复引导、指纹止损、Error/Output 分离。Phase 3 把失败结果的**上下文表达**协议化。

## 决策记录

### D1. 表达层隔离
Protocol 只负责"渲染给模型看的格式",不改 ToolResult——工具协议(结构)与模型表达(格式)解耦。

### D2. 协议格式
```
<tool_failure>
status: FAILED
category: <FailureCategory>
summary: <Error 摘要,≤200>
recovery: <FailureHint;空 → 按 category 默认动作>
diagnostic: <Output,≤4000>
</tool_failure>
```
- `status: FAILED` 最高优先级语义:模型首先知道"这不是成功 observation"
- tools 段渲染不走 markdown(既有),`<tool_failure>` 标签原样显示,无 HTML 冲突;ansi.Wrap 下折行不影响协议语义

### D3. 截断与标记
- summary ≤200:`NormalizeToolResult` 让旧工具 `Error=Output` 可能很长,截断防膨胀
- diagnostic ≤4000:防编译日志/堆栈/stderr 污染;**截断时加 `diagnostic truncated: true`**,模型知道需另行查看完整输出
- 层级:Protocol 截断(渲染内)→ `clampToolOutput`(16KB)→ `clampTurnToolOutput`(轮内合计),互不冲突

### D4. recovery 空值兜底
`FailureHint` 可空(旧工具)。空时渲染按 category 的默认动作(复用 Phase 1 `defaultFailureAction`),保证协议字段完整。

### D5. 与 nudge 并存
Protocol(是什么)→ nudge(怎么办)→ agent 决策。两者不互相替代,都不删除。

### D6. 兼容历史
只影响新渲染的失败消息;旧会话既有 `Error + "\n" + Output` 格式不回改(避免压缩/replay 行为变化)。

## 数据流

```
ToolResult{Success:false,...}
    ↓
RenderToolResultContent(失败分支)
    ↓
RenderToolFailureProtocol
    ├─ summary 截断 200
    ├─ recovery 兜底
    ├─ diagnostic 截断 4000(+truncated 标记)
    ↓
convo tool message <tool_failure>...</tool_failure>
    ↓
失败 → handleToolFailure(Phase 1,不变)→ nudge
```
