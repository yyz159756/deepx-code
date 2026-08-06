# proposal: Failure Protocol v2 — 模型侧失败状态协议升级

## Why

Phase 3.1 的 `<tool_failure>` 是"格式包装"(status/category/summary/diagnostic),但模型仍不知道:失败是否可继续尝试(retryable)、下一步恢复方向(recovery_action)。模型需从诊断文本自行推断。

## What Changes

1. 协议升级:加 `protocol_version: 1`、`retryable`、`recovery_action` 字段(固定顺序)
2. 新增 `RecoveryAction` 枚举 + `IsRetryable`/`GetRecoveryAction` 纯函数(category → 策略,不依赖 tracker)
3. `FailureHint` 不进协议(双轨:recovery_action=结构化策略,FailureHint=自然语言进 nudge)

## 设计要点

- 纯渲染:protocol 层无业务判断(无 if category 分支);截断只在渲染副本,不改 ToolResult
- retryable 语义:**存在后续恢复可能性 ≠ 允许原参数立即重试**(行为控制仍由 nudge/tracker 负责)
- 保守默认:timeout/network → true;execution_error 等杂项桶 → false
- 字段顺序固定:token 稳定、缓存友好、diff 明确
- 不动 ToolResult / ChatMessage / tracker / nudge / system prompt

## 不做的事

- ❌ 不做 ChatMessage.Metadata(provider 不支持 tool message metadata,模型看不到;无消费方)
- ❌ 不改 nudge(Phase 1 行为控制与 Phase 3.3 状态表达分离)
