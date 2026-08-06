# proposal: Failure Identity — 失败事件唯一身份

## Why

Phase 1/2/3 已实现恢复、结构化、协议化,但失败事件**缺少唯一身份**:多个失败无法稳定关联,UI 无法引用某次具体失败,debug 靠时间/文本匹配,未来 metadata protocol 无法引用 failure event。

```
Update: not_found × 3 → 系统无法区分"同一次失败"还是"三个独立失败"
```

## What Changes(修正范围)

1. 新增 `FailureID`(agent runtime 级):每次失败事件唯一标识,stream 局部递增(`f_001`、`f_002`)
2. `FailureTracker` 扩展:指纹 → 最近失败 ID 关联(首次失败生成,存 tracker)
3. `ToolCallResultMsg` 带 FailureID:UI/debug 事件通道(模型无需见 ID)

## 不做的事

- ❌ 修改 ToolResult(失败 ID 属 agent runtime,不进工具结果)
- ❌ 渲染协议加 `id:`(模型不消费,UI 走事件通道;待 Phase 3.3 metadata 时一并考虑)
- ❌ telemetry metrics(deepx 无 metrics 基建;结构化 log 亦无 logger 基建,暂不做)
- ❌ 改变模型恢复逻辑(恢复仍由 FailureCategory/Hint/Tracker 负责)

## 设计要点(详见 design.md)

- ID 格式 `f_<counter>`:纯身份,不编码业务(tool/category 已有字段)
- 生命周期:执行态,stream 结束清理;不进入长期 memory
- 稳定性:首次失败生成存 tracker,重放渲染复用同一 ID
