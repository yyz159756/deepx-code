# tasks: 工具失败恢复

## 1. Phase 1:ToolResult 扩展(tools 包)
- [ ] 1.1 `tools/tools.go`:`ToolResult` 增加 `FailureCategory` / `FailureHint` 字段(带注释,向后兼容)
- [ ] 1.2 新增 `tools/failure.go`:`FailureCategory` 类型 + 7 个常量(`FailureCategoryUnknown/NotFound/InvalidArgument/PermissionDenied/Execution/Timeout/Network`,前缀统一、避免语义重复)+ 关键词 fallback 分类函数 `ClassifyFailure(output string) FailureCategory`
- [ ] 1.3 `tools/command.go`:Bash 失败返回带 `FailureCategory: execution_error`(超时路径 `timeout`)与中文 `FailureHint`
- [ ] 1.4 `tools/edit_file.go`:Update 失败(找不到 old_string / 多处匹配 / 参数错)返回带对应 category(not_found / invalid_argument)与中文 `FailureHint`
- [ ] 1.5 `tools/write_file.go`:Write 失败(超限 / 路径 / 写入)返回带 category 与中文 `FailureHint`
- [ ] 1.6 单元测试:以上工具的失败返回含 category/hint;`ClassifyFailure` 关键词覆盖(中英)

## 2. Phase 1:Agent 失败恢复引导(agent 包)
- [ ] 2.1 `agent/llm.go`:新增 `handleToolFailure(tc ToolCall, result tools.ToolResult, tracker *failureTracker) string`——分类 → 生成 nudge(带动作引导),返回 nudge 文本
- [ ] 2.2 在工具结果回填处(llm.go 工具循环):`result.Success == false` 时调用 2.1,非空则 `append(convo, Role:"user")`
- [ ] 2.3 单测:Update/Bash 失败 → 注入含"不要原样重试"的 user nudge;成功 → 不注入

## 3. Phase 2:失败循环检测(agent 包)
- [ ] 3.1 新增 `agent/failure_tracker.go`:`failureTracker`(map[fp]count + lastNudgeFp)、`fingerprint(tc, category)`(Update: tool+path+hash(old_string);Write: tool+path;Bash: normalize(executable+subcommand)+category,normalize=trim+collapse whitespace+extract)、`clear(fp)`
- [ ] 3.2 分级:1 正常恢复 / 2 soft nudge / 3 hard nudge(禁止同参数)/ 5 `errRepeatedToolFailureLoop` 上报 UI
- [ ] 3.3 nudge 去重:同指纹同级别不重复注入
- [ ] 3.4 成功路径清除:`result.Success` → `tracker.clear(指纹)`(与 2.2 同一位置)
- [ ] 3.5 单测:同 Update 指纹连续 3 次失败 → hard nudge;第 5 次 → loop error;中间成功 → 计数清零

## 4. 收尾
- [ ] 4.1 `go build ./...` + 全量相关测试(agent/tools)
- [ ] 4.2 归档:delta 应用进 `openspec/specs/agent/spec.md` 基线,change 移入 archive
