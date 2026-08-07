# tasks: 工具结果段超长行自动换行

## 1. 渲染层
- [ ] 1.1 `tui/model.go` kindTools 分支(4362):`colorizeDiffBlock(...)` 后加 `ansi.Wrap(inner, barInnerWidth(width, kind))`
- [ ] 1.2 确认 import `ansi` 已存在(model.go 已有 `github.com/charmbracelet/x/ansi`)

## 2. 测试
- [ ] 2.1 单测:kindTools 段超长行被 wrap(窄宽下折行,不截断)
- [ ] 2.2 单测:多行工具列表不合并(单 `\n` 语义保留)
- [ ] 2.3 单测:diff 块染色行 wrap 后颜色延续(ANSI 序列安全)
- [ ] 2.4 回归:现有 chat_log / tool_display 测试全绿

## 3. 收尾
- [ ] 3.1 `go build ./...` + tui 全量测试
- [ ] 3.2 归档:delta 应用进基线,change 移 archive
