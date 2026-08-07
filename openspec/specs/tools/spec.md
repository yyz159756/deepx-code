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
