# PR: feat(agent): 工具失败恢复与复读止损

> 关联规格:`openspec/specs/agent/spec.md`(基线已归档)
> 分支:`feat/tool-failure-recovery`(已合并进 dev)

## 背景(Why)

工具调用失败(`Success:false`)目前只是**普通工具输出**进入对话历史,没有任何恢复机制:

- 失败后无引导 → 模型默认动作是**原样重试**(复读同一命令 / 同一 old_string),浪费轮次
- 失败提示是纯文本,与成功输出形态无区分 → 模型容易**模仿失败形态**
- **无失败循环检测** → 模型可以无限复读(对比:已有 completionGate/truncatedToolNudge 防死循环,但没有失败指纹检测)

## 改动内容(What)

### 1. ToolResult 扩展:失败携带类别与提示
`ToolResult` 增加可选字段 `FailureCategory`(统一错误语义)与 `FailureHint`(工具的中文恢复建议)。Tool 只报告事实,不负责恢复。

### 2. Agent 失败恢复引导
工具 `Success:false` 时,agent 注入一条 **user-role 恢复引导**(复用 truncatedToolNudge 既有通道):"该调用失败,不要原样重试——先 <按类别引导的动作>"。分类优先工具自带 `FailureCategory`,旧工具按错误文本关键词回退。

### 3. 失败循环检测与分级止损
按**失败指纹**统计连续失败,分级干预:1 标准恢复 → 2 soft → 3 hard(禁止同参数)→ 5 终止上报 UI(仿 errTruncatedToolLoop)。

### 4. 成功清除失败状态
同工具+路径调用成功 → 清除该指纹计数(状态机闭环,避免后续失败误判升级)。

## 技术方案要点(详见 design.md)

| 决策 | 结论 |
|---|---|
| nudge 通道 | **user role**(deepx 无 internal nudge 通道;system 注入破坏前缀缓存) |
| 指纹粒度 | Update: `tool+path+hash(normalized_old_string)`;Write: `tool+path`;Bash: `normalize(executable+subcommand)+category`(normalize=trim+collapse whitespace) |
| tracker 生命周期 | StartStream 局部状态(执行态,非对话知识;stream 结束/压缩重置) |
| 失败分类 | 7 枚举(unknown/not_found/invalid_argument/permission_denied/execution/timeout/network),工具自带为主,关键词回退(中英) |
| 成功清除 | 按 tool+path 基础指纹前缀清除 |

## 验证

### 单元测试(全绿)
- **tools**(5):`ClassifyFailure` 中英关键词 11 例、Update/Bash/Write 失败元数据(category+hint)
- **agent**(4):分级升级(1→2→3→5)、成功清除、旧工具关键词回退、Bash 指纹 normalize

### 黑箱测试(真实 exe `0.2.106-dev-40113cc`)
| 场景 | 结果 |
|---|---|
| Update 失败(not_found) | 注入 standard nudge"不要原样重试——请先 Read 文件确认实际内容"✅ |
| Bash 失败(execution) | 注入 nudge"检查命令与输出中的错误信息"(工具自带 hint 优先)✅ |
| 同指纹复读 ×2 | 第 1 次 standard → 第 2 次 **soft**("连续失败 2 次")✅ |
| 指纹隔离 | 不同文件(path+old_string)各自独立计数,不串扰 ✅ |

## 文件清单

```
tools/tools.go           ToolResult + FailureCategory/FailureHint(向后兼容)
tools/failure.go         类型 + 7 常量 + ClassifyFailure 关键词回退
tools/command.go         Bash 失败:timeout / execution + 中文 hint
tools/edit_file.go       Update 失败:not_found/invalid_argument + hint
tools/write_file.go      Write 失败:invalid_argument/permission_denied + hint
agent/failure_tracker.go failureTracker(指纹/分级/去重/成功清除)+ handleToolFailure
agent/llm.go             StartStream 局部 tracker;失败注入 user nudge;abort 上报
tools/failure_test.go    tools 侧测试(5)
agent/failure_tracker_test.go  agent 侧测试(4)
openspec/                proposal/design/tasks/spec delta + 基线 specs/agent/spec.md
```

## 风险与取舍

- 失败 nudge 每失败注入,多几条 user 消息(必要);同指纹去重防膨胀
- 分类回退靠关键词,个别旧工具措辞不匹配落 unknown(可接受,工具自带为主)
- 支持未来新工具(Browser/Git/DB 等):只需填 FailureCategory/Hint
