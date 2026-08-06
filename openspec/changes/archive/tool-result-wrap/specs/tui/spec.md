# tui / 工具结果段显示

## ADDED Requirements

### Requirement: 工具结果段超长行自动换行
The TUI SHALL wrap lines that exceed the chat viewport width inside the tools segment (kindTools), so long tool output and failure messages wrap instead of being truncated.

#### Scenario: 工具失败提示超长行折行
- **WHEN** a tool fails and the failure line (e.g. `✗ Bash 失败: warning: ...`) exceeds the chat area width
- **THEN** the TUI SHALL wrap the line at the available width instead of truncating it at the viewport edge
- **AND** the wrapped continuation SHALL stay within the tools segment quote bar

#### Scenario: 工具列表长行折行且不合并行
- **WHEN** a tools segment contains multiple lines (tool call list / diff block / failure text) and some lines exceed the width
- **THEN** the TUI SHALL wrap only the overlong lines at display width
- **AND** SHALL NOT merge distinct lines (single `\n` semantics preserved — the reason tools segment skips markdown rendering)

#### Scenario: diff 块染色行折行颜色延续
- **WHEN** a wrapped line is inside a colorized diff block (`+` / `-` lines)
- **THEN** the wrap SHALL preserve the ANSI color across the continuation
