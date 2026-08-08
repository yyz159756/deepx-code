# PR: feat(agent): 工具失败处理协议(恢复引导 + 失败状态结构化 + 上下文协议 + 可观测性)

> 分支:`feat/tool-failure-recovery`(已合入 dev `2413d67`)
> 规格基线:`openspec/specs/agent/spec.md`(5 个 change 已归档)
> 关联:`openspec/changes/archive/{tool-failure-recovery, tool-failure-error-field, tool-failure-protocol, failure-identity, tool-failure-protocol-v2}`

## 背景(Why)

工具调用失败(`Success:false`)目前只是普通工具输出进入对话历史,agent 缺少统一的失败处理协议:

- 失败后 **agent 层无恢复引导**(工具自身仅有零散错误文本与个别 hint)→ 模型默认动作容易变成**原样重试**(复用同一命令 / 同一 old_string),浪费轮次
- 失败结果仅作为普通 tool output 存在,**与成功 observation 混合** → 上下文表达层无法稳定区分"执行失败事件"与"正常工具 observation"
- **无针对工具执行失败(`Success:false`)的循环检测**(既有 truncated/empty 循环检测只覆盖截断与空响应)→ 模型可无限重复失败调用
- 失败原因与工具 observation 共用 `Output` → 缺少稳定字段区分"为什么失败"与"工具观察到什么"
- 失败结果以普通文本进入模型上下文 → **降低失败诊断文本被模型当作可执行模式复用的风险**(措辞:降低风险,非"解决模仿")

**实测现象**:Update `old_string` 抄错后模型反复用同一错误串重试;Bash 命令失败后原样复读。长任务浪费轮次,极端陷入失败循环。

## Implementation Stages(What)

### Phase 1:失败恢复与复读止损(`tool-failure-recovery`)

**1. ToolResult 扩展:失败携带类别与提示**
`ToolResult` 增加 `FailureCategory`(7 枚举:unknown/not_found/invalid_argument/permission_denied/execution_error/timeout/network)+ `FailureHint`(中文恢复建议)。**Tool 只报告事实,恢复策略由 agent 层统一处理,避免每个 tool 自己实现失败循环逻辑。**

**2. Agent 失败恢复引导**
工具 `Success:false` 后注入 **user-role 恢复引导**(复用 truncatedToolNudge 既有通道),引导**包含"不要原样重试"以及下一步诊断动作**。分类优先工具自带 `FailureCategory`,旧工具错误文本关键词 fallback(中英)。

**3. 失败循环检测与分级止损**
按工具类型建立失败指纹(Update: `tool+path+hash(old_string)`;Write: `tool+path`;Bash: `normalize(exec+subcommand)+category`),避免不同失败任务互相污染;连续失败分级(**阈值可配置**,如 1 标准 → 2 soft → 3 hard(禁止同参数)→ 5 终止上报 UI,仿 `errTruncatedToolLoop`)。

**4. 成功清除失败状态**
同工具+路径成功 → 清除指纹计数(状态机闭环,避免后续失败误判升级)。

### Phase 2:失败结果结构化(`tool-failure-error-field`)

**5. ToolResult 增加 Error 字段(失败摘要)**
```go
// Error is a short failure summary.
// It describes WHY execution failed.
// Diagnostic details should remain in Output.
Error string
```
语义:`Output` = 原始 observation(成功结果/失败诊断);`Error` = 为什么失败(摘要)。**Bash 特例**:`Error` 保存执行状态摘要(exit code),`Output` 保留 stdout/stderr 诊断——**避免将诊断输出全部迁移到 Error 字段,导致 agent 丢失必要上下文**。

**6. 模型上下文渲染:失败 = Error + "\n" + Output**
`RenderToolResultContent` 函数化(成功=Output;失败=原因+诊断),渲染逻辑集中管理。

**7. 兼容转换 `NormalizeToolResult`**
agent 入口(executeTool 后):旧工具 `!Success && Error==""` → `Error=Output` 且 Output 保留(仅作为兼容 fallback,新迁移工具应提供独立 Error 摘要)。

**8. 工具迁移**
Update/Bash/Write 失败路径补 `Error` 摘要(Output 保留诊断)。

### Phase 3.1:失败结果上下文协议化(`tool-failure-protocol`)

**9. Failure Protocol Renderer**
`RenderToolFailureProtocol()`:失败 tool 消息渲染为 `<tool_failure>` 协议(status/category/summary/recovery/diagnostic),通过统一协议格式表达失败事件,降低失败诊断文本被当作普通 observation 处理的风险。**表达层隔离**:不改 ToolResult/tracker/nudge/UI;summary ≤200、diagnostic ≤4000(带 truncated 标记);recovery 空值兜底(category 默认动作)。只影响新消息,不迁移旧会话;不影响 prefix cache(tool result 是动态 token)。

### Phase 3.2:失败事件唯一身份(`failure-identity`)

**10. FailureID**
每次失败事件生成 stream 局部唯一 ID(`f_001`、`f_002`…),存 `FailureTracker`(fingerprint → 最近 ID)。通过 **`ToolCallResultMsg.FailureID` 事件通道**发给 UI/debug。**FailureID 不参与模型决策、不进入 prompt,仅用于执行事件关联、UI 展示和调试定位**(可观测性增强,非核心失败处理)。成功清除同步清 IDs;每次失败新 ID(事件语义)。

### Phase 3.3:模型侧状态协议 v2(`tool-failure-protocol-v2`)

**11. 协议升级:retryable / recovery_action / protocol_version**
```
<tool_failure>
protocol_version: 1
status: failed
category: not_found
retryable: false
recovery_action: inspect_before_retry
summary: ...
diagnostic: ...
</tool_failure>
```
- `RecoveryAction` 枚举(5):inspect_before_retry / modify_arguments / retry_with_backoff / request_permission / abort
- `IsRetryable(category)`:timeout/network → true,其余 → false(**保守默认**,execution_error 是杂项桶)
  - **语义**:retryable 表示"存在后续恢复可能性",**≠ 允许原参数立即重试**(行为控制仍归 nudge/tracker);**retryable=false 不表示工具永远不可调用,仅表示当前失败状态下不建议直接重复当前调用**
- `GetRecoveryAction(category)`:纯函数映射(category → 策略)
- `FailureHint` 不进协议(双轨:recovery_action=结构化策略,FailureHint=自然语言进 nudge)
- 字段顺序固定:token 稳定、缓存友好、diff 明确
- 不做 ChatMessage.Metadata(provider 不支持 tool message metadata,模型看不到;无消费方)

## 技术方案要点(详见各 change design.md)

| 决策 | 结论 |
|---|---|
| nudge 通道 | user role(deepx 无 internal nudge 通道;system 注入破坏前缀缓存) |
| 指纹粒度 | Update: tool+path+hash(old_string);Write: tool+path;Bash: normalize(exec+subcommand)+category |
| tracker 生命周期 | StartStream 局部状态(执行态;stream 结束/压缩重置) |
| 失败分类 | 7 枚举,工具自带为主,关键词回退(中英) |
| Error/Output 边界 | Error=摘要;Output=诊断(保留)。字段注释铁律防维护者写反 |
| 协议纯渲染 | protocol 层无业务判断(无 if category);截断只在渲染副本,不改 ToolResult |
| retryable 语义 | 存在恢复可能性 ≠ 允许原参数立即重试;nudge 仍拦同参数立即重试,不冲突 |
| 双轨恢复提示 | recovery_action(结构化,协议/planner)vs FailureHint(自然语言,nudge) |
| FailureID 通道 | ToolCallResultMsg 事件(UI/debug),不进模型上下文 |
| prefix cache | 只改 role=tool content(动态区),不动 system/schema |

## 验证

### 单元测试(全绿)
- **tools**:ClassifyFailure 中英 11 例、Update/Bash/Write 失败元数据(category+hint+Error+Output 保留)
- **agent**:
  - 分级升级(1→2→3→5)、成功清除、旧工具回退、Bash 指纹 normalize
  - NormalizeToolResult 兼容(legacy Error=Output / 已迁移不覆盖)
  - RenderToolResultContent(成功=Output / 失败=协议)
  - RenderToolFailureProtocol:协议字段(v1 5 字段 / v2 7 字段固定顺序)、summary 200 截断、diagnostic 4000 截断 + truncated 标记、FailureHint 不进协议
  - IsRetryable 7 例、GetRecoveryAction 7 例(纯函数映射)
  - FailureID:每次失败新 ID、递增唯一、成功清除后新 ID

### 黑箱测试(真实 exe)
| 场景 | 结果 |
|---|---|
| Update 失败(not_found) | `<tool_failure>` 协议完整渲染 + standard 引导"不要原样重试——请先 Read"✅ |
| Bash 失败(execution) | summary=exit status / diagnostic=stderr 保留 + 执行类引导 ✅ |
| 同参数复读 ×2 | 第 1 次 standard → 第 2 次 soft ✅ |
| 指纹隔离 | 不同文件(path+old_string)独立计数 ✅ |
| 三层机制 | 协议(是什么)+ 引导(怎么办)+ 止损(分级)并存 ✅ |
| Phase 3.3 retryable 不改变 nudge | not_found retryable=false → 模型 inspect_before_retry(Read),而非立即重复 Update ✅ |
| Phase 3.3 与 Phase 1 止损不冲突 | execution_error retryable=false → 保持 nudge + tracker 分级,不增加重复调用 ✅ |

## 文件清单

```
tools/tools.go           ToolResult + FailureCategory/FailureHint/Error(注释铁律)
tools/failure.go         FailureCategory 类型 + 7 常量 + ClassifyFailure 关键词回退
tools/command.go         Bash 失败:timeout/execution + Error 摘要 + hint
tools/edit_file.go       Update 失败:not_found/invalid_argument + Error + hint
tools/write_file.go      Write 失败:invalid_argument/permission_denied + Error
agent/failure_tracker.go failureTracker(指纹/分级/成功清除)+ failure identity state(IDs)+ Normalize + Render
agent/failure_protocol.go RenderToolFailureProtocol(v1→v2,截断,纯渲染)
agent/failure_metadata.go RecoveryAction + IsRetryable + GetRecoveryAction(纯函数)
agent/llm.go             StartStream tracker;失败注入 user nudge;Normalize;convo 用 Render;abort 上报;FailureID 事件
tui/model.go             失败显示带 (f_xxx) ID
tools/failure_test.go    tools 侧测试
agent/failure_tracker_test.go  分级/清除/指纹/ID
agent/failure_render_test.go   Normalize/Render/协议/截断
agent/failure_metadata_test.go IsRetryable/GetRecoveryAction/字段顺序
openspec/                proposal/design/tasks/spec delta ×5 + 基线 specs/agent/spec.md
```

## 风险与取舍

- 失败 nudge 每失败注入(必要);同指纹去重防膨胀
- legacy Error/Output 重复是兼容取舍(注释说明);新工具给简洁摘要
- 分类回退靠关键词,个别旧工具落 unknown(工具自带为主)
- retryable 是保守默认(execution_error 一律 false);未来 Bash 细分(命令不存在/crash/依赖缺失)再调
- 协议化降低失败文本被当作执行模板的风险(非保证);彻底隔离需底层 tool API 支持
- 支持未来工具(Browser/Git/DB):实现 FailureCategory/Error/Hint 即可,协议自动覆盖

## 关联

- UI 换行修复(工具结果段超长行折行)为独立 PR,见 `openspec/changes/archive/tool-result-wrap/pr.md`
- 上游功能请求:`openspec/changes/archive/tool-failure-recovery/issue.md`
