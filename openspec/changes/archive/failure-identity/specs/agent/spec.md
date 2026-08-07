# agent / 工具失败恢复

## ADDED Requirements

### Requirement: 失败事件唯一身份(FailureID)
The agent SHALL assign a unique FailureID to each tool failure event (stream-local, monotonic), associate it with the failure fingerprint, and expose it to the UI through the tool-result event message — without entering ToolResult, the failure protocol rendering, or model recovery logic.

#### Scenario: 每次失败生成唯一 ID
- **WHEN** a tool call fails
- **THEN** the agent SHALL generate a stream-local unique FailureID (e.g. `f_001`) for that failure event
- **AND** consecutive failures of the same fingerprint SHALL get distinct IDs (each failure is a distinct event)
- **AND** the ID SHALL be associated with the failure fingerprint in the tracker

#### Scenario: ID 不进入工具结果与模型恢复
- **WHEN** a tool call fails
- **THEN** the FailureID SHALL NOT be added to the `ToolResult` structure
- **AND** the FailureID SHALL NOT be rendered inside the `<tool_failure>` protocol (model does not consume it)
- **AND** model recovery logic (category/hint/tracker escalation) SHALL NOT depend on the ID

#### Scenario: UI 通过事件通道获取 ID
- **WHEN** a tool call fails and the agent emits the tool-result event to the UI
- **THEN** the event message SHALL carry the FailureID (when available)
- **AND** the failure display in the UI MAY show the ID alongside the failure marker
