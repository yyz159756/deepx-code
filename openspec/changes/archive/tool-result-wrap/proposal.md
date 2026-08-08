# proposal: 工具结果段超长行自动换行

## Why

工具失败提示显示不自动换行:超长内容被视口截断(如 `✗ Bash 失败: warning: in the working copy of 'openspec/...` 只显示到 `tool-failu…`)。

根因:`tui/model.go:4362` kindTools 段渲染只做 `colorizeDiffBlock`(染色),**不做 wordWrap**——tools 段注释(4350)说明"跳过 glamour 保留 raw \n"是避免多 tool 行的单 `\n` 被 markdown 当 soft break 拼成一行,但**跳过后没有补 wrap**,导致超长行直接溢出截断。

影响:工具失败提示、长命令工具列表、长参数行都可能在窄终端下被截断,用户看不到完整失败原因。

## What Changes

### 1. kindTools 段渲染补 wordWrap
`tui/model.go:4362` 的 kindTools 分支,在 `colorizeDiffBlock` 之后加 `ansi.Wrap(inner, barInnerWidth(width, kind))`:
- 超长行按显示宽度折行,不再溢出截断
- 保留 `\n` 语义(按行 wrap,不合并行)——不与"跳过 markdown 防 soft break"的设计冲突
- diff 块染色行折行时颜色延续(charmbracelet/x/ansi 的 Wrap 对 ANSI 序列安全)

### 2. 不改变的行为
- 工具列表逐行显示、失败提示、diff 染色逻辑不变
- 行号/选区(wrap 后行号)依赖 `ansi.Wrap` 已有机制,一致

## 不做的事

- ❌ 不把 tools 段改回 markdown 渲染(那会引入 soft-break 拼接,注释 4350 已说明)
- ❌ 不改失败提示的拼装(model.go:2705 的截断 200 已够,问题在渲染层)
