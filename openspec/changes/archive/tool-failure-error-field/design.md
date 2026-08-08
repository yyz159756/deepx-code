# design: 工具失败结构化状态

## 上下文

Phase 1 已落地 `FailureCategory`/`FailureHint` + 恢复引导 + 复读止损(基线 `openspec/specs/agent/spec.md`)。Phase 2 把失败原因升级为结构化字段,与工具观察分离。

## 决策记录

### D1. Error = 失败摘要,Output = 原始观察(保留)
| 字段 | 职责 |
|---|---|
| Output | 工具产生的原始观察(成功结果 / 失败时的诊断输出,如编译错误、堆栈) |
| Error | 失败摘要:为什么失败(exit status / 未找到目标 / 参数不合法…) |

**字段注释铁律**(防未来维护者写反):
```go
// Error is a short failure summary.
// It describes WHY execution failed.
// Diagnostic details should remain in Output.
Error string
```
禁止反模式:`Error = stderr; Output = ""`(为结构化丢掉 agent 最需要的诊断信息)。

**Bash 关键决策**:`Error` = `command failed: exit status 1`(带"command failed"前缀,独立进上下文时语义完整);`Output` = stdout+stderr 原样。

### D2. convo 渲染:函数化,失败 = Error + "\n" + Output
不散落在 llm.go——定义独立函数(未来 Phase 3 扩展格式只改一处):
```go
func RenderToolResultContent(r tools.ToolResult) string {
    if r.Success {
        return r.Output
    }
    switch {
    case r.Error != "" && r.Output != "":
        return r.Error + "\n" + r.Output // 原因 + 诊断,两个都要
    case r.Error != "":
        return r.Error
    default:
        return r.Output
    }
}
```

### D3. 兼容转换放 agent 入口
`NormalizeToolResult` 在 `executeTool` 返回后调用(非 tool 层——tool 不知道历史兼容需求):
```go
// legacy fallback:旧工具只有 Output,无法提供摘要 → Error=Output 且 Output 保留。
// 已知副作用:legacy 失败时 Error 与 Output 内容重复——可接受的取舍,
// 新迁移工具应提供简洁 Error 摘要(避免重复)。
if !r.Success && r.Error == "" && r.Output != "" {
    r.Error = r.Output
}
```
旧工具失败:Error=Output(摘要=原文),行为与 Phase 1 完全一致,无回归;reviewer 疑问("为什么重复")由此注释回答。

### D4. UI 保持兼容
失败显示沿用现有逻辑(Output 渲染);不强推结构化格式。category 徽标为可选增强,低优先。

### D5. 不动 FailureTracker / nudge
tracker 依赖 `FailureCategory + toolCall + args`(不依赖 Output/Error),nudge 通道不变。

## 数据流

```
executeTool → result
    ↓
NormalizeToolResult(兼容:Error=Output fallback)
    ↓
convo 渲染:
  success → Content = Output
  failure → Content = Error + "\n" + Output
    ↓
失败 → handleToolFailure(Phase 1,不动)
```

## 消费点(必须全改)

1. `agent/llm.go` convo tool 消息 Content(核心:失败不能渲染空)
2. `agent/llm.go` ToolCallResultMsg.Output(UI 消息,失败时传 Error+Output 或保持 Output?——保持 Output 兼容,UI 展示不变)
3. `tui/tool_display.go` 失败渲染(保持现状,Output 即可)
