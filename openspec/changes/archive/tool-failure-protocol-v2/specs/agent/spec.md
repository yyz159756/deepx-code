# agent / 工具失败恢复

## MODIFIED Requirements

### Requirement: 失败结果上下文协议化(Failure Protocol)
The agent SHALL render failed tool results in model context as a structured failure protocol, so the model can distinguish a failure event from a normal observation and know the failure state, recovery direction, and whether further recovery is possible.

#### Scenario: 协议含结构化状态字段
- **WHEN** a tool call returns `Success:false`
- **THEN** the tool message in context SHALL be rendered as `<tool_failure>` protocol containing, in fixed order: `protocol_version`, `status: failed`, `category`, `retryable`, `recovery_action`, `summary`, and `diagnostic`
- **AND** `retryable` SHALL be derived from the failure category (timeout/network → true; others → false), meaning "further recovery may be possible", NOT "allowed to immediately retry the identical call" (behavior control remains with the nudge/tracker)
- **AND** `recovery_action` SHALL be a stable enum from the canonical set (inspect_before_retry / modify_arguments / retry_with_backoff / request_permission / abort) mapped by category
- **AND** `summary` SHALL be truncated to at most 200 chars, `diagnostic` to at most 4000 chars (truncation marker on diagnostic)
- **AND** the tool hint (natural language) SHALL NOT be embedded in the protocol (it belongs to the recovery nudge)
- **AND** a successful tool result SHALL remain plain `Output` (no protocol wrapper)

#### Scenario: 协议与恢复引导并存
- **WHEN** a tool call fails
- **THEN** the failure protocol message (what happened, state) SHALL be followed by the existing recovery nudge (what to do next)
- **AND** the failure tracker escalation and nudge wording SHALL remain unchanged (behavior control and state expression are separate layers)
