# PR: feat(agent): 工具失败恢复协议(恢复引导 + 复读止损 + 结构化失败状态)

> 关联规格:`openspec/specs/agent/spec.md`(基线已归档,含两阶段)
> 分支:`feat/tool-failure-recovery`(已含远端 main ae8f16d)
> 阶段:Phase 1(`754e1df`)+ Phase 2(`469b9f2`)

## 背景(Why)

工具调用失败(`Success:false`)目前只是**普通工具输出**进入对话历史,没有任何恢复机制:

- 失败后无引导 → 模型默认动作是**原样重试**(复读同一命令 / 同一 old_string),浪费轮次
- 失败提示是纯文本,与成功输出形态无区分 → 模型容易**模仿失败形态**
- **无失败循环检测** → 模型可以无限复读
- 失败信息与成功 observation 共用 `Output`,语义混合,模型层无法结构化区分"失败原因"与"工具观察"

## 改动内容(What)

### Phase 1:失败恢复与复读止损(`754e1df`)

**1. ToolResult 扩展:失败携带类别与提示**
`ToolResult` 增加 `FailureCategory`(7 枚举)+ `FailureHint`(中文恢复建议)。Tool 只报告事实,不负责恢复。

**2. Agent 失败恢复引导**
工具 `Success:false` 时注入 **user-role 恢复引导**(复用 truncatedToolNudge 既有通道):"该调用失败,不要原样重试——先 <按类别引导的动作>"。分类优先工具自带,旧工具关键词回退(中英)。

**3. 失败循环检测与分级止损**
失败指纹(Update: `tool+path+hash(old_string)`;Write: `tool+path`;Bash: `normalize(exec+subcommand)+category`)连续失败分级:1 标准 → 2 soft → 3 hard(禁止同参数)→ 5 终止上报 UI。

**4. 成功清除失败状态**
同工具+路径成功 → 清除指纹计数(状态机闭环)。

### Phase 2:失败结果结构化状态(`469b9f2`)

**5. ToolResult 增加 Error 字段(失败摘要)**
```go
// Error is a short failure summary.
// It describes WHY execution failed.
// Diagnostic details should remain in Output.
Error string
```
语义:`Output` = 原始观察(成功结果 / 失败诊断);`Error` = 为什么失败(摘要)。**Bash:Error = `command failed: exit status N`,Output = stdout+stderr 原样**(不犯"为结构化丢诊断"反模式)。

**6. 模型上下文渲染:失败 = Error + "\n" + Output**
`RenderToolResultContent` 函数化(成功=Output;失败=原因+诊断,两者都要),不散落 llm.go。

**7. 兼容转换 `NormalizeToolResult`**
agent 入口(executeTool 后):旧工具 `!Success && Error==""` → `Error=Output`(**不清空 Output**;已知取舍:legacy 可能 Error/Output 重复,新迁移工具给简洁摘要)。

**8. 工具迁移**
Update/Bash/Write 失败返回补 `Error` 摘要(Output 保留)。

## 技术方案要点(详见两 change 的 design.md)

| 决策 | 结论 |
|---|---|
| nudge 通道 | **user role**(deepx 无 internal nudge 通道;system 注入破坏前缀缓存) |
| 指纹粒度 | Update: tool+path+hash(old_string);Write: tool+path;Bash: normalize(exec+subcommand)+category |
| tracker 生命周期 | StartStream 局部状态(执行态;stream 结束/压缩重置) |
| 失败分类 | 7 枚举,工具自带为主,关键词回退(中英) |
| Error/Output 边界 | Error=摘要;Output=诊断(保留)。字段注释铁律防维护者写反 |
| 渲染函数化 | `RenderToolResultContent`(Phase 3 扩展格式只改一处) |
| 兼容 | `NormalizeToolResult` agent 入口,Error=Output 不清空 |

## 验证

### 单元测试(全绿)
- **tools**:`ClassifyFailure` 中英 11 例、Update/Bash/Write 失败元数据(category+hint+Error+Output 保留)
- **agent**:分级升级(1→2→3→5)、成功清除、旧工具回退、Bash 指纹 normalize、`NormalizeToolResult` 兼容(legacy Error=Output / 已迁移不覆盖)、`RenderToolResultContent` 四分支、**Error/Output 不丢失回归**

### 黑箱测试(真实 exe `40113cc`)
| 场景 | 结果 |
|---|---|
| Update 失败(not_found) | standard nudge"不要原样重试——请先 Read 文件确认实际内容"✅ |
| Bash 失败(execution) | nudge"检查命令与输出"(工具 hint 优先)✅ |
| 同指纹复读 ×2 | 第 1 次 standard → 第 2 次 soft("连续失败 2 次")✅ |
| 指纹隔离 | 不同文件(path+old_string)各自独立计数 ✅ |

## 文件清单

```
tools/tools.go           ToolResult + FailureCategory/FailureHint/Error(注释铁律)
tools/failure.go         类型 + 7 常量 + ClassifyFailure 关键词回退
tools/command.go         Bash 失败:timeout/execution + Error 摘要 + hint
tools/edit_file.go       Update 失败:not_found/invalid_argument + Error + hint
tools/write_file.go      Write 失败:invalid_argument/permission_denied + Error
agent/failure_tracker.go failureTracker(指纹/分级/去重/成功清除)+ NormalizeToolResult + RenderToolResultContent
agent/llm.go             StartStream tracker;失败注入 user nudge;Normalize;convo 用 Render;abort 上报
tools/failure_test.go    tools 侧测试
agent/failure_tracker_test.go  agent 侧测试(Phase 1)
agent/failure_render_test.go   agent 侧测试(Phase 2)
openspec/                proposal/design/tasks/spec delta ×2 + 基线 specs/agent/spec.md
```

## 风险与取舍

- 失败 nudge 每失败注入(必要);同指纹去重防膨胀
- legacy Error/Output 重复是兼容取舍(注释说明);新工具给简洁摘要
- 分类回退靠关键词,个别旧工具落 unknown(工具自带为主)
- 支持未来新工具(Browser/Git/DB):填 FailureCategory/Error/Hint 即可
