# agent / 工具失败恢复

## Requirements

### Requirement: 工具失败携带类别与提示
The agent tool system SHALL provide a machine-readable failure category and an optional human-readable recovery hint whenever a tool call fails, so the agent can classify and recover without parsing error prose.

#### Scenario: 工具失败返回类别与提示
- **WHEN** a tool call fails with a known failure type (e.g. target text not found in `Update`)
- **THEN** the tool result SHALL include a `FailureCategory` value from the canonical enum (not_found / invalid_argument / permission_denied / execution_error / timeout / network / unknown)
- **AND** the tool result MAY include a `FailureHint` in Chinese suggesting the concrete next step (e.g. "请先 Read 文件确认实际内容,不要凭记忆构造 old_string")

#### Scenario: 旧工具失败分类回退
- **WHEN** a tool result carries no `FailureCategory`
- **THEN** the agent SHALL classify the failure by keyword matching against the output text (covering both Chinese and English error phrases)
- **AND** classification failure SHALL fall back to `unknown`

### Requirement: 工具失败后的恢复引导
The agent SHALL inject a recovery guidance message (as a `user`-role message, following the existing nudge mechanism) whenever a tool call fails, instructing the model not to retry the identical call and to diagnose first.

#### Scenario: 失败后注入恢复引导
- **WHEN** a tool call returns `Success:false`
- **THEN** the agent SHALL append a `user`-role message containing: failure category, "do not retry identically", and a category-appropriate next action (Read to verify / inspect command output / check URL / etc.)
- **AND** the message SHALL NOT be injected when the call succeeds

#### Scenario: 引导不重复
- **WHEN** the same failure fingerprint fails repeatedly at the same escalation level
- **THEN** the agent SHALL NOT append a duplicate identical nudge for that level

### Requirement: 失败复读止损
The agent SHALL track repeated failures by fingerprint and escalate, terminating the loop at a bounded threshold.

#### Scenario: 分级升级
- **WHEN** the same failure fingerprint fails the 1st time
- **THEN** the agent SHALL inject the standard recovery guidance
- **AND** on the 2nd consecutive failure of the same fingerprint the agent SHALL inject a soft nudge (check the assumption before retrying)
- **AND** on the 3rd consecutive failure the agent SHALL inject a hard nudge (do not call the same tool with identical parameters; inspect state, change approach, or explain the blocker)
- **AND** on the 5th consecutive failure the agent SHALL stop the loop and surface a readable error to the UI (mirroring the existing truncated-tool-loop error)

#### Scenario: 成功后清除失败状态(状态机闭环)
- **WHEN** a tool call with a tracked failure fingerprint `F` eventually succeeds
- **THEN** the agent SHALL clear the failure count for fingerprint `F` from the tracker
- **AND** a later failure of the same fingerprint SHALL be counted from the first level again, not misjudged as a higher escalation level
