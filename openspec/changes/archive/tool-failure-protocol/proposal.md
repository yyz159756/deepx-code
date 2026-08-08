# proposal: Failure Protocol — 失败结果上下文协议化

## Why

Phase 1/2 后失败已有 `FailureCategory`/`Error`/`Output` 结构化与恢复引导,但失败结果仍以普通 tool content 文本进入模型上下文:

```
role=tool
old_string not found
expected:
abc
actual:
def
```

模型需自行判断"这是失败事件,诊断只是分析依据,不该作为后续工具调用模板复用"。弱模型或长上下文下,失败诊断文本可能被当作普通 observation 或执行模式吸收。

目标:增加统一 Failure Protocol 层,**降低失败诊断文本被模型当作可执行模式复用的风险**。

## What Changes

1. 新增 `RenderToolFailureProtocol()`:失败 ToolResult 渲染为 `<tool_failure>` 协议(status/category/summary/recovery/diagnostic)
2. `RenderToolResultContent` 失败分支改调 protocol(成功仍 Output)
3. 截断:summary ≤200、diagnostic ≤4000(带截断标记);recovery 空值兜底(按 category 默认动作)
4. 不改变 ToolResult / FailureTracker / nudge / UI / tool 实现

## 设计要点(详见 design.md)

- 表达层隔离:ToolResult 不动,Protocol 只负责模型上下文表达
- 与 nudge 职责分离:Protocol 回答"发生了什么",nudge 回答"下一步怎么办"
- 截断层级:Protocol 截断 → clampToolOutput → clampTurnToolOutput,三层不冲突
- 兼容历史:只影响新消息,不迁移旧会话
- 不影响 prefix cache(tool result 是动态 token)

## 不做的事

- ❌ 不改 ToolResult 结构
- ❌ 不改 nudge / tracker / UI / 工具实现
- ❌ 不迁移历史消息格式
- ❌ 不做 Phase 3.2(failure_id)/ 3.3(metadata)——后续独立演进
