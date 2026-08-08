# Issue: 增加工具失败恢复机制,避免 agent 执行失败复读循环

> 面向 deepx 上游(itmisx/deepx-code)的功能请求
> 已有参考实现(fork 分支 `feat/tool-failure-recovery`,commit `754e1df` + `469b9f2`);完整设计文档 `pr2.md` 可附在 issue 评论/附件中,正文已自包含

## 背景(Why)

工具调用失败(`Success:false`)目前只是普通工具输出进入对话历史,agent 缺少统一的失败恢复协议:

- 失败后 **agent 层无恢复引导**(工具自身仅有零散错误文本与个别 hint,如 Update 的"请先 Read 确认",但不构成统一恢复协议)→ 模型默认动作容易变成**原样重试**(复用同一命令 / 同一 old_string),浪费轮次
- 失败结果仅作为普通 tool output 存在,**与成功 observation 混合** → 模型缺少显式的失败状态语义,容易将失败后的动作继续视为有效执行路径
- **无针对工具执行失败(`Success:false`)的循环检测**(既有 truncated/empty 循环检测只覆盖截断与空响应,不覆盖失败复读)→ 模型可无限重复失败调用
- 失败原因与工具 observation 共用 `Output` → 缺少稳定字段区分"为什么失败"与"工具观察到什么"

**实测现象**:Update `old_string` 抄错后,模型反复用同一错误串重试;Bash 命令失败后原样复读。长任务中浪费轮次,极端情况陷入失败循环。

## 建议方案(What)

以下为一个可行实现方向,参考实现已验证可运行(详见文末):

**方向一:失败恢复与复读止损**

1. `ToolResult` 增加 `FailureCategory`(如 not_found、execution_error 等)+ `FailureHint`(中文恢复建议);Tool 只报告事实。**恢复策略由 agent 层统一处理,避免每个 tool 自己实现失败循环逻辑。**
2. Agent 在 `Success:false` 后增加恢复引导机制(可复用现有 nudge 通道,如 truncatedToolNudge),**引导内容应包含"不要原样重试"以及下一步诊断动作**;分类优先工具提供的类别,旧工具用错误文本关键词 fallback(中英)
3. 可按工具类型建立失败指纹(例如 Update/Write/Bash 分别基于目标资源、参数和错误类别生成),避免不同失败任务的失败状态互相污染;连续失败分级(**阈值可配置**,如 1 标准引导 → 2 soft → 3 hard(禁止同参数)→ 5 终止上报 UI,仿 `errTruncatedToolLoop`)。参考实现使用:Update `tool+path+hash(old_string)` / Write `tool+path` / Bash `normalize(exec+subcommand)+category`——具体指纹设计可按上游偏好调整
4. 同指纹成功执行后清除 tracker 状态

**方向二:失败状态结构化**

5. `ToolResult` 增加 `Error` 字段(**失败摘要**,如 `command failed: exit status 1`);`Output` 保留诊断 observation——**避免将诊断输出全部迁移到 Error 字段,导致 agent 丢失必要上下文**。对于 Bash 等执行型工具:`Error` 保存执行状态摘要(exit code),`Output` 保留 stdout/stderr 等诊断信息
6. 模型上下文渲染:失败 = `Error + "\n" + Output`(原因 + 诊断,函数化 `RenderToolResultContent`)
7. 兼容转换 `NormalizeToolResult`(agent 入口):旧工具 `Error=Output` 且 Output 保留(仅作为兼容 fallback,新迁移工具应提供独立 Error 摘要)
8. 工具迁移:Update/Bash/Write 失败路径补 `Error` 摘要

**方向三:失败结果与模型可执行上下文隔离**
方向二解决字段语义分离(Error/Output 内部结构),方向三进一步解决**这些字段进入模型上下文后的表达方式**(渲染给模型看到的格式)。

9. 区分工具失败状态信息与诊断 observation,降低失败诊断文本被模型当作可执行模式复用的风险。当前失败结果直接以文本形式进入上下文:

```
role=tool
old_string not found
```

模型无法区分:这是工具执行失败的诊断信息,还是工具返回的正常 observation。可增加结构化失败标记或统一包装格式:

```
<tool_failure>
category: not_found
summary: old_string not found

diagnostic:
...
</tool_failure>
```

帮助模型区分:该结果表示上一次工具调用未成功;诊断文本用于分析失败原因,而不是成功执行结果。

10. 根据实际上下文窗口和模型行为,可进一步考虑将失败诊断信息与核心恢复信号分离:
- 恢复决策依赖:`FailureCategory` + `FailureHint`
- 调试分析依赖:`Output` 中的详细诊断

避免长诊断输出占据主要上下文,影响 agent 后续规划。

### 三个方向的关系

三个方向分别解决不同层面的问题:

- **方向一:控制 agent 行为**——失败后如何恢复、防止重复失败调用
- **方向二:规范工具结果协议**——明确失败原因与诊断输出的职责、提供稳定数据结构
- **方向三:优化模型上下文表达**——降低失败诊断文本被误认为正常 observation 或执行模式的风险,为未来 tool protocol 演进提供基础

```
失败调用
    |
    +-- Agent不知道下一步怎么办
    |       |
    |       +-- Phase 1
    |           FailureHint + nudge + tracker
    |
    +-- Tool结果没有结构
    |       |
    |       +-- Phase 2
    |           Error / Output 分离
    |
    +-- Tool消息格式污染模型上下文
            |
            +-- Phase 3(长期)
                Tool protocol redesign
                (可能影响 prefix cache)
```



## 期望行为(验收标准)

- 工具失败后,对话中出现恢复引导(user 消息,"不要原样重试 + 按类别动作")
- **失败恢复引导后,agent 下一步行为从原参数重试转向诊断操作(Read/检查命令等)**——证明行为改善,而非仅"消息出现"
- 同指纹连续失败:第 2 次 soft、第 3 次 hard、第 5 次终止(不无限复读)
- 失败 tool 消息渲染含 `Error` 摘要与诊断输出,两者都不丢
- 同指纹成功后计数清除(不误判后续失败等级)
- 旧工具(未迁移)失败行为不回退(兼容转换)

## 参考实现

fork 分支 `yyz159756:feat/tool-failure-recovery`:
- `754e1df` — Phase 1(失败恢复与复读止损)
- `469b9f2` — Phase 2(失败结果结构化)

关键文件:`tools/failure.go`、`agent/failure_tracker.go`、`tools/{command,edit_file,write_file}.go`、`agent/llm.go`;行为规格见 `openspec/specs/agent/spec.md`。

若上游采纳,可参考该实现验证设计可行性(cherry-pick 或按设计重写均可);若不采纳该方案,也建议至少补"失败后禁止原样重试"的引导(改动最小的一步)。
