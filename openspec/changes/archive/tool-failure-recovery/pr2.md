# PR: feat(agent): 工具失败恢复协议(恢复引导 + 复读止损 + 结构化失败状态)

> 关联规格:`openspec/specs/agent/spec.md`(基线已归档,含两阶段)
> 分支:`feat/tool-failure-recovery`(已含远端 main ae8f16d)
> 阶段:Phase 1(`754e1df`)+ Phase 2(`469b9f2`)

## 背景(Why)

工具调用失败(`Success:false`)目前只是普通工具输出进入对话历史，agent 缺少统一的失败恢复协议:

- 失败后 **agent 层无恢复引导**(工具自身仅有零散错误文本与个别 hint,如 Update 的"请先 Read 确认",但不构成统一恢复协议)→ 模型默认动作容易变成**原样重试**(复用同一命令 / 同一 old_string),浪费轮次
- 失败结果仅作为普通 tool output 存在 → 模型缺少显式的失败状态语义，容易将失败后的动作继续视为有效执行路径
- **无针对工具执行失败(`Success:false`)的循环检测**(既有 truncated/empty 循环检测只覆盖截断与空响应,不覆盖失败复读)→ 模型可无限重复失败调用
- 失败原因与工具 observation 共用 `Output` → 缺少稳定字段区分"为什么失败"与"工具观察到什么"

本 PR 建立统一工具失败协议:

- Phase 1:失败识别 → 恢复引导 → 复读止损
- Phase 2:失败状态结构化(Error) → 保留诊断观察(Output)

目标不是隐藏失败信息，而是在保留 agent 所需诊断上下文的同时，让失败状态可被 agent 正确识别和处理。

---

## 改动内容(What)

### Phase 1:失败恢复与复读止损(`754e1df`)

**1. ToolResult 扩展:失败携带类别与提示**

`ToolResult` 增加 `FailureCategory`(7 枚举)+ `FailureHint`(恢复建议)。

Tool 只报告事实:
- 失败类型
- 恢复提示

恢复策略由 Agent 层统一控制。

---

**2. Agent 失败恢复引导**

工具 `Success:false` 时注入 **user-role 恢复引导**(复用已有 nudge 通道):

```

该调用失败，不要原样重试——先 <按类别执行诊断动作>

```

分类优先使用工具提供的 `FailureCategory`，旧工具使用错误文本关键词 fallback(中英)。

---

**3. 失败循环检测与分级止损**

基于失败指纹检测重复失败:

- Update:
  `tool + path + hash(normalized_old_string)`

- Write:
  `tool + path`

- Bash:
  `normalize(executable + subcommand) + category`

连续失败分级:

```

1: standard recovery nudge
2: soft warning
3: hard intervention(禁止同参数继续重试)
5: abort / 上报 UI

```

---

**4. 成功清除失败状态**

同指纹成功执行后清除 tracker 状态，避免历史失败影响后续正常调用。

---

### Phase 2:失败结果结构化状态(`469b9f2`)

**5. ToolResult 增加 Error 字段(失败摘要)**

```go
// Error is a short failure summary.
// It describes WHY execution failed.
// Diagnostic details should remain in Output.
Error string
```

字段语义:

| 字段     | 职责                                 |
| ------ | ---------------------------------- |
| Output | 工具产生的原始 observation(成功结果 / 失败诊断输出) |
| Error  | 失败摘要(为什么失败)                        |

示例:

Bash:

```
Error:
command failed: exit status 1

Output:
stdout + stderr 原始诊断
```

禁止反模式:

```
Error = stderr
Output = ""
```

避免为了结构化丢失 agent 最需要的诊断信息。

---

**6. 模型上下文渲染:失败 = Error + "\n" + Output**

新增:

```go
RenderToolResultContent()
```

统一处理 tool message:

* 成功:

  ```
  Output
  ```

* 失败:

  ```
  Error
  Output
  ```

原因和诊断信息同时提供给模型。

渲染逻辑集中管理，未来扩展失败格式只需修改一处。

---

**7. 兼容转换 `NormalizeToolResult`**

agent 入口(executeTool 后)增加兼容层:

旧工具:

```
Success=false
Error=""
Output=<失败信息>
```

自动转换:

```
Error=Output
Output 保留
```

说明:

* legacy 工具可能出现 Error/Output 重复
* 这是兼容历史工具的取舍
* 新迁移工具应提供简洁 Error 摘要

---

**8. 工具迁移**

Update/Bash/Write 失败路径补充 Error 摘要:

* Update:

  * 未找到
  * 参数错误
  * 匹配失败
  * 路径不在 workspace / 文件读取失败

* Bash:

  * exit status
  * timeout

* Write:

  * 参数错误
  * 写入失败

所有工具继续保留 Output 诊断信息。

---

## 技术方案要点(详见两个 change 的 design.md)

| 决策              | 结论                                                                                            |
| --------------- | --------------------------------------------------------------------------------------------- |
| nudge 通道        | **user role**(deepx 无 internal nudge 通道;system 注入破坏前缀缓存)                                      |
| 指纹粒度            | Update: tool+path+hash(old_string);Write: tool+path;Bash: normalize(exec+subcommand)+category |
| tracker 生命周期    | StartStream 局部状态(执行态;stream 结束/压缩重置)                                                          |
| 失败分类            | 7 枚举;工具自带为主;关键词 fallback(中英)                                                                  |
| Error/Output 边界 | Error=失败摘要;Output=原始观察/诊断                                                                     |
| Bash 语义         | Error=exit status 摘要;Output=stdout+stderr                                                     |
| 渲染函数化           | `RenderToolResultContent`                                                                     |
| 兼容策略            | `NormalizeToolResult` agent 入口;legacy Error=Output 且 Output 保留                                |

---

## 验证

### 单元测试(全绿)

* **tools**

  * `ClassifyFailure` 中英关键词 11 例
  * Update/Bash/Write 失败元数据:

    * category
    * hint
    * Error
    * Output 保留

* **agent**

  * 分级升级(1→2→3→5)
  * 成功清除 tracker
  * 旧工具 fallback
  * Bash 指纹 normalize
  * `NormalizeToolResult`

    * legacy Error=Output
    * 已迁移工具不覆盖 Error
  * `RenderToolResultContent`

    * success
    * Error only
    * Error + Output
    * Output fallback
  * Error/Output 不丢失回归测试

---

## 黑箱测试(真实 exe `04f8a98`,含 Phase 1+2)

| 场景                   | 结果                                          |
| -------------------- | ------------------------------------------- |
| Update 失败(not_found) | standard nudge:"不要原样重试——请先 Read 文件确认实际内容" ✅ |
| Bash 失败(execution)   | nudge:"检查命令与输出中的错误信息"(工具 hint 优先) ✅         |
| 同指纹复读 ×2             | 第 1 次 standard → 第 2 次 soft("连续失败 2 次") ✅   |
| 指纹隔离                 | 不同文件(path+old_string)各自独立计数 ✅               |

---

## 文件清单

```
tools/tools.go
    ToolResult + FailureCategory/FailureHint/Error

tools/failure.go
    类型 + 7 常量 + ClassifyFailure fallback

tools/command.go
    Bash 失败 Error 摘要 + hint

tools/edit_file.go
    Update 失败 Error + hint

tools/write_file.go
    Write 失败 Error

agent/failure_tracker.go
    failureTracker
    NormalizeToolResult
    RenderToolResultContent

agent/llm.go
    StartStream tracker
    failure nudge
    Normalize
    convo Render
    abort 上报

tools/failure_test.go

agent/failure_tracker_test.go

agent/failure_render_test.go

openspec/
    proposal/design/tasks/spec delta ×2
    基线 specs/agent/spec.md
```

---

## 风险与取舍

* 失败 nudge 每失败注入(必要);tracker 防止重复提示膨胀
* legacy Error/Output 重复是兼容历史工具的取舍
* 新工具应提供简洁 Error 摘要，Output 保留诊断
* 分类 fallback 依赖关键词，旧工具可能落 unknown(工具自带优先)
* 不隐藏失败文本，避免丢失 agent 推理所需诊断信息
* 支持未来工具(Browser/Git/DB):实现 FailureCategory + Error + Hint 即可
