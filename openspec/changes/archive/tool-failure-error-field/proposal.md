# proposal: 工具失败结构化状态(Error 字段)

## Why

Phase 1 已解决失败复读/循环/分类,但失败信息仍全部堆在 `Output`,与成功 observation 语义混合:

- 失败时 `Output` 同时承载"错误文本"与"原始输出",模型层无法结构化区分
- 旧工具失败分类靠关键词回退(文本解析)——有 `FailureCategory` 后本可免
- 未来工具接入时,"失败原因"与"工具观察"没有稳定字段分界

Phase 2 目标(收缩版):**为失败增加结构化状态(Error 摘要),同时保留原始工具观察能力(Output 诊断)**——不做 UI 大改、不隐藏错误文本。

## What Changes

### 1. ToolResult 增加 Error 字段
```go
type ToolResult struct {
    Success bool
    Output  string // 工具产生的原始观察(成功结果 / 失败时的诊断输出)
    Error   string // 失败摘要:为什么失败(exit status / 简短原因)
    FailureCategory FailureCategory
    FailureHint string
}
```
语义:`Output` = observation;`Error` = failure state。Bash 明确:`Error` = exit status 摘要,`Output` = stdout+stderr 原样保留(诊断不丢)。

### 2. 模型上下文渲染(convo 层)
失败时 tool 消息 content = `Error` + "\n" + `Output`(模型两个都需要:失败原因 + 诊断输出);成功 = `Output`。

### 3. 兼容转换(NormalizeToolResult)
放 **agent 入口(executeTool 后)**:旧工具 `!Success && Error=="" && Output!=""` → `Error = Output`(**不清空 Output**)。旧工具自动升级,行为不回退。

### 4. 工具迁移
Update/Bash/Write 失败返回补 `Error` 摘要(Output 保留现有内容)。

### 5. UI 保持兼容
不做 `[Tool Execution Failed]` 格式大改(历史格式稳定性优先);失败时仅可加 category 徽标(可选,低优先)。

## 不做的事

- ❌ UI/display 全面迁移
- ❌ Bash stderr 强行塞 Error(诊断输出留在 Output)
- ❌ 隐藏错误文本
