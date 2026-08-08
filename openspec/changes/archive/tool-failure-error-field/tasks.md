# tasks: 工具失败结构化状态

## 1. ToolResult + Error 字段
- [ ] 1.1 `tools/tools.go`:`ToolResult` 增加 `Error` 字段(注释:失败摘要,Output 保留原始观察)
- [ ] 1.2 确认序列化 / 现有构造点不受影响(字段可选,向后兼容)

## 2. Agent 兼容转换 + convo 渲染
- [ ] 2.1 新增 `NormalizeToolResult(r ToolResult) ToolResult`:旧工具 `!Success && Error=="" && Output!=""` → `Error=Output`(**不清空 Output**;注释说明 legacy 重复是可接受取舍)
- [ ] 2.2 `agent/llm.go` executeTool 后调用 NormalizeToolResult
- [ ] 2.3 新增 `RenderToolResultContent(r ToolResult) string`:成功 = Output;失败 = Error+Output(各自非空时拼接/兜底)
- [ ] 2.4 convo tool 消息 Content 改用 RenderToolResultContent
- [ ] 2.5 ToolCallResultMsg.Output 保持兼容(失败仍传完整展示文本)

## 3. 工具迁移(失败补 Error 摘要)
- [ ] 3.1 `tools/edit_file.go`:Update 各失败点补 `Error`(未找到/多处/参数错),Output 保留现有提示文本
- [ ] 3.2 `tools/command.go`:Bash 失败补 `Error: command failed: exit status N`(超时补 `command failed: 超时(Ns)`),Output 保留 stdout+stderr
- [ ] 3.3 `tools/write_file.go`:Write 失败补 `Error`(超限/路径/写入失败)

## 4. 测试
- [ ] 4.1 单测:`NormalizeToolResult` 兼容(旧工具 Error=Output 且 Output 不清空;已迁移工具 Error 不覆盖)
- [ ] 4.2 单测:`RenderToolResultContent` 四分支(成功=Output;Error+Output=拼接;只 Error=Error;只 Output=Output)
- [ ] 4.3 单测:**Error/Output 不丢失**(失败 `{Error:"exit status 1", Output:"stack trace"}` → 渲染含两者)
- [ ] 4.4 单测:Update/Bash/Write 失败返回含 Error 摘要
- [ ] 4.5 回归:Phase 1 测试(failure_tracker/恢复引导)全绿

## 5. 收尾
- [ ] 5.1 `go build ./...` + agent/tools 全量
- [ ] 5.2 归档:spec delta 应用进基线,change 移入 archive
