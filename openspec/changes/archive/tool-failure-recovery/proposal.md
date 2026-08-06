# proposal: 工具失败恢复与复读止损

## Why

工具调用失败(`Success:false`)目前只是**普通工具输出**进入对话历史,没有任何恢复机制:

- 失败后无引导 → 模型默认动作是**原样重试**(复读同一命令 / 同一 old_string),浪费轮次
- 失败提示是纯文本,与成功输出形态无区分 → 模型容易**模仿失败形态**(write_file.go 已有此顾虑)
- **无失败循环检测** → 模型可以无限复读(对比:已有 completionGate/truncatedToolNudge 防死循环,但没有失败指纹检测)

现状证据:Update `old_string` 抄错后模型反复用同一错误串重试;Bash 命令失败后原样复读。

## What Changes

### 1. ToolResult 扩展:失败携带类别与提示
`ToolResult` 增加可选字段 `FailureCategory`(统一错误语义)与 `FailureHint`(该工具的中文恢复建议)。
Tool **只报告事实**(发生了什么 + 可选建议),不负责恢复。

### 2. Agent 失败恢复引导(Phase 1)
工具 `Success:false` 时,agent 注入一条 **user role 的恢复引导**(复用 truncatedToolNudge 的既有通道):
"该调用失败,不要原样重试——先 <按类别引导的动作>,确认后再调用"。
分类优先用工具自带的 `FailureCategory`,旧工具按错误文本关键词 fallback。

### 3. 失败循环检测与分级止损(Phase 2)
按**失败指纹**统计连续失败次数,分级干预:
- 第 1 次:正常恢复引导
- 第 2 次(同指纹):soft nudge("检查假设后再重试")
- 第 3 次(同指纹):hard nudge("禁止同参数重试,必须改变方法或说明卡点")
- 第 5 次(同指纹):终止循环,上报 UI 错误(仿 errTruncatedToolLoop)

### 4. 成功清除失败状态(Phase 2)
同指纹工具调用**成功**时,清除该指纹的计数,避免后续误判。

## 设计决策(详见 design.md)

- nudge 通道用 **user role**(deepx 无独立 internal nudge 通道;system 注入会破坏前缀缓存)
- Bash 指纹用 **可执行名 + 子命令 + 类别**(非全串 hash,防小改动绕过)
- FailureTracker 放 **StartStream 局部状态**(执行态,非对话知识;压缩/重启重置)
