# tasks: Failure Protocol 渲染

## 1. 实现
- [ ] 1.1 新增 `agent/failure_protocol.go`:`RenderToolFailureProtocol(r tools.ToolResult) string`(模板:status/category/summary/recovery/diagnostic)
- [ ] 1.2 summary 截断 ≤200(截断加 `…`);diagnostic 截断 ≤4000(截断加 `diagnostic truncated: true`)
- [ ] 1.3 recovery 空值兜底:FailureHint 空 → `defaultFailureAction(category)`(复用 failure_tracker.go 既有函数)
- [ ] 1.4 `agent/llm.go`:`RenderToolResultContent` 失败分支改调 `RenderToolFailureProtocol`

## 2. 测试
- [ ] 2.1 更新 `agent/failure_render_test.go`:失败分支断言改为协议格式(含 status FAILED / category / summary / diagnostic)
- [ ] 2.2 新增截断测试:summary >200 截断;diagnostic >4000 截断 + truncated 标记
- [ ] 2.3 新增 recovery 空值兜底测试(无 hint → 默认动作)
- [ ] 2.4 回归:Normalize/不丢失/Phase 1 分级测试全绿

## 3. 收尾
- [ ] 3.1 `go build ./...` + agent/tools 全量
- [ ] 3.2 归档:delta 应用进基线,change 移 archive
