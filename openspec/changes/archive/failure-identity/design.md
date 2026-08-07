# design: Failure Identity

## 上下文

Phase 1/2/3 已落地。本 change 给失败事件建立唯一身份,服务 debug/UI/未来 protocol 引用。

## 决策记录

### D1. FailureID 生成
stream 局部递增 counter(`f_001`、`f_002`…),在 `handleToolFailure` 首次生成,存 `failureTracker`。
- 纯身份,不编码业务(tool/category 已在其它字段)
- 单 stream 内唯一即够(失败事件只在 stream 生命周期内有效)

### D2. FailureTracker 扩展
保持既有 `counts map[string]int` + `lastNudge map[string]int`(Phase 1 测试依赖,不破坏),新增:
```go
ids     map[string]string // fingerprint → 最近失败 ID
counter int              // stream 局部递增
```
- `bump` 时若无 ID 则生成(首次失败),存 ids[fp]
- `clearByTool`/`clear` 同步删 ids[fp](成功清除)
- `LastID(fp)` 供渲染/UI 取

### D3. 事件通道(不进渲染协议)
ID 通过 `ToolCallResultMsg.FailureID` 发给 UI(已有事件消息,llm.go:1111),**不进 `<tool_failure>` 渲染**:
- 模型不消费 ID(纯身份引用),塞进上下文是语义噪音 + token 成本
- UI/debug 从事件消息拿 ID,不从模型上下文文本解析
- 若未来 Phase 3.3 metadata 需要引用,ID 已在 runtime 可用

### D4. 稳定性
ID 首次生成存 tracker;同一 fingerprint 后续失败生成**新** ID(事件语义:每次失败一个事件);重放/重连渲染历史时复用已存 ID(不重新生成)。

### D5. 生命周期
执行态:StartStream 局部(同 tracker),stream 结束清理;不进入长期 memory/压缩摘要(压缩保留 failure summary 即可)。

## 数据流

```
executeTool → ToolResult(Success:false)
    ↓
handleToolFailure(ft)
    ├─ bump:生成 FailureID(f_001)存 ids[fp]
    ├─ 分级:1/2/3/5(不变)
    ↓
ToolCallResultMsg{FailureID: f_001, ...} → UI(显示 ID)
convo tool message(<tool_failure> 协议,不含 ID)
```
