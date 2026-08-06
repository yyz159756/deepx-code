# agent / 工具失败恢复

## ADDED Requirements

### Requirement: 失败结果上下文协议化(Failure Protocol)
The agent SHALL render failed tool results in model context as a structured failure protocol (status/category/summary/recovery/diagnostic) instead of plain `Error + "\n" + Output`, so the model can distinguish a failure event from a normal observation.

#### Scenario: 失败结果渲染为协议
- **WHEN** a tool call returns `Success:false`
- **THEN** the tool message in context SHALL be rendered as `<tool_failure>` protocol containing: `status: FAILED`, `category`, `summary`, `recovery`, and `diagnostic`
- **AND** `summary` SHALL be the failure summary (from `Error`), truncated to at most 200 chars
- **AND** `diagnostic` SHALL carry the raw diagnostic observation (from `Output`), truncated to at most 4000 chars with a `diagnostic truncated: true` marker when truncated
- **AND** `recovery` SHALL be the tool hint, or a category-default action when the hint is empty
- **AND** a successful tool result SHALL remain plain `Output` (no protocol wrapper)

#### Scenario: 协议与恢复引导并存
- **WHEN** a tool call fails
- **THEN** the failure protocol message (what happened) SHALL be followed by the existing recovery nudge (what to do next)
- **AND** the failure tracker escalation (Phase 1) SHALL remain unchanged
