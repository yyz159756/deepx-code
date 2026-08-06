# agent / 工具失败恢复

## Requirements

### Requirement: 工具失败携带类别、摘要与提示
The agent tool system SHALL provide a machine-readable failure category, a failure summary, and an optional human-readable recovery hint whenever a tool call fails, so the agent can classify and recover without parsing error prose.

#### Scenario: 工具失败返回类别、摘要与原始观察
- **WHEN** a migrated tool call fails with a known failure type (e.g. target text not found in `Update`)
- **THEN** the tool result SHALL include a `FailureCategory` value from the canonical enum (not_found / invalid_argument / permission_denied / execution_error / timeout / network / unknown)
- **AND** the tool result SHALL include an `Error` field holding the failure summary (why it failed, e.g. "command failed: exit status 1" / "未找到目标")
- **AND** the tool result SHALL keep the raw diagnostic observation in `Output` (e.g. stderr / compiler errors), not discard it for the sake of structure
- **AND** the tool result MAY include a `FailureHint` in Chinese suggesting the concrete next step

#### Scenario: 旧工具失败分类与兼容回退
- **WHEN** a legacy tool result (not yet migrated) carries no `FailureCategory` or no `Error`
- **THEN** the agent SHALL classify the failure by keyword matching against the output text (covering both Chinese and English error phrases)
- **AND** the agent SHALL normalize the result by copying `Output` into `Error` when `Error` is empty, **without clearing `Output`** (known trade-off: legacy fallback may duplicate `Error` and `Output`; migrated tools provide concise `Error`)
- **AND** classification failure SHALL fall back to `unknown`

### Requirement: 工具失败后的恢复引导
The agent SHALL inject a recovery guidance message (as a `user`-role message, following the existing nudge mechanism) whenever a tool call fails, instructing the model not to retry the identical call and to diagnose first.

#### Scenario: 失败后注入恢复引导
- **WHEN** a tool call returns `Success:false`
- **THEN** the agent SHALL append a `user`-role message containing: failure category, "do not retry identically", and a category-appropriate next action (Read to verify / inspect command output / check URL / etc.)
- **AND** the agent SHALL render the failed tool message in context as `Error` plus the diagnostic `Output` (reason and evidence), never as an empty observation
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
