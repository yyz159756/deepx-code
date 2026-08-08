# PR: fix(tui): 工具结果段超长行自动换行

> 分支:`fix/bash-fail-display-wrap`(已合入 dev `b5758d3`)
> 规格:`openspec/changes/archive/tool-result-wrap/`(已归档,基线 `openspec/specs/tui/spec.md`)
> 独立 PR——与失败恢复协议(openspec/PR.md)无依赖,单独演进

## 背景(Why)

工具失败提示显示不自动换行:超长内容被视口截断。

实测:
```
✗ Bash 失败: warning: in the working copy of 'openspec/...'  ← 只显示到 "…tool-failu" 截断
```

根因:`tui/model.go` kindTools 段渲染只做 `colorizeDiffBlock`(染色),**不做 wordWrap**。tools 段注释说明"跳过 glamour 保留 raw \n"是避免多 tool 行的单 `\n` 被 markdown 当 soft break 拼成一行——但**跳过后没有补 wrap**,导致超长行直接溢出截断。

影响:工具失败提示、长命令工具列表、长参数行在窄终端下被截断,用户看不到完整失败原因。

## 改动(What)

### 1. kindTools 段渲染补 wordWrap
`tui/model.go` kindTools 分支,`colorizeDiffBlock` 之后加 `ansi.Wrap(inner, barInnerWidth(width, kind), "")`:
- 超长行按显示宽度折行,不再溢出截断
- 保留 `\n` 语义(按行 wrap,不合并行)——不与"跳过 markdown 防 soft break"的设计冲突
- diff 块染色行折行时颜色延续(`charmbracelet/x/ansi` 的 Wrap 对 ANSI 序列安全)

### 2. 不改变的行为
- 工具列表逐行显示、失败提示、diff 染色逻辑不变
- 行号/选区(wrap 后行号)依赖 `ansi.Wrap` 已有机制,一致

## 不做的事

- ❌ 不把 tools 段改回 markdown 渲染(引入 soft-break 拼接,注释已说明)
- ❌ 不改失败提示的拼装(model.go 的 200 字符截断已够,问题在渲染层)

## 验证

### 单元测试(3 个,全绿)
| 测试 | 验证 |
|---|---|
| `TestKindTools_WrapsLongFailureLine` | 超长失败行折成多行(窄宽),内容完整不截断 |
| `TestKindTools_PreservesLineBreaks` | 多行工具列表不合并(单 `\n` 语义保留) |
| `TestKindTools_WrapPreservesDiffColor` | diff 染色行 wrap 后 y 总数完整 + ANSI 颜色存在 |

### 回归
- tui 全量 `1.14s`(quote_bar / up_wrap / 选区等既有测试无回归)

## 文件清单

```
tui/model.go               kindTools 渲染加 ansi.Wrap(一行)
tui/tool_result_wrap_test.go  3 个测试(新)
openspec/                  proposal/spec delta/tasks + 基线 specs/tui/spec.md
```

## 效果(重启后)

窄终端下 `✗ Bash 失败: warning: ...` 超长行自动折行显示完整,不再 `tool-failu…` 截断;长命令列表、diff 同样折行(颜色延续)。

## 风险与取舍

- ansi.Wrap 对既有 ANSI 染色内容安全(库级保证),折行处颜色延续
- 工具列表长行折行是行为改进(原先截断更差)
- 与失败恢复协议(openspec/PR.md)无共享代码,可独立合入/回滚
