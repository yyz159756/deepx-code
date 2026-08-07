# tasks: Git 工具

## 1. 实现
- [ ] 1.1 `tools/git.go`:`Git(args map[string]any) ToolResult`(exec 直调,args 数组,cwd,timeout,exit code 语义,16KB 截断)
- [ ] 1.2 tools 注册:`tools.Tools` 加 Git 定义(参数 schema + 描述)
- [ ] 1.3 system prompt:agent 提示"git 操作优先用 Git 工具(避免 PowerShell 误报)"

## 2. UI
- [ ] 2.1 `tui/tool_display.go` extractMainArg 加 Git case:`args` join 显示(如 "Git (status --short)")

## 3. 测试
- [ ] 3.1 Git 工具测试:status / diff(exit 1 视为成功)/ 非 git 目录(exit 128 → 失败 + 诊断)
- [ ] 3.2 UI 测试:Git 调用行显示 args 摘要
- [ ] 3.3 回归:`go build ./...` + tools/tui 全量

## 4. 收尾
- [ ] 4.1 归档:delta 应用基线,change 移 archive
