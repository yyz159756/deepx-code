# tools / git

## Requirements

### Requirement: Git 工具
The toolset SHALL provide a `Git` tool that executes git commands by directly invoking the git executable (bypassing the shell), so git operations are not affected by PowerShell/cmd stderr exit-code pollution and arguments are passed as an array without quoting issues.

#### Scenario: 直接执行 git,不经过 shell
- **WHEN** the agent calls `Git` with `args` (git parameter array, e.g. `["status","--short"]`) and optional `cwd`
- **THEN** the tool SHALL execute git directly (no cmd/powershell wrapper)
- **AND** `cwd` SHALL default to the workspace root when omitted

#### Scenario: git exit code 语义
- **WHEN** git exits 0 → the result SHALL be successful, Output = stdout
- **WHEN** git exits 1 (normal result, e.g. diff has changes / no match) → the result SHALL be successful, with `[exit] 1` noted in Output
- **WHEN** git exits ≥ 2 (real error) → the result SHALL fail, with `Error` = summary (first stderr line), `Output` = stdout + stderr (diagnostics preserved), `FailureCategory` = execution_error

#### Scenario: 失败结果结构化
- **WHEN** the Git tool fails (exit ≥ 2)
- **THEN** the result SHALL carry `Error` (short summary) + `Output` (full diagnostics) + `FailureCategory` + `FailureHint`, so the failure protocol renders it uniformly

### Requirement: 工具失败结果契约
Tool failure results SHALL expose a standard, machine-readable shape — FailureCategory, Error summary, and raw Output diagnostic — so the failure protocol renders uniformly regardless of which tool failed. This is a tool-level result contract, NOT a requirement that every tool implement the full agent failure protocol (mechanism/strategy separation: tools report facts, the agent decides recovery).

#### Scenario: 失败工具返回结构化字段
- **WHEN** a tool call fails
- **THEN** the tool result SHALL include a `FailureCategory` from the canonical enum (not_found / invalid_argument / permission_denied / execution_error / timeout / network / unknown)
- **AND** the tool result SHALL include an `Error` field holding a short failure summary (why it failed)
- **AND** the tool result SHALL keep the raw diagnostic observation in `Output`, not discard it for the sake of structure
- **AND** the tool result MAY include a `FailureHint` in Chinese suggesting the concrete next step

#### Scenario: 超时使用 timeout 类别
- **WHEN** a tool execution exceeds its configured timeout
- **THEN** the result SHALL fail with `FailureCategory` = timeout (MUST use the timeout category, not a generic execution_error)
- **AND** the result SHALL include an `Error` summarizing the timeout
- **AND** the result SHALL preserve any partial output in `Output`
