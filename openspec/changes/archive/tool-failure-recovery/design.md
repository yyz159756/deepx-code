# design: 工具失败恢复架构

## 上下文

`tools.ToolResult{Output, Success}`(tools.go:41)是所有工具的统一返回。失败以 `Success:false` 进对话历史,目前 agent 无干预。

## 决策记录

### D1. nudge 通道用 user role
deepx **没有**独立 internal nudge 通道。所有现有 nudge(`truncatedToolNudge`/`emptyResponseNudge`/`completionGate`)都是 `append(convo, ChatMessage{Role:"user", Content:nudge})`。失败恢复引导复用同一模式:
- system 注入会破坏前缀缓存(核心优化)
- 模型已适应该模式(Claude Code 同款),不把 nudge 当真人需求

### D2. 指纹粒度
| 工具 | 指纹 |
|---|---|
| Update | `tool + path + hash(normalized_old_string)` |
| Write | `tool + path` |
| Bash | `normalize(executable + subcommand) + failure_category` |
| 其它 | `tool + failure_category`(兜底) |

Bash 不用全串 hash:模型加空格/参数就变指纹,检测被绕过;取首 token 又太粗(`npm install` vs `python train.py` 无法区分)。取"可执行名 + 子命令"。

**normalize 流程**(Bash 指纹前置):raw command → **trim** → **collapse whitespace**(连续空白压成单个空格)→ **extract executable/subcommand**。否则 `npm  install`(双空格)与 `npm install` 仍会产生两个指纹。

### D3. FailureTracker 生命周期
放 **StartStream 局部状态**(同 `gateNudges`,llm.go:678)。工具失败是**执行态**,不是对话知识:
- 不入 conversation memory、不入全局状态
- stream 结束即重置;压缩后重置(可接受,压缩本身已重规划)
- 每指纹计数 map + 最近 nudge 指纹(防重复注入相同提示)

### D4. 失败分类
`FailureCategory` 枚举(tools 包),常量名统一 `FailureCategory*` 前缀、避免语义重复:

```go
const (
    FailureCategoryUnknown            FailureCategory = "unknown"
    FailureCategoryNotFound           FailureCategory = "not_found"
    FailureCategoryInvalidArgument    FailureCategory = "invalid_argument"
    FailureCategoryPermissionDenied   FailureCategory = "permission_denied"
    FailureCategoryExecution          FailureCategory = "execution_error"
    FailureCategoryTimeout            FailureCategory = "timeout"
    FailureCategoryNetwork            FailureCategory = "network"
)
```

工具自带为主;旧工具按错误文本关键词 fallback(中英:"未找到"/"not found"/"missing"/"不存在" → not_found 等)。

### D5. 成功后清除
同指纹成功调用 → `tracker.clear(fingerprint)`。否则同文件失败 2 次后成功、再失败会被误判为"第 3 次"。

## 数据流

```
executeTool → ToolResult{Success:false, FailureCategory, FailureHint}
    ↓
HandleToolFailure(result, toolCall, &tracker)
    ├─ 分类:工具字段 → 关键词 fallback
    ├─ 指纹:按工具生成
    ├─ 计数:tracker[fp]++(去重:同 fp 同级别已注入则不重复)
    ├─ 分级:1 正常 / 2 soft / 3 hard / 5 终止
    └─ 输出 nudge(或 errRepeatedToolFailureLoop)
    ↓
append(convo, Role:"user", nudge)
```

成功路径:`result.Success` → `tracker.clear(指纹)`。
